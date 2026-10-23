#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""SZ1 acceptance: real MariaDB (TLS) in disposable containers.

Runs the Go integration tests against a throwaway MariaDB, then (unless
--skip-containers) starts two real replicas of the built image against the same
database and checks migration-once, restart/removal, outage, recovery and that
no secret appears in logs, container config, image history or compose output.
Only task-owned, uniquely named containers, networks and images are touched.
"""
import argparse
import base64
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile
import time

from web_test_support import ROOT, api, redact, register_secret, request, run, wait_until

DB_IMAGE = 'mariadb:12.3'


def sh(command, **kwargs):
    return run(command, cwd=ROOT, **kwargs)


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def make_certificates(directory):
    def openssl(*args):
        subprocess.run(['openssl', *args], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    d = Path(directory)
    openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=SZ1 Test CA', '-keyout', str(d / 'ca.key'), '-out', str(d / 'ca.pem'))
    openssl('req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost', '-keyout', str(d / 'server.key'), '-out', str(d / 'server.csr'))
    (d / 'ext.cnf').write_text('subjectAltName=DNS:localhost,DNS:state-db\nextendedKeyUsage=serverAuth\n')
    openssl('x509', '-req', '-days', '1', '-in', str(d / 'server.csr'), '-CA', str(d / 'ca.pem'), '-CAkey', str(d / 'ca.key'), '-CAcreateserial', '-extfile', str(d / 'ext.cnf'), '-out', str(d / 'server.pem'))
    openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=Unrelated CA', '-keyout', str(d / 'other.key'), '-out', str(d / 'other.pem'))


def start_database(name, certs, root_password, tls=True, network=None, alias=None, publish=True):
    command = ['docker', 'create', '--name', name, '--label', 'dbmanager.task=SZ1', '-e', 'MARIADB_ROOT_PASSWORD=' + root_password, '--entrypoint', '/bin/sh']
    if publish:
        command += ['-p', '127.0.0.1::3306']
    if network:
        command += ['--network', network] + (['--network-alias', alias] if alias else [])
    server = ['mariadbd', '--ssl-ca=/fixture/ca.pem', '--ssl-cert=/fixture/server.pem', '--ssl-key=/fixture/server.key'] if tls else ['mariadbd', '--skip-ssl']
    script = 'chown -R mysql /fixture && chmod 600 /fixture/server.key && exec /usr/local/bin/docker-entrypoint.sh "$@"'
    run(command + [DB_IMAGE, '-c', script, 'fixture'] + server)
    run(['docker', 'cp', str(certs), name + ':/fixture'])
    run(['docker', 'start', name])

    def ready():
        env = os.environ | {'MYSQL_PWD': root_password}
        return subprocess.run(['docker', 'exec', '-e', 'MYSQL_PWD', name, 'mariadb-admin', '--protocol=tcp', '--host=127.0.0.1', '--user=root', '--skip-ssl', 'ping'], env=env, capture_output=True).returncode == 0
    # The image restarts the server after init; require two consecutive successes.
    streak = 0
    for _ in range(240):
        streak = streak + 1 if ready() else 0
        if streak >= 3:
            break
        time.sleep(.5)
    else:
        raise RuntimeError('Disposable MariaDB did not start: ' + name)
    if not publish:
        return None
    return json.loads(run(['docker', 'inspect', '--format', '{{json .NetworkSettings.Ports}}', name]))['3306/tcp'][0]['HostPort']


def mysql(container, root_password, statement):
    run(['docker', 'exec', '-e', 'MYSQL_PWD=' + root_password, container, 'mariadb', '--protocol=tcp', '--host=127.0.0.1', '--user=root', '--skip-ssl', '-e', statement])


def render_stack_files():
    env = os.environ | {'DBM_STATE_DB_HOST': 'state-db'}
    plain = run(['docker', 'stack', 'config', '-c', 'deploy/swarm.yaml'], cwd=ROOT, env=env)
    state = run(['docker', 'stack', 'config', '-c', 'deploy/swarm.yaml', '-c', 'deploy/swarm.state.yaml', '-c', 'deploy/swarm.state-migrate.yaml'], cwd=ROOT, env=env)
    assert 'DBM_STATE' not in plain and 'DBM_STATE_DB_HOST: state-db' in state and 'dbmanager_state_server_key_v1' in state and 'replicas: 1' in state
    print('Swarm files: merged stack renders (rendering only, no Swarm deployment).', flush=True)


def go_tests(certs, port, notls_port, root_password):
    env = os.environ | {
        'DBM_TEST_STATE_PORT': port, 'DBM_TEST_STATE_NOTLS_PORT': notls_port,
        'DBM_TEST_STATE_CA_FILE': str(Path(certs) / 'ca.pem'), 'DBM_TEST_STATE_ROOT_PASSWORD': root_password,
    }
    result = subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/serverstate'], cwd=ROOT, env=env, text=True, capture_output=True)
    summary = [line for line in result.stdout.splitlines() if line.startswith(('--- ', 'ok', 'FAIL', 'PASS'))]
    print('\n'.join(summary), flush=True)
    if result.returncode:
        print(redact(result.stdout[-4000:]), file=sys.stderr)
        raise RuntimeError('Go integration tests failed')
    skipped = [line for line in summary if 'SKIP' in line]
    if skipped:
        raise RuntimeError('Integration tests were skipped: ' + ', '.join(skipped))


def containers(args, certs, root_password, suffix):
    project = 'dbmanager-sz1-' + suffix
    network = project + '-net'
    db = project + '-db'
    image = args.image or project + ':test'
    secrets_root = ROOT / 'secrets'
    secrets_root.mkdir(mode=0o700, exist_ok=True)
    runtime_password = register_secret(secrets.token_hex(16))
    migrate_password = register_secret(secrets.token_hex(16))
    server_key = register_secret(base64.b64encode(secrets.token_bytes(32)).decode())
    token = register_secret(secrets.token_urlsafe(48))
    created, image_created = [], False
    with tempfile.TemporaryDirectory(prefix=project + '-', dir=secrets_root) as temporary:
        d = Path(temporary)
        files = {'api_token': token, 'state_db_password': runtime_password, 'state_db_migrate_password': migrate_password, 'state_server_key': server_key}
        for name, content in files.items():
            (d / name).write_text(content + '\n')
            (d / name).chmod(0o444)
        (d / 'state_db_ca.pem').write_text((Path(certs) / 'ca.pem').read_text())
        (d / 'state_db_ca.pem').chmod(0o444)
        (d / 'other_ca.pem').write_text((Path(certs) / 'other.pem').read_text())
        (d / 'other_ca.pem').chmod(0o444)
        port = free_port()
        override = d / 'override.yaml'
        override.write_text(f'services:\n  app:\n    networks: [dbnet]\n    healthcheck:\n      interval: 3s\n      timeout: 3s\n      retries: 3\nnetworks:\n  dbnet:\n    external: true\n    name: {network}\n')
        env = dict(os.environ, DBM_IMAGE=image, DBM_PORT=str(port), DBM_TOKEN_PATH=str(d / 'api_token'),
                   DBM_STATE_DB_HOST='state-db', DBM_STATE_DB_NAME='sz1_state', DBM_STATE_DB_USER='sz1_runtime',
                   DBM_STATE_DB_PASSWORD_PATH=str(d / 'state_db_password'), DBM_STATE_SERVER_KEY_PATH=str(d / 'state_server_key'),
                   DBM_STATE_DB_CA_PATH=str(d / 'state_db_ca.pem'), DBM_STATE_DB_MIGRATE_USER='sz1_migrate',
                   DBM_STATE_DB_MIGRATE_PASSWORD_PATH=str(d / 'state_db_migrate_password'))
        compose = ['docker', 'compose', '-f', 'compose.yaml', '-f', 'compose.state.yaml', '-f', 'compose.state-migrate.yaml', '-f', str(override), '-p', project]
        base = f'http://127.0.0.1:{port}'
        second = project + '-replica2'

        def replica_args(name, ca='state_db_ca.pem'):
            args = ['docker', 'run', '-d', '--name', name, '--label', 'dbmanager.task=SZ1', '--network', network, '--read-only',
                    '--tmpfs', '/tmp:rw,noexec,nosuid,size=16m', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges:true',
                    '--health-interval', '3s', '--health-retries', '3']
            for variable, value in {'DBM_STATE_DB_HOST': 'state-db', 'DBM_STATE_DB_NAME': 'sz1_state', 'DBM_STATE_DB_USER': 'sz1_runtime',
                                    'DBM_STATE_DB_PASSWORD_FILE': '/run/secrets/state_db_password', 'DBM_STATE_SERVER_KEY_FILE': '/run/secrets/state_server_key',
                                    'DBM_STATE_DB_CA_FILE': '/run/secrets/state_db_ca', 'DBM_STATE_DB_MIGRATE_USER': 'sz1_migrate',
                                    'DBM_STATE_DB_MIGRATE_PASSWORD_FILE': '/run/secrets/state_db_migrate_password'}.items():
                args += ['-e', f'{variable}={value}']
            for secret, source in {'api_token': 'api_token', 'state_db_password': 'state_db_password', 'state_server_key': 'state_server_key',
                                   'state_db_ca': ca, 'state_db_migrate_password': 'state_db_migrate_password'}.items():
                args += ['-v', f'{d / source}:/run/secrets/{secret}:ro']
            return args + [image]

        def healthy(name):
            return run(['docker', 'inspect', '--format', '{{.State.Health.Status}}', name]) == 'healthy'

        def migration_lines(name):
            logs = subprocess.run(['docker', 'logs', name], capture_output=True, text=True)
            return [line for line in (logs.stdout + logs.stderr).splitlines() if 'Zustandsspeicher' in line]

        try:
            if not args.skip_build:
                sh(['docker', 'build', '-t', image, '.'])
                image_created = True
            run(['docker', 'network', 'create', '--label', 'dbmanager.task=SZ1', network]); created.append(('network', network))
            start_database(db, certs, root_password, network=network, alias='state-db', publish=False); created.append(('container', db))
            for statement in ['CREATE DATABASE sz1_state CHARACTER SET utf8mb4',
                              f"CREATE USER 'sz1_runtime'@'%' IDENTIFIED BY '{runtime_password}' REQUIRE SSL",
                              f"CREATE USER 'sz1_migrate'@'%' IDENTIFIED BY '{migrate_password}' REQUIRE SSL",
                              "GRANT SELECT, INSERT, UPDATE, DELETE ON sz1_state.* TO 'sz1_runtime'@'%'",
                              "GRANT ALL PRIVILEGES ON sz1_state.* TO 'sz1_migrate'@'%'"]:
                mysql(db, root_password, statement)

            # Negative: an unrelated CA must prevent start (fail closed) with a TLS message.
            bad = project + '-badca'
            created.append(('container', bad))
            run(replica_args(bad, ca='other_ca.pem'))
            wait_until(lambda: run(['docker', 'inspect', '--format', '{{.State.Status}}', bad]) == 'exited', 'bad-CA replica exit')
            assert run(['docker', 'inspect', '--format', '{{.State.ExitCode}}', bad]) == '1'
            assert any('TLS-Zertifikatsprüfung' in line for line in subprocess.run(['docker', 'logs', bad], capture_output=True, text=True).stderr.splitlines())
            print('Container: unrelated CA prevents start (exit 1, TLS message).', flush=True)

            # Two replicas start concurrently against the same empty database.
            created.append(('container', second))
            with concurrent.futures.ThreadPoolExecutor(2) as pool:
                first_job = pool.submit(lambda: run(compose + ['up', '-d', '--no-build'], env=env))
                second_job = pool.submit(lambda: run(replica_args(second)))
                first_job.result(); second_job.result()
            first = run(compose + ['ps', '-q', 'app'], env=env)
            wait_until(lambda: healthy(first) and healthy(second), 'both replicas healthy', attempts=240)
            protocol = {'replica1': migration_lines(first), 'replica2': migration_lines(second)}
            applied = [line for lines in protocol.values() for line in lines if 'angewendet' in line]
            current = [line for lines in protocol.values() for line in lines if 'Schema aktuell' in line]
            print('Migration protocol (start, two concurrent replicas):')
            for replica, lines in protocol.items():
                for line in lines:
                    print(f'  {replica}: {line}')
            assert len(applied) == 1 and len(current) == 1, protocol
            assert api(base, '/healthz')['state_store'] == 'ok'
            assert api(base, '/api/v1/capabilities', token)['credential_storage'] is False
            request(base, '/api/v1/capabilities', 'wrong-token', expected=401)
            print('Container: migration applied exactly once, both replicas healthy, Bearer mode intact.', flush=True)

            # Removing one replica and restarting the other keeps state; no second migration.
            run(['docker', 'rm', '-f', second])
            run(['docker', 'restart', first])
            wait_until(lambda: healthy(first), 'replica restart', attempts=240)
            assert not any('angewendet' in line for line in migration_lines(first)[-2:]), migration_lines(first)
            row_count = run(['docker', 'exec', '-e', 'MYSQL_PWD=' + root_password, db, 'mariadb', '--protocol=tcp', '--host=127.0.0.1', '--user=root', '--skip-ssl', '-N', '-e', 'SELECT COUNT(*) FROM sz1_state.schema_migrations'])
            assert row_count == '1', row_count
            print('Container: replica removal and restart keep schema state, no repeated migration.', flush=True)

            # Outage: fail closed + unhealthy; recovery without restarting the replica.
            restarts_before = run(['docker', 'inspect', '--format', '{{.RestartCount}} {{.State.StartedAt}}', first])
            run(['docker', 'stop', db])
            wait_until(lambda: request(base, '/healthz', expected=503)[1] is not None, 'healthz 503 during outage', attempts=60)
            request(base, '/api/v1/capabilities', token, expected=503)
            wait_until(lambda: run(['docker', 'inspect', '--format', '{{.State.Health.Status}}', first]) == 'unhealthy', 'Docker health unhealthy', attempts=240)
            run(['docker', 'start', db])
            wait_until(lambda: api(base, '/healthz')['status'] == 'ok', 'recovery after database return', attempts=240)
            wait_until(lambda: healthy(first), 'Docker health recovered', attempts=240)
            assert api(base, '/api/v1/capabilities', token)['credential_storage'] is False
            assert run(['docker', 'inspect', '--format', '{{.RestartCount}} {{.State.StartedAt}}', first]) == restarts_before
            print('Container: outage -> /healthz 503, API 503, Docker unhealthy; return -> recovered without restarting the replica.', flush=True)

            # No secret in logs, container config, image metadata or rendered compose output.
            haystack = []
            for name in (first, bad):
                logs = subprocess.run(['docker', 'logs', name], capture_output=True, text=True)
                haystack += [logs.stdout, logs.stderr, run(['docker', 'inspect', name])]
            haystack += [run(['docker', 'history', '--no-trunc', image]), run(['docker', 'inspect', image]), run(compose + ['config'], env=env)]
            joined = '\n'.join(haystack)
            for label, value in {'db password': runtime_password, 'migration password': migrate_password, 'server key': server_key, 'api token': token, 'root password': root_password}.items():
                assert value not in joined, label + ' leaked'
            print('Container: no secret in logs, container config, image history or compose output.', flush=True)
        finally:
            subprocess.run(compose + ['down', '--volumes', '--remove-orphans'], cwd=ROOT, env=env, capture_output=True)
            for kind, name in reversed(created):
                subprocess.run(['docker', 'rm', '-fv', name] if kind == 'container' else ['docker', 'network', 'rm', name], capture_output=True)
            if image_created and not args.image:
                subprocess.run(['docker', 'image', 'rm', image], capture_output=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--skip-containers', action='store_true')
    parser.add_argument('--skip-build', action='store_true')
    parser.add_argument('--image')
    args = parser.parse_args()
    if args.skip_build and not args.image:
        parser.error('--skip-build requires --image')
    suffix = secrets.token_hex(5)
    root_password = register_secret(secrets.token_hex(16))
    owned = []
    with tempfile.TemporaryDirectory(prefix='dbmanager-sz1-certs-') as certs:
        make_certificates(certs)
        try:
            tls_db, plain_db = f'dbmanager-sz1-{suffix}-gotls', f'dbmanager-sz1-{suffix}-goplain'
            owned.append(tls_db)
            tls_port = start_database(tls_db, certs, root_password)
            owned.append(plain_db)
            plain_port = start_database(plain_db, certs, root_password, tls=False)
            render_stack_files()
            go_tests(certs, tls_port, plain_port, root_password)
            if not args.skip_containers:
                containers(args, certs, root_password, suffix)
        finally:
            for name in reversed(owned):
                subprocess.run(['docker', 'rm', '-fv', name], capture_output=True)
    print('SZ1: disposable databases, containers, networks, volumes and temporary credentials removed.', flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(redact(error), file=sys.stderr)
        sys.exit(1)
