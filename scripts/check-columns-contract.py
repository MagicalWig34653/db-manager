#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard frozen v1-v8 contract bytes and the additive v9 column schemas and fixtures.

Importable: validate(schema_ref, value) checks a value against a v9 component schema.
"""
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent
FROZEN = {
    'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be',
    'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838',
    'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308',
    'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf',
    'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95',
    'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887',
    'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17',
    'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
}
FIXTURES = ROOT / 'contracts/fixtures/columns'
NEW_SCHEMAS = {'ColumnTypeSelection','ColumnConstant','ColumnDefault','ColumnDefinition','ColumnPosition','ColumnChange','ColumnsPlanRequest','ColumnsExecuteRequest','ColumnsConfirmation','ColumnsPlan','ColumnsResult','ColumnsCapabilities'}
ROUTES = {'/api/v9/capabilities','/api/v9/columns/plan','/api/v9/columns/alter'}
INHERITED_SHA256 = 'b6d0286ed25d42c8ccef7d87d1e1ad61711a1813319a48d7333199d7d6d12a0e'


def load_v9():
    return json.loads((ROOT / 'contracts/api-v0.9.0.json').read_text())


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
    """Small validator for the OpenAPI subset the v9 schemas use."""
    out = []
    if '$ref' in schema:
        return errors(spec, resolve(spec, schema['$ref']), value, path)
    if 'oneOf' in schema and sum(not errors(spec, option, value, path) for option in schema['oneOf']) != 1:
        return [f'{path}: must match exactly one variant']
    if 'anyOf' in schema:
        if not any(not errors(spec, option, value, path) for option in schema['anyOf']):
            return [f'{path}: matches none of anyOf']
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
        if 'pattern' in schema and not re.search(schema['pattern'], value):
            out.append(f'{path}: pattern {schema["pattern"]}')
    if isinstance(value, list):
        if len(value) > schema.get('maxItems', 1 << 62):
            out.append(f'{path}: too many items')
        if len(value) < schema.get('minItems', 0):
            out.append(f'{path}: too few items')
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
    spec = spec or load_v9()
    return errors(spec, {'$ref': '#/components/schemas/' + component}, value)


def main():
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    spec = load_v9()
    assert spec['info']['version'] == '0.9.0'
    inherited = {'paths': {k:v for k,v in spec['paths'].items() if k not in ROUTES}, 'schemas': {k:v for k,v in spec['components']['schemas'].items() if k not in NEW_SCHEMAS}}
    assert hashlib.sha256(json.dumps(inherited, sort_keys=True, separators=(',', ':')).encode()).hexdigest() == INHERITED_SHA256
    v7 = json.loads((ROOT / 'contracts/api-v0.7.0.json').read_text())
    assert spec['components']['securitySchemes'] == v7['components']['securitySchemes']
    for route in ROUTES:
        for op in spec['paths'][route].values():
            assert op['security'] == [{'bearerAuth': []}]
            assert {'200','400','401','404','409','413','422','default'} <= set(op['responses'])
    schemas = spec['components']['schemas']
    plan, execute = schemas['ColumnsPlanRequest'], schemas['ColumnsExecuteRequest']
    assert set(execute['properties']) == set(plan['properties']) | {'confirm','expected_plan_sha256'}
    assert {'confirm','expected_plan_sha256'} <= set(execute['required'])
    def walk(node, strict=False):
        if isinstance(node, dict):
            if '$ref' in node:
                resolve(spec,node['$ref'])
            if strict and node.get('type') == 'object':
                assert node.get('additionalProperties') is False
            for value in node.values():
                walk(value,strict)
        elif isinstance(node,list):
            for value in node:
                walk(value,strict)
    walk(spec)
    for name in NEW_SCHEMAS:
        walk(schemas[name],True)
    checked=0
    for path in sorted(FIXTURES.glob('*.json')):
        component, _, rest = path.name.partition('.')
        problems = validate(component,json.loads(path.read_text()),spec)
        assert bool(problems) == rest.endswith('.invalid.json'), (path.name,problems)
        checked += 1
    assert checked >= 20, checked
    print(f'Frozen v1-v8 bytes, inherited v8 definitions, additive v9 routes/schemas and {checked} fixtures passed.')


if __name__ == '__main__':
    main()
