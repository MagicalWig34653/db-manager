#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise only disposable, uniquely named database containers and fixtures."""
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent

def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def main():
    run('docker', 'ps', '--format', '{{.Names}} {{.Status}}')
    prefix = 'dbmanager-engine-' + secrets.token_hex(4)
    owned = []
    with tempfile.TemporaryDirectory(prefix=prefix) as directory:
        fixture = Path(directory)
        run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=DB Manager Test CA', '-keyout', str(fixture/'ca.key'), '-out', str(fixture/'ca.pem'), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        run('openssl', 'req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost', '-keyout', str(fixture/'server.key'), '-out', str(fixture/'server.csr'), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        (fixture/'ext.cnf').write_text('subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n')
        run('openssl', 'x509', '-req', '-days', '1', '-in', str(fixture/'server.csr'), '-CA', str(fixture/'ca.pem'), '-CAkey', str(fixture/'ca.key'), '-CAcreateserial', '-extfile', str(fixture/'ext.cnf'), '-out', str(fixture/'server.pem'), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        specs = [
            ('mysql', 'mysql:8.4', 3306, ['MYSQL_ROOT_PASSWORD=public-b1-fixture','MYSQL_ROOT_HOST=%','MYSQL_DATABASE=probe'], ['mysqld','--ssl-ca=/fixture/ca.pem','--ssl-cert=/fixture/server.pem','--ssl-key=/fixture/server.key'], 'mysql'),
            ('maria', 'mariadb:12.3', 3306, ['MARIADB_ROOT_PASSWORD=public-b1-fixture','MARIADB_DATABASE=probe'], ['mariadbd','--ssl-ca=/fixture/ca.pem','--ssl-cert=/fixture/server.pem','--ssl-key=/fixture/server.key'], 'mysql'),
            ('postgres', 'postgres:18.6-alpine', 5432, ['POSTGRES_PASSWORD=public-b1-fixture','POSTGRES_DB=probe'], ['postgres','-c','ssl=on','-c','ssl_cert_file=/fixture/server.pem','-c','ssl_key_file=/fixture/server.key'], 'postgres'),
            ('no_tls', 'mariadb:12.3', 3306, ['MARIADB_ROOT_PASSWORD=public-b1-fixture','MARIADB_DATABASE=probe'], ['mariadbd','--skip-ssl'], 'mysql'),
        ]
        environment = os.environ.copy()
        environment['DBM_TEST_CA_FILE'] = str(fixture/'ca.pem')
        environment['DBM_TEST_PASSWORD'] = 'public-b1-fixture'
        try:
            for name, image, port, variables, command, user in specs:
                container = prefix+'-'+name
                args=['docker','create','--name',container,'--label','dbmanager.task=B1','-p',f'127.0.0.1::{port}','--entrypoint','/bin/sh']
                for variable in variables: args += ['-e',variable]
                script=f'chown -R {user} /fixture && chmod 600 /fixture/server.key && exec /usr/local/bin/docker-entrypoint.sh "$@"'
                args += [image,'-c',script,'fixture']+command
                run(*args,stdout=subprocess.DEVNULL);owned.append(container)
                run('docker','cp',str(fixture),container+':/fixture',stdout=subprocess.DEVNULL)
                run('docker','start',container,stdout=subprocess.DEVNULL)
                for _ in range(120):
                    if name=='postgres': check=['docker','exec',container,'pg_isready','-U','postgres','-d','probe']
                    else: check=['docker','exec',container,'mysqladmin' if name=='mysql' else 'mariadb-admin','--ssl-mode=REQUIRED' if name=='mysql' else '--skip-ssl','--protocol=tcp','--host=127.0.0.1','--user=root','--password=public-b1-fixture','ping']
                    if subprocess.run(check,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0: break
                    time.sleep(.5)
                else:
                    run('docker','logs','--tail','25',container)
                    raise RuntimeError('Disposable database startup failed: '+name)
                bindings=json.loads(subprocess.check_output(['docker','inspect',container],text=True))[0]['NetworkSettings']['Ports']
                environment['DBM_TEST_'+name.upper()+'_PORT']=bindings[str(port)+'/tcp'][0]['HostPort']
            run('go','test','-count=1','-v','./internal/database',env=environment,cwd=ROOT)
        finally:
            for container in reversed(owned):
                subprocess.run(['docker','rm','-f','-v',container],stdout=subprocess.DEVNULL,check=True)

if __name__=='__main__': main()
