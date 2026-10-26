#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Independently parse real W-P28 output files (php, PyYAML, ElementTree, json, html)
and compare their typed cells with the CodeGen JSON-lines of the same export.

Usage: check-extended-export-files.py DIR   (files named placeholder-<driver>.<format>)
"""
import base64
import html
import json
from pathlib import Path
import re
import subprocess
import sys
import xml.etree.ElementTree as ET

import yaml

SUFFIX = {'xml': '</export>\n', 'yaml': '}\n', 'latex': '% DBMANAGER EXPORT COMPLETE\n',
          'latex-longtable': '% DBMANAGER EXPORT COMPLETE\n',
          'mediawiki': '<!-- DBMANAGER EXPORT COMPLETE -->\n', 'texy': '<!-- DBMANAGER EXPORT COMPLETE -->\n',
          'php-array': ');\n', 'codegen': '{"complete":true}\n'}


def cells_codegen(text):
    lines = [json.loads(line) for line in text.splitlines()]
    assert lines[-1] == {'complete': True}
    return [row['row'] for row in lines if 'row' in row]


def cells_yaml(text):
    doc = yaml.safe_load(text)
    assert doc['complete'] is True and doc == json.loads(text)
    return doc['tables'][0]['rows']


def cells_xml(text):
    root = ET.fromstring(text.encode())
    assert root.find('complete').text == 'true'
    out = []
    for row in root.iter('row'):
        cells = []
        for cell in row.findall('cell'):
            kind = cell.find('kind').text or ''
            node = cell.find('value')
            value = node.text or ''
            if node.get('encoding') == 'base64':
                value = base64.b64decode(value).decode()
            cells.append({'kind': kind} if kind == 'null' else {'kind': kind, 'value': value})
        out.append(cells)
    return out


def cells_php(path):
    code = ('$a=include $argv[1]; if($a["complete"]!==true) exit(3);'
            'echo json_encode($a["tables"][0]["rows"], JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES);')
    result = subprocess.run(['php', '-r', code, str(path)], capture_output=True, text=True, check=True)
    rows = json.loads(result.stdout)
    return [[{'kind': c['kind']} if c['kind'] == 'null' else c for c in row] for row in rows]


def decode_visible(value):
    return json.loads(re.sub(r'\\textbackslash\{\}', r'\\', re.sub(r'\\([{}$&#%_^~])', r'\1', value)))


def cells_latex(text):
    rows = []
    for line in text.splitlines():
        if line.startswith(('%', '\\begin', '\\end')) or not line.endswith(' \\\\'):
            continue
        parts = re.split(r' & ', line[:-3])
        rows.append(parts)
    return rows[1:]  # first row is the header


def cells_wiki(text, sep, texy):
    rows = []
    for line in text.splitlines():
        if texy and line.startswith('|') and line.endswith('|'):
            rows.append(line[2:-2].split(' | '))
        elif not texy and line.startswith('| '):
            rows.append(line[2:].split(' || '))
    return rows[1:]


def main(directory):
    checked = 0
    for codegen in sorted(Path(directory).glob('placeholder-*.codegen')):
        driver = codegen.name.split('-', 1)[1].rsplit('.', 1)[0]
        base = Path(directory) / f'placeholder-{driver}'
        want = cells_codegen(codegen.read_text())
        assert want, driver
        for fmt in SUFFIX:
            path = base.with_suffix('.' + fmt)
            raw = path.read_text(encoding='utf-8')
            assert raw.endswith(SUFFIX[fmt]), (driver, fmt, raw[-40:])
            if fmt == 'codegen':
                got = want
            elif fmt == 'yaml':
                got = cells_yaml(raw)
            elif fmt == 'xml':
                got = cells_xml(raw)
            elif fmt == 'php-array':
                got = cells_php(path)
            else:
                if fmt.startswith('latex'):
                    visible = [[decode_visible(c) for c in r] for r in cells_latex(raw)]
                else:
                    visible = [[json.loads(html.unescape(c)) for c in r]
                               for r in cells_wiki(raw, None, fmt == 'texy')]
                got = visible
            assert got == want, (driver, fmt, got, want)
            checked += 1
    assert checked, 'no files'
    print(f'Independent parsers confirmed {checked} output files.')


if __name__ == '__main__':
    main(sys.argv[1])
