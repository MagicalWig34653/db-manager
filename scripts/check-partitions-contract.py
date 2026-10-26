#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive v21 contract and strict fixture schemas."""
import hashlib
import importlib.util
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
module_spec = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(overview)
SPEC = json.loads((ROOT / 'contracts/api-v0.21.0.json').read_text())


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
    overview.main()
    frozen = {'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be', 'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838', 'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308', 'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf', 'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95', 'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887', 'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17', 'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264', 'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8', 'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5', 'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620', 'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd'}
    for name, digest in frozen.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    previous = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    assert SPEC['info']['version'] == '0.21.0'
    for name, route in previous['paths'].items():
        assert SPEC['paths'][name] == route, name
    for name, schema in previous['components']['schemas'].items():
        assert SPEC['components']['schemas'][name] == schema, name
    routes = {'/api/v21/tables/partitions/list', '/api/v21/tables/prefix/list', '/api/v21/capabilities', '/api/v21/tables/partitions/plan', '/api/v21/tables/partitions/execute', '/api/v21/tables/prefix/execute', '/api/v21/tables/prefix/plan'}
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
    print(f'Additive v21 routes, strict schemas and {len(fixtures)} fixtures passed; SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.21.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
