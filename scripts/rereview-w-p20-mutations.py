#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Independent W-P20 re-review: mutation probes for the review follow-ups (versioned AEAD context,
keyed index hashes, secret rules, owner resolver, quota eviction, migration check, duplicate keys).
Each mutation edits one source line, runs the tests that must notice it and reverts via git.
Live mutations share one disposable MariaDB 12.3 (own containers only).

Usage: scripts/rereview-w-p20-mutations.py [N01 N05 ...]"""
import subprocess
import sys

from dbfix import Fixtures, PASSWORD, ROOT

CS, HT, SS = 'internal/configstore/', 'internal/httpapi/config.go', 'internal/serverstate/configrepo.go'
# (id, file, old, new, kind) kind: unit | live | guard
MUTATIONS = [
    ('N01 version not in AEAD context', SS, '+ nkey + "|" + strconv.FormatInt(version, 10))', '+ nkey)', 'live'),
    ('N02 update seals the old version', SS, '\t\trow.Version++\n', '\t\trow.Version += 0\n', 'live'),
    ('N03 insert seals version 2', SS, '\trow.Version = 1\n\tsealed, err := r.seal(owner, row)', '\trow.Version = 2\n\tsealed, err := r.seal(owner, row)', 'live'),
    ('N04 update bumps version by 2', SS, 'version = version + 1, updated_at', 'version = version + 2, updated_at', 'live'),
    ('N05 index key replaced instead of ignored', SS, 'INSERT IGNORE INTO config_meta', 'REPLACE INTO config_meta', 'live'),
    ('N06 index key 8 bytes', SS, 'fresh := make([]byte, 32)', 'fresh := make([]byte, 8)', 'live'),
    ('N07 scope hash without owner salt', CS + 'service.go', 'macHex(index, "scope", owner, scope)', 'macHex(index, "scope", scope)', 'unit'),
    ('N08 MAC without key', CS + 'validate.go', 'hmac.New(sha256.New, key)', 'hmac.New(sha256.New, nil)', 'unit'),
    ('N09 confusables not applied', CS + 'secrets.go', 'if ascii, ok := confusables[r]; ok {', 'if ascii, ok := confusables[r]; ok && false {', 'unit'),
    ('N10 combining marks kept', CS + 'secrets.go', 'case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r):', 'case unicode.Is(unicode.Cf, r):', 'unit'),
    ('N11 hangul fillers kept', CS + 'secrets.go', 'case r == 0x115F || r == 0x1160 || r == 0x3164 || r == 0xFFA0:', 'case false:', 'unit'),
    ('N12 sensitive key rule off', CS + 'secrets.go', 'pass|secret|token|pwd|credential|api', 'zzz|credential|api', 'unit'),
    ('N13 curl -u rule off', CS + 'secrets.go', '(?:-u|--user|--password|--http-password)[ =]\\S*:\\S+', '(?:-zz)[ =]\\S*:\\S+', 'unit'),
    ('N14 password= pair rule off', CS + 'secrets.go', '\\b(?:pass(?:word|wd|phrase)?|pwd|secret|token)\\s*[=:]\\s*[^\\s,;\'"]{2,}', 'zzzzzzzzzzzzzz', 'unit'),
    ('N15 basic auth rule off', CS + 'secrets.go', '\\bbasic\\s+[A-Za-z0-9+/=]{8,}', 'zzzzzzzzzz', 'unit'),
    ('N16 resolver may return single', HT, 'if !ok || owner == SingleOwner {', 'if !ok {', 'unit'),
    ('N17 envelope duplicate walk off', HT, 'if err != nil || seen[key] {', 'if err != nil {', 'unit'),
    ('N18 byte quota evicts recents first', SS, "ORDER BY kind = 'history' DESC, updated_at", "ORDER BY kind = 'history' ASC, updated_at", 'live'),
    ('N19 byte quota frees nothing counted', SS, '\t\tneed -= freed\n', '\t\tneed -= 0\n', 'live'),
    ('N20 migration verify skipped', 'internal/serverstate/migrate.go', '\t\tVerify: func(ctx context.Context, q queryer) error {\n\t\t\treturn verifyIndexes(', '\t\tVerify: func(ctx context.Context, q queryer) error {\n\t\t\treturn nil\n\t\t\treturn verifyIndexes(', 'live'),
    ('N21 history opt-in not checked in repo', SS, '\t\tif lim.Require != nil {', '\t\tif false {', 'live'),
    ('N22 owner quota raised, guard must notice', CS + 'types.go', 'var MaxOwnerBytes int64 = 32 << 20', 'var MaxOwnerBytes int64 = 64 << 20', 'guard'),
    ('N23 history per-scope quota raised, guard must notice', CS + 'types.go', 'SQL: true, PerScope: 200, PerOwner: 5000', 'SQL: true, PerScope: 201, PerOwner: 5000', 'guard'),
    ('N24 index key cached never (service)', CS + 'service.go', '\tif s.indexKey == nil {', '\tif s.indexKey == nil || true {', 'unit'),
    # Replacements for review probes M19/M20/M06 whose patterns no longer exist after the cfg2 change.
    ('N25 AEAD context without owner', SS, '"cfg2|" + owner + "|"', '"cfg2|" + "|"', 'live'),
    ('N26 AEAD context without scope', SS, '+ id + "|" + scope + "|" + nkey', '+ id + "|" + nkey', 'live'),
    ('N27 forbidden literal off', CS + 'service.go', 'if strings.Contains(folded, value) {', 'if strings.Contains(folded, value) && false {', 'unit'),
]


def sh(*args, env=None):
    return subprocess.run(args, cwd=ROOT, env=env, text=True, capture_output=True)


def run_one(entry, env):
    name, path, old, new, kind = entry
    source = (ROOT / path).read_text()
    if source.count(old) != 1:
        return name, 'PROBE-INVALID (pattern count %d)' % source.count(old)
    (ROOT / path).write_text(source.replace(old, new))
    try:
        if sh('go', 'build', './...').returncode:
            return name, 'PROBE-INVALID (does not compile)'
        if kind == 'guard':
            result = sh('python3', 'scripts/check-config-store-contract.py')
        elif kind == 'unit':
            result = sh('go', 'test', '-count=1', '-timeout', '150s', './internal/configstore', '-run', '.')
            if result.returncode == 0:
                result = sh('go', 'test', '-count=1', '-timeout', '150s', './internal/httpapi', '-run', 'Config|Review|Rereview')
        else:
            result = sh('go', 'test', '-count=1', '-timeout', '150s', './internal/serverstate', '-run', 'TestConfigRepoConformanceOnMariaDB|TestConfigRowsAreEncryptedAndBoundToTheirOwner|TestReviewLive|TestRereviewLive', env=env)
        return name, 'killed' if result.returncode else 'SURVIVED'
    finally:
        sh('git', 'checkout', '--', path)


if __name__ == '__main__':
    only = sys.argv[1:]
    results = []
    with Fixtures('w-p20-rm', systems=('maria',)) as fx:
        env = fx.environment()
        env.update({'DBM_TEST_STATE_PORT': fx.ports['maria'], 'DBM_TEST_STATE_CA_FILE': str(fx.fixture_dir / 'ca.pem'),
                    'DBM_TEST_STATE_ROOT_PASSWORD': PASSWORD})
        for entry in MUTATIONS:
            if only and not any(entry[0].startswith(prefix) for prefix in only):
                continue
            results.append(run_one(entry, env))
            print(*results[-1], flush=True)
    assert not fx.leftovers()
    survived = [name for name, state in results if state != 'killed']
    print('mutations:', len(results), 'not killed:', survived)
