#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive v13 contract and strict fixture schemas."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
module_spec = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(overview)
SPEC = json.loads((ROOT / 'contracts/api-v0.13.0.json').read_text())


column_spec = importlib.util.spec_from_file_location('columns_guard', ROOT / 'scripts/check-columns-contract.py')
columns = importlib.util.module_from_spec(column_spec)
column_spec.loader.exec_module(columns)


def validate(component, value):
    return columns.errors(SPEC, {'$ref': '#/components/schemas/' + component}, value)


def main():
    overview.main()
    frozen = {'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17', 'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264', 'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8', 'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5', 'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620', 'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd'}
    for name, digest in frozen.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    previous = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    assert SPEC['info']['version'] == '0.13.0'
    for name, route in previous['paths'].items():
        assert SPEC['paths'][name] == route, name
    for name, schema in previous['components']['schemas'].items():
        assert SPEC['components']['schemas'][name] == schema, name
    routes = {'/api/v13/capabilities', '/api/v13/indexes/plan', '/api/v13/indexes/alter'}
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
    # Every object in a new schema, including action/result variants, is strict.
    def strict(node):
        if isinstance(node, dict):
            if node.get('type') == 'object':
                assert node.get('additionalProperties') is False
            for value in node.values():
                strict(value)
        elif isinstance(node, list):
            for value in node:
                strict(value)
    for name in set(SPEC['components']['schemas']) - set(previous['components']['schemas']):
        strict(SPEC['components']['schemas'][name])
    for name in ('IndexesPlanRequest', 'IndexesExecuteRequest'):
        variants = SPEC['components']['schemas'][name]['oneOf']
        assert len(variants) == 9
        for variant in variants:
            required = set(variant['required'])
            assert ('confirm' in required) == (name == 'IndexesExecuteRequest')
            assert ('expected_plan_sha256' in required) == (name == 'IndexesExecuteRequest')
    fixtures = list((ROOT / 'contracts/fixtures/indexes').glob('*.json'))
    for path in fixtures:
        component = path.name.split('.')[0]
        problems = validate(component, json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    assert len(fixtures) >= 17
    print(f'Additive v13 routes, strict schemas and {len(fixtures)} fixtures passed; SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.13.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
