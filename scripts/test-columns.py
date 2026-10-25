#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P08 fixtures sequentially; --sqlite-only needs no Docker/network.

Optional --keep DIRECTORY preserves real engine response dumps for review.
"""
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
    spec = importlib.util.spec_from_file_location('check_columns', ROOT / 'scripts/check-columns-contract.py')
    check = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(check)
    seen = set()
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p08-dump-') as scratch:
        dump = args.keep.resolve() if args.keep else Path(scratch)
        dump.mkdir(parents=True, exist_ok=True)
        def run(env, pattern):
            env['DBM_COLUMNS_DUMP'] = str(dump)
            subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', '-run', pattern], cwd=ROOT, env=env, check=True)
        run(os.environ.copy(), 'TestSQLiteColumns|TestColumns')
        if not args.sqlite_only:
            subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
            for system in ('mysql', 'maria', 'postgres'):
                with Fixtures('w-p08', systems=(system,)) as fx:
                    run(fx.environment(), 'TestLiveColumns/' + system + '$')
                left = subprocess.check_output(['docker','ps','-a','--filter','label=dbmanager.fixture=' + fx.prefix,'--format','{{.Names}}'], text=True).split()
                assert not left, left
        for path in sorted(dump.glob('*.json')):
            test, component, _ = path.name.rsplit('.', 2)
            problems = check.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(test.split('-')[0])
        assert 'TestSQLiteColumns' in seen and 'http' in seen, seen
        if not args.sqlite_only:
            assert {'TestLiveColumns_mysql','TestLiveColumns_maria','TestLiveColumns_postgres'} <= seen, seen
        print(f'columns ok: {len(list(dump.glob("*.json")))} real responses validated; suites: {sorted(seen)}')


if __name__ == '__main__':
    main()
