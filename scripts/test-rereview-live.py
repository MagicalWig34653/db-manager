#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Re-review wave 4: run the TestReReview_/TestReviewA_ live tests on throw-away containers,
one engine after the other. Optional argument: -run regex; env MUTATE=file|old|new applies one
mutation to the working tree for the duration of the run (restored afterwards)."""
import os, subprocess, sys
from dbfix import Fixtures, ROOT
run = sys.argv[1] if len(sys.argv) > 1 else 'TestReReview_|TestReviewA_'
mut = os.environ.get('MUTATE')
if mut:
    rel, old, new = mut.split('|')
    p = ROOT / rel; src = p.read_text(); assert src.count(old) == 1; p.write_text(src.replace(old, new))
rc = 0
try:
    for system in ('mysql', 'maria', 'postgres'):
        with Fixtures('rereview', systems=(system,)) as fx:
            r = subprocess.run(['go', 'test', '-count=' + os.environ.get('COUNT','1'), '-v', '-run', run, './internal/database'], cwd=ROOT, env=fx.environment())
            print(system, 'exit', r.returncode); rc |= r.returncode
        assert not fx.leftovers()
finally:
    if mut:
        subprocess.run(['git', 'checkout', '--', rel], cwd=ROOT)
sys.exit(rc)
