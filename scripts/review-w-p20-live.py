#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Independent W-P20 review: attack tests against a disposable MariaDB 12.3 (own containers only)."""
import subprocess
import sys

from dbfix import Fixtures, PASSWORD, ROOT

with Fixtures('w-p20-rv', systems=('maria',)) as fx:
    env = fx.environment()
    env.update({'DBM_TEST_STATE_PORT': fx.ports['maria'], 'DBM_TEST_STATE_CA_FILE': str(fx.fixture_dir / 'ca.pem'),
                'DBM_TEST_STATE_ROOT_PASSWORD': PASSWORD})
    result = subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/serverstate', '-run', 'TestReviewLive|TestRereviewLive'], cwd=ROOT, env=env,
                            text=True, capture_output=True)
    print(result.stdout[-9000:], result.stderr[-2000:])
assert not fx.leftovers()
sys.exit(result.returncode)
