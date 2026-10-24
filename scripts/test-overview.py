#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run the W-P06 overview suites on SQLite, MySQL 8.4, MariaDB 12.3 and PostgreSQL 18.6.

Real engine results are dumped and validated against the v7 schemas.
"""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from dbfix import Fixtures, ROOT  # noqa: E402

spec = importlib.util.spec_from_file_location('check_overview', ROOT / 'scripts/check-overview-contract.py')
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)

COMPONENTS = {'databases': 'DatabasesResult', 'objects': 'ObjectsResult', 'definition-table': 'DefinitionResult', 'definition-view': 'DefinitionResult', 'server-info': 'ServerInfoResult'}


def main():
    keep = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else None  # optional directory that keeps the dumps
    subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    with Fixtures('w-p06') as fx, tempfile.TemporaryDirectory(prefix='dbmanager-w-p06-dump-') as scratch:
        dump = str(keep or scratch)
        Path(dump).mkdir(parents=True, exist_ok=True)
        env = fx.environment()
        env['DBM_OVERVIEW_DUMP'] = dump
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestSQLiteOverview|TestLiveOverview|TestOverviewValidation'], env=env, cwd=ROOT, check=True)
        spec_v7 = check.load_v7()
        seen = set()
        for path in sorted(Path(dump).glob('*.json')):
            test, kind, _ = path.name.rsplit('.', 2)
            problems = check.validate(COMPONENTS[kind], json.loads(path.read_text()), spec_v7)
            assert not problems, (path.name, problems)
            seen.add((test, kind))
        tests = {t for t, _ in seen}
        assert {'TestSQLiteOverview', 'TestLiveOverview_mysql', 'TestLiveOverview_maria', 'TestLiveOverview_postgres'} <= tests, tests
        assert all((t, k) in seen for t in tests for k in COMPONENTS), seen
        print(f'overview ok: {len(seen)} real engine results validated against API 0.7 for {sorted(tests)}')
    left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + fx.prefix, '--format', '{{.Names}}'], text=True).split()
    assert not left, left


if __name__ == '__main__':
    main()
