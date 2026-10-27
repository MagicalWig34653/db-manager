#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""W-P20 live test: configuration store on a real MariaDB server state.

Starts one disposable TLS MariaDB (scripts/dbfix.py, task-owned containers only), runs the
shared repository conformance suite, the replica/encryption/outage tests and the HTTP flow
against it, and validates the recorded real responses against API 0.24. Without Docker use
--no-docker (in-memory conformance and HTTP unit tests only).
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from dbfix import Fixtures, PASSWORD, ROOT


def load_guard():
    spec = importlib.util.spec_from_file_location('config_guard', ROOT / 'scripts/check-config-store-contract.py')
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    return guard


def go(args, env=None):
    result = subprocess.run(['go', 'test', '-race', '-count=1', *args], cwd=ROOT, env=env, text=True, capture_output=True)
    lines = [line for line in result.stdout.splitlines() if line.startswith(('--- ', 'ok', 'FAIL', 'PASS', 'panic'))]
    print('\n'.join(lines), flush=True)
    if result.returncode:
        print(result.stdout[-6000:], result.stderr[-2000:], file=sys.stderr)
        raise SystemExit('go test failed: ' + ' '.join(args))
    return result.stdout


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--no-docker', action='store_true')
    parser.add_argument('--keep', type=Path, help='write the recorded real responses here')
    args = parser.parse_args()
    go(['./internal/configstore/...', './internal/httpapi'])
    if args.no_docker:
        print('W-P20: in-memory conformance and HTTP tests passed (no MariaDB run).')
        return
    subprocess.run(['docker', 'info', '--format', '{{.ServerVersion}}'], check=True, stdout=subprocess.DEVNULL)
    guard = load_guard()
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p20-dump-') as scratch:
        dump = args.keep.resolve() if args.keep else Path(scratch)
        dump.mkdir(parents=True, exist_ok=True)
        with Fixtures('w-p20', systems=('maria',)) as fx:
            env = fx.environment()
            env.update({
                'DBM_TEST_STATE_PORT': fx.ports['maria'],
                'DBM_TEST_STATE_CA_FILE': str(fx.fixture_dir / 'ca.pem'),
                'DBM_TEST_STATE_ROOT_PASSWORD': PASSWORD,
                'DBM_CONFIG_DUMP': str(dump),
            })
            out = go(['-v', './internal/serverstate', '-run', 'TestConfig'], env)
            if '--- SKIP' in out:
                raise SystemExit('live tests were skipped')
            for name in ('TestConfigRepoConformanceOnMariaDB', 'TestConfigRepoTwoReplicasShareOneState', 'TestConfigRowsAreEncryptedAndBoundToTheirOwner',
                         'TestConfigRepoRuntimeUserCannotChangeSchemaAndOutageIsClassified', 'TestConfigRepoOutageMapsToUnavailableAndRecovers', 'TestConfigAPIOnRealStateStore'):
                assert '--- PASS: ' + name in out, name + ' did not run'
            # The existing state-store suite must still pass on the new migration (runs against the same server).
            go(['-v', './internal/serverstate', '-run', 'TestEncryptedFields|TestRuntimeUserHasLeastPrivilege|TestPlanIsDryRun|TestConcurrentMigration'], env)
        assert not fx.leftovers()
        paths = sorted(dump.glob('*.json'))
        assert len(paths) >= 12, 'no real response dumps'
        for path in paths:
            component = path.name.split('.')[0]
            problems = guard.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
        print(f'W-P20: {len(paths)} real responses validated against API 0.24; MariaDB container removed.')


if __name__ == '__main__':
    main()
