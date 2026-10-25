#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard frozen predecessors, additive v12 schemas and synthetic fixtures."""
import csv
import hashlib
import importlib.util
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent
base_spec = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
base = importlib.util.module_from_spec(base_spec)
base_spec.loader.exec_module(base)
FROZEN = dict(base.FROZEN, **{'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17'})


def validate(schema, value, spec):
    """Validate the JSON Schema subset used by this contract, without downloads."""
    if isinstance(schema, bool):
        return schema
    if '$ref' in schema:
        return validate(base.resolve(spec, schema['$ref']), value, spec)
    kind = schema.get('type')
    if kind and not any(base._is(k, value) for k in (kind if isinstance(kind, list) else [kind])):
        return False
    if 'const' in schema and value != schema['const']:
        return False
    if 'enum' in schema and value not in schema['enum']:
        return False
    if 'not' in schema and validate(schema['not'], value, spec):
        return False
    if 'allOf' in schema and not all(validate(s, value, spec) for s in schema['allOf']):
        return False
    if 'anyOf' in schema and not any(validate(s, value, spec) for s in schema['anyOf']):
        return False
    if 'oneOf' in schema and sum(validate(s, value, spec) for s in schema['oneOf']) != 1:
        return False
    if 'if' in schema:
        branch = 'then' if validate(schema['if'], value, spec) else 'else'
        if not validate(schema.get(branch, {}), value, spec):
            return False
    if isinstance(value, dict):
        if any(k not in value for k in schema.get('required', [])):
            return False
        props = schema.get('properties', {})
        if schema.get('additionalProperties') is False and any(k not in props for k in value):
            return False
        if any(not validate(props[k], v, spec) for k, v in value.items() if k in props):
            return False
    elif isinstance(value, list):
        if not schema.get('minItems', 0) <= len(value) <= schema.get('maxItems', float('inf')):
            return False
        if schema.get('uniqueItems') and len({json.dumps(v, sort_keys=True) for v in value}) != len(value):
            return False
        if any(not validate(schema.get('items', {}), v, spec) for v in value):
            return False
    elif isinstance(value, str):
        if not schema.get('minLength', 0) <= len(value) <= schema.get('maxLength', float('inf')):
            return False
        if 'pattern' in schema and not re.search(schema['pattern'], value):
            return False
    elif isinstance(value, (int, float)) and not isinstance(value, bool):
        if value < schema.get('minimum', -float('inf')) or value > schema.get('maximum', float('inf')):
            return False
    return True


def main():
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    old = json.loads((ROOT / 'contracts/api-v0.7.0.json').read_text())
    spec = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    assert spec['info']['version'] == '0.12.0'
    for name, definition in old['paths'].items():
        assert spec['paths'][name] == definition, name
    for name, definition in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == definition, name
    routes = set(spec['paths']) - set(old['paths'])
    assert routes == {'/api/v12/export', '/api/v12/capabilities'}
    for route in routes:
        for operation in spec['paths'][route].values():
            assert operation['security'] == [{'bearerAuth': []}]

    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                base.resolve(spec, node['$ref'])
            for item in node.values():
                walk(item)
        elif isinstance(node, list):
            for item in node:
                walk(item)
    walk(spec)
    for name, schema in spec['components']['schemas'].items():
        if name not in old['components']['schemas'] and schema.get('type') == 'object':
            assert schema.get('additionalProperties') is False, name
    format_spec = json.loads((ROOT / 'contracts/export-format-v1.schema.json').read_text())
    for name, definition in format_spec['$defs'].items():
        expected = json.loads(json.dumps(spec['components']['schemas'][name]).replace('#/components/schemas/', '#/$defs/'))
        assert definition == expected, name
    fixtures = ROOT / 'contracts/fixtures/export'
    checked = 0
    for path in sorted(fixtures.glob('*.json')):
        component = path.name.split('.')[0]
        result = validate(spec['components']['schemas'][component], json.loads(path.read_text()), spec)
        assert result != path.name.endswith('.invalid.json'), path.name
        if component in ('ExportDocument', 'ResultSnapshot'):
            assert validate(format_spec, json.loads(path.read_text()), format_spec) == result, path.name
        checked += 1
    assert checked >= 12
    for path, delimiter in [(fixtures / 'sample.csv', ','), (fixtures / 'sample-excel.csv', ';')]:
        with path.open(encoding='utf-8-sig', newline='') as stream:
            records = list(csv.reader(stream, delimiter=delimiter))
        assert records[0][0].lstrip("'") == '_record'
        assert records[-1][0].lstrip("'") == 'complete'
        assert all(len(row) == len(records[0]) for row in records)
    assert (fixtures / 'sample.sql').read_text().endswith('-- DBMANAGER EXPORT COMPLETE\n')
    lock = json.loads((ROOT / 'web-build.lock.json').read_text())
    assert 'api-v0.12.0.json' not in json.dumps(lock), 'provisional contract must not enter the web lock'
    print(f'Frozen v1-v7 bytes; additive provisional v12; strict schemas; {checked} JSON and 3 stream fixtures passed.')


if __name__ == '__main__':
    main()
