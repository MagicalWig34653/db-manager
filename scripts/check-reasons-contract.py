#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Guard API 0.23: v22 plus the AccountResult.reason enum extension; fixtures."""
import importlib.util
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / 'scripts'))
import contract_chain  # noqa: E402

source = importlib.util.spec_from_file_location('object_guard', ROOT / 'scripts/check-object-ops-contract.py')
guard = importlib.util.module_from_spec(source)
source.loader.exec_module(guard)


def main():
    contract_chain.check(23)
    old, new = contract_chain.spec(22), contract_chain.spec(23)
    assert new['info']['version'] == '0.23.0'
    assert set(new['paths']) == set(old['paths'])
    changed = [n for n in new['components']['schemas'] if new['components']['schemas'][n] != old['components']['schemas'].get(n)]
    assert changed == ['AccountResult'], changed
    reasons = new['components']['schemas']['AccountResult']['properties']['reason']['enum']
    assert reasons == old['components']['schemas']['AccountResult']['properties']['reason']['enum'] + ['grant_without_effect']
    count = 0
    for path in sorted((ROOT / 'contracts/fixtures/reasons').glob('*.json')):
        problems = guard.errors(new, {'$ref': '#/components/schemas/' + path.name.split('.')[0]}, json.loads(path.read_text()))
        assert bool(problems) == path.name.endswith('.invalid.json'), (path.name, problems)
        count += 1
    assert count >= 2
    print(f'Frozen v1-v22 bytes, cumulative v23 (AccountResult.reason += grant_without_effect), {count} fixtures passed.')


if __name__ == '__main__':
    main()
