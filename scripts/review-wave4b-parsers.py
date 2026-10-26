#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Reviewer B (wave 4): independent parsers over nasty export dumps (nasty.<format>).
Usage: review-wave4b-parsers.py DIR. Compares every parser's cells with the json export."""
import base64, json, subprocess, sys, xml.etree.ElementTree as ET
from pathlib import Path
import yaml

d = Path(sys.argv[1])
ref = json.loads((d / 'nasty.json').read_text())['tables'][0]['rows']
def norm(rows):
    return [[(c['kind'], c.get('value')) if c['kind'] != 'null' else ('null', None) for c in r] for r in rows]
want = norm(ref)
res = {}
def check(name, fn):
    try:
        got = fn()
        if got == want:
            res[name] = 'OK'
        else:
            bad = [(i, w, g) for i, (w, g) in enumerate(zip(want, got)) if w != g]
            res[name] = 'MISMATCH %d/%d rows, first: %r' % (len(bad), len(want), bad[:1])
    except Exception as e:
        res[name] = 'ERROR %s: %s' % (type(e).__name__, str(e)[:200].replace('\n', ' '))

def pyyaml():
    doc = yaml.safe_load((d / 'nasty.yaml').read_text())
    assert doc['complete'] is True
    return norm(doc['tables'][0]['rows'])
def pyyaml_c():
    doc = yaml.load((d / 'nasty.yaml').read_text(), Loader=yaml.CSafeLoader)
    return norm(doc['tables'][0]['rows'])
def ruby():
    code = 'require "yaml";require "json";puts JSON.generate(YAML.safe_load(File.read(ARGV[0]))["tables"][0]["rows"])'
    out = subprocess.run(['ruby', '-e', code, str(d / 'nasty.yaml')], capture_output=True, text=True, check=True).stdout
    return norm(json.loads(out))
def xml_():
    root = ET.fromstring((d / 'nasty.xml').read_bytes())
    rows = []
    for row in root.iter('row'):
        cells = []
        for cell in row.findall('cell'):
            k = cell.find('kind').text or ''
            n = cell.find('value'); v = n.text or ''
            if n.get('encoding') == 'base64': v = base64.b64decode(v).decode()
            cells.append(('null', None) if k == 'null' else (k, v))
        rows.append(cells)
    return rows
def php():
    code = '$a=include $argv[1]; if($a["complete"]!==true) exit(3); echo json_encode($a["tables"][0]["rows"], JSON_INVALID_UTF8_SUBSTITUTE);'
    out = subprocess.run(['php', '-r', code, str(d / 'nasty.php-array')], capture_output=True, text=True, check=True).stdout
    return norm(json.loads(out))
def codegen():
    rows = [json.loads(l)['row'] for l in (d / 'nasty.codegen').read_text().split('\n') if l.startswith('{"row"')]
    return norm(rows)
for n, f in [('pyyaml-safe', pyyaml), ('pyyaml-C', pyyaml_c), ('ruby-psych', ruby), ('xml-etree', xml_), ('php-include', php), ('codegen-json', codegen)]:
    check(n, f)
x = subprocess.run(['xmllint', '--noout', str(d / 'nasty.xml')], capture_output=True, text=True)
res['xmllint'] = 'OK' if x.returncode == 0 else 'ERROR ' + x.stderr[:200]
for k, v in res.items(): print('%-14s %s' % (k, v))
