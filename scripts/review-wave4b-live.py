#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Reviewer B (wave 4): run the TestReviewB* live attack tests on throwaway containers, one engine at a time."""
import os
import subprocess
import sys

from dbfix import Fixtures, ROOT

for system in ('mysql', 'maria', 'postgres'):
    with Fixtures('rev-b', systems=(system,)) as fx:
        subprocess.run(['go', 'test', '-race', '-count=1', '-v', './internal/database', '-run', 'TestReviewB(29Live|17B27Live)'],
                       cwd=ROOT, env=fx.environment(), check=False)
    assert not fx.leftovers()
