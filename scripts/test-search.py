#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run and schema-check W-P14 on dbfix-owned disposable TLS databases."""
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
    spec = importlib.util.spec_from_file_location('check_search', ROOT / 'scripts/check-search-contract.py')
    check = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(check)
    systems = () if args.sqlite_only else ('mysql', 'maria', 'postgres')
    if not args.sqlite_only:
        subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    with Fixtures('w-p14', systems=systems) as fx, tempfile.TemporaryDirectory(prefix='dbmanager-w-p14-results-') as scratch:
        env = fx.environment()
        env['DBM_SEARCH_DUMP'] = scratch
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', '-run',
                        'TestSQLiteSearch|TestSQLiteReplaceSafety|TestLiveSearch|TestSearchHTTP'], cwd=ROOT, env=env, check=True)
        components = {'search': 'SearchResult', 'plan': 'ReplacePlan', 'result': 'ReplaceResult',
                      'browse': 'BrowseResult', 'rejected': 'ReplaceResult', 'unknown': 'ReplaceResult', 'capabilities': 'SearchCapabilities'}
        seen = set()
        for path in sorted(Path(scratch).glob('*.json')):
            test, kind, _ = path.name.rsplit('.', 2)
            problems = check.validate(components[kind], json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(test)
        expected = {'TestSQLiteSearch', 'TestSQLiteReplaceSafety', 'TestSearchHTTP'}
        if not args.sqlite_only:
            expected |= {'TestLiveSearch_mysql', 'TestLiveSearch_maria', 'TestLiveSearch_postgres'}
        assert expected <= seen, (expected, seen)
        print('search: native results validated for ' + ', '.join(sorted(seen)))
    if not args.sqlite_only:
        assert not fx.leftovers(), 'owned fixture containers remain'


if __name__ == '__main__':
    main()
