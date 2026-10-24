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
    for version in [3, 4, 5]:
        assert api(base, f'/api/v{version}/capabilities', expected=401)['error']['code'] == 'unauthorized'
        assert api(base, f'/api/v{version}/capabilities', token)['contract_version'] == f'0.{version}.0'
        missing, _ = request(base, f'/api/v{version}/unknown', expected=404)
        assert b'<html' not in missing.lower()
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
        elif item['path'].startswith('assets/') and item['path'].endswith('.js'):
            assert 'immutable' in headers['Cache-Control']
            assert headers['Content-Type'].startswith('text/javascript')
        elif item['path'].startswith('assets/') and item['path'].endswith('.css'):
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
    """Exercise confirmed row APIs, then prove stored typed data survives replacement."""
    connection = {'driver': 'sqlite', 'database': 'deployment_fixture'}
    table = {'schema': 'main', 'name': 'saved'}
    query(base, token, connection, "CREATE TABLE saved(id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL DEFAULT 'default', nullable TEXT, precise TEXT, payload BLOB)")
    query(base, token, connection, "CREATE INDEX saved_value_partial ON saved(value DESC) WHERE nullable IS NULL")
    context = {'connection': connection, 'table': table}
    assert api(base, '/api/v3/tables/eligibility', token, context)['eligible']
    assert api(base, '/api/v4/tables/insert-eligibility', token, context)['eligible']
    precise = '12345678901234567890.12345678901234567890'
    values = [{'column': name, 'value': cell} for name, cell in [
        ('value', {'kind': 'text', 'value': 'before-update'}),
        ('nullable', {'kind': 'null'}),
        ('precise', {'kind': 'text', 'value': precise}),
        ('payload', {'kind': 'binary', 'value': 'AP8='})]]
    created = api(base, '/api/v4/rows/insert', token, {**context, 'values': values, 'confirm': True})
    assert created['outcome'] == 'committed'
    original = created['row']
    updated = api(base, '/api/v3/rows/update', token, {**context, 'expected': original, 'changes': [{'column': 'value', 'value': {'kind': 'text', 'value': 'Neustart ✓'}}], 'confirm': True})
    assert updated['outcome'] == 'committed' and updated['changed']
    conflict = api(base, '/api/v3/rows/update', token, {**context, 'expected': original, 'changes': [{'column': 'value', 'value': {'kind': 'text', 'value': 'must-not-overwrite'}}], 'confirm': True}, expected=409)
    assert conflict['error']['code'] == 'row_conflict'
    default = api(base, '/api/v4/rows/insert', token, {**context, 'values': [], 'confirm': True})
    assert default['row']['values'][1] == {'kind': 'text', 'value': 'default'}
    deleted = api(base, '/api/v3/rows/delete', token, {**context, 'expected': default['row'], 'confirm': True})
    assert deleted['outcome'] == 'committed' and deleted['changed']
    expected = [[{'kind': 'text', 'value': 'Neustart ✓'}, {'kind': 'null'}, {'kind': 'text', 'value': precise}, {'kind': 'binary', 'value': 'AP8='}]]
    assert query(base, token, connection, 'SELECT value, nullable, precise, payload FROM saved')['results'][0]['rows'] == expected
    before_structure = api(base, '/api/v5/tables/structure', token, context)
    index = next(item for item in before_structure['indexes'] if item['name'] == 'saved_value_partial')
    assert index['partial'] == 'yes'
    assert any(term['column'] == 'value' and term['direction'] == 'desc' for term in index['terms'] if term['kind'] == 'column')
    restart()
    wait_until(lambda: api(base, '/healthz')['status'] == 'ok', 'restart health')
    assert query(base, token, connection, 'SELECT value, nullable, precise, payload FROM saved')['results'][0]['rows'] == expected
    assert api(base, '/api/v4/capabilities', token)['contract_version'] == '0.4.0'
    structure = api(base, '/api/v5/tables/structure', token, context)
    assert structure == before_structure
    assert structure['coverage']['indexes'] == 'catalog'
    assert structure['coverage']['checks'] == 'unavailable'
    assert any(constraint['kind'] == 'primary_key' for constraint in structure['constraints'])


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
