#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Re-review wave 4: compare the whole YAML export document with the JSON export
(cells, column names, table names) through PyYAML (pure and C loader) and Ruby Psych.
Usage: review-wave4-rereview-yaml.py DIR  (DIR from DBM_REREVIEW_DUMP)."""
import json, subprocess, sys
from pathlib import Path
import yaml

d = Path(sys.argv[1])
ref = json.loads((d / 'probe.json').read_text(encoding='utf-8'))
text = (d / 'probe.yaml').read_text(encoding='utf-8')
bad = 0
def report(name, fn):
    global bad
    try:
        got = fn()
        ok = got['tables'] == ref['tables'] and got['complete'] is True
        if not ok:
            for a, b in zip(ref['tables'], got['tables']):
                if a != b:
                    print(name, 'MISMATCH in table', repr(a['table']['name'])[:40]); break
    except Exception as e:
        ok = False
        print(name, 'ERROR', type(e).__name__, str(e)[:160].replace('\n', ' '))
    print('%-12s %s' % (name, 'OK' if ok else 'FAIL')); bad += (not ok)
report('pyyaml-safe', lambda: yaml.safe_load(text))
if hasattr(yaml, 'CSafeLoader'):
    report('pyyaml-C', lambda: yaml.load(text, Loader=yaml.CSafeLoader))
else:
    print('pyyaml-C     n/a (no libyaml binding)')
def ruby():
    code = 'require "yaml";require "json";puts JSON.generate(YAML.safe_load(File.read(ARGV[0], encoding: "UTF-8")))'
    p = subprocess.run(['ruby', '-e', code, str(d / 'probe.yaml')], capture_output=True, text=True)
    if p.returncode: raise RuntimeError(p.stderr[:200])
    return json.loads(p.stdout)
report('ruby-psych', ruby)
sys.exit(1 if bad else 0)
