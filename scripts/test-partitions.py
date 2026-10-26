#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P29 fixtures sequentially, validate response dumps; SQLite needs no Docker."""
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
    spec = importlib.util.spec_from_file_location('partitions_guard', ROOT / 'scripts/check-partitions-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p29-dump-') as scratch:
        dump = args.keep.resolve() if args.keep else Path(scratch)
        dump.mkdir(parents=True, exist_ok=True)
        def run(env, pattern):
            env['DBM_PARTITIONS_DUMP'] = str(dump)
            subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', '-run', pattern], cwd=ROOT, env=env, check=True)
        run(os.environ.copy(), 'TestSQLite(Prefix|Partitions)|TestPartition|TestStructureOutcomes')
        if not args.sqlite_only:
            subprocess.run(['docker', 'info', '--format', '{{.ServerVersion}}'], check=True)
            for system in ('mysql', 'maria', 'postgres'):
                with Fixtures('w-p29', systems=(system,)) as fx:
                    run(fx.environment(), 'TestLivePartitions/' + system + '$')
                assert not fx.leftovers()
        paths = list(dump.glob('*.json'))
        assert paths, 'no real response dumps'
        for path in paths:
            component = path.name.rsplit('.', 2)[1]
            problems = guard.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
        print(f'W-P29: {len(paths)} real responses validated')


if __name__ == '__main__':
    main()
