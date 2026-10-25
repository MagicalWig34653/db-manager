#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive provisional v15 contract and synthetic fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
module_spec = importlib.util.spec_from_file_location('object_guard', ROOT / 'scripts/check-object-ops-contract.py')
base = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(base)
ROUTES = {'/api/v15/' + route for route in ('capabilities', 'search', 'tables/browse', 'replace/plan', 'replace')}


def load():
    return json.loads((ROOT / 'contracts/api-v0.15.0.json').read_text())


def validate(component, value):
    spec = load()
    return base.errors(spec, {'$ref': '#/components/schemas/' + component}, value)


def main():
    base.main()
    assert hashlib.sha256((ROOT / "contracts/api-v0.8.0.json").read_bytes()).hexdigest() == "74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264"
    old = base.load_v8()
    spec = load()
    assert spec['info']['version'] == '0.15.0'
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
    print('Additive v15 routes, strict schemas, references and search fixtures passed.')
    print('SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.15.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
