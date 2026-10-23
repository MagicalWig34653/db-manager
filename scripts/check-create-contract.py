#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard frozen v1-v4 contracts and additive v6 structured table creation schemas."""
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
v2path=ROOT/'contracts/api-v0.2.0.json'
assert hashlib.sha256(v2path.read_bytes()).hexdigest()=='c15a848f01bb779429fd68a2b6eb6bba1575d3e07a4c68e6796cf7840a661838'
original=ROOT/'contracts/api-v0.1.0.json'
assert hashlib.sha256(original.read_bytes()).hexdigest()=='4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be'
v3path=ROOT/'contracts/api-v0.3.0.json'
assert hashlib.sha256(v3path.read_bytes()).hexdigest()=='ec77c9739cda11df3f4cb9fe756a632e67840fdc0f6190deee1aa1d1333dc308'
v4path=ROOT/'contracts/api-v0.4.0.json'
assert hashlib.sha256(v4path.read_bytes()).hexdigest()=='4763ebf8632a9ef977cf7d3a6037299fc4dafde0a581fe10cb34d844f8356fbf'
v5path=ROOT/'contracts/api-v0.5.0.json'
assert hashlib.sha256(v5path.read_bytes()).hexdigest()=='b5225339511d80fe4f52f09118fdad2060d8a92f31d44b73dd89fe9f76178c95'
v1=json.loads(v5path.read_text())
v2=json.loads((ROOT/'contracts/api-v0.6.0.json').read_text())
assert v2['info']['version']=='0.6.0'
for name,definition in v1['paths'].items():assert v2['paths'][name]==definition
for name,definition in v1['components']['schemas'].items():assert v2['components']['schemas'][name]==definition
assert set(v2['paths'])-set(v1['paths'])=={'/api/v6/capabilities','/api/v6/tables/create'}
def check(node):
    if isinstance(node,dict):
        if '$ref' in node:
            target=v2
            assert node['$ref'].startswith('#/')
            for part in node['$ref'][2:].split('/'):target=target[part.replace('~1','/').replace('~0','~')]
        for value in node.values():check(value)
    elif isinstance(node,list):
        for value in node:check(value)
check(v2)
print('Frozen v1-v5 bytes, additive v6 definitions/routes and local schema references passed.')
