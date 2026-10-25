#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Validate additive API 0.17, frozen predecessors, strict schemas and fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
source = importlib.util.spec_from_file_location('object_guard', ROOT / 'scripts/check-object-ops-contract.py')
previous = importlib.util.module_from_spec(source)
source.loader.exec_module(previous)


def load():
    return json.loads((ROOT / 'contracts/api-v0.17.0.json').read_text())


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
    old = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    spec = load()
    frozen = {
        'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be',
        'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838',
        'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308',
        'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf',
        'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95',
        'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887',
        'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17',
        'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
        'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8',
        'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5',
        'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620',
        'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd',
    }
    for name, digest in frozen.items():
        assert hashlib.sha256((ROOT / "contracts" / name).read_bytes()).hexdigest() == digest, name
    assert spec['info']['version'] == '0.17.0'
    for key in ('paths',):
        for name, value in old[key].items():
            assert spec[key][name] == value, name
    assert spec['components']['securitySchemes'] == old['components']['securitySchemes']
    for name, value in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == value, name
    # Intermediate packages are optional in this isolated worktree. The lead
    # incorporates their additions before integration when those files exist.
    baseline_paths = set(old['paths'])
    baseline_schemas = set(old['components']['schemas'])
    for version in range(13, 17):
        path = ROOT / f'contracts/api-v0.{version}.0.json'
        if not path.exists():
            continue
        intermediate = json.loads(path.read_text())
        for name, value in intermediate['paths'].items():
            assert spec['paths'].get(name) == value, name
        for name, value in intermediate['components']['schemas'].items():
            assert spec['components']['schemas'].get(name) == value, name
        assert spec['components']['securitySchemes'] == intermediate['components']['securitySchemes']
        baseline_paths.update(intermediate['paths'])
        baseline_schemas.update(intermediate['components']['schemas'])
    routes = {'/api/v17/capabilities'} | {'/api/v17/programs/' + name for name in ('list', 'describe', 'dependencies', 'plan', 'execute')}
    assert set(spec['paths']) - baseline_paths == routes
    for path in routes:
        for op in spec['paths'][path].values():
            assert op['security'] == [{'bearerAuth': []}]
    for name in set(spec['components']['schemas']) - baseline_schemas:
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
    print(f'Programs API 0.17 additive guard passed; {len(fixtures)} fixtures')


if __name__ == '__main__':
    main()
