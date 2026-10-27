#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard the additive v24 contract (configuration store), its strict schemas and fixtures."""
import hashlib
import importlib.util
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / 'scripts'))
import contract_chain  # noqa: E402

module_spec = importlib.util.spec_from_file_location('overview_guard', ROOT / 'scripts/check-overview-contract.py')
overview = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(overview)
SPEC = json.loads((ROOT / 'contracts/api-v0.24.0.json').read_text())
FIXTURES = ROOT / 'contracts/fixtures/config-store'
ROUTES = {
    '/api/v24/capabilities', '/api/v24/config/status', '/api/v24/config/export', '/api/v24/config/import',
    '/api/v24/config/clear/plan', '/api/v24/config/clear/execute', '/api/v24/config/{kind}', '/api/v24/config/{kind}/{id}',
}
KINDS = ['history', 'bookmarks', 'searches', 'favorites', 'recents', 'settings', 'layouts']


def validate(component, value):
    problems = overview.validate(component, value, SPEC)

    def extra(schema, value):
        if '$ref' in schema:
            return extra(overview.resolve(SPEC, schema['$ref']), value)
        if isinstance(value, str) and 'pattern' in schema and not re.search(schema['pattern'], value):
            problems.append('pattern mismatch')
        if isinstance(value, list):
            if len(value) < schema.get('minItems', 0):
                problems.append('too few items')
            for item in value:
                extra(schema.get('items', {}), item)
        if isinstance(value, dict):
            for key, item in value.items():
                if key in schema.get('properties', {}):
                    extra(schema['properties'][key], item)
    extra({'$ref': '#/components/schemas/' + component}, value)
    return problems


def main():
    contract_chain.check(24)
    previous = json.loads((ROOT / 'contracts/api-v0.23.0.json').read_text())
    assert SPEC['info']['version'] == '0.24.0'
    assert set(SPEC['paths']) - set(previous['paths']) == ROUTES
    for name in ROUTES:
        for method, operation in SPEC['paths'][name].items():
            assert operation['security'] == [{'bearerAuth': []}], (name, method)
    for name, schema in SPEC['components']['schemas'].items():
        if name not in previous['components']['schemas'] and schema.get('type') == 'object':
            assert schema['additionalProperties'] is False, name
            for key, child in schema['properties'].items():
                # Only the opaque `data` documents and the search definition are open objects.
                if child.get('type') == 'object':
                    assert key in ('data', 'definition') and child.get('additionalProperties') is True, (name, key)
    assert SPEC['components']['schemas']['ConfigKind']['enum'] == KINDS

    def walk(node):
        if isinstance(node, dict):
            if '$ref' in node:
                overview.resolve(SPEC, node['$ref'])
            for item in node.values():
                walk(item)
        elif isinstance(node, list):
            for item in node:
                walk(item)
    walk(SPEC)
    # Every contract route is registered in the Go sources, every Go route is in the contract.
    source = (ROOT / 'internal/httpapi/config.go').read_text()
    registered = set(re.findall(r'"(?:GET|POST|PUT|DELETE) (/api/v24/[^"]+)"', source))
    contract_routes = {name for name in ROUTES}
    assert registered == contract_routes, registered ^ contract_routes
    # Limits in the contract match the Go policy.
    types = (ROOT / 'internal/configstore/types.go').read_text()
    assert 'ContractVersion = "0.24.0"' in types
    assert 'var MaxOwnerBytes int64 = 32 << 20' in types
    fixtures = sorted(FIXTURES.glob('*.json'))
    for path in fixtures:
        component = path.name.split('.')[0]
        problems = validate(component, json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
    assert len(fixtures) >= 20, len(fixtures)
    real = [p for p in fixtures if '.real-' in p.name]
    assert real, 'no real fixtures'
    print(f'Additive v24 routes, strict schemas and {len(fixtures)} fixtures ({len(real)} real) passed; SHA256 '
          + hashlib.sha256((ROOT / 'contracts/api-v0.24.0.json').read_bytes()).hexdigest())


if __name__ == '__main__':
    main()
