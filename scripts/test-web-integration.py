#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run the real embedded UI against a Go CLI, Compose, SQLite and PostgreSQL TLS."""
import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import sys
import tempfile
import time

from web_test_support import ROOT, api, query, redact, register_secret, run, verify_http, verify_image, wait_until, start_postgres_fixture


def unused_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def create_tls(folder):
    folder.mkdir()
    for args in [
        ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=DB Manager F4b Test CA', '-keyout', str(folder/'ca.key'), '-out', str(folder/'ca.pem')],
        ['req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=f4b-postgres', '-keyout', str(folder/'server.key'), '-out', str(folder/'server.csr')],
    ]:
        run(['openssl', *args])
    (folder/'ext.cnf').write_text('subjectAltName=DNS:localhost,DNS:f4b-postgres\nextendedKeyUsage=serverAuth\n')
    run(['openssl', 'x509', '-req', '-days', '1', '-in', str(folder/'server.csr'), '-CA', str(folder/'ca.pem'), '-CAkey', str(folder/'ca.key'), '-CAcreateserial', '-extfile', str(folder/'ext.cnf'), '-out', str(folder/'server.pem')])
    (folder/'ca.pem').chmod(0o444)


def seed(base, token, postgres):
    fixtures = ROOT / 'tests/web-runtime/fixtures'
    for driver, connection in [('sqlite', {'driver':'sqlite','database':'browser_fixture'}), ('postgres', postgres)]:
        api(base, '/api/v1/connections/test', token, {'connection': connection})
        for action in ['create', 'insert']:
            query(base, token, connection, (fixtures/f'{driver}-{action}.sql').read_text())
        table='"odd""name"'
        types='id INTEGER PRIMARY KEY, huge TEXT' if driver=='sqlite' else 'id BIGINT PRIMARY KEY, huge NUMERIC(40,20)'
        payload='BLOB' if driver=='sqlite' else 'BYTEA'
        generated='VIRTUAL' if driver=='sqlite' else 'STORED'
        query(base,token,connection,f"CREATE TABLE {table} ({types}, nullable TEXT DEFAULT 'fallback', empty_text TEXT, payload {payload}, generated_text TEXT GENERATED ALWAYS AS (empty_text || 'x') {generated})")
        values=[]
        for i in [*range(1,131),9223372036854775807]:
            binary_sql="X'00FF'" if driver=='sqlite' else "decode('00ff','hex')"
            nullable='NULL' if i%2 else "'present'"
            values.append(f"({i},'12345678901234567890.12345678901234567890',{nullable},'',{binary_sql})")
        query(base,token,connection,f'INSERT INTO {table}(id,huge,nullable,empty_text,payload) VALUES '+','.join(values))
        query(base,token,connection,f'CREATE VIEW f3_view AS SELECT huge,nullable FROM {table}')
        key = 'id INTEGER PRIMARY KEY AUTOINCREMENT' if driver == 'sqlite' else 'id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY'
        numeric = 'TEXT' if driver == 'sqlite' else 'NUMERIC(40,20)'
        query(base, token, connection, f"CREATE TABLE f4_rows ({key}, label TEXT NOT NULL DEFAULT 'default-label', optional TEXT, empty_text TEXT, precise {numeric}, payload {payload}, generated_text TEXT GENERATED ALWAYS AS (label || '-generated') {generated})")
        binary_sql = "X'00FF'" if driver == 'sqlite' else "decode('00ff','hex')"
        query(base, token, connection, "INSERT INTO f4_rows(label,optional,empty_text,precise,payload) VALUES('original',NULL,'','12345678901234567890.12345678901234567890'," + binary_sql + ")")
        query(base, token, connection, 'CREATE VIEW f4_view AS SELECT * FROM f4_rows')
        query(base, token, connection, 'CREATE TABLE f4_no_pk(value TEXT)')
        query(base, token, connection, "INSERT INTO f4_no_pk VALUES('read-only')")



