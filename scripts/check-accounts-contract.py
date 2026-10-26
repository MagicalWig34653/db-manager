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
    spec = spec or json.loads((ROOT / 'contracts/api-v0.15.0.json').read_text())
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
    for name, digest in dict(base.FROZEN, **{'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
        'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8',
        'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5',
        'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620',
        'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd',
        'api-v0.15.0.json': 'cd4681a200045c2fdd5ddf9f1bdc2a356c998b30e640495121925ca8a609dad1',
        'api-v0.13.0.json': 'e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7', 'api-v0.14.0.json': 'b57c3e5dcf403c3e16d8564c68d0220453625b25f6f206fdc128d2e249656c1e'}).items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    old = json.loads((ROOT / 'contracts/api-v0.14.0.json').read_text())
    spec = json.loads((ROOT / 'contracts/api-v0.15.0.json').read_text())
    assert spec['info']['version'] == '0.15.0'
    for key in ('paths',):
        for name, definition in old[key].items():
            assert spec[key][name] == definition, name
    for name, definition in old['components']['schemas'].items():
        assert spec['components']['schemas'][name] == definition, name
    assert old['components']['securitySchemes'] == spec['components']['securitySchemes']
    added = set(spec['paths']) - set(old['paths'])
    actions = ('create', 'rename', 'drop', 'password', 'grant', 'revoke', 'lock',
               'unlock', 'role_grant', 'role_revoke', 'copy', 'flush')
    routes = {'/api/v15/capabilities'} | {
        '/api/v15/accounts/' + action for action in ('list', 'inspect', 'access')}
    for action in actions:
        routes |= {'/api/v15/accounts/' + action, '/api/v15/accounts/' + action + '/plan'}
    assert added == routes, added ^ routes

    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                base.resolve(spec, node['$ref'])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(spec)
    for path in added:
        assert path.startswith('/api/v15/')
        assert set(spec['paths'][path]) == ({'get'} if path.endswith('/capabilities') else {'post'})
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
    print(f'Frozen v1-v14 bytes, additive v15, strict schemas and {count} placeholder fixtures passed')


if __name__ == '__main__':
    main()
