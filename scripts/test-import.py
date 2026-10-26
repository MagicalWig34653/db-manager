#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run import tests with dbfix fixtures, or SQLite only without --live."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--live', action='store_true')
    parser.add_argument('--keep', type=Path)
    args = parser.parse_args()
    module = importlib.util.spec_from_file_location('import_guard', ROOT / 'scripts/check-import-contract.py')
    guard = importlib.util.module_from_spec(module)
    module.loader.exec_module(guard)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p12-results-') as temporary:
        scratch = args.keep.resolve() if args.keep else Path(temporary)
        scratch.mkdir(parents=True, exist_ok=True)
        env = os.environ.copy()
        for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT', 'DBM_TEST_CA_FILE'):
            env.pop(key, None)
        env['DBM_IMPORT_DUMP'] = str(scratch)
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', './internal/sqlsplit', '-run', 'TestSQLiteImport|TestImport'], env=env, cwd=ROOT, check=True)
        for system in (('mysql', 'maria', 'postgres') if args.live else ()):
            with Fixtures('w-p12', systems=(system,)) as fixture:
                live_env = fixture.environment()
                live_env['DBM_IMPORT_DUMP'] = str(scratch)
                subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveImport/' + system + '$'], env=live_env, cwd=ROOT, check=True)
            left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + fixture.prefix, '--format', '{{.Names}}'], text=True).split()
            assert not left, left
        results = list(scratch.glob('*.real-*.json'))
        assert len(results) >= (15 if args.live else 5), len(results)
        for path in results:
            problems = guard.validate(path.name.split('.')[0], json.loads(path.read_text()))
            assert not problems, (path.name, problems)
        print(f'W-P12: {len(results)} real responses validated; SQLite' + (', MySQL, MariaDB, PostgreSQL' if args.live else ' only'))


if __name__ == '__main__':
    main()