def browser(directory, settings, install_dependencies):
    workspace = directory/'browser'
    shutil.copytree(ROOT/'tests/web-runtime', workspace)
    config = directory/'browser-config.json'
    settings['outputDir'] = str(directory/'browser-output')
    config.write_text(json.dumps(settings))
    config.chmod(0o600)
    environment = {key:value for key,value in os.environ.items() if key in {'PATH','HOME','TMPDIR','SYSTEMROOT','SystemRoot','PLAYWRIGHT_BROWSERS_PATH'}}
    environment.update({'DBM_TEST_CONFIG':str(config),'CI':'1'})
    run(['npm','ci','--no-audit','--no-fund'],cwd=workspace,env=environment)
    install = ['npx','playwright','install']
    if install_dependencies:
        install += ['--with-deps']
    run([*install,'chromium'],cwd=workspace,env=environment)
    output=run(['npm','test','--','--reporter=line'],cwd=workspace,env=environment)
    print(redact(output),flush=True)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--browser-deps',action='store_true',help='Install Chromium system packages on Linux')
    arguments=parser.parse_args()
    suffix=secrets.token_hex(6)
    prefix='dbmanager-f4b-web-'+suffix
    image=prefix+':test'
    network=prefix+'-net'
    postgres_name=prefix+'-postgres'
    token=register_secret(secrets.token_urlsafe(48))
    invalid_token=register_secret(secrets.token_urlsafe(48))
    password=register_secret(secrets.token_urlsafe(32))
    parent=ROOT/'secrets';parent.mkdir(mode=0o700,exist_ok=True)
    owned=[];network_created=False;compose_started=False;image_created=False;process=None
    with tempfile.TemporaryDirectory(prefix=prefix+'-',dir=parent) as temporary:
        directory=Path(temporary)
        token_file=directory/'api_token';token_file.write_text(token+'\n');token_file.chmod(0o444)
        tls=directory/'tls';create_tls(tls)
        pg_env=directory/'postgres.env';pg_env.write_text('POSTGRES_PASSWORD='+password+'\nPOSTGRES_DB=f4b_probe\n');pg_env.chmod(0o600)
        compose=['docker','compose','-f',str(ROOT/'compose.yaml'),'-f',str(directory/'compose-fixture.json'),'-p',prefix]
        environment=dict(os.environ,DBM_IMAGE=image,DBM_PORT=str(unused_port()),DBM_TOKEN_PATH=str(token_file))
        override={'services':{'app':{'networks':['fixture'],'environment':{'DBM_ALLOWED_HOSTS':'f4b-postgres,f4b-postgres-wrong','DBM_DB_CA_FILE':'/run/db-ca/ca.pem'},'volumes':[{'type':'bind','source':str(tls/'ca.pem'),'target':'/run/db-ca/ca.pem','read_only':True}]}},'networks':{'fixture':{'external':True,'name':network}}}
        (directory/'compose-fixture.json').write_text(json.dumps(override))
        try:
            run(['docker','network','create',network]);network_created=True
            script='chown -R postgres /fixture && chmod 600 /fixture/server.key && exec /usr/local/bin/docker-entrypoint.sh "$@"'
            run(['docker','create','--name',postgres_name,'--label','dbmanager.task=F4b','--network',network,'--network-alias','f4b-postgres','--network-alias','f4b-postgres-wrong','-p','127.0.0.1::5432','--env-file',str(pg_env),'--entrypoint','/bin/sh','postgres:18.6-alpine','-c',script,'fixture','postgres','-c','ssl=on','-c','ssl_cert_file=/fixture/server.pem','-c','ssl_key_file=/fixture/server.key']);owned.append(postgres_name)
            run(['docker','cp',str(tls),postgres_name+':/fixture'])
            start_postgres_fixture(postgres_name,directory,'f4b_probe')
            pg_port=int(json.loads(run(['docker','inspect','--format','{{json .NetworkSettings.Ports}}',postgres_name]))['5432/tcp'][0]['HostPort'])
            cli=directory/'db-manager';run(['go','build','-trimpath','-o',str(cli),'./cmd/db-manager'],cwd=ROOT)
            cli_port=unused_port();cli_base=f'http://127.0.0.1:{cli_port}'
            cli_env=dict(os.environ,DBM_TOKEN_FILE=str(token_file),DBM_ADDRESS=f'127.0.0.1:{cli_port}',DBM_DATA_DIR=str(directory/'cli-data'),DBM_ALLOWED_HOSTS='localhost,127.0.0.1',DBM_DB_CA_FILE=str(tls/'ca.pem'))
            with (directory/'cli.log').open('w') as log:
                process=subprocess.Popen([str(cli)],env=cli_env,stdout=log,stderr=log)
            wait_until(lambda:api(cli_base,'/healthz')['status']=='ok','real CLI server')
            verify_http(cli_base,token)
            native_pg={'driver':'postgres','host':'localhost','port':pg_port,'database':'f4b_probe','username':'postgres','password':password,'tls_mode':'verify-full'}
            api(cli_base,'/api/v1/connections/test',token,{'connection':native_pg})
            wrong={**native_pg,'host':'127.0.0.1'}
            api(cli_base,'/api/v1/connections/test',token,{'connection':wrong},expected=422)
            query(cli_base,token,{'driver':'sqlite','database':'cli_fixture'},'SELECT 9223372036854775807 AS precise')
            process.terminate();process.wait(timeout=15);process=None
            print('CLI: embedded assets/CSP/auth, real SQLite and verified PostgreSQL TLS, wrong-host refusal passed.',flush=True)
            run(['docker','build','-t',image,'.'],cwd=ROOT);image_created=True;verify_image(image)
            compose_started=True;run(compose+['up','-d','--no-build'],env=environment)
            base='http://127.0.0.1:'+environment['DBM_PORT']
            wait_until(lambda:api(base,'/healthz')['status']=='ok','Compose web server')
            verify_http(base,token)
            pg={**native_pg,'host':'f4b-postgres','port':5432}
            api(base,'/api/v1/connections/test',token,{'connection':pg})
            api(base,'/api/v1/connections/test',token,{'connection':{**pg,'host':'f4b-postgres-wrong'}},expected=422)
            seed(base,token,pg)
            browser(directory,{'baseURL':base,'token':token,'invalidToken':invalid_token,'postgres':pg,'webCommit':json.loads((ROOT/'web-build.lock.json').read_text())['commit'],'licenseHashes':{item['path']:item['sha256'] for item in json.loads((ROOT/'internal/webui/assets-manifest.json').read_text())['files'] if item['path'].endswith('.txt')}},arguments.browser_deps)
            print('Compose browser: real SQLite/PostgreSQL TLS, catalog, precision/NULL/empty/binary, truncation, cancellation, auth/session reset and CSP passed.',flush=True)
        finally:
            if process is not None:
                process.terminate();process.wait(timeout=15)
            if compose_started:
                run(compose+['down','--volumes','--remove-orphans'],env=environment)
            for name in reversed(owned):
                run(['docker','rm','-fv',name])
            if network_created:run(['docker','network','rm',network])
            if image_created:run(['docker','image','rm',image])
    print('F4b web: only task-owned processes, containers, networks, volumes, image and temporary credentials removed.',flush=True)


if __name__=='__main__':
    try:
        main()
    except Exception as error:
        print(redact(error),file=sys.stderr)
        sys.exit(1)
