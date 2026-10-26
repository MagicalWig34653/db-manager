#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Validate frozen contracts, additive read-only v17 schemas and synthetic fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent
loader = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(loader)
loader.loader.exec_module(overview)
loader = importlib.util.spec_from_file_location('programs_guard', ROOT / 'scripts/check-programs-contract.py')
programs = importlib.util.module_from_spec(loader)
loader.loader.exec_module(programs)
FROZEN = dict(overview.FROZEN)
FROZEN.update({
    'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17',
    'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
    'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8',
    'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5',
    'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620',
    'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd',
    'api-v0.16.0.json': 'd90ac3fda8295a4b8a7095c72eb459c6433fce90f729d7a6b247760d86de7bf3',
})

def load():
    return json.loads((ROOT / 'contracts/api-v0.17.0.json').read_text())


def errors(spec, schema, value):
    if '$ref' in schema:
        return errors(spec, overview.resolve(spec, schema['$ref']), value)
    out = overview.errors(spec, schema, value)
    if isinstance(value, str) and 'pattern' in schema and not re.fullmatch(schema['pattern'], value):
        out.append('pattern mismatch')
    if isinstance(value, list):
        if len(value) < schema.get('minItems', 0):
            out.append('too few items')
        for item in value:
            out += errors(spec, schema.get('items', {}), item)
    if isinstance(value, dict):
        for name, item in value.items():
            if name in schema.get('properties', {}):
                out += errors(spec, schema['properties'][name], item)
    return out


def validate(component, value):
    spec = load()
    return errors(spec, overview.resolve(spec, '#/components/schemas/' + component), value)


def main():
    programs.main()  # freezes v1..v15 and the v16 programs guard
    spec = load()
    assert spec['info']['version'] == '0.17.0'
    for filename, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / filename).read_bytes()).hexdigest() == digest, filename
    base = json.loads((ROOT / 'contracts/api-v0.16.0.json').read_text())
    for group in ('paths',):
        for name, definition in base[group].items():
            assert spec[group][name] == definition, name
    for name, definition in base['components']['schemas'].items():
        assert spec['components']['schemas'][name] == definition, name
    routes = {'capabilities', 'status', 'variables', 'processes'}
    assert set(spec['paths']) - set(base['paths']) == {'/api/v17/' + r for r in routes}
    for route in routes:
        for op in spec['paths']['/api/v17/' + route].values():
            assert op['security'] == [{'bearerAuth': []}]
    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                overview.resolve(spec, node['$ref'])
            for child in node.values():
                walk(child)
        elif isinstance(node, list):
            for child in node:
                walk(child)
    walk(spec)
    for name, schema in spec['components']['schemas'].items():
        if name.startswith('ServerAdmin') and schema.get('type') == 'object':
            assert schema.get('additionalProperties') is False, name
    assert set(spec['components']['schemas']) - set(base['components']['schemas']) == {
        'ServerAdminReadRequest', 'ServerAdminValue', 'ServerAdminProcess',
        'ServerAdminReadResult', 'ServerAdminDriverCapabilities',
        'ServerAdminCapabilities', 'ServerAdminError',
    }
    assert set(spec['components']['schemas']['ServerAdminDriverCapabilities']['properties']) == {'status', 'variables', 'processes'}
    assert spec['components']['schemas']['ServerAdminCapabilities']['properties']['features']['items']['enum'] == ['status', 'variables', 'processes']
    count = 0
    for path in sorted((ROOT / 'contracts/fixtures/server-admin').glob('*.json')):
        problems = validate(path.name.split('.')[0], json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
        count += 1
    assert count >= 15, count
    print(f'Frozen v1-v16 bytes, additive read-only v17, references, strict schemas and {count} fixtures passed.')


if __name__ == '__main__':
    main()
