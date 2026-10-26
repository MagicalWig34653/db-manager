#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-only
"""Reviewer B (wave 4): mutation probes. Applies one source mutation at a time, runs the package tests
(first without, then with the reviewer tests) and always restores the file via git checkout."""
import subprocess, sys
from pathlib import Path
ROOT = Path(__file__).resolve().parent.parent
D = 'internal/database/'
M = [
 # (package, file, old, new, test regexp)
 ('17', D+'serveradmin.go', '\t\t\tsensitive = true', '\t\t\tsensitive = false', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'allowed := !variable || ', 'allowed := true || ', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'r.Limit > 200 ||', 'r.Limit > 2000 ||', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'len(value) <= 64 &&', 'len(value) <= 6400 &&', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'strings.Trim(value, "0123456789.+-eE") == ""', 'true', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'r.Offset > 10000 ||', 'r.Offset > 100000 ||', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', '(r.Scope != "" && r.Scope != "GLOBAL" && r.Scope != "SESSION")', 'false', 'Admin|ReviewB17'),
 ('17', D+'serveradmin.go', 'LOCATE(?,USER)>0', 'LOCATE(?,USER)>=0', 'Admin|ReviewB17'),
 ('27', D+'monitor.go', 'MatchString(r.SQL)', 'MatchString("SELECT 1")', 'Monitor|ReviewB27'),
 ('27', D+'monitor.go', '(?i)^SELECT', '(?i)SELECT', 'Monitor|ReviewB27'),
 ('27', D+'monitor.go', '^[A-Za-z0-9_-]+\\.[0-9]{1,20}$', '^[A-Za-z0-9_\'-]+\\.[0-9]{1,20}$', 'Monitor|ReviewB27'),
 ('27', D+'monitor.go', 'if len(value) > 256 {\n\t\treturn false\n\t}', 'if false {\n\t\treturn false\n\t}', 'Monitor|ReviewB27'),
 ('27', D+'monitor.go', 'r.Limit > 200 ||', 'r.Limit > 2000 ||', 'Monitor|ReviewB27'),
 ('27', D+'monitor_details.go', 'ReadOnly: true', 'ReadOnly: false', 'Monitor|ReviewB27'),
 ('27', D+'monitor_details.go', 'if kind, ok := node["Node Type"].(string); ok && monitorValue(kind,', 'if kind, ok := node["Node Type"].(string); ok && monitorValue(kind+"|"+kind,', 'Monitor|ReviewB27'),
 ('27', D+'monitor.go', '} else if r.Log != "" || r.Position != 0 {', '} else if false {', 'Monitor|ReviewB27'),
 ('28', D+'export_format.go', 'return "\'" + s\n', 'return s\n', 'Export|ReviewB28'),
 ('28', D+'export_format.go', 'return \'\\uFFFD\'', 'return r', 'Export|ReviewB28'),
 ('28', D+'export_text.go', "case '{', '}', '$', '&', '#', '%', '_':", "case '{', '}', '$', '&', '#', '_':", 'Export|ReviewB28'),
 ('28', D+'export_text.go', "case '\\\\':\n\t\t\tb.WriteString(`\\textbackslash{}`)", "case 'Z':\n\t\t\tb.WriteString(`\\textbackslash{}`)", 'Export|ReviewB28'),
 ('28', D+'export_text.go', 'fmt.Fprintf(&b, "\\\\x%02X", s[i])', 'if s[i] == 0x24 || s[i] == 0x22 || s[i] == 0x5c {\n\t\t\tb.WriteByte(s[i])\n\t\t\tcontinue\n\t\t}\n\t\tfmt.Fprintf(&b, "\\\\x%02X", s[i])', 'Export|ReviewB28'),
 ('28', D+'export_text.go', 'fmt.Fprintf(&b, "&#%d;", r)', 'if r == 124 {\n\t\t\t\tb.WriteRune(r)\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\tfmt.Fprintf(&b, "&#%d;", r)', 'Export|ReviewB28'),
 ('28', D+'export_text.go', '_ = xml.EscapeText(&b, []byte(value))', '_ = xml.EscapeText(&bytes.Buffer{}, []byte(value)); b.WriteString(value)', 'Export|ReviewB28'),
 ('28', D+'export_text.go', 'r >= 0xe000 && r <= 0xfffd', 'r >= 0xe000', 'Export|ReviewB28'),
 ('28', D+'export_text.go', 'return f.write("<complete>true</complete></export>\\n")', 'return f.write("</export>\\n")', 'Export|ReviewB28'),
 ('28', D+'export_text.go', 'if !utf8.ValidString(c.Value) {', 'if !utf8.ValidString(c.Value) && false {', 'Export|ReviewB28'),
 ('28', D+'export.go', 'default:\n\t\treturn public("export_busy"', 'default:\n\t\treturn nil\n\t\tpublic("export_busy"', 'Export|ReviewB28'),
 ('29', D+'prefix.go', 'len([]rune(r.Prefix)), r.Prefix}\n\tcase "mysql"', 'len(r.Prefix), r.Prefix}\n\tcase "mysql"', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'prefix.go', 'if !strings.HasPrefix(name, r.Prefix) {\n\t\t\tcontinue\n\t\t}', '', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'partitions.go', 'err := dbplan.Check(p.Plan, hash, confirm, p.Action, p.Confirmation.TargetName, p.Confirmation.Destructive)', 'err := error(nil)', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'partitions.go', 'destructive := true', 'destructive := false', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'prefix.go', 'r.Action != "copy")', 'false)', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'objectops_plan.go', 'strings.ReplaceAll(value, "\'", "\'\'")', 'value', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'prefix.go', 'item.Status = "not_applied"\n\t\t\tif res.Outcome == "outcome_unknown" {', 'item.Status = "applied"\n\t\t\tif res.Outcome == "outcome_unknown" {', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'prefix.go', 'case firstOpen && res.Outcome == "outcome_unknown":', 'case false:', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'prefix.go', 'if len(out.Tables) > core.PrefixMaxTables {', 'if len(out.Tables) > 100000 {', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'partitions.go', 'if driver == "mysql" {\n\t\t\tseen[part.Table.Name] = true', 'if false {\n\t\t\tseen[part.Table.Name] = true', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'partitions.go', '!opName(driver, d.Name) || seen[d.Name]', 'seen[d.Name]', 'Partition|Prefix|Structure|ReviewB29'),
 ('29', D+'partitions.go', 'if failed {', 'if failed && false {', 'Partition|Prefix|Structure|ReviewB29'),
]
only = sys.argv[1] if len(sys.argv) > 1 else None
def run(pattern, skip):
    cmd = ['go', 'test', '-count=1', './internal/database', '-run', pattern]
    if skip: cmd += ['-skip', 'ReviewB']
    return subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True).returncode
for pkg, f, old, new, pat in M:
    if only and pkg != only: continue
    path = ROOT / f
    src = path.read_text()
    if src.count(old) < 1:
        print(f'W-P{pkg} {f}: PATTERN MISSING {old[:50]!r}'); continue
    try:
        path.write_text(src.replace(old, new, 1))
        b = subprocess.run(['go', 'build', './internal/database'], cwd=ROOT, capture_output=True, text=True)
        if b.returncode:
            print(f'W-P{pkg} {old[:50]!r}: BUILD FAIL {b.stderr[:100]}'); continue
        orig = run(pat, True)
        both = run(pat, False)
        print(f'W-P{pkg} {old[:55]!r:60} original-tests:{"killed" if orig else "SURVIVED"}  with-review:{"killed" if both else "SURVIVED"}', flush=True)
    finally:
        subprocess.run(['git', 'checkout', '--', f], cwd=ROOT)
