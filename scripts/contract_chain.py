# SPDX-License-Identifier: AGPL-3.0-only
"""Shared guard for the cumulative contract chain v1..v22.

Every version N pins all predecessors v1..v(N-1) by SHA-256, carries the matching
info.version and is a strict superset (paths, schemas, security schemes) of v(N-1).
New pins are appended on integration; pinned SHAs never change.
"""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PINS = {
    1: '4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be',
    2: 'c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838',
    3: 'ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308',
    4: '4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf',
    5: 'b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95',
    6: 'b08b6aba858f72a3bad1fce2933e487eed017f5e35b4690cfd9fae3a590a6887',
    7: 'c6fd8c91315e25185d949d309397c7c0d80cc67fcab87ae521e38068e7004d17',
    8: '74aa39ed06311f2c45afc274a2412a0ac30912e3a5683ec96b812fd473a28264',
    9: '508b9951d102ec44bf4cafc9fe444f7266b31f898d9e91c887f4bef39735e3f8',
    10: '077a0e0222963b112bd6ea528ef5668d2ff7fbe2b2d3555669487c4beaf55cc5',
    11: 'b3bcc38b44b020335151aead207e281f9c54bdc7463d6fb1ddd7a6cc22396620',
    12: '7f0446edccdc368a202e4eaae5689d4b44d641e34bdf77ec8314de9f3e3f2dcd',
    13: 'e05a2402a8e6ec6a1901afeac03969e34d4fa1821e50a5c8675a6e20f5c1d3d7',
    14: 'b57c3e5dcf403c3e16d8564c68d0220453625b25f6f206fdc128d2e249656c1e',
    15: 'cd4681a200045c2fdd5ddf9f1bdc2a356c998b30e640495121925ca8a609dad1',
    16: 'd90ac3fda8295a4b8a7095c72eb459c6433fce90f729d7a6b247760d86de7bf3',
    17: 'ec2f75e2159f06fcc841c19089ba82fa9e58b998f55dfc27efcabed6771b39f3',
    18: '0d44d0c68f9d4d697eab30e60d8237db4a50663f91b512d919135a415f42375e',
    19: '98de2bbe9302506b20cbd44d50448bee841d51572fe0fd3d3c7cee76d24017ea',
    20: '6aa1a09df215d5602bc343d72bf91fd9a944a026bac5fdee9155d9cf5ffe6e8c',
    21: '52ccefab4b92fab7a2772668de74cefb437aa60f8ed60f3abb8bd9d5296fec11',
}


def spec(version):
    return json.loads((ROOT / 'contracts' / f'api-v0.{version}.0.json').read_text())


def check(version):
    """Pin v1..v(version-1) and verify the cumulative chain up to `version`."""
    for number in range(1, version):
        data = (ROOT / 'contracts' / f'api-v0.{number}.0.json').read_bytes()
        assert hashlib.sha256(data).hexdigest() == PINS[number], f'api-v0.{number}.0.json'
    for number in range(2, version + 1):
        old, new = spec(number - 1), spec(number)
        assert new['info']['version'] == f'0.{number}.0', number
        for name, value in old['paths'].items():
            assert new['paths'].get(name) == value, (number, name)
        for name, value in old['components']['schemas'].items():
            assert new['components']['schemas'].get(name) == value, (number, name)
        assert new['components'].get('securitySchemes') == old['components'].get('securitySchemes'), number
