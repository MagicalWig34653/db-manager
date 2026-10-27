#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Independent W-P20 review: mutation probes. Each mutation edits one source line, runs the tests
that must notice it and reverts via git. Live mutations share one disposable MariaDB 12.3."""
import subprocess
import sys

from dbfix import Fixtures, PASSWORD, ROOT

CS, HT, SS = 'internal/configstore/', 'internal/httpapi/config.go', 'internal/serverstate/configrepo.go'
# (id, file, old, new, kind) kind: unit | live
MUTATIONS = [
    ('M01 no NFKC fold', CS + 'secrets.go', 'text = norm.NFKC.String(text)', 'text = text + norm.NFC.String("")', 'unit'),
    ('M02 keep format chars', CS + 'secrets.go', 'unicode.Is(unicode.Me, r):\n\t\t\treturn -1', 'unicode.Is(unicode.Me, r):\n\t\t\treturn r', 'unit'),
    ('M03 drop password rule', CS + 'secrets.go', '(?i)pass(?:word|wd|phrase)', '(?i)zzzzzzzz', 'unit'),
    ('M04 drop credential URL rule', CS + 'secrets.go', '[a-z][a-z0-9+.\\-]*://[^/\\s:@]*:[^/\\s@]+@', 'zzzzzzzz://q', 'unit'),
    ('M05 long run 32 -> 80', CS + 'secrets.go', '{32,}', '{80,}', 'unit'),
    ('M06 forbidden literal off', CS + 'service.go', 'strings.Contains(folded, value)', 'strings.Contains(folded, value) && false', 'unit'),
    ('M07 history gate off', CS + 'service.go', 'if spec.Kind == KindHistory {\n\t\tindex, err', 'if spec.Kind == KindHistory && false {\n\t\tindex, err', 'unit'),
    ('M08 history check always true', CS + 'service.go', 'return json.Unmarshal(data, &v) == nil && v.On', 'return true', 'unit'),
    ('M09 clear confirmation off', CS + 'service.go', 'if confirmation != confirmationFor(total) {', 'if false {', 'unit'),
    ('M10 validOwner always true', CS + 'validate.go', 'return ownerPattern.MatchString(owner)', 'return true', 'unit'),
    ('M11 list ignores scope', CS + 'service.go', 'AllScopes: f.Scope == nil', 'AllScopes: true', 'unit'),
    ('M12 allow duplicate keys', CS + 'validate.go', '|| seen[key]', '', 'unit'),
    ('M13 depth limit off', CS + 'validate.go', 'depth >= MaxJSONDepth', 'depth >= 99', 'unit'),
    ('M14 sql char limit +1', CS + 'validate.go', 'runes(sql) > MaxSQLChars || len(sql) > MaxSQLBytes {', 'runes(sql) > MaxSQLChars+1 || len(sql) > MaxSQLBytes {', 'unit'),
    ('M15 no 428', HT, 'if raw == "" {', 'if false {', 'unit'),
    ('M16 no RequireState', HT, 'return a.auth(a.RequireState(http.HandlerFunc(', 'return a.auth((http.HandlerFunc(', 'unit'),
    ('M17 capabilities unauthenticated', HT, 'a.auth(http.HandlerFunc(a.configCapabilities))', 'http.HandlerFunc(a.configCapabilities)', 'unit'),
    ('M18 wrong privacy nkey', CS + 'service.go', 'scope, privacyKey)', 'scope, privacyKey+"x")', 'unit'),
    ('M19 AEAD context without owner', SS, '[]string{owner, string(kind), id,', '[]string{string(kind), id,', 'live'),
    ('M20 AEAD context without scope', SS, 'id, scope, nkey, strconv', 'id, nkey, strconv', 'live'),
    ('M21 AEAD context without kind', SS, '[]string{owner, string(kind), id,', '[]string{owner, id,', 'live'),
    ('M22 owner lock without FOR UPDATE', SS, 'FROM config_owners WHERE owner_id = ? FOR UPDATE', 'FROM config_owners WHERE owner_id = ?', 'live'),
    ('M23 Get ignores owner', SS, 'row, err := r.scan(owner, kind, r.s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind', 'row, err := r.scan(owner, kind, r.s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE ? IS NOT NULL AND kind', 'live'),
    ('M24 per-scope quota off by one', SS, 'inScope - lim.PerScope + 1', 'inScope - lim.PerScope', 'live'),
    ('M25 version check off', SS, 'expect != configstore.AnyVersion && row.Version != expect', 'false', 'live'),
    ('M26 owner byte quota off', SS, 'need := used + size - lim.OwnerBytes', 'need := used - lim.OwnerBytes', 'live'),
    ('M27 clear ignores stale hash', SS, 'if d.Hash != hash {', 'if false {', 'live'),
    ('M28 list ignores owner', SS, 'query := "SELECT " + itemColumns + " FROM config_items WHERE owner_id = ? AND kind = ?"', 'query := "SELECT " + itemColumns + " FROM config_items WHERE ? IS NOT NULL AND kind = ?"', 'live'),
    ('M29 delete ignores version', SS, 'expect != configstore.AnyVersion && int64(version) != expect', 'false', 'live'),
    ('M30 FIFO off for history', 'internal/configstore/types.go', '{Kind: KindHistory, FIFO: true,', '{Kind: KindHistory, FIFO: false,', 'unit'),
    ('M31 import future time check off', CS + 'service.go', 't.Year() < 2000 || t.After(s.now().Add(24*time.Hour))', 'false', 'unit'),
    ('M32 cursor id pattern off', CS + 'service.go', '!idPattern.MatchString(id) {\n\t\treturn Cursor{}, false', 'false {\n\t\treturn Cursor{}, false', 'unit'),
    ('M33 NUL allowed', CS + 'validate.go', 'r < 0x20 || r == 0x7f', 'r < 0x00 || r == 0x7f', 'unit'),
    ('M34 version not incremented', SS, 'version = version + 1', 'version = version', 'live'),
    ('M35 export ignores owner', SS, 'FROM config_items WHERE owner_id = ? ORDER BY updated_at, item_id', 'FROM config_items WHERE ? IS NOT NULL ORDER BY updated_at, item_id', 'live'),
    ('M36 clear ignores owner', SS, 'where := "owner_id = ? AND kind IN (', 'where := "? IS NOT NULL AND kind IN (', 'live'),
    ('M37 history update allowed', CS + 'service.go', 'return Item{}, ErrInvalid // history entries are immutable; delete instead', 'return Item{}, nil', 'unit'),
    ('M38 secrets scan skips keys of objects', CS + 'validate.go', 'c.strs = append(c.strs, key)', '', 'unit'),
    ('M39 Touch off for recents', 'internal/configstore/types.go', 'Derived: true, FIFO: true, Touch: true,', 'Derived: true, FIFO: true, Touch: false,', 'unit'),
    ('M40 client_ref idempotence off', CS + 'service.go', 'case spec.Idempotent || row.ClientRef != "":', 'case spec.Idempotent:', 'unit'),
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
        if kind == 'unit':
            result = sh('go', 'test', '-count=1', './internal/configstore', '-run', '.')
            if result.returncode == 0:
                result = sh('go', 'test', '-count=1', './internal/httpapi', '-run', 'TestConfig|TestReview(Holds|Gap)_')
        else:
            result = sh('go', 'test', '-count=1', './internal/serverstate', '-run', 'TestConfig|TestReviewLive', env=env)
        return name, 'killed' if result.returncode else 'SURVIVED'
    finally:
        sh('git', 'checkout', '--', path)


if __name__ == '__main__':
    only = sys.argv[1:]
    results = []
    with Fixtures('w-p20-mu', systems=('maria',)) as fx:
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
    sys.exit(0)
