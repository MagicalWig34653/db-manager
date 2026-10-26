#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P13 against real SQLite and disposable dbfix TLS databases."""
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
    spec = importlib.util.spec_from_file_location('rows_bulk_guard', ROOT / 'scripts/check-rows-bulk-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    # Keep temporary output in the assigned worktree, including in restricted runs.
    with tempfile.TemporaryDirectory(prefix='.w-p13-results-', dir=ROOT) as scratch:
        if args.keep:
            scratch = str(args.keep.resolve())
            Path(scratch).mkdir(parents=True, exist_ok=True)
        env = os.environ.copy()
        for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT', 'DBM_TEST_CA_FILE'):
            env.pop(key, None)
        env['DBM_ROWS_BULK_DUMP'] = scratch
        subprocess.run(['go', 'test', '-race', '-count=1', './internal/database', './internal/httpapi', '-run', 'TestSQLiteRowsBulk|TestRowsBulk'], cwd=ROOT, env=env, check=True)
        for system in (() if args.sqlite_only else ('mysql', 'maria', 'postgres')):
            with Fixtures('w-p13', systems=(system,)) as fx:
                live_env = fx.environment()
                live_env['DBM_ROWS_BULK_DUMP'] = scratch
                subprocess.run(['go', 'test', '-race', '-count=1', './internal/database', '-run', 'TestLiveRowsBulk/' + system + '$'], cwd=ROOT, env=live_env, check=True)
        results = list(Path(scratch).glob('*.real-*.json'))
        assert results
        for path in results:
            problems = guard.validate(path.name.split('.')[0], json.loads(path.read_text()))
            assert not problems, (path.name, problems)
        expected = 5 if args.sqlite_only else 14
        assert len(results) == expected, (len(results), expected)
        print(f'W-P13: {len(results)} real response fixtures validated')


if __name__ == '__main__':
    main()
