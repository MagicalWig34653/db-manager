#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run owned, disposable W-P17 fixtures and validate actual response schemas."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
from dbfix import Fixtures, ROOT

loader = importlib.util.spec_from_file_location('server_admin_guard', ROOT / 'scripts/check-server-admin-contract.py')
guard = importlib.util.module_from_spec(loader)
loader.loader.exec_module(guard)

def main():
    env = dict(os.environ)
    env['GOPROXY'] = 'off'
    subprocess.run(['go', 'test', '-race', '-count=1', './internal/database', './internal/httpapi', '-run', 'TestSQLiteServerAdmin|TestServerAdmin'], cwd=ROOT, env=env, check=True)
    subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    for system in ('mysql', 'maria', 'postgres'):
        with Fixtures('w-p17', systems=(system,)) as fx, tempfile.TemporaryDirectory(prefix='dbmanager-w-p17-dump-') as scratch:
            live_env = fx.environment(env)
            live_env['DBM_SERVER_ADMIN_DUMP'] = scratch
            subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveServerAdmin/' + system + '$'], cwd=ROOT, env=live_env, check=True)
            paths = list(Path(scratch).glob('*.json'))
            assert len(paths) >= 10, (system, len(paths))
            for path in paths:
                problems = guard.validate(path.name.split('.')[0], json.loads(path.read_text()))
                assert not problems, (system, path.name, problems)
            print(f'{system}: {len(paths)} actual responses validated against API 0.17')

if __name__ == '__main__':
    main()
