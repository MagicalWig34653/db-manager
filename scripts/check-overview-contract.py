#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard frozen v1-v6 contract bytes and the additive v7 read-only overview schemas and fixtures.

Importable: validate(schema_ref, value) checks a value against a v7 component schema.
"""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FROZEN = {
    'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be',
    'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838',
    'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308',
    'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf',
    'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95',
    'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887',
}
V7_ROUTES = {'/api/v7/capabilities', '/api/v7/databases', '/api/v7/objects', '/api/v7/definition', '/api/v7/server-info'}
FIXTURES = ROOT / 'contracts/fixtures/overview'


def load_v7():
    return json.loads((ROOT / 'contracts/api-v0.7.0.json').read_text())


def resolve(spec, ref):
    assert ref.startswith('#/'), ref
    node = spec
    for part in ref[2:].split('/'):
        node = node[part.replace('~1', '/').replace('~0', '~')]
    return node


def _is(kind, value):
    if kind == 'null':
        return value is None
    if kind == 'boolean':
        return isinstance(value, bool)
    if kind == 'integer':
        return isinstance(value, int) and not isinstance(value, bool)
    if kind == 'number':
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    return isinstance(value, {'string': str, 'array': list, 'object': dict}[kind])


def errors(spec, schema, value, path='$'):
    """Small validator for the OpenAPI subset the v7 schemas use."""
    out = []
    if '$ref' in schema:
        return errors(spec, resolve(spec, schema['$ref']), value, path)
    kinds = schema.get('type')
    if kinds is not None:
        kinds = kinds if isinstance(kinds, list) else [kinds]
        if not any(_is(k, value) for k in kinds):
            return [f'{path}: type {kinds} expected, got {type(value).__name__}']
    if 'const' in schema and value != schema['const']:
        out.append(f'{path}: const {schema["const"]!r}')
    if 'enum' in schema and value not in schema['enum']:
        out.append(f'{path}: not in enum {schema["enum"]}')
    if isinstance(value, bool):
        return out
    if isinstance(value, (int, float)):
        if 'minimum' in schema and value < schema['minimum']:
            out.append(f'{path}: below minimum')
        if 'maximum' in schema and value > schema['maximum']:
            out.append(f'{path}: above maximum')
    if isinstance(value, str):
        if len(value) < schema.get('minLength', 0):
            out.append(f'{path}: too short')
        if len(value) > schema.get('maxLength', 1 << 62):
            out.append(f'{path}: too long')
    if isinstance(value, list):
        if len(value) > schema.get('maxItems', 1 << 62):
            out.append(f'{path}: too many items')
        for index, item in enumerate(value):
            out += errors(spec, schema.get('items', {}), item, f'{path}[{index}]')
    if isinstance(value, dict):
        props = schema.get('properties', {})
        for key in schema.get('required', []):
            if key not in value:
                out.append(f'{path}: missing {key}')
        for key, item in value.items():
            if key in props:
                out += errors(spec, props[key], item, f'{path}.{key}')
            elif schema.get('additionalProperties') is False:
                out.append(f'{path}: unexpected {key}')
        for rule in schema.get('allOf', []):
            if_ = rule.get('if')
            if if_ is None or not errors(spec, {'type': 'object', 'properties': if_['properties']}, value, path):
                if 'then' in rule:
                    out += errors(spec, rule['then'], value, path)
    return out


def validate(component, value, spec=None):
    spec = spec or load_v7()
    return errors(spec, {'$ref': '#/components/schemas/' + component}, value)


def main():
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    v6 = json.loads((ROOT / 'contracts/api-v0.6.0.json').read_text())
    v7 = load_v7()
    assert v7['info']['version'] == '0.7.0'
    for name, definition in v6['paths'].items():
        assert v7['paths'][name] == definition, name
    for name, definition in v6['components']['schemas'].items():
        assert v7['components']['schemas'][name] == definition, name
    assert set(v7['paths']) - set(v6['paths']) == V7_ROUTES
    # Every route is Bearer-authenticated; every reference resolves locally.
    for route in V7_ROUTES:
        for operation in v7['paths'][route].values():
            assert operation['security'] == [{'bearerAuth': []}], route

    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                resolve(v7, node['$ref'])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(v7)
    # Fixtures: <component>.<name>.json valid; <component>.<name>.invalid.json must be rejected.
    checked = 0
    for path in sorted(FIXTURES.glob('*.json')):
        component, _, rest = path.name.partition('.')
        value = json.loads(path.read_text())
        problems = validate(component, value, v7)
        if rest.endswith('.invalid.json'):
            assert problems, f'{path.name} must be rejected'
        else:
            assert not problems, f'{path.name}: {problems}'
        checked += 1
    assert checked >= 20, checked
    print(f'Frozen v1-v6 bytes, additive v7 routes/schemas, local references and {checked} fixtures passed.')


if __name__ == '__main__':
    main()
