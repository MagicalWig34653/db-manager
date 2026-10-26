#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive v14 contract and synthetic fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
module_spec = importlib.util.spec_from_file_location('object_guard', ROOT / 'scripts/check-object-ops-contract.py')
base = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(base)
FROZEN = dict(base.FROZEN, **{
    'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
    'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8',
    'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5',
    'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620',
    'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd',
    'api-v0.13.0.json': 'e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7',
})
ROUTES = {'/api/v14/' + route for route in ('capabilities', 'search', 'tables/browse', 'replace/plan', 'replace')}


def load():
    return json.loads((ROOT / 'contracts/api-v0.14.0.json').read_text())


def validate(component, value):
    spec = load()
    return base.errors(spec, {'$ref': '#/components/schemas/' + component}, value)


def main():
    base.main()
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    old = json.loads((ROOT / 'contracts/api-v0.13.0.json').read_text())
    assert old['info']['version'] == '0.13.0'
    spec = load()
    assert spec['info']['version'] == '0.14.0'
    assert spec['components']['securitySchemes'] == old['components']['securitySchemes']
    for name, value in old['paths'].items():
        assert spec['paths'][name] == value, name
    for name, value in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == value, name
    assert set(spec['paths']) - set(old['paths']) == ROUTES
    for route in ROUTES:
        for op in spec['paths'][route].values():
            assert op['security'] == [{'bearerAuth': []}]
            assert {'200', '400', '401', '404', '409', '422', 'default'} <= set(op['responses'])
    new_schemas = set(spec['components']['schemas']) - set(old['components']['schemas'])
    for name in new_schemas:
        schema = spec['components']['schemas'][name]
        if schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                base.resolve(spec, node['$ref'])
            for v in node.values():
                walk(v)
        elif isinstance(node, list):
            for v in node:
                walk(v)
    walk(spec)
    for path in sorted((ROOT / 'contracts/fixtures/search').glob('*.json')):
        problems = validate(path.name.split('.')[0], json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    plan = spec['components']['schemas']['ReplaceRequest']
    execute = spec['components']['schemas']['ReplaceExecuteRequest']
    assert set(execute['required']) == set(plan['required']) | {'confirm', 'expected_plan_sha256'}
    print('Pinned v1-v13 strictly; v14 additive on v13')
    print('Additive v14 routes, strict schemas, references and search fixtures passed.')
    print('SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.14.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
