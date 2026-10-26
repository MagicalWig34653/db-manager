#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Verify frozen v1-v12, optional intermediates and the additive v21 export."""
import hashlib
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FROZEN = {'api-v0.1.0.json': '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be', 'api-v0.2.0.json': 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838', 'api-v0.3.0.json': 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308', 'api-v0.4.0.json': '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf', 'api-v0.5.0.json': 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95', 'api-v0.6.0.json': 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887', 'api-v0.7.0.json': 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17', 'api-v0.8.0.json': '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264', 'api-v0.9.0.json': '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8', 'api-v0.10.0.json': '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5', 'api-v0.11.0.json': 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620', 'api-v0.12.0.json': '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd'}
module = importlib.util.spec_from_file_location('export_guard', ROOT / 'scripts/check-export-contract.py')
guard = importlib.util.module_from_spec(module)
module.loader.exec_module(guard)


def main():
    for name, digest in FROZEN.items():
        assert hashlib.sha256((ROOT / 'contracts' / name).read_bytes()).hexdigest() == digest, name
    spec = json.loads((ROOT / 'contracts/api-v0.21.0.json').read_text())
    old = json.loads((ROOT / 'contracts/api-v0.12.0.json').read_text())
    assert spec['info']['version'] == '0.21.0'
    for section, key in [('paths', None), ('components', 'schemas')]:
        previous = old[section] if key is None else old[section][key]
        current = spec[section] if key is None else spec[section][key]
        for name, definition in previous.items():
            assert current[name] == definition, name
    assert set(spec['paths']) - set(old['paths']) == {'/api/v21/export', '/api/v21/capabilities'}
    for version in range(13, 22):
        path = ROOT / f'contracts/api-v0.{version}.0.json'
        if path.exists():
            candidate = json.loads(path.read_text())
            assert candidate['info']['version'] == f'0.{version}.0'
            assert '/api/v21/export' not in candidate['paths']
    for name, schema in spec['components']['schemas'].items():
        if name not in old['components']['schemas'] and schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
    for path in ('/api/v21/export', '/api/v21/capabilities'):
        for operation in spec['paths'][path].values():
            assert operation['security'] == [{'bearerAuth': []}]
    def walk(value):
        if isinstance(value, dict):
            if '$ref' in value:
                guard.base.resolve(spec, value['$ref'])
            for item in value.values():
                walk(item)
        elif isinstance(value, list):
            for item in value:
                walk(item)
    walk(spec)
    fixtures = list((ROOT / 'contracts/fixtures/extended-export').glob('*.json'))
    assert len(fixtures) >= 8
    for path in fixtures:
        name = path.name.split('.')[0]
        valid = guard.validate(spec['components']['schemas'][name], json.loads(path.read_text()), spec)
        assert valid != path.name.endswith('.invalid.json'), path.name
    print(f'Frozen v1-v12; additive v21; strict schemas; {len(fixtures)} placeholder fixtures passed.')


if __name__ == '__main__':
    main()
