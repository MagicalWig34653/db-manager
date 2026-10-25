#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run isolated W-P10 fixtures sequentially and validate real engine responses."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from dbfix import Fixtures, ROOT

loader = importlib.util.spec_from_file_location('maintenance_guard', ROOT / 'scripts/check-maintenance-contract.py')
guard = importlib.util.module_from_spec(loader)
loader.loader.exec_module(guard)


def main():
    subprocess.run(['docker', 'info', '--format', '{{.ServerVersion}}'], check=True)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p10-') as scratch:
        if len(sys.argv) > 1:  # optional directory that keeps the real response dumps
            scratch = str(Path(sys.argv[1]).resolve())
            Path(scratch).mkdir(parents=True, exist_ok=True)
        env = os.environ.copy()
        env['DBM_MAINTENANCE_DUMP'] = scratch
        subprocess.run(['go', 'test', '-race', '-count=1', './internal/database', './internal/httpapi', '-run', 'TestSQLiteMaintenance|TestMaintenance'], cwd=ROOT, env=env, check=True)
        for system in ('mysql', 'maria', 'postgres'):
            with Fixtures('w-p10', systems=(system,)) as fx:
                env = fx.environment()
                env['DBM_MAINTENANCE_DUMP'] = scratch
                subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', '^TestLiveMaintenance/' + system + '$'], cwd=ROOT, env=env, check=True)
            left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + fx.prefix, '--format', '{{.Names}}'], text=True).split()
            assert not left, left
        seen = set()
        for path in Path(scratch).glob('*.json'):
            test, component, label, _ = path.name.split('.')
            problems = guard.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(test)
        assert {'TestSQLiteMaintenance', 'TestLiveMaintenance_mysql', 'TestLiveMaintenance_maria', 'TestLiveMaintenance_postgres'} <= seen, seen
        print('W-P10 real responses validated for SQLite, MySQL, MariaDB and PostgreSQL; own containers removed.')


if __name__ == '__main__':
    main()
