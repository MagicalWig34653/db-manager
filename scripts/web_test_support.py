# SPDX-License-Identifier: AGPL-3.0-only
"""Shared disposable-runtime checks; never print credentials or request bodies."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
HIDDEN = []


def register_secret(value):
    HIDDEN.append(value)
    return value


def redact(value):
    text = str(value)
    for hidden in HIDDEN:
        if hidden:
            text = text.replace(hidden, '<redacted>')
    return text.replace(str(ROOT), '<workspace>').replace(str(Path.home()), '<home>')


def run(command, *, cwd=None, env=None, input=None):
    result = subprocess.run(command, cwd=cwd, env=env, input=input, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(redact(f'{command[0]} {command[1] if len(command) > 1 else ""} failed ({result.returncode}):\n{result.stdout}\n{result.stderr}'))
    return result.stdout.strip()


def wait_until(check, description, attempts=120):
    for _ in range(attempts):
        try:
            if check():
                return
        except (OSError, ValueError, RuntimeError, urllib.error.URLError):
            pass
        time.sleep(0.5)
    raise RuntimeError('Timed out: ' + description)


def request(base, route, token=None, body=None, expected=200):
    headers = {}
    if token is not None:
        headers['Authorization'] = 'Bearer ' + token
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(base + route, data=data, headers=headers)
    try:
        response = urllib.request.urlopen(req, timeout=15)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        content = response.read(10 * 1024 * 1024)
        if response.status != expected:
            raise RuntimeError(f'{route} returned HTTP {response.status}, expected {expected}')
        return content, response.headers


def api(base, route, token=None, body=None, expected=200):
    return json.loads(request(base, route, token, body, expected)[0])


def query(base, token, connection, sql, **options):
    return api(base, '/api/v1/query', token, {'connection': connection, 'sql': sql, **options})


def verify_http(base, token):
    assert api(base, '/healthz')['status'] == 'ok'
    assert api(base, '/api/v1/capabilities', expected=401)['error']['code'] == 'unauthorized'
    capabilities = api(base, '/api/v1/capabilities', token)
    assert capabilities['contract_version'] == '0.1.0'
    assert set(capabilities['drivers']) == {'sqlite', 'mysql', 'postgres'}
    assert api(base, '/api/v2/capabilities', expected=401)['error']['code'] == 'unauthorized'
    table_capabilities = api(base, '/api/v2/capabilities', token)
    assert table_capabilities['contract_version'] == '0.2.0' and table_capabilities['mutations'] is False
    assert set(table_capabilities['features']) == {'table_describe', 'table_browse'}
    unknown, _ = request(base, '/api/v2/unknown', expected=404)
    assert b'<html' not in unknown.lower()
    manifest = json.loads((ROOT / 'internal/webui/assets-manifest.json').read_text())
    for item in manifest['files']:
        content, headers = request(base, '/' if item['path'] == 'index.html' else '/' + item['path'])
        assert hashlib.sha256(content).hexdigest() == item['sha256']
        assert headers['X-Content-Type-Options'] == 'nosniff'
        if item['path'] == 'index.html':
            assert headers['Cache-Control'] == 'no-store'
            policy = headers['Content-Security-Policy']
            assert "script-src 'self';" in policy and 'unsafe-eval' not in policy
            assert "script-src-attr 'none';" in policy and "frame-ancestors 'none'" in policy
            assert b'lang="de"' in content
        elif item['path'].endswith('.js'):
            assert 'immutable' in headers['Cache-Control']
            assert headers['Content-Type'].startswith('text/javascript')
        elif item['path'].endswith('.css'):
            assert 'immutable' in headers['Cache-Control']
            assert headers['Content-Type'].startswith('text/css')


def verify_image(image):
    paths = [(ROOT / 'LICENSE', '/usr/share/licenses/db-manager/LICENSE')]
    paths += [(path, '/usr/share/licenses/db-manager/go/' + path.name) for path in sorted((ROOT / 'licenses').glob('*.txt'))]
    paths += [(ROOT / 'web-build.lock.json', '/usr/share/db-manager/web-build.lock.json'),
              (ROOT / 'internal/webui/assets-manifest.json', '/usr/share/db-manager/web-assets-manifest.json')]
    actual = run(['docker', 'run', '--rm', '--entrypoint', 'sha256sum', image, *[target for _, target in paths]])
    found = {line.split()[1]: line.split()[0] for line in actual.splitlines()}
    for original, target in paths:
        assert found[target] == hashlib.sha256(original.read_bytes()).hexdigest()
    labels = json.loads(run(['docker', 'image', 'inspect', '--format', '{{json .Config.Labels}}', image]))
    assert labels['org.opencontainers.image.source'] == 'https://github.com/MagicalWig34653/db-manager'
    assert labels['org.opencontainers.image.licenses'] == 'AGPL-3.0-only'
    pin = json.loads((ROOT / 'web-build.lock.json').read_text())['commit']
    assert labels['org.dbmanager.frontend.source'].endswith('/' + pin)
    run(['docker', 'run', '--rm', '--entrypoint', 'sh', image, '-c', 'if command -v node >/dev/null || command -v npm >/dev/null; then exit 9; fi'])


def verify_persistence(base, token, restart):
    connection = {'driver': 'sqlite', 'database': 'deployment_fixture'}
    query(base, token, connection, 'CREATE TABLE saved(value TEXT)')
    query(base, token, connection, "INSERT INTO saved VALUES('Neustart ✓')")
    assert query(base, token, connection, 'SELECT value FROM saved')['results'][0]['rows'][0][0]['value'] == 'Neustart ✓'
    restart()
    wait_until(lambda: api(base, '/healthz')['status'] == 'ok', 'restart health')
    assert query(base, token, connection, 'SELECT value FROM saved')['results'][0]['rows'][0][0]['value'] == 'Neustart ✓'


def start_postgres_fixture(container, directory, database):
    """Prove socket-only bootstrap is insufficient before waiting for final TCP."""
    barrier = directory / 'postgres-bootstrap-barrier.sh'
    barrier.write_text('touch /fixture/bootstrap_waiting\nwhile [ ! -e /fixture/release_bootstrap ]; do sleep 0.1; done\n')
    run(['docker', 'cp', str(barrier), container + ':/docker-entrypoint-initdb.d/00_readiness_regression.sh'])
    run(['docker', 'start', container])
    wait_until(lambda: run(['docker', 'exec', container, 'test', '-e', '/fixture/bootstrap_waiting']) == '', 'PostgreSQL bootstrap barrier')
    probe = ['docker', 'exec', container, 'pg_isready', '-U', 'postgres', '-d', database]
    assert 'accepting connections' in run(probe)
    tcp_probe = [*probe, '-h', '127.0.0.1']
    assert subprocess.run(tcp_probe, capture_output=True).returncode != 0, 'Bootstrap is not final TCP readiness'
    run(['docker', 'exec', container, 'touch', '/fixture/release_bootstrap'])
    wait_until(lambda: 'accepting connections' in run(tcp_probe), 'final PostgreSQL TCP readiness')
    print('PostgreSQL readiness regression: socket-only bootstrap reproduced and refused; final TCP accepted.', flush=True)
