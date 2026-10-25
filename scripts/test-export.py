#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Run the export round-trip locally; --live adds disposable dbfix databases."""
import argparse
import csv
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile

from dbfix import Fixtures, ROOT


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--live', action='store_true')
    parser.add_argument('--keep', type=Path)
    args = parser.parse_args()
    env = os.environ.copy()
    # The caller may choose an existing sandbox-writable build cache.
    with tempfile.TemporaryDirectory(prefix='dbmanager-export-cache-') as cache:
        env.setdefault('GOCACHE', cache)
        dump = Path(cache) / 'responses'
        dump.mkdir()
        env['DBM_EXPORT_DUMP'] = str(dump)
        command = ['go', 'test', '-race', '-count=1', '-v', './internal/database', './internal/httpapi',
                   '-run', 'TestSQLiteExport|TestExport|TestLiveExportRoundTrip']
        if args.live:
            subprocess.run(['docker', 'ps', '--format', '{{.Names}} {{.Status}}'], check=True)
            # One disposable container at a time to spare the host.
            subprocess.run(command, cwd=ROOT, env={k: v for k, v in env.items() if not k.startswith('DBM_TEST_')}, check=True)
            for system in ('mysql', 'maria', 'postgres'):
                with Fixtures('w-p11', systems=(system,)) as fx:
                    live_env = fx.environment()
                    live_env['GOCACHE'] = env['GOCACHE']
                    live_env['DBM_EXPORT_DUMP'] = str(dump)
                    subprocess.run(command[:-1] + ['TestLiveExportRoundTrip/' + system + '$'], cwd=ROOT, env=live_env, check=True)
                left = subprocess.check_output(['docker', 'ps', '-a', '--filter',
                                               'label=dbmanager.fixture=' + fx.prefix,
                                               '--format', '{{.Names}}'], text=True).split()
                assert not left, left
        else:
            # Never consume credentials or ports belonging to another live job.
            for key in ('DBM_TEST_MYSQL_PORT', 'DBM_TEST_MARIA_PORT', 'DBM_TEST_POSTGRES_PORT'):
                env.pop(key, None)
            subprocess.run(command, cwd=ROOT, env=env, check=True)
        module_spec = importlib.util.spec_from_file_location('export_guard', ROOT / 'scripts/check-export-contract.py')
        guard = importlib.util.module_from_spec(module_spec)
        module_spec.loader.exec_module(guard)
        spec = json.loads((ROOT / 'contracts/api-v0.11.0.json').read_text())
        for filename, component in [('sqlite.json', 'ExportDocument'), ('capabilities.json', 'ExportCapabilities')]:
            value = json.loads((dump / filename).read_text())
            assert guard.validate(spec['components']['schemas'][component], value, spec), filename
            if component == 'ExportDocument':
                for table in value['tables']:
                    assert all(len(row) == len(table['columns']) for row in table['rows'])
        for filename, delimiter in [('sqlite.csv', ','), ('sqlite.excel-csv', ';')]:
            with (dump / filename).open(encoding='utf-8-sig', newline='') as stream:
                records = list(csv.reader(stream, delimiter=delimiter))
            assert records[-1][0].lstrip("'") == 'complete', filename
            assert all(len(row) == len(records[0]) for row in records), filename
        for live in sorted(dump.glob('live-*.json')):
            value = json.loads(live.read_text())
            assert guard.validate(spec['components']['schemas']['ExportDocument'], value, spec), live.name
        if args.keep:
            import shutil
            shutil.copytree(dump, args.keep, dirs_exist_ok=True)
        print('Actual JSON/capabilities schema validation and independent CSV readers passed.')
        print('Export round-trip: '  + ('SQLite/MySQL/MariaDB/PostgreSQL' if args.live else 'SQLite; live databases not requested'))


if __name__ == '__main__':
    main()
