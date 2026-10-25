#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P19 suites and validate real responses, optionally without Docker."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT

source = importlib.util.spec_from_file_location('program_guard', ROOT / 'scripts/check-programs-contract.py')
check = importlib.util.module_from_spec(source)
source.loader.exec_module(check)


def run(env, package, pattern, dump):
    subprocess.run(['go', 'test', '-race', '-count=1', '-v', package, '-run', pattern],
                   env=dict(env, DBM_PROGRAMS_DUMP=str(dump)), cwd=ROOT, check=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--sqlite-only', action='store_true')
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p19-dump-') as directory:
        dump = Path(directory)
        run(os.environ, './internal/database', '^TestProgram', dump)
        run(os.environ, './internal/httpapi', '^TestProgramsHTTP$', dump)
        if not args.sqlite_only:
            subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
            for system in ('mysql', 'maria', 'postgres'):
                with Fixtures('w-p19', systems=(system,)) as fx:
                    run(fx.environment(), './internal/database', '^TestLivePrograms/' + system + '$', dump)
                left = subprocess.check_output(['docker', 'ps', '-a', '--filter',
                                               'label=dbmanager.fixture=' + fx.prefix,
                                               '--format', '{{.Names}}'], text=True).split()
                assert not left, left
        spec = check.load()
        paths = list(dump.glob('*.json'))
        assert len(paths) >= 7, len(paths)
        for path in paths:
            component = path.name.rsplit('.', 2)[1]
            problems = check.validate(component, json.loads(path.read_text()), spec)
            assert not problems, (path.name, problems)
        if not args.sqlite_only:
            for system in ('mysql', 'maria', 'postgres'):
                assert any(path.name.startswith('TestLivePrograms_' + system + '.') for path in paths), system
        print(f'Programs tests passed; {len(paths)} real response dumps validated; sqlite_only={args.sqlite_only}')


if __name__ == '__main__':
    main()
