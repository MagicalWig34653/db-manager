#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Re-review wave 4: mutation probes per fix. Each mutant edits one source file, runs the
targeted tests (SQLite, no containers) and is restored with git checkout.
Usage: review-wave4-rereview-mutations.py  (run in a throw-away worktree)."""
import subprocess, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DB = './internal/database'
M = [
 # id, file, old, new, pkg, run regex
 ('L1 consume always true', 'internal/database/plan_ledger.go', 'if t, ok := l.consumed[key]; ok && now.Sub(t) < planLedgerTTL {\n\t\treturn false\n\t}', '', DB, 'TestReviewA_(CSVImportReplayRefused|BulkCopyReplayRefused)'),
 ('L2 arm does nothing', 'internal/database/plan_ledger.go', '\tdelete(l.consumed, key)\n}', '}', DB, 'TestReviewA_NewPreviewRearmsPlan'),
 ('L3 import: consume removed', 'internal/database/import.go', 'if !e.plans.consume("import:" + p.SHA256) {', 'if false {', DB, 'TestReviewA_CSVImportReplayRefused'),
 ('L4 bulk: consume removed', 'internal/database/rows_bulk.go', 'if !e.plans.consume("bulk:" + plan.PlanSHA256) {', 'if false {', DB, 'TestReviewA_BulkCopyReplayRefused'),
 ('L5 ledger never evicts', 'internal/database/plan_ledger.go', '\t\t\tdelete(l.consumed, oldestKey)\n', '', DB, 'TestReReview_PlanLedgerBounded'),
 ('L6 expired entries never purged', 'internal/database/plan_ledger.go', '\t\t\tif now.Sub(t) >= planLedgerTTL {\n\t\t\t\tdelete(l.consumed, k)\n\t\t\t} else if', '\t\t\tif false {\n\t\t\t} else if', DB, 'TestReReview_PlanLedgerExpiry'),
 ('L7 ledger without mutex', 'internal/database/plan_ledger.go', 'func (l *planLedger) consume(key string) bool {\n\tl.mu.Lock()\n\tdefer l.mu.Unlock()', 'func (l *planLedger) consume(key string) bool {', DB, 'TestReReview_PlanLedgerRace'),
 ('R1 rolled_back never set', 'internal/database/import_sql.go', 'out.Statements[j].Outcome = "rolled_back"', 'out.Statements[j].Outcome = "completed"', DB, 'TestReviewA_SQLImportPartialStateHonesty'),
 ('B1 file budget back to 1 MiB', 'internal/database/rows_bulk_values.go', '1<<20+base64.StdEncoding.EncodedLen(core.MaxFileValueBytes)); err', '1<<20); err', DB, 'TestReviewA_FileReplaceNearLimit|TestReReview_FileReplaceAtLimit'),
 ('C1 cancel keeps inserted count', 'internal/database/import.go', '\t\t\tout.Inserted = 0\n', '', DB, 'TestReviewA_|TestLive|TestSQLiteImport'),
 ('C2 ErrTxDone treated as unknown (live engines only)', 'internal/database/import.go', 'err != nil && !errors.Is(err, sql.ErrTxDone)', 'err != nil && errors.Is(err, sql.ErrTxDone)', DB, 'TestReviewA_|TestSQLiteImport'),
 ('Y1 yaml U+007F', 'internal/database/export_text.go', 'r == 0x7f || ', '', DB, 'TestYAMLSafeJSON|TestReReviewYAML'),
 ('Y2 yaml C1 range', 'internal/database/export_text.go', 'r >= 0x80 && r <= 0x9f || ', '', DB, 'TestYAMLSafeJSON|TestReReviewYAML'),
 ('Y3 yaml U+FFFE', 'internal/database/export_text.go', ' || r == 0xfffe', '', DB, 'TestYAMLSafeJSON|TestReReviewYAML'),
 ('Y4 yaml U+FFFF', 'internal/database/export_text.go', ' || r == 0xffff', '', DB, 'TestYAMLSafeJSON|TestReReviewYAML'),
 ('Y5 yaml escape not applied to header', 'internal/database/export_text.go', 'strings.TrimSuffix(string(yamlSafeJSON(b)), "}")', 'strings.TrimSuffix(string(b), "}")', DB, 'TestYAMLSafeJSON|TestReReviewYAML|TestReviewB28'),
 ('N1 monitor: 1045 unavailable', 'internal/database/monitor.go', 'case 1064, 1109,', 'case 1045, 1044, 1064, 1109,', DB, 'TestReReview_Monitor'),
 ('N2 monitor: profiling errors unavailable', 'internal/database/monitor.go', 'if action != "profiling" && monitorUnavailable(err) {', 'if monitorUnavailable(err) {', DB, 'TestReReview_Monitor'),
 ('S1 server-admin: exact counters masked again', 'internal/database/serveradmin.go', 'case "max_connections", "connections", "max_used_connections", "max_user_connections":\n\t\tknown = true', 'case "no_such":\n\t\tknown = true', DB, 'TestWave4|TestReviewB17|TestServerAdmin|TestAdmin'),
 ('S2 server-admin: sensitive list dropped', 'internal/database/serveradmin.go', '"password", "passwd", "secret", "token", "key", "path", "file", "dir", "ssl", "tls", "connection", "credential"', '"zzz"', DB, 'TestWave4|TestReviewB17|TestServerAdmin|TestAdmin'),
 ('V1 emitted reason not in contract (code)', 'internal/database/accounts.go', 'Reason: "grant_without_effect"', 'Reason: "grant_without_effect_x"', DB, 'TestEmittedReasonsAreInLatestContract'),
 ('V2 enum value removed (contract)', 'contracts/api-v0.23.0.json', '"grant_without_effect"', '"grant_without_effect_removed"', DB, 'TestEmittedReasonsAreInLatestContract'),
]

def sh(*a):
    return subprocess.run(a, cwd=ROOT, capture_output=True, text=True)

for name, rel, old, new, pkg, run in M:
    p = ROOT / rel
    src = p.read_text()
    if src.count(old) != 1:
        print('%-48s PATTERN-NOT-FOUND (%d)' % (name, src.count(old))); continue
    p.write_text(src.replace(old, new))
    try:
        r = sh('go', 'test', '-count=1', '-run', run, pkg)
        out = r.stdout + r.stderr
        status = 'KILLED' if r.returncode else ('SURVIVED' if 'no tests to run' not in out else 'NO-TESTS')
        if r.returncode and ('build failed' in out):
            status = 'BUILD-FAILED'
        print('%-48s %s' % (name, status))
    finally:
        sh('git', 'checkout', '--', rel)
