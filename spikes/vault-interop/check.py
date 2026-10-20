#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Build both synthetic codecs and check cross-language positive/negative vectors."""
import argparse
import base64
import copy
import datetime
import hashlib
import json
from pathlib import Path
import subprocess
import platform
import tempfile
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent


def run(args, cwd=None):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    return result.stdout


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()


def flip(text):
    value = bytearray(base64.b64decode(text)); value[0] ^= 1
    return base64.b64encode(value).decode()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go-binary', type=Path)
    parser.add_argument('--swift-binary', type=Path)
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    lock_root = HERE / 'swift/Vendor/argon2'
    lock = json.loads((lock_root / 'upstream-lock.json').read_text())
    for name, expected in lock['sha256'].items():
        assert hashlib.sha256((lock_root / name).read_bytes()).hexdigest() == expected, name
    with tempfile.TemporaryDirectory(prefix='dbmanager-vault-check-', dir='/tmp') as temporary:
        directory = Path(temporary)
        go = args.go_binary
        if go is None:
            run(['go', 'test', '-race', '-count=1', './...'], HERE / 'go')
            run(['go', 'vet', './...'], HERE / 'go')
            go = directory / 'go-codec'
            run(['go', 'build', '-o', str(go), './cmd/vaultspike'], HERE / 'go')
        swift = args.swift_binary
        if swift is None:
            scratch = directory / 'swift'
            run(['swift', 'test', '--scratch-path', str(scratch), '-j', '4'], HERE / 'swift')
            run(['swift', 'build', '--scratch-path', str(scratch), '-c', 'release', '-j', '4'], HERE / 'swift')
            binary_root = run(['swift', 'build', '--scratch-path', str(scratch), '-c', 'release', '--show-bin-path'], HERE / 'swift').strip()
            swift = Path(binary_root) / 'vault-spike'
        codecs = {'go': go, 'swift': swift}
        counts = {'positive': 0, 'negative': 0}
        timings = []

        def verify(codec, file, secrets, mode, expected=True, minimum=0):
            start = time.monotonic()
            result = subprocess.run([str(codecs[codec]), 'verify', str(file), str(secrets), mode, str(minimum)], capture_output=True, text=True)
            body = json.loads(result.stdout)
            accepted = result.returncode == 0 and body.get('ok') is True
            assert accepted == expected, f'{codec}: {file.name}/{mode}: unexpected acceptance={accepted}'
            counts['positive' if expected else 'negative'] += 1
            return (time.monotonic() - start) * 1000

        for writer in codecs:
            source = directory / writer
            emit_report = json.loads(run([str(codecs[writer]), 'emit', str(source)]))
            valid = source / 'vault.json'; secrets = source / 'synthetic-secrets.json'
            original = json.loads(valid.read_bytes())
            for reader in codecs:
                for mode in ('password', 'recovery'):
                    elapsed = verify(reader, valid, secrets, mode)
                    timings.append({'writer': writer, 'reader': reader, 'mode': mode, 'process_wall_ms': round(elapsed, 3)})
                    verify(reader, valid, secrets, mode, expected=False, minimum=2)

            cases = {}
            def change(name, operation):
                item = copy.deepcopy(original); operation(item); cases[name] = canonical(item)
            change('entry-removal', lambda v: v.update(entries=[]))
            change('entry-order', lambda v: v['entries'].reverse())
            change('duplicate-entry-id', lambda v: v['entries'][1].update(id=v['entries'][0]['id']))
            change('duplicate-root-nonce', lambda v: v['entries'][1].update(key_nonce=v['entries'][0]['key_nonce']))
            change('entry-context', lambda v: v['entries'][0].update(revision=2))
            change('vault-context', lambda v: v.update(vault_id='0' * 32))
            change('epoch-context', lambda v: v.update(epoch=2))
            change('generation', lambda v: v.update(generation=2))
            change('password-root', lambda v: v['password'].update(wrapped_root=flip(v['password']['wrapped_root'])))
            change('recovery-root', lambda v: v['recovery'].update(wrapped_root=flip(v['recovery']['wrapped_root'])))
            change('payload', lambda v: v['entries'][0].update(payload=flip(v['entries'][0]['payload'])))
            change('mac', lambda v: v.update(mac=flip(v['mac'])))
            change('kdf-memory', lambda v: v['password']['kdf'].update(memory_kib=2147483648))
            change('kdf-time', lambda v: v['password']['kdf'].update(iterations=4294967295))
            change('kdf-lanes', lambda v: v['password']['kdf'].update(parallelism=0))
            change('version', lambda v: v.update(version=2))
            change('null', lambda v: v.update(entries=None))
            change('unknown-property', lambda v: v.update(ignored=True))
            change('wrong-nonce-length', lambda v: v['password'].update(nonce=''))
            change('too-many-entries', lambda v: v.update(entries=[dict(copy.deepcopy(v['entries'][0]), id=f'{i:032x}') for i in range(65)]))
            change('payload-too-large', lambda v: v['entries'][0].update(payload=base64.b64encode(bytes(65553)).decode()))
            raw = valid.read_bytes()
            cases['truncated'] = raw[:-1]
            cases['whitespace'] = raw + b'\n'
            cases['duplicate-key'] = raw.replace(b'"version":1', b'"version":1,"version":1', 1)
            cases['floating-integer'] = raw.replace(b'"version":1', b'"version":1.0', 1)
            cases['deep-json'] = b'{"deep":' + b'[' * 100 + b'0' + b']' * 100 + b'}'
            cases['file-too-large'] = b'x' * (8 * 1024 * 1024 + 1)
            for name, data in cases.items():
                bad = source / (name + '.json'); bad.write_bytes(data)
                for reader in codecs:
                    for mode in ('password', 'recovery'):
                        verify(reader, bad, secrets, mode, expected=False)
            wrong = json.loads(secrets.read_text()); wrong['password'] = 'wrong-' + wrong['password']
            wrong_file = source / 'wrong-password.json'; wrong_file.write_text(json.dumps(wrong))
            for reader in codecs:
                verify(reader, valid, wrong_file, 'password', expected=False)
                verify(reader, valid, wrong_file, 'recovery')
            wrong = json.loads(secrets.read_text()); wrong['recovery'] = base64.b64encode(bytes(32)).decode()
            wrong_file.write_text(json.dumps(wrong))
            for reader in codecs:
                verify(reader, valid, wrong_file, 'recovery', expected=False)
                verify(reader, valid, wrong_file, 'password')
            timings.append({'writer': writer, 'operation': 'emit', 'codec_report_ms': emit_report['duration_ms']})

        # Committed examples are explicitly public synthetic vectors, never user material.
        for fixture in sorted((ROOT / 'contracts/fixtures/vault-v1').glob('*/vault.json')):
            for reader in codecs:
                for mode in ('password', 'recovery'):
                    verify(reader, fixture, fixture.parent / 'synthetic-secrets.json', mode)
        report = {'synthetic_only': True, 'counts': counts, 'measurements': timings,
                  'checked_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  'machine': platform.machine(), 'go_version': run(['go', 'version']).strip(),
                  'swift_version': run(['swift', '--version']).strip(),
                  'build_mode': 'Go default optimized; Swift release; process wall includes startup',
                  'argon2_reference_commit': lock['commit'],
                  'schema_sha256': hashlib.sha256((ROOT / 'contracts/vault-v1.schema.json').read_bytes()).hexdigest()}
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
