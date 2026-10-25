#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run accounts tests with exclusively owned sequential dbfix fixtures."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT

loader = importlib.util.spec_from_file_location('accounts_contract', ROOT / 'scripts/check-accounts-contract.py')
contract = importlib.util.module_from_spec(loader)
loader.loader.exec_module(contract)


def main():
    # Fail before any container creation if the daemon is unavailable.
    subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    with tempfile.TemporaryDirectory(prefix='dbmanager-w-p18-results-') as dump:
        for system in ('mysql', 'maria', 'postgres'):
            with Fixtures('w-p18', systems=(system,)) as fixture:
                env = fixture.environment()
                env['DBM_ACCOUNTS_DUMP'] = dump
                subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', '^TestLiveAccounts/' + system + '$'], cwd=ROOT, env=env, check=True)
            assert not fixture.leftovers()
        seen = set()
        for path in Path(dump).glob('*.json'):
            component = path.name.split('.')[-2]
            problems = contract.validate(component, json.loads(path.read_text()))
            assert not problems, (path.name, problems)
            seen.add(path.name.split('.')[0])
        assert seen == {'TestLiveAccounts_' + s for s in ('mysql', 'maria', 'postgres')}, seen
        print('Accounts: three live drivers, independent catalog checks and response schemas passed')


if __name__ == '__main__':
    main()
