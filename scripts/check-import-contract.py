#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard immutable v1-v12 and additive v18 import schemas and fixtures."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
module_spec = importlib.util.spec_from_file_location('objectops_guard', ROOT / 'scripts/check-object-ops-contract.py')
guard = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(guard)
SPEC = json.loads((ROOT / 'contracts/api-v0.18.0.json').read_text())
FROZEN = {'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be', 'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838', 'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308', 'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf', 'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95', 'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887', 'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17', 'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264', 'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8', 'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5', 'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620', 'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd'}
ROUTES = {'/api/v18/capabilities', '/api/v18/imports/plan', '/api/v18/imports/execute', '/api/v18/imports/{id}', '/api/v18/imports/{id}/cancel'}


def validate(component, value):
    return guard.errors(SPEC, {'$ref': '#/components/schemas/' + component}, value)


def main():
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    # Concurrent feature versions are optional in this worktree. Their absence
    # cannot weaken the required v1-v12 pins or the additive v12 baseline.
    for version in range(13, 18):
        path = ROOT / f'contracts/api-v0.{version}.0.json'
        if path.exists():
            assert json.loads(path.read_text())['info']['version'] == f'0.{version}.0'
    previous = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    assert SPEC['info']['version'] == '0.18.0'
    for name, route in previous['paths'].items():
        assert SPEC['paths'][name] == route, name
    for name, schema in previous['components']['schemas'].items():
        assert SPEC['components']['schemas'][name] == schema, name
    assert set(SPEC['paths']) - set(previous['paths']) == ROUTES
    for route in ROUTES:
        for operation in SPEC['paths'][route].values():
            assert operation['security'] == [{'bearerAuth': []}]
    for name, schema in SPEC['components']['schemas'].items():
        if name not in previous['components']['schemas'] and schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                guard.resolve(SPEC, node['$ref'])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)
    walk(SPEC)
    fixtures = list((ROOT / 'contracts/fixtures/import').glob('*.json'))
    assert len(fixtures) >= 12
    for path in fixtures:
        problems = validate(path.name.split('.')[0], json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    assert list((ROOT / 'contracts/fixtures/import').glob('*.real-*.json')), 'real SQLite fixtures required'
    print(f'Frozen v1-v12, optional v13-v17, additive v18 and {len(fixtures)} fixtures passed; SHA256 ' + hashlib.sha256((ROOT / 'contracts/api-v0.18.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
