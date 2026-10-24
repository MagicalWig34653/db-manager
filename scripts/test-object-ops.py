#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run the W-P07 object operation suites on SQLite, MySQL 8.4, MariaDB 12.3 and PostgreSQL 18.6.

The database systems start one after the other (one disposable container at a time) to spare
the host. Real engine results are dumped and validated against the v8 schemas.
"""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent))
from dbfix import Fixtures, ROOT  # noqa: E402

spec = importlib.util.spec_from_file_location('check_object_ops', ROOT / 'scripts/check-object-ops-contract.py')
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)

KINDS = {'capabilities': 'ObjectOpCapabilities', 'http-plan': 'ObjectOpPlan', 'http-result': 'ObjectOpResult', 'plan': 'ObjectOpPlan', 'plan-options': 'ObjectOpPlan', 'plan-database': 'ObjectOpPlan', 'dependencies': 'DependenciesResult'}
SYSTEMS = (('mysql', 'mysql'), ('maria', 'maria'), ('postgres', 'postgres'))


def component(kind):
    return KINDS.get(kind) or ('ObjectOpResult' if kind.startswith('result') else None)


def go_test(env, pattern, dump, package='./internal/database'):
    env = dict(env, DBM_OBJECTOPS_DUMP=dump)
    subprocess.run(['go', 'test', '-race', '-count=1', '-v', package, '-run', pattern], env=env, cwd=ROOT, check=True)


def main():
    keep = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else None  # optional directory that keeps the dumps
    subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    import os
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p07-dump-') as scratch:
        dump = str(keep or scratch)
        Path(dump).mkdir(parents=True, exist_ok=True)
        go_test(os.environ.copy(), 'TestSQLiteObjectOps|TestExecuteOutcomeClassification|TestObjectOpRequestValidation', dump)
        go_test(os.environ.copy(), 'TestObjectOpsHTTP', dump, './internal/httpapi')
        prefixes = []
        for label, name in SYSTEMS:
            with Fixtures('w-p07', systems=(label,)) as fx:
                prefixes.append(fx.prefix)
                go_test(fx.environment(), 'TestLiveObjectOps/' + name, dump)
            left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + fx.prefix, '--format', '{{.Names}}'], text=True).split()
            assert not left, left
        spec_v8 = check.load_v8()
        seen = set()
        for path in sorted(Path(dump).glob('*.json')):
            test, kind, _ = path.name.rsplit('.', 2)
            target = component(kind)
            assert target, path.name
            problems = check.validate(target, json.loads(path.read_text()), spec_v8)
            assert not problems, (path.name, problems)
            seen.add((test, kind))
        tests = {t for t, _ in seen}
        assert {'TestSQLiteObjectOps', 'TestLiveObjectOps_mysql', 'TestLiveObjectOps_maria', 'TestLiveObjectOps_postgres', 'TestObjectOpsHTTP'} <= tests, tests
        for t in tests - {'TestObjectOpsHTTP'}:
            have = {k for tt, k in seen if tt == t}
            assert {'plan', 'dependencies', 'result', 'result-rejected', 'result-truncate', 'result-rename', 'result-copy'} <= have, (t, have)
        print(f'object ops ok: {len(seen)} real engine results validated against API 0.8 for {sorted(tests)}')


if __name__ == '__main__':
    main()
