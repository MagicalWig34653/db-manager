#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Capture the real pinned embedded product using disposable synthetic SQLite."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
PIN = '8aab52bc9be8ccaf84dbdb0eda61ec46a60181d3'


def main():
    with tempfile.TemporaryDirectory(prefix='dbm-pages-capture-') as folder:
        temp = Path(folder)
        archive = temp / 'source.tar'
        with archive.open('wb') as stream:
            subprocess.run(['git', 'archive', PIN], cwd=ROOT, stdout=stream, check=True)
        source = temp / 'source'
        source.mkdir()
        with tarfile.open(archive) as stream:
            stream.extractall(source, filter='data')
        env = {key: value for key, value in os.environ.items() if key in {'PATH', 'HOME', 'TMPDIR', 'GOCACHE', 'GOPATH', 'PLAYWRIGHT_BROWSERS_PATH'}}
        subprocess.run(['go', 'build', '-trimpath', '-o', str(temp / 'db-manager'), './cmd/db-manager'], cwd=source, env=env, check=True)
        token = secrets.token_urlsafe(40)
        token_file = temp / 'token'
        token_file.write_text(token)
        token_file.chmod(0o600)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        base = f'http://127.0.0.1:{port}'
        env.update(DBM_TOKEN_FILE=str(token_file), DBM_ADDRESS=f'127.0.0.1:{port}', DBM_DATA_DIR=str(temp / 'data'), DBM_ALLOWED_HOSTS='localhost,127.0.0.1')
        with (temp / 'server.log').open('wb') as log:
            process = subprocess.Popen([str(temp / 'db-manager')], env=env, stdout=log, stderr=log)
            try:
                for _ in range(120):
                    try:
                        with urllib.request.urlopen(base + '/healthz', timeout=1) as response:
                            if response.status == 200:
                                break
                    except OSError:
                        time.sleep(0.25)
                else:
                    raise RuntimeError('Disposable product did not start')
                statements = [
                    'CREATE TABLE projekte (id INTEGER PRIMARY KEY, projekt TEXT NOT NULL, status TEXT, budget TEXT, notiz TEXT)',
                    "INSERT INTO projekte VALUES (1,'Website-Relaunch','In Arbeit','12500.00',NULL),(2,'Datenmigration','Geplant','8400.00',''),(3,'Kundenportal','In Prüfung','21950.50','Abnahme vorbereiten'),(4,'Dokumentation','In Arbeit','3200.00','Deutsch und offen'),(5,'Schnittstellen','Abgeschlossen','6750.00',NULL),(6,'Qualitätssicherung','Geplant','4100.00','Synthetische Beispieldaten')",
                ]
                for statement in statements:
                    body = json.dumps({'connection': {'driver': 'sqlite', 'database': 'pages_demo'}, 'sql': statement}).encode()
                    request = urllib.request.Request(base + '/api/v1/query', data=body, headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
                    with urllib.request.urlopen(request, timeout=10) as response:
                        assert response.status == 200
                config = temp / 'capture.json'
                config.write_text(json.dumps({'base': base, 'token': token}))
                config.chmod(0o600)
                browser_env = dict(env, DBM_CAPTURE_CONFIG=str(config))
                subprocess.run(['node', 'tests/capture.mjs'], cwd=ROOT / 'site', env=browser_env, check=True)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
    output = ROOT / 'site/assets/screenshots'
    files = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(output.glob('*.png'))}
    (output / 'provenance.json').write_text(json.dumps({'product_repository': 'https://github.com/MagicalWig34653/db-manager', 'product_commit': PIN, 'platform': 'Chromium desktop/tablet web; no native Apple screenshot', 'data': 'Synthetic disposable SQLite fixture defined in site/capture.py', 'files': files}, indent=2) + '\n')
    print(f'Captured {len(files)} real product images; own server and temporary data removed')


if __name__ == '__main__':
    main()
