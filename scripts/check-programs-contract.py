#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Validate additive API 0.19, frozen predecessors, strict schemas and fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
source = importlib.util.spec_from_file_location('object_guard', ROOT / 'scripts/check-object-ops-contract.py')
previous = importlib.util.module_from_spec(source)
source.loader.exec_module(previous)


def load():
    return json.loads((ROOT / 'contracts/api-v0.19.0.json').read_text())


def errors(spec, schema, value, path='$'):
    # Extend the existing validator with JSON Schema's not constraint, recursively.
    if '$ref' in schema:
        return errors(spec, previous.resolve(spec, schema['$ref']), value, path)
    result = []
    flat = {k: v for k, v in schema.items() if k not in ('anyOf', 'allOf', 'not', 'properties', 'items')}
    if 'properties' in schema:
        flat['properties'] = {k: {} for k in schema['properties']}
    if 'items' in schema:
        flat['items'] = {}
    result += previous.errors(spec, flat, value, path)
    if 'not' in schema and not errors(spec, schema['not'], value, path):
        result.append(path + ': forbidden schema matched')
    if 'anyOf' in schema and not any(not errors(spec, option, value, path) for option in schema['anyOf']):
        result.append(path + ': no anyOf matched')
    for option in schema.get('allOf', []):
        result += errors(spec, option, value, path)
    if isinstance(value, dict):
        for key, child in schema.get('properties', {}).items():
            if key in value:
                result += errors(spec, child, value[key], path + '.' + key)
    if isinstance(value, list):
        for index, child in enumerate(value):
            result += errors(spec, schema.get('items', {}), child, path + '[' + str(index) + ']')
    return result


def validate(component, value, spec=None):
    return errors(spec or load(), {'$ref': '#/components/schemas/' + component}, value)


def main():
    previous.main()
    old = previous.load_v8()
    spec = load()
    assert hashlib.sha256((ROOT / 'contracts/api-v0.8.0.json').read_bytes()).hexdigest() == '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264'
    assert spec['info']['version'] == '0.19.0'
    for key in ('paths',):
        for name, value in old[key].items():
            assert spec[key][name] == value, name
    assert spec['components']['securitySchemes'] == old['components']['securitySchemes']
    for name, value in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == value, name
    routes = {'/api/v19/capabilities'} | {'/api/v19/programs/' + name for name in ('list', 'describe', 'dependencies', 'plan', 'execute')}
    assert set(spec['paths']) - set(old['paths']) == routes
    for path in routes:
        for op in spec['paths'][path].values():
            assert op['security'] == [{'bearerAuth': []}]
    for name in set(spec['components']['schemas']) - set(old['components']['schemas']):
        item = spec['components']['schemas'][name]
        if item.get('type') == 'object':
            assert item['additionalProperties'] is False, name
    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                previous.resolve(spec, node['$ref'])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(spec)
    fixtures = list((ROOT / 'contracts/fixtures/programs').glob('*.json'))
    assert len(fixtures) >= 12
    for path in fixtures:
        problems = validate(path.name.split('.')[0], json.loads(path.read_text()), spec)
        assert bool(problems) == ('.invalid.' in path.name), (path.name, problems)
    # Self checks ensure nested unknown fields and branch requirements are enforced.
    assert validate('ProgramChangeRequest', {'connection': {'driver': 'sqlite', 'database': 'placeholder'}, 'object': {'kind': 'view', 'name': 'v'}, 'action': 'create'})
    print(f'Programs API 0.19 additive guard passed; {len(fixtures)} fixtures')


if __name__ == '__main__':
    main()
