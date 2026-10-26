#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive v22 contract and strict fixture schemas."""
import hashlib
import importlib.util
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / 'scripts'))
import contract_chain  # noqa: E402
module_spec = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(overview)
base_spec = importlib.util.spec_from_file_location('programs_guard', ROOT / 'scripts/check-programs-contract.py')
base = importlib.util.module_from_spec(base_spec)
base_spec.loader.exec_module(base)
SPEC = json.loads((ROOT / 'contracts/api-v0.22.0.json').read_text())


def validate(component, value):
    problems = overview.validate(component, value, SPEC)

    def extra(schema, value):
        if '$ref' in schema:
            return extra(overview.resolve(SPEC, schema['$ref']), value)
        if isinstance(value, str) and 'pattern' in schema and not re.search(schema['pattern'], value):
            problems.append('pattern mismatch')
        if isinstance(value, list):
            if len(value) < schema.get('minItems', 0):
                problems.append('too few items')
            for item in value:
                extra(schema.get('items', {}), item)
        if isinstance(value, dict):
            for key, item in value.items():
                if key in schema.get('properties', {}):
                    extra(schema['properties'][key], item)
    extra({'$ref': '#/components/schemas/' + component}, value)
    return problems


def main():
    base.main()
    contract_chain.check(22)
    previous = json.loads((ROOT / 'contracts/api-v0.21.0.json').read_text())
    assert SPEC['info']['version'] == '0.22.0'
    for name, route in previous['paths'].items():
        assert SPEC['paths'][name] == route, name
    for name, schema in previous['components']['schemas'].items():
        assert SPEC['components']['schemas'][name] == schema, name
    routes = {'/api/v22/tables/partitions/list', '/api/v22/tables/prefix/list', '/api/v22/capabilities', '/api/v22/tables/partitions/plan', '/api/v22/tables/partitions/execute', '/api/v22/tables/prefix/execute', '/api/v22/tables/prefix/plan'}
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
                overview.resolve(SPEC, node['$ref'])
            for item in node.values():
                walk(item)
        elif isinstance(node, list):
            for item in node:
                walk(item)
    walk(SPEC)
    fixtures = list((ROOT / 'contracts/fixtures/partitions').glob('*.json'))
    for path in fixtures:
        component = path.name.split('.')[0]
        problems = validate(component, json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    assert len(fixtures) >= 10
    print(f'Additive v22 routes, strict schemas and {len(fixtures)} fixtures passed; SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.22.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
