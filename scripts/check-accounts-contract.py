#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard additive accounts contract, strict requests, secret isolation and fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
loader = importlib.util.spec_from_file_location('object_contract', ROOT / 'scripts/check-object-ops-contract.py')
base = importlib.util.module_from_spec(loader)
loader.loader.exec_module(base)


def validate(component, value, spec=None):
    spec = spec or json.loads((ROOT / 'contracts/api-v0.18.0.json').read_text())
    schema = {'$ref': '#/components/schemas/' + component}
    return base.errors(spec, schema, value) + map_errors(spec, schema, value)


def map_errors(spec, schema, value):
    if '$ref' in schema:
        return map_errors(spec, base.resolve(spec, schema['$ref']), value)
    out = []
    if isinstance(value, dict):
        properties = schema.get('properties', {})
        extra = schema.get('additionalProperties')
        for key, item in value.items():
            if key in properties:
                out += map_errors(spec, properties[key], item)
            elif isinstance(extra, dict):
                out += base.errors(spec, extra, item, '$.' + key)
                out += map_errors(spec, extra, item)
    if isinstance(value, list):
        for item in value:
            out += map_errors(spec, schema.get('items', {}), item)
    return out


def main():
    for name, digest in dict(base.FROZEN, **{'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264'}).items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    old = base.load_v8()
    spec = json.loads((ROOT / 'contracts/api-v0.18.0.json').read_text())
    assert spec['info']['version'] == '0.18.0'
    for key in ('paths',):
        for name, definition in old[key].items():
            assert spec[key][name] == definition, name
    for name, definition in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == definition, name
    assert old['components']['securitySchemes'] == spec['components']['securitySchemes']
    added = set(spec['paths']) - set(old['paths'])
    assert len(added) == 28, added
    for path in added:
        assert path.startswith('/api/v18/')
        for operation in spec['paths'][path].values():
            assert operation['security'] == [{'bearerAuth': []}]
            assert {'200','400','403','404','409','422','default'} <= set(operation['responses'])
    schemas = spec['components']['schemas']
    for name, schema in schemas.items():
        if name.startswith('Account'):
            assert schema['additionalProperties'] is False, name
            if name.endswith('ExecuteRequest'):
                assert {'confirm','expected_plan_sha256'} <= set(schema['required'])
                if 'secret' in schema['properties']:
                    assert schema['properties']['secret']['writeOnly'] is True
                    assert 'secret' in schema['required']
            elif 'properties' in schema:
                assert 'secret' not in schema['properties'], name
    count = 0
    for path in sorted((ROOT / 'contracts/fixtures/accounts').glob('*.json')):
        component = path.name.split('.')[0]
        problems = validate(component, json.loads(path.read_text()), spec)
        assert bool(problems) == ('.invalid.' in path.name), (path.name, problems)
        count += 1
    assert count >= 16
    invalid_limits = json.loads((ROOT / 'contracts/fixtures/accounts/AccountInfo.postgres-placeholder.json').read_text())
    invalid_limits['limits']['connections'] = 'INVALID_PLACEHOLDER'
    assert validate('AccountInfo', invalid_limits, spec)
    print(f'Frozen v1-v8 bytes, additive v18, strict schemas and {count} placeholder fixtures passed')


if __name__ == '__main__':
    main()
