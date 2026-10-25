#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Exercise pinned predecessors and optional parallel v13 reconciliation offline."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import tempfile

ROOT = Path(__file__).resolve().parent.parent


def main():
    module_spec = importlib.util.spec_from_file_location('search_guard', ROOT / 'scripts/check-search-contract.py')
    guard = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(guard)
    with tempfile.TemporaryDirectory(prefix='dbmanager-search-guard-') as scratch:
        guard.ROOT = Path(scratch)
        shutil.copytree(ROOT / 'contracts', guard.ROOT / 'contracts')
        contracts = guard.ROOT / 'contracts'
        predecessor = contracts / 'api-v0.13.0.json'
        # Always exercise absence, even after the integrating lead supplies v13.
        predecessor.unlink(missing_ok=True)
        baseline = json.loads((contracts / 'api-v0.12.0.json').read_text())
        current = json.loads((contracts / 'api-v0.14.0.json').read_text())
        # Use only v12 + this package's definitions for this isolated test.
        current['paths'] = {k: v for k, v in current['paths'].items() if k in baseline['paths'] or k in guard.ROUTES}
        current['components']['schemas'] = {k: v for k, v in current['components']['schemas'].items()
                                           if k in baseline['components']['schemas'] or k.startswith(('Search', 'Replace', 'Extended'))}
        target = contracts / 'api-v0.14.0.json'
        target.write_text(json.dumps(current))

        def check(accepted):
            try:
                with contextlib.redirect_stdout(io.StringIO()):
                    guard.main()
            except (AssertionError, KeyError):
                assert not accepted, 'valid predecessor configuration rejected'
            else:
                assert accepted, 'invalid predecessor configuration accepted'

        check(True)
        optional = json.loads(json.dumps(baseline))
        optional['info']['version'] = '0.13.0'
        route = '/api/v13/synthetic-guard-probe'
        optional['paths'][route] = baseline['paths']['/api/v12/capabilities']
        predecessor.write_text(json.dumps(optional))
        check(False)
        current['paths'][route] = optional['paths'][route]
        target.write_text(json.dumps(current))
        check(True)
        # A parallel predecessor must preserve v12 as well.
        optional['paths']['/api/v12/capabilities'] = {}
        predecessor.write_text(json.dumps(optional))
        check(False)
        predecessor.unlink()
        del current['paths'][route]
        target.write_text(json.dumps(current))
        pinned = contracts / 'api-v0.12.0.json'
        pinned.write_bytes(pinned.read_bytes() + b'\n')
        check(False)
    print('Search guard: optional v13 absence/presence, reconciliation and frozen-byte rejection passed.')


if __name__ == '__main__':
    main()
