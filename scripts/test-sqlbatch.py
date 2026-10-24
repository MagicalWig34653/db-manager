#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P15 with dbfix fixtures, or SQLite only with --sqlite-only."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--sqlite-only', action='store_true')
    args = parser.parse_args()
    systems = () if args.sqlite_only else ('mysql', 'maria', 'postgres')
    spec = importlib.util.spec_from_file_location('sqlbatch_guard', ROOT / 'scripts/check-sqlbatch-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    with Fixtures('w-p15', systems=systems) as fx, tempfile.TemporaryDirectory(prefix='dbmanager-w-p15-results-') as scratch:
        env = fx.environment()
        if args.sqlite_only:
            for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT', 'DBM_TEST_CA_FILE'):
                env.pop(key, None)
        env['DBM_SQLBATCH_DUMP'] = scratch
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', '-run', 'TestSQLiteSQLBatch|TestSQLBatchValidation|TestLiveSQLBatch|TestSQLBatchHTTP'], env=env, cwd=ROOT, check=True)
        results = list(Path(scratch).glob('*.json'))
        expected = {'TestSQLiteSQLBatch', 'http'}
        if systems:
            expected.update('TestLiveSQLBatch_' + name for name in systems)
        seen = set()
        for path in results:
            name, component, _ = path.name.rsplit('.', 2)
            problems = guard.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(name.removesuffix('-explain'))
        assert seen == expected, (seen, expected)
        print(f'W-P15: {len(results)} real results validated; systems: SQLite' + (', MySQL, MariaDB, PostgreSQL' if systems else ' only'))


if __name__ == '__main__':
    main()
