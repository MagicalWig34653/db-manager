#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Reusable disposable database fixtures for package test scripts.

Starts MySQL 8.4, MariaDB 12.3 and PostgreSQL 18.6 (own throw-away CA, server
certificate with SAN localhost, TLS on) plus a SQLite directory. Every resource
carries the unique prefix ``dbmanager-<package>-<hex>`` and the label
``dbmanager.task=<package>``; cleanup removes only resources created here.

Usage pattern for a package script::

    from dbfix import Fixtures
    with Fixtures('w-p07', systems=('mysql', 'maria', 'postgres')) as fx:
        env = fx.environment()   # DBM_TEST_*_PORT, DBM_TEST_CA_FILE, DBM_TEST_SQLITE_DIR
        subprocess.run(['go', 'test', ...], env=env, cwd=fx.root, check=True)
"""
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parent.parent
PASSWORD = 'public-b1-fixture'
CA_SUBJECT = '/CN=DB Manager Test CA'
ALL_SYSTEMS = ('mysql', 'maria', 'postgres')
PACKAGE_PATTERN = re.compile(r'^[a-z0-9][a-z0-9-]{0,24}$')

# name -> (image, port, env, server command, owner of /fixture)
SPECS = {
    'mysql': ('mysql:8.4', 3306,
              ['MYSQL_ROOT_PASSWORD=' + PASSWORD, 'MYSQL_ROOT_HOST=%', 'MYSQL_DATABASE=probe'],
              ['mysqld', '--ssl-ca=/fixture/ca.pem', '--ssl-cert=/fixture/server.pem', '--ssl-key=/fixture/server.key'], 'mysql'),
    'maria': ('mariadb:12.3', 3306,
              ['MARIADB_ROOT_PASSWORD=' + PASSWORD, 'MARIADB_DATABASE=probe'],
              ['mariadbd', '--ssl-ca=/fixture/ca.pem', '--ssl-cert=/fixture/server.pem', '--ssl-key=/fixture/server.key'], 'mysql'),
    'postgres': ('postgres:18.6-alpine', 5432,
                 ['POSTGRES_PASSWORD=' + PASSWORD, 'POSTGRES_DB=probe'],
                 ['postgres', '-c', 'ssl=on', '-c', 'ssl_cert_file=/fixture/server.pem', '-c', 'ssl_key_file=/fixture/server.key'], 'postgres'),
}


def run(*args, **kwargs):
    kwargs.setdefault('check', True)
    return subprocess.run(args, **kwargs)


def quiet(*args, **kwargs):
    return run(*args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, **kwargs)


def create_ca(directory):
    """Write ca.pem, server.pem and server.key (SAN localhost) into directory."""
    d = Path(directory)
    quiet('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', CA_SUBJECT,
          '-keyout', str(d / 'ca.key'), '-out', str(d / 'ca.pem'))
    quiet('openssl', 'req', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=localhost',
          '-keyout', str(d / 'server.key'), '-out', str(d / 'server.csr'))
    (d / 'ext.cnf').write_text('subjectAltName=DNS:localhost\nextendedKeyUsage=serverAuth\n')
    quiet('openssl', 'x509', '-req', '-days', '1', '-in', str(d / 'server.csr'), '-CA', str(d / 'ca.pem'),
          '-CAkey', str(d / 'ca.key'), '-CAcreateserial', '-extfile', str(d / 'ext.cnf'), '-out', str(d / 'server.pem'))


def ready_command(name, container):
    if name == 'postgres':
        return ['docker', 'exec', container, 'pg_isready', '-U', 'postgres', '-d', 'probe']
    admin = 'mysqladmin' if name == 'mysql' else 'mariadb-admin'
    ssl = '--ssl-mode=REQUIRED' if name == 'mysql' else '--skip-ssl'
    return ['docker', 'exec', container, admin, ssl, '--protocol=tcp', '--host=127.0.0.1', '--user=root',
            '--password=' + PASSWORD, 'ping']


class Fixtures:
    """Context manager owning containers, CA files and the SQLite directory."""

    def __init__(self, package, systems=ALL_SYSTEMS, attempts=240):
        if not PACKAGE_PATTERN.match(package):
            raise ValueError('package must match ' + PACKAGE_PATTERN.pattern)
        unknown = set(systems) - set(SPECS)
        if unknown:
            raise ValueError('unknown systems: ' + ', '.join(sorted(unknown)))
        self.package = package
        self.systems = tuple(systems)
        self.attempts = attempts
        self.prefix = 'dbmanager-' + package + '-' + secrets.token_hex(4)
        self.root = ROOT
        self.containers = []
        self.ports = {}
        self._tmp = None
        self.fixture_dir = None
        self.sqlite_dir = None

    def __enter__(self):
        self._tmp = tempfile.TemporaryDirectory(prefix=self.prefix + '-')
        base = Path(self._tmp.name)
        self.fixture_dir = base / 'tls'
        self.sqlite_dir = base / 'sqlite'
        self.fixture_dir.mkdir()
        self.sqlite_dir.mkdir()
        try:
            if self.systems:
                create_ca(self.fixture_dir)
            for name in self.systems:
                self._start(name)
        except BaseException:
            self.cleanup()
            raise
        return self

    def __exit__(self, *exc):
        self.cleanup()
        return False

    def _start(self, name):
        image, port, variables, command, owner = SPECS[name]
        container = self.prefix + '-' + name
        args = ['docker', 'create', '--name', container, '--label', 'dbmanager.task=' + self.package,
                '--label', 'dbmanager.fixture=' + self.prefix, '-p', '127.0.0.1::' + str(port), '--entrypoint', '/bin/sh']
        for variable in variables:
            args += ['-e', variable]
        script = ('chown -R ' + owner + ' /fixture && chmod 600 /fixture/server.key && '
                  'exec /usr/local/bin/docker-entrypoint.sh "$@"')
        args += [image, '-c', script, 'fixture'] + command
        self.containers.append(container)  # register first so a failed create is still cleaned
        quiet(*args)
        quiet('docker', 'cp', str(self.fixture_dir), container + ':/fixture')
        quiet('docker', 'start', container)
        for _ in range(self.attempts):
            if subprocess.run(ready_command(name, container), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                break
            time.sleep(.5)
        else:
            run('docker', 'logs', '--tail', '25', container)
            raise RuntimeError('Disposable database startup failed: ' + name)
        inspected = json.loads(subprocess.check_output(['docker', 'inspect', container], text=True))[0]
        self.ports[name] = inspected['NetworkSettings']['Ports'][str(port) + '/tcp'][0]['HostPort']

    def environment(self, base=None):
        """Environment for child processes; names match the existing Go live tests."""
        env = dict(os.environ if base is None else base)
        if self.systems:
            env['DBM_TEST_CA_FILE'] = str(self.fixture_dir / 'ca.pem')
        for name, port in self.ports.items():
            env['DBM_TEST_' + name.upper() + '_PORT'] = port
        env['DBM_TEST_SQLITE_DIR'] = str(self.sqlite_dir)
        env['DBM_TEST_PASSWORD'] = PASSWORD
        return env

    def cleanup(self):
        """Remove only the containers and directories created by this instance."""
        failures = []
        for container in reversed(self.containers):
            result = subprocess.run(['docker', 'rm', '-f', '-v', container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if result.returncode != 0:
                failures.append(container)
        self.containers = failures
        if self._tmp is not None:
            self._tmp.cleanup()
            self._tmp = None
        if failures:
            raise RuntimeError('Could not remove containers: ' + ', '.join(failures))

    def leftovers(self):
        """Names of containers carrying this instance's label (should be empty after cleanup)."""
        out = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + self.prefix,
                                       '--format', '{{.Names}}'], text=True)
        return out.split()
