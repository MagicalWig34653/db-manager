#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run W-P28 SQLite/HTTP checks; --live uses isolated dbfix fixtures serially."""
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
    parser.add_argument('--live', action='store_true')
    args = parser.parse_args()
    env = os.environ.copy()
    env['GOPROXY'] = 'off'
    env['GOSUMDB'] = 'off'
    for key in list(env):
        if key.startswith('DBM_TEST_'):
            del env[key]
    with tempfile.TemporaryDirectory(prefix='dbmanager-wp28-') as scratch:
        env.setdefault('GOCACHE', str(Path(scratch) / 'cache'))
        dump = Path(scratch) / 'responses'
        dump.mkdir()
        env['DBM_EXTENDED_EXPORT_DUMP'] = str(dump)
        command = ['go', 'test', '-race', '-count=1', '-v', './internal/core',
                   './internal/database', './internal/httpapi', '-run',
                   'TestSQLiteExtended|TestExtendedExport|TestLiveExtendedExport']
        subprocess.run(command, cwd=ROOT, env=env, check=True)
        if args.live:
            subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
            for system in ('mysql', 'maria', 'postgres'):
                with Fixtures('w-p28', systems=(system,)) as fixtures:
                    live = fixtures.environment(env)
                    live['GOCACHE'] = env['GOCACHE']
                    live['GOPROXY'] = 'off'
                    live['GOSUMDB'] = 'off'
                    live['DBM_EXTENDED_EXPORT_DUMP'] = str(dump)
                    subprocess.run(command[:-1] + ['TestLiveExtendedExport/' + system + '$'],
                                   cwd=ROOT, env=live, check=True)
        module = importlib.util.spec_from_file_location('extended_guard', ROOT / 'scripts/check-extended-export-contract.py')
        guard = importlib.util.module_from_spec(module)
        module.loader.exec_module(guard)
        spec = json.loads((ROOT / 'contracts/api-v0.21.0.json').read_text())
        actual = json.loads((dump / 'ExtendedExportCapabilities.actual.json').read_text())
        assert guard.guard.validate(spec['components']['schemas']['ExtendedExportCapabilities'], actual, spec)
        guard.main()
        print('SQLite typed XML/YAML round-trips, reference escapes, dump restore, HTTP/GZip/limits passed.')
        print('Live fixtures: ' + ('requested and passed' if args.live else 'not requested'))


if __name__ == '__main__':
    main()
