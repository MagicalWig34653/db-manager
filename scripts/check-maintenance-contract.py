#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard additive provisional maintenance schemas and handwritten fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent
loader = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(loader)
loader.loader.exec_module(overview)
FROZEN = dict(overview.FROZEN, **{'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17'})
ROUTES = {'/api/v11/capabilities', '/api/v11/tables/maintenance/plan', '/api/v11/tables/maintenance'}


def load_spec():
    return json.loads((ROOT / 'contracts/api-v0.11.0.json').read_text())


def errors(spec, schema, value, path='$'):
    if '$ref' in schema:
        return errors(spec, overview.resolve(spec, schema['$ref']), value, path)
    if 'oneOf' in schema:
        if sum(not errors(spec, branch, value, path) for branch in schema['oneOf']) != 1:
            return [f'{path}: exactly one branch required']
    # Reuse scalar validation, but recurse here so oneOf, pattern, minItems
    # and conditional branches are checked at every nesting level.
    scalar = {k: v for k, v in schema.items() if k not in ('properties', 'items', 'allOf')}
    out = overview.errors(spec, scalar, value, path)
    if isinstance(value, str) and 'pattern' in schema and re.search(schema['pattern'], value) is None:
        out.append(f'{path}: pattern mismatch')
    if isinstance(value, list):
        if len(value) < schema.get('minItems', 0):
            out.append(f'{path}: too few items')
        for i, item in enumerate(value):
            out += errors(spec, schema.get('items', {}), item, f'{path}[{i}]')
    if isinstance(value, dict):
        props = schema.get('properties', {})
        # Required and additional properties were handled by scalar validation;
        # pass declared keys as unconstrained there to avoid false positives.
        out = [e for e in out if ': unexpected ' not in e]
        for key, item in value.items():
            if key in props:
                out += errors(spec, props[key], item, f'{path}.{key}')
            elif schema.get('additionalProperties') is False:
                out.append(f'{path}: unexpected {key}')
    for rule in schema.get('allOf', []):
        if 'if' not in rule or not errors(spec, rule['if'], value, path):
            out += errors(spec, rule.get('then', rule), value, path)
        elif 'else' in rule:
            out += errors(spec, rule['else'], value, path)
    return out


def validate(component, value, spec=None):
    return errors(spec or load_spec(), {'$ref': '#/components/schemas/' + component}, value)


def main():
    for name, sha in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == sha, name
    old = overview.load_v7()
    spec = load_spec()
    assert spec['info']['version'] == '0.11.0'
    for section, current in [('paths', spec['paths']), ('schemas', spec['components']['schemas'])]:
        previous = old['paths'] if section == 'paths' else old['components']['schemas']
        assert all(current[k] == v for k, v in previous.items()), section
    assert set(spec['paths']) - set(old['paths']) == ROUTES
    for path in ROUTES:
        for operation in spec['paths'][path].values():
            assert operation['security'] == [{'bearerAuth': []}]
    def walk(value):
        if isinstance(value, dict):
            if '$ref' in value:
                overview.resolve(spec, value['$ref'])
            for child in value.values():
                walk(child)
        elif isinstance(value, list):
            for child in value:
                walk(child)
    walk(spec)
    for name, schema in spec['components']['schemas'].items():
        if name.startswith('Maintenance') and schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
    count = 0
    for path in sorted((ROOT / 'contracts/fixtures/maintenance').glob('*.json')):
        component = path.name.split('.')[0]
        problems = validate(component, json.loads(path.read_text()), spec)
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
        count += 1
    assert count >= 20
    print(f'Frozen v1-v7 bytes, additive provisional v11, references and {count} fixtures passed.')


if __name__ == '__main__':
    main()
