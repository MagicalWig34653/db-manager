#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run real SQLite and isolated dbfix network-engine monitor tests."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT, SPECS


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--sqlite-only', action='store_true')
    parser.add_argument('--keep', type=Path)
    args = parser.parse_args()
    # Configure only disposable fixtures; production never changes logs or extensions.
    image, port, variables, command, owner = SPECS['postgres']
    SPECS['postgres'] = (image, port, variables, command + ['-c', 'shared_preload_libraries=pg_stat_statements'], owner)
    for system in ('mysql', 'maria'):
        image, port, variables, command, owner = SPECS[system]
        SPECS[system] = (image, port, variables, command + [
            '--log-bin=fixture-bin', '--server-id=27', '--performance-schema=ON',
            '--slow-query-log=ON', '--log-output=TABLE', '--long-query-time=0'], owner)
    spec = importlib.util.spec_from_file_location('monitor_guard', ROOT / 'scripts/check-monitor-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p27-results-') as scratch:
        if args.keep:
            scratch = str(args.keep.resolve())
            Path(scratch).mkdir(parents=True, exist_ok=True)
        env = os.environ.copy()
        for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT', 'DBM_TEST_CA_FILE'):
            env.pop(key, None)
        env['DBM_MONITOR_DUMP'] = scratch
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi', '-run', 'TestSQLiteMonitor|TestMonitor'], cwd=ROOT, env=env, check=True)
        systems = () if args.sqlite_only else ('mysql', 'maria', 'postgres')
        for system in systems:
            with Fixtures('w-p27', systems=(system,)) as fixture:
                live_env = fixture.environment()
                live_env['DBM_MONITOR_DUMP'] = scratch
                live_env['DBM_TEST_MONITOR_FIXTURE'] = '1'
                subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveMonitor/' + system + '$'], cwd=ROOT, env=live_env, check=True)
        paths = list(Path(scratch).glob('*.MonitorResult.json'))
        assert len(paths) >= 2
        for path in paths:
            problems = guard.validate('MonitorResult', json.loads(path.read_text()))
            assert not problems, (path.name, problems)
        print(f'W-P27: {len(paths)} real results validated; systems: SQLite' + (', MySQL, MariaDB, PostgreSQL' if systems else ' only'))


if __name__ == '__main__':
    main()
