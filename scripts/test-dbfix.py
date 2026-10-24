#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Prove scripts/dbfix.py: start fixtures, run the v6 reference case, verify cleanup."""
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from dbfix import Fixtures, ROOT  # noqa: E402


def main():
    subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
    prefix = None
    with Fixtures('w-p01') as fx:
        prefix = fx.prefix
        env = fx.environment()
        for name in ('mysql', 'maria', 'postgres'):
            assert env['DBM_TEST_' + name.upper() + '_PORT'].isdigit(), name
        assert Path(env['DBM_TEST_CA_FILE']).is_file() and Path(env['DBM_TEST_SQLITE_DIR']).is_dir()
        assert len(fx.containers) == 3 and all(c.startswith(prefix) for c in fx.containers)
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestLiveCreate'], env=env, cwd=ROOT, check=True)
        tmp = Path(env['DBM_TEST_CA_FILE']).parent.parent
    left = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'label=dbmanager.fixture=' + prefix, '--format', '{{.Names}}'], text=True).split()
    assert not left, left
    assert not tmp.exists(), tmp
    print('dbfix ok: containers and temp directory removed (' + prefix + ')')


if __name__ == '__main__':
    main()
