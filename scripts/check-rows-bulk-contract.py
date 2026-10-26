#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Check v19 additions, frozen v1..v12 and marked/real W-P13 fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location('sqlbatch_guard', ROOT / 'scripts/check-sqlbatch-contract.py')
previous_guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(previous_guard)
SPEC = json.loads((ROOT / 'contracts/api-v0.19.0.json').read_text())
FROZEN_V12 = '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd'


def validate(component, value):
    original = previous_guard.SPEC
    previous_guard.SPEC = SPEC
    try:
        return previous_guard.validate(component, value)
    finally:
        previous_guard.SPEC = original


def main():
    previous_guard.main()
    path = ROOT / 'contracts/api-v0.12.0.json'
    assert hashlib.sha256(path.read_bytes()).hexdigest() == FROZEN_V12
    previous = json.loads(path.read_text())
    assert SPEC['info']['version'] == '0.19.0'
    # Versions 13..18 belong to other packages and need not exist in this worktree.
    for version in range(13, 19):
        optional = ROOT / f'contracts/api-v0.{version}.0.json'
        if optional.exists():
            json.loads(optional.read_text())
    for name, route in previous['paths'].items():
        assert SPEC['paths'][name] == route, name
    for name, schema in previous['components']['schemas'].items():
        assert SPEC['components']['schemas'][name] == schema, name
    routes = {'/api/v19/' + route for route in ('capabilities', 'rows/plan', 'rows/execute', 'values/upload', 'values/download', 'foreign-keys/lookup')}
    assert set(SPEC['paths']) - set(previous['paths']) == routes
    for route in routes:
        for operation in SPEC['paths'][route].values():
            assert operation['security'] == [{'bearerAuth': []}]
    for name, schema in SPEC['components']['schemas'].items():
        if name not in previous['components']['schemas'] and schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                previous_guard.overview.resolve(SPEC, node['$ref'])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(SPEC)
    fixtures = list((ROOT / 'contracts/fixtures/rows-bulk').glob('*.json'))
    for path in fixtures:
        assert '.real-' in path.name or '-placeholder' in path.name or '.invalid.' in path.name, path
        problems = validate(path.name.split('.')[0], json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    assert len(fixtures) >= 10
    print(f'Additive API 0.19: {len(fixtures)} fixtures passed; SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.19.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
