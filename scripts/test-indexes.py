#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P09 with dbfix fixtures, or SQLite only with --sqlite-only."""
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
    parser.add_argument('--sqlite-only', action='store_true')
    parser.add_argument('--keep', type=Path)
    args = parser.parse_args()
    systems = () if args.sqlite_only else ('mysql', 'maria', 'postgres')
    spec = importlib.util.spec_from_file_location('indexes_guard', ROOT / 'scripts/check-indexes-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p09-results-') as scratch:
        if args.keep:
            scratch = str(args.keep.resolve())
            Path(scratch).mkdir(parents=True, exist_ok=True)
        base = os.environ.copy()
        for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT', 'DBM_TEST_CA_FILE'):
            base.pop(key, None)
        base['DBM_INDEXES_DUMP'] = scratch
        packages = ['./internal/database', './internal/httpapi']
        subprocess.run(['go', 'test', '-race', '-count=1', '-v'] + packages + ['-run', 'TestSQLiteIndexes|TestIndexesValidation|TestIndexesHTTP'], env=base, cwd=ROOT, check=True)
        for system in systems:  # one disposable container at a time
            with Fixtures('w-p09', systems=(system,)) as fx:
                env = fx.environment()
                env['DBM_INDEXES_DUMP'] = scratch
                subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveIndexes/' + system + '$'], env=env, cwd=ROOT, check=True)
            left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + fx.prefix, '--format', '{{.Names}}'], text=True).split()
            assert not left, left
        results = list(Path(scratch).glob('*.json'))
        expected = {'TestSQLiteIndexes', 'http'}
        if systems:
            expected.update('TestLiveIndexes_' + name for name in systems)
        seen = set()
        for path in results:
            name, component, _ = path.name.rsplit('.', 2)
            problems = guard.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(name.rsplit('-', 1)[0] if name != 'http' else name)
        assert seen == expected, (seen, expected)
        print(f'W-P09: {len(results)} real results validated; systems: SQLite' + (', MySQL, MariaDB, PostgreSQL' if systems else ' only'))


if __name__ == '__main__':
    main()
