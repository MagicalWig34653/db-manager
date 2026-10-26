#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run the independent wave 4 review A attack tests against disposable fixtures."""
import os
import subprocess
import sys

from dbfix import Fixtures, ROOT


def main():
    failed = []
    for system in ('mysql', 'maria', 'postgres'):
        with Fixtures('rev-a', systems=(system,)) as fixture:
            env = fixture.environment()
            result = subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestReviewA_'], env={**os.environ, **env, 'DBM_REVIEW_A_FINDINGS': os.environ.get('DBM_REVIEW_A_FINDINGS', '')}, cwd=ROOT)
            if result.returncode:
                failed.append(system)
    print('FAILED: ' + ','.join(failed) if failed else 'all attack tests passed')
    sys.exit(1 if failed else 0)


if __name__ == '__main__':
    main()
