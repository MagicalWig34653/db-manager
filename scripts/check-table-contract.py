#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the frozen v1 contract and additive v2 schema references."""
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
original=ROOT/'contracts/api-v0.1.0.json'
assert hashlib.sha256(original.read_bytes()).hexdigest()=='4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be'
v1=json.loads(original.read_text())
v2=json.loads((ROOT/'contracts/api-v0.2.0.json').read_text())
assert v2['info']['version']=='0.2.0'
for name,definition in v1['paths'].items():assert v2['paths'][name]==definition
for name,definition in v1['components']['schemas'].items():assert v2['components']['schemas'][name]==definition
assert set(v2['paths'])-set(v1['paths'])=={'/api/v2/capabilities','/api/v2/tables/describe','/api/v2/tables/browse'}
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
print('Frozen v1 bytes, additive v2 definitions/routes and local schema references passed.')
