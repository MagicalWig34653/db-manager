#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Verify installed tool versions and byte-identical upstream notices."""
import hashlib
import importlib.metadata
import json
from pathlib import Path

SITE = Path(__file__).resolve().parent


def main():
    count = 0
    for record in json.loads((SITE / 'licenses/python-inventory.json').read_text()):
        distribution = importlib.metadata.distribution(record['name'])
        assert distribution.version == record['version'], record['name']
        originals = [distribution.locate_file(file) for file in distribution.files
                     if '.dist-info/' in str(file) and any(word in file.name.lower() for word in ['license', 'notice', 'copying'])]
        assert len(originals) == len(record['notices']), record['name']
        hashes = sorted(hashlib.sha256(file.read_bytes()).hexdigest() for file in originals)
        assert hashes == sorted(item['sha256'] for item in record['notices']), record['name']
        count += 1
    lock = json.loads((SITE / 'package-lock.json').read_text())
    for record in json.loads((SITE / 'licenses/npm-inventory.json').read_text()):
        root = SITE / 'node_modules' / record['name']
        installed = json.loads((root / 'package.json').read_text())
        assert installed['version'] == record['version'], record['name']
        entry = lock['packages']['node_modules/' + record['name']]
        assert entry['integrity'] == record['integrity'], record['name']
        for notice in record['notices']:
            prefix = f'licenses/npm/{record["name"]}/{record["version"]}/'
            original = root / notice['path'].removeprefix(prefix)
            assert hashlib.sha256(original.read_bytes()).hexdigest() == notice['sha256'], notice['path']
        count += 1
    print(f'{count} installed dependency versions and upstream original notices verified')


if __name__ == '__main__':
    main()
