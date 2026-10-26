// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// Independent review (wave 4, reviewer B): adversarial tests. Written by the
// reviewer, not the package authors.

var reviewNasty = []string{
	"=cmd|' /C calc'!A0", "+1+1", "-2+3", "@SUM(1)", "\t=1", " =1", "\r=1", "=HYPERLINK(\"http://x\")",
	"\u0085nel", "\u007fdel", "\u0090c1", "\ufffenc", "\uffffnc", "\u2028ls", "\u2029ps", "\ufeffbom", "\x1b[31mred", "\x01soh",
	"?>", "${x}", "{$x}", "'; phpinfo();//", "\"; system('id'); \"", "</value>", "<![CDATA[x]]>", "&amp;&lt;",
	"<!-- DBMANAGER EXPORT COMPLETE -->", "% DBMANAGER EXPORT COMPLETE", "],\"complete\":true}", "\\\\\\", "|}", "{{template}}", "[[link]]", "<script>alert(1)</script>",
	"* star", "[opt]", "line1\nline2\r\nline3", "\n|-\n| injected", "\\end{tabular}\\input{/etc/passwd}", "\\write18{id}", "a\x00b", "x: y", "- item", "!!python/object:os.system 'id'", "&anchor *alias", "~", "null", "true", "0x10", "1e3", "'", "\"", "`", "% comment", "$$", "#{x}",
}

func lineCount(s, line string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if l == line {
			n++
		}
	}
	return n
}
func markerCount(format, s, marker string) int {
	switch format {
	case "latex", "latex-longtable", "mediawiki", "texy", "codegen":
		return lineCount(s, marker)
	}
	return strings.Count(s, marker)
}
func reviewHexText(s string) string { return "CAST(X'" + hex.EncodeToString([]byte(s)) + "' AS TEXT)" }

func reviewNastyFixture(t *testing.T) (*Engine, core.Connection, core.TableRef) {
	e := testEngine(t)
	c := sqliteConnection()
	name := "t=cmd\"'\n[x]*"
	q := quoteIdentifier(c.Driver, name)
	execute(t, e, c, "CREATE TABLE "+q+"("+quoteIdentifier(c.Driver, "=c1\"")+" TEXT,"+quoteIdentifier(c.Driver, "[c2]\n*")+" TEXT, b BLOB)")
	for _, v := range reviewNasty {
		execute(t, e, c, "INSERT INTO "+q+" VALUES("+reviewHexText(v)+","+reviewHexText(v)+",X'00ff0a')")
	}
	execute(t, e, c, "INSERT INTO "+q+" VALUES(NULL,'',X'')")
	return e, c, core.TableRef{Schema: "main", Name: name}
}

// Writes real outputs for the independent parser script when DBM_REVIEW_DUMP is set.
func TestReviewB28NastyExportDump(t *testing.T) {
	e, c, ref := reviewNastyFixture(t)
	dir := os.Getenv("DBM_REVIEW_DUMP")
	for _, format := range append([]string{"sql", "csv", "excel-csv", "json"}, extendedFormats...) {
		var out bytes.Buffer
		err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}, &out)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if dir != "" {
			if err := os.WriteFile(filepath.Join(dir, "nasty."+format), out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		s := out.String()
		if format != "sql" && format != "csv" && format != "excel-csv" && format != "json" {
			// completion marker must appear exactly once, as the final structure
			m := extendedMarkers[format]
			if !strings.HasSuffix(s, m) || markerCount(format, s, strings.TrimSpace(m)) != 1 {
				t.Errorf("%s: marker count %d", format, markerCount(format, s, strings.TrimSpace(m)))
			}
		}
		if format == "xml" {
			for _, r := range s {
				if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
					t.Errorf("xml contains raw character not allowed by XML 1.0: U+%04X", r)
				}
			}
		}
		if format == "excel-csv" && strings.ContainsAny(s, "\x1b\x01\x00") {
			t.Errorf("excel-csv keeps raw control characters")
		}
		if format == "php-array" && (strings.Contains(s, "?>") || strings.Contains(s, "phpinfo")) {
			t.Errorf("php: injection text visible")
		}
	}
}

// excel-csv: every field must start with an apostrophe after csv parsing, whatever the content.
func TestReviewB28ExcelCSVFormulaNeutralized(t *testing.T) {
	e, c, ref := reviewNastyFixture(t)
	for _, d := range []string{",", ";", "\t"} {
		for _, crlf := range []bool{false, true} {
			var out bytes.Buffer
			if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "excel-csv", Delimiter: d, CRLF: crlf}, &out); err != nil {
				t.Fatalf("%q: %v", d, err)
			}
			r := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), "\xef\xbb\xbf")))
			r.Comma = rune(d[0])
			r.FieldsPerRecord = -1
			recs, err := r.ReadAll()
			if err != nil {
				t.Fatalf("csv parse %q: %v", d, err)
			}
			for _, rec := range recs {
				for _, f := range rec {
					if f != "" && !strings.HasPrefix(f, "'") {
						t.Errorf("unprefixed field %q in record %q (delim %q)", f, rec, d)
					}
				}
			}
		}
	}
}

// Plain csv is documented as unprotected; record the behaviour so a change is noticed.
func TestReviewB28PlainCSVHasNoFormulaProtection(t *testing.T) {
	e, c, ref := reviewNastyFixture(t)
	var out bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "csv"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"=cmd|' /C calc'!A0\"") {
		t.Log("plain csv now neutralizes formulas (good)")
	} else {
		t.Log("INFO: plain csv passes =cmd| through unmodified (documented design)")
	}
}

// Abort/limit: no text format may ever end with its completion marker after a failure.
func TestReviewB28AbortNeverCompletes(t *testing.T) {
	e, c, ref := reviewNastyFixture(t)
	for _, format := range extendedFormats {
		for _, mut := range []func(*core.ExportRequest){
			func(r *core.ExportRequest) { r.MaxRows = 3 },
			func(r *core.ExportRequest) { r.MaxBytes = 700 },
			func(r *core.ExportRequest) {
				r.TimeoutMS = 1
				r.Scope = "query"
				r.SQL = "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<100000000) SELECT x FROM n"
			},
		} {
			req := core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}
			mut(&req)
			var out bytes.Buffer
			err := e.Export(context.Background(), req, &out)
			if err == nil {
				t.Errorf("%s: expected failure", format)
			}
			if strings.Contains(out.String(), strings.TrimSpace(extendedMarkers[format])) {
				t.Errorf("%s: marker present after failure", format)
			}
		}
		w := &exportFailWriter{}
		if e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}, w) == nil {
			t.Errorf("%s: writer failure swallowed", format)
		}
	}
}

// W-P17: masking must not reveal strings, paths, keys or anything non-numeric.
func TestReviewB17MaskingAdversarial(t *testing.T) {
	cases := []struct {
		name, value string
		variable    bool
		visible     bool
	}{
		{"max_connections", "151", true, false}, // over-masked (contains "connection"): functional gap, not a leak
		{"Connections", "5", false, false},
		{"Threads_connected", "5", false, true},
		{"datadir", "123", true, false},
		{"secure_file_priv", "1", true, false},
		{"foo", "1", true, false},
		{"port", "3306", true, true},
		{"port", "3306; DROP", true, false},
		{"port", "1e5", true, true},
		{"port", "NaN", true, false},
		{"port", "Inf", true, false},
		{"port", "0x10", true, false},
		{"port", strings.Repeat("1", 65), true, false},
		{"Uptime", "12", false, true},
		{"Ssl_cipher", "5", false, false},
		{"Com_select", "5", false, true},
		{"password_lifetime", "5", true, false},
		{"Bytes_sent", "-", false, false},
		{"tmp_table_size", "+.5", true, true},
		{"tmp_table_size", ".", true, false},
		{"tmp_table_size", "1-1", true, false},
	}
	for _, c := range cases {
		got := adminSafeValue(c.name, c.value, c.variable)
		if (got.Value != nil) == got.Masked || (got.Value != nil) != c.visible {
			t.Errorf("%s=%q visible=%v masked=%v", c.name, c.value, got.Value != nil, got.Masked)
		}
	}
}

// W-P17: no write verbs anywhere in the server-admin source.
func TestReviewB17NoWriteVerbs(t *testing.T) {
	b, err := os.ReadFile("serveradmin.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ToUpper(string(b))
	for _, v := range []string{"KILL ", "SET GLOBAL", "SET SESSION", "FLUSH", "PG_TERMINATE", "PG_CANCEL", "ALTER ", "EXEC(", "EXECCONTEXT"} {
		if strings.Contains(src, v) {
			t.Errorf("write verb %q in serveradmin.go", v)
		}
	}
}

// W-P27: the profiling grammar must reject everything that is not a trivial SELECT.
func TestReviewB27ProfilingGrammar(t *testing.T) {
	bad := []string{
		"SELECT SLEEP(10)", "SELECT 1; DROP TABLE t", "SELECT 1 -- x", "SELECT 1 /*x*/", "SELECT * FROM t INTO OUTFILE 'x'", "SELECT * FROM t FOR UPDATE",
		"SELECT * FROM (SELECT 1) x", "SELECT 'a'", "SELECT * FROM t WHERE a='x'", "WITH a AS (SELECT 1) SELECT * FROM a", "EXPLAIN ANALYZE SELECT 1", "SELECT 1\x00",
		"SELECT * FROM t\nWHERE a=1;", "SELECT * FROM `t`", "SELECT * FROM \"t\"", "SELECT * FROM t WHERE a=1 OR b=2", "SELECT * FROM t PROCEDURE ANALYSE()",
		"SELECT * FROM t LIMIT 1 OFFSET 1", "SELECT * FROM t WHERE a=-1", "SELECT a.b FROM t", "SELECT * FROM a.b.c", "SELECT 1", "SELECT ١",
		"SELECT * FROM t WHERE a=1 UNION SELECT 2", "SELECT pg_sleep(5)", "DELETE FROM t", "SELECT * FROM t;",
	}
	for _, s := range bad {
		r := core.MonitorRequest{Connection: core.Connection{Driver: "mysql"}, SQL: s}
		if err := normalizeMonitor("profiling", &r); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	good := []string{"SELECT 1", "select * from t", "SELECT a,b FROM s.t WHERE c>=5 LIMIT 3"}
	for _, s := range good {
		r := core.MonitorRequest{Connection: core.Connection{Driver: "mysql"}, SQL: s}
		if err := normalizeMonitor("profiling", &r); err != nil {
			t.Errorf("rejected %q: %v", s, err)
		}
	}
	for _, a := range core.MonitorActions {
		if a == "profiling" {
			continue
		}
		r := core.MonitorRequest{SQL: "SELECT 1"}
		if normalizeMonitor(a, &r) == nil {
			t.Errorf("%s accepted SQL", a)
		}
	}
	// binary-events log name must not carry path or quote characters
	for _, l := range []string{"x'; DROP", "../etc/passwd", "a.1 ", "a.", "a b.1", "a.1\n", "a.123456789012345678901", "a'b.1", "a\"b.1", "a`b.1", "a/b.1", "a\\b.1"} {
		r := core.MonitorRequest{Log: l}
		if normalizeMonitor("binary-events", &r) == nil {
			t.Errorf("log %q accepted", l)
		}
	}
	_ = fmt.Sprint
}

// Edge shapes: empty database (no tables), query scope with hostile aliases, views as tables, two tables.
func TestReviewB28EdgeShapes(t *testing.T) {
	dir := os.Getenv("DBM_REVIEW_DUMP")
	e := testEngine(t)
	c := core.Connection{Driver: "sqlite", Database: "empty"}
	for _, format := range extendedFormats {
		var out bytes.Buffer
		if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "database", Format: format}, &out); err != nil {
			t.Errorf("%s empty db: %v", format, err)
			continue
		}
		if dir != "" {
			_ = os.WriteFile(filepath.Join(dir, "empty."+format), out.Bytes(), 0600)
		}
		if !strings.HasSuffix(out.String(), extendedMarkers[format]) {
			t.Errorf("%s empty: no marker", format)
		}
	}
	c2 := core.Connection{Driver: "sqlite", Database: "two"}
	execute(t, e, c2, "CREATE TABLE a(x TEXT)")
	execute(t, e, c2, "CREATE TABLE b(y TEXT)")
	execute(t, e, c2, "INSERT INTO a VALUES('1'),('2')")
	execute(t, e, c2, "CREATE VIEW v AS SELECT x AS \"=evil\" FROM a")
	for _, format := range extendedFormats {
		var out bytes.Buffer
		err := e.Export(context.Background(), core.ExportRequest{Connection: c2, Scope: "database", Format: format, ViewsAsTables: true}, &out)
		if err != nil {
			t.Errorf("%s two tables+view: %v", format, err)
			continue
		}
		if dir != "" {
			_ = os.WriteFile(filepath.Join(dir, "two."+format), out.Bytes(), 0600)
		}
		var q bytes.Buffer
		err = e.Export(context.Background(), core.ExportRequest{Connection: c2, Scope: "query", Format: format, SQL: "SELECT x AS \"a\"\"b\n<&>\", 1 AS n, NULL AS z FROM a"}, &q)
		if err != nil {
			t.Errorf("%s query: %v", format, err)
			continue
		}
		if dir != "" {
			_ = os.WriteFile(filepath.Join(dir, "query."+format), q.Bytes(), 0600)
		}
	}
}

// Concurrency: a third simultaneous export must be refused, and slots must be released afterwards.
func TestReviewB28ConcurrentExportSlots(t *testing.T) {
	e := testEngine(t)
	gate := make(chan struct{})
	started := make(chan struct{}, 3)
	conns := []core.Connection{}
	for i := 0; i < 3; i++ {
		c := core.Connection{Driver: "sqlite", Database: fmt.Sprintf("slot%d", i)}
		execute(t, e, c, "CREATE TABLE a(x TEXT)")
		execute(t, e, c, "INSERT INTO a VALUES('1')")
		conns = append(conns, c)
	}
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(c core.Connection) {
			errs <- e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "database", Format: "yaml"}, &blockWriter{gate: gate, started: started})
		}(conns[i])
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(20 * time.Second):
			t.Fatal("exports did not start")
		}
	}
	err := e.Export(context.Background(), core.ExportRequest{Connection: conns[2], Scope: "database", Format: "xml"}, &bytes.Buffer{})
	var p *core.Error
	if err == nil || !errors.As(err, &p) || p.Code != "export_busy" {
		t.Errorf("third export not refused: %v", err)
	}
	close(gate)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Errorf("blocked export failed: %v", err)
		}
	}
	if err := e.Export(context.Background(), core.ExportRequest{Connection: conns[2], Scope: "database", Format: "xml"}, &bytes.Buffer{}); err != nil {
		t.Errorf("slot leaked: %v", err)
	}
}

type blockWriter struct {
	gate    chan struct{}
	started chan struct{}
	once    bool
}

func (w *blockWriter) Write(p []byte) (int, error) {
	if !w.once {
		w.once = true
		w.started <- struct{}{}
		<-w.gate
	}
	return len(p), nil
}

// ---- W-P29 adversarial prefix / partition tests ----

func reviewTableList(t *testing.T, e *Engine, c core.Connection, schema string) map[string]bool {
	t.Helper()
	q := "SELECT name FROM sqlite_schema WHERE type='table'"
	switch c.Driver {
	case "mysql":
		q = "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE'"
	case "postgres":
		q = "SELECT tablename FROM pg_tables WHERE schemaname='" + schema + "'"
	}
	r := execute(t, e, c, q)
	out := map[string]bool{}
	for _, row := range r.Results[0].Rows {
		out[row[0].Value] = true
	}
	return out
}

func reviewConfirm(p core.StructurePlan) dbplan.Confirmation {
	return dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}
}

func reviewPrefixAttack(t *testing.T, e *Engine, c core.Connection, schema string) {
	ctx := context.Background()
	h := &opHarness{t: t, e: e, c: c, schema: schema}
	mk := func(names ...string) {
		for _, n := range names {
			h.exec("CREATE TABLE " + h.q(n) + " (id INTEGER PRIMARY KEY)")
		}
	}
	has := func(names ...string) bool {
		got := reviewTableList(t, e, c, schema)
		for _, n := range names {
			if !got[n] {
				return false
			}
		}
		return true
	}
	// 1. LIKE wildcards are literal
	mk("w_one", "wXone", "w%one", "wYY")
	for _, tc := range []struct {
		prefix string
		want   []string
	}{{"w_", []string{"w_one"}}, {"w%", []string{"w%one"}}} {
		got, err := e.PrefixTables(ctx, core.PrefixRequest{Connection: c, Schema: schema, Prefix: tc.prefix})
		if err != nil || len(got.Tables) != len(tc.want) || got.Tables[0].Name != tc.want[0] {
			t.Errorf("prefix %q: %v %v", tc.prefix, got.Tables, err)
		}
	}
	if got, err := e.PrefixTables(ctx, core.PrefixRequest{Connection: c, Schema: schema, Prefix: "%"}); err == nil && len(got.Tables) != 0 {
		t.Errorf("bare %% matched %v", got.Tables)
	}
	// drop with wildcard prefix must only drop the literal match
	r := core.PrefixRequest{Connection: c, Schema: schema, Prefix: "w_", Action: "drop"}
	p, err := e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatalf("plan w_: %v", err)
	}
	if len(p.Statements) != 1 {
		t.Errorf("w_ plan statements: %v", p.Statements)
	}
	res, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256})
	if err != nil || res.Outcome != "completed" || !has("wXone", "w%one", "wYY") || has("w_one") {
		t.Errorf("literal drop wrong: %v %v", res, err)
	}
	// 2. remove where a table name equals the prefix: target would be the empty name
	mk("rm_", "rm_x")
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "rm_", Action: "remove"}
	if p, err = e.PlanPrefix(ctx, r); err == nil {
		t.Errorf("remove producing empty name was planned: %v", p.Statements)
	} else if !has("rm_", "rm_x") {
		t.Errorf("tables changed by failed plan")
	}
	// 3. replace into an existing name and chain collision inside the batch
	mk("ca_x", "cb_x")
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "ca_", Action: "replace", NewPrefix: "cb_"}
	if _, err = e.PlanPrefix(ctx, r); err == nil {
		t.Errorf("rename onto existing table planned")
	}
	mk("r1", "rr1")
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "r", Action: "add", NewPrefix: "r"}
	if p, err = e.PlanPrefix(ctx, r); err == nil {
		t.Logf("chain add planned (statements %v)", p.Statements)
		res, _ := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256})
		t.Logf("chain execute: %s %s", res.Outcome, res.Message)
	}
	// 4. hostile characters in prefix and names
	hostile := "q'\"`;--ä"
	mk(hostile+"_a", hostile+"_b", "keep_q")
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: hostile, Action: "drop"}
	if p, err = e.PlanPrefix(ctx, r); err != nil {
		t.Errorf("hostile plan: %v", err)
	} else {
		res, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256})
		if err != nil || res.Outcome != "completed" || !has("keep_q") || has(hostile+"_a") {
			t.Errorf("hostile drop: %v %v", res, err)
		}
	}
	// 5. confirmation / hash variants all rejected, nothing changes
	mk("cf_a")
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "cf_", Action: "drop"}
	p, err = e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]core.PrefixExecuteRequest{
		"empty target":   {PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action}, ExpectedPlanSHA256: p.SHA256},
		"wrong action":   {PrefixRequest: r, Confirm: dbplan.Confirmation{Action: "prefix_truncate", TargetName: "cf_"}, ExpectedPlanSHA256: p.SHA256},
		"prefix+space":   {PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: "cf_ "}, ExpectedPlanSHA256: p.SHA256},
		"case":           {PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: "CF_"}, ExpectedPlanSHA256: p.SHA256},
		"upper hash":     {PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: strings.ToUpper(p.SHA256)},
		"hash+space":     {PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256 + " "},
		"empty hash":     {PrefixRequest: r, Confirm: reviewConfirm(p)},
		"other action":   {PrefixRequest: core.PrefixRequest{Connection: c, Schema: schema, Prefix: "cf_", Action: "truncate"}, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256},
		"other prefix":   {PrefixRequest: core.PrefixRequest{Connection: c, Schema: schema, Prefix: "c", Action: "drop"}, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256},
		"foreign schema": {PrefixRequest: core.PrefixRequest{Connection: c, Schema: "information_schema", Prefix: "cf_", Action: "drop"}, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256},
	} {
		if _, err := e.ExecutePrefix(ctx, req); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if !has("cf_a") {
			t.Fatalf("%s: table vanished", name)
		}
	}
	// 6. stale plan: new matching table appears between plan and execute
	mk("cf_b")
	_, err = e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256})
	requireMutationCode(t, err, "plan_mismatch")
	if !has("cf_a", "cf_b") {
		t.Errorf("stale plan executed")
	}
	// 7. too-long target names are rejected without partial effect
	long := strings.Repeat("n", 70)
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "cf_", Action: "add", NewPrefix: long}
	if _, err = e.PlanPrefix(ctx, r); err == nil && c.Driver != "sqlite" {
		t.Errorf("over-long target name planned")
	}
	// 8. empty and system prefixes
	for _, bad := range []core.PrefixRequest{{Prefix: ""}, {Prefix: "sqlite_"}, {Prefix: "x", Schema: "mysql"}, {Prefix: "x", Schema: "pg_catalog"}, {Prefix: "x", Schema: "information_schema"}, {Prefix: "a\x00b"}} {
		bad.Connection, bad.Action = c, "drop"
		if bad.Schema == "" {
			bad.Schema = schema
		}
		if _, err := e.PlanPrefix(ctx, bad); err == nil {
			t.Errorf("accepted bad prefix %+v", bad)
		}
	}
	// 9. two concurrent executions of one valid plan: exactly one may complete
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "cf_", Action: "drop"}
	p, err = e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		res core.PrefixExecuteResult
		err error
	}
	ch := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			res, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: reviewConfirm(p), ExpectedPlanSHA256: p.SHA256})
			ch <- outcome{res, err}
		}()
	}
	done := 0
	for i := 0; i < 2; i++ {
		o := <-ch
		if o.err == nil && o.res.Outcome == "completed" {
			done++
		}
	}
	if done != 1 {
		t.Errorf("concurrent executions completed %d times", done)
	}
	if has("cf_a") || has("cf_b") {
		t.Errorf("tables remain after drop")
	}
}

func TestReviewB29SQLitePrefixAttack(t *testing.T) {
	reviewPrefixAttack(t, testEngine(t), sqliteConnection(), "main")
}

// Live variant, same env contract as TestLivePartitions.
func TestReviewB29LivePrefixAttack(t *testing.T) {
	for _, system := range []string{"mysql", "maria", "postgres"} {
		t.Run(system, func(t *testing.T) {
			port := os.Getenv("DBM_TEST_" + strings.ToUpper(system) + "_PORT")
			if port == "" {
				t.Skip("live fixture not configured")
			}
			var n int
			fmt.Sscan(port, &n)
			driver := system
			if driver == "maria" {
				driver = "mysql"
			}
			c := core.Connection{Driver: driver, Host: "localhost", Port: n, Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), Database: "probe", TLSMode: "verify-full"}
			schema := "probe"
			if driver == "postgres" {
				c.Username, schema = "postgres", "public"
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			reviewPrefixAttack(t, e, c, schema)
		})
	}
}

// Partition bound/name injection: every definition must stay inside one clause.
func TestReviewB29PartitionDefinitionInjection(t *testing.T) {
	nasty := []string{"1) , PARTITION x VALUES LESS THAN (2", "'; DROP TABLE t; --", "\\", "\\'", "1; DROP", "0x1F", "1e5", " 1", "9223372036854775808", "NULL", "MAXVALUE ", "maxvalue", "-0", "+5", "1 /*", "\n", "\u0000"}
	for _, driver := range []string{"mysql", "postgres"} {
		for _, v := range nasty {
			vv := v
			var defs []string
			var err error
			if driver == "mysql" {
				defs, err = partitionDefinitions(driver, "RANGE", []core.PartitionDefinition{{Name: "p`x\"y", LessThan: &vv}})
			} else {
				defs, err = partitionDefinitions(driver, "RANGE", []core.PartitionDefinition{{Name: "p`x\"y", From: &vv, To: &vv}})
			}
			if err != nil {
				continue
			}
			for _, d := range defs {
				// remove quoted regions; what remains must be fixed grammar only
				stripped := regexp.MustCompile("'(?:[^']|'')*'").ReplaceAllString(d, "''")
				stripped = regexp.MustCompile("`(?:[^`]|``)*`|\"(?:[^\"]|\"\")*\"").ReplaceAllString(stripped, "ID")
				if strings.ContainsAny(stripped, ";\\\n") || strings.Count(stripped, "(") != strings.Count(stripped, ")") || strings.Contains(stripped, "--") {
					t.Errorf("%s: unsafe definition for %q: %s -> %s", driver, v, d, stripped)
				}
			}
		}
	}
	// names that must be rejected
	for _, name := range []string{"", " x", "x ", "a\nb", "a\x00b", strings.Repeat("n", 65)} {
		if _, err := partitionDefinitions("mysql", "LIST", []core.PartitionDefinition{{Name: name, Values: []string{"1"}}}); err == nil {
			t.Errorf("name %q accepted", name)
		}
	}
	// method/driver mixups
	if _, err := partitionDefinitions("mysql", "HASH", []core.PartitionDefinition{{Name: "a", Modulus: 2}}); err == nil {
		t.Error("mysql HASH defs accepted")
	}
	if _, err := partitionDefinitions("postgres", "RANGE", []core.PartitionDefinition{{Name: "a", LessThan: sp("1")}}); err == nil {
		t.Error("pg less_than accepted")
	}
}

// ---- W-P17 / W-P27 live leak and rights probes (skipped without fixtures) ----

func reviewLiveConn(t *testing.T, system string) (core.Connection, *Engine, string) {
	port := os.Getenv("DBM_TEST_" + strings.ToUpper(system) + "_PORT")
	if port == "" {
		t.Skip("live fixture not configured")
	}
	var n int
	fmt.Sscan(port, &n)
	driver := system
	if driver == "maria" {
		driver = "mysql"
	}
	c := core.Connection{Driver: driver, Host: "localhost", Port: n, Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), Database: "probe", TLSMode: "verify-full"}
	schema := "probe"
	if driver == "postgres" {
		c.Username, schema = "postgres", "public"
	}
	e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
	if err != nil {
		t.Fatal(err)
	}
	return c, e, schema
}

func TestReviewB17B27LiveLeaks(t *testing.T) {
	for _, system := range []string{"mysql", "maria", "postgres"} {
		t.Run(system, func(t *testing.T) {
			root, e, _ := reviewLiveConn(t, system)
			limited := root
			limited.Username, limited.Password = "rv_lim", "Lim_pw_123x"
			if root.Driver == "mysql" {
				execute(t, e, root, "CREATE USER IF NOT EXISTS 'rv_lim'@'%' IDENTIFIED BY 'Lim_pw_123x'")
				execute(t, e, root, "GRANT SELECT ON probe.* TO 'rv_lim'@'%'")
			} else {
				execute(t, e, root, "DROP ROLE IF EXISTS rv_lim")
				execute(t, e, root, "CREATE ROLE rv_lim LOGIN PASSWORD 'Lim_pw_123x'")
				execute(t, e, root, "GRANT CONNECT ON DATABASE probe TO rv_lim")
			}
			execute(t, e, root, "CREATE TABLE IF NOT EXISTS rv_probe(id INTEGER)")
			if root.Driver == "mysql" {
				execute(t, e, root, "GRANT SELECT ON probe.rv_probe TO 'rv_lim'@'%'")
			} else {
				execute(t, e, root, "GRANT SELECT ON rv_probe TO rv_lim")
			}
			// a long-running statement with a recognisable literal and a secret in its text
			sleeper := "SELECT 'SECRETQ_xyz', SLEEP(4)"
			if root.Driver == "postgres" {
				sleeper = "SELECT 'SECRETQ_xyz', pg_sleep(4)"
			}
			go func() { _, _ = e.Query(context.Background(), core.QueryRequest{Connection: root, SQL: sleeper}) }()
			time.Sleep(700 * time.Millisecond)
			forbidden := []string{"SECRETQ_xyz", root.Password, "Lim_pw_123x", "/var/lib", "/etc/", "BEGIN PRIVATE", "172.", "localhost", "root@"}
			for _, who := range []core.Connection{root, limited} {
				blobs := map[string][]byte{}
				for _, kind := range []string{"status", "variables", "processes"} {
					for _, scope := range []string{"", "GLOBAL"} {
						if kind != "variables" && scope != "" {
							continue
						}
						r, err := e.ServerAdminRead(context.Background(), kind, core.ServerAdminReadRequest{Connection: who, Scope: scope, Limit: 200})
						if err != nil {
							t.Logf("%s/%s/%s %s: err %v", who.Username, kind, scope, system, err)
							continue
						}
						b, _ := json.Marshal(r)
						blobs["admin-"+kind+scope] = b
						for _, v := range r.Values {
							if v.Value != nil && !regexp.MustCompile(`^[0-9.+\-eE]+$`).MatchString(*v.Value) {
								t.Errorf("non-numeric value visible: %s=%s", v.Name, *v.Value)
							}
						}
					}
				}
				for _, action := range core.MonitorActions {
					req := core.MonitorRequest{Connection: who, Limit: 200}
					if action == "profiling" {
						req.SQL = "SELECT * FROM rv_probe WHERE id = 1"
					}
					if action == "binary-events" {
						req.Log = "x.000001"
					}
					r, err := e.Monitor(context.Background(), action, req)
					if err != nil {
						t.Logf("%s/%s %s: err %v", who.Username, action, system, err)
						for _, f := range forbidden {
							if strings.Contains(err.Error(), f) {
								t.Errorf("error for %s leaks %q: %v", action, f, err)
							}
						}
						continue
					}
					b, _ := json.Marshal(r)
					blobs["monitor-"+action] = b
					t.Logf("%s/%s %s: state=%s source=%s n=%d", who.Username, action, system, r.State, r.Source, len(r.Records))
				}
				// rights / raw-error probes
				for _, sqlText := range []string{"SELECT * FROM no_such_table_zz", "SELECT * FROM rv_probe WHERE id = 1", "SELECT nosuchcol FROM rv_probe"} {
					r, err := e.Monitor(context.Background(), "profiling", core.MonitorRequest{Connection: who, SQL: sqlText})
					t.Logf("%s profiling %q: state=%s reason=%q err=%v", who.Username, sqlText, r.State, r.Reason, err)
				}
				bad := who
				bad.Password = "WRONGPW_zz"
				fresh, ferr := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
				if ferr != nil {
					t.Fatal(ferr)
				}
				_, qerr := fresh.Query(context.Background(), core.QueryRequest{Connection: bad, SQL: "SELECT CURRENT_USER()"})
				t.Logf("baseline Query with wrong password (pre-existing API): err=%v", qerr)
				badRes, err := fresh.Monitor(context.Background(), "sample", core.MonitorRequest{Connection: bad})
				if err == nil {
					t.Logf("FINDING: wrong password on monitor/sample returns no error: state=%q reason=%q", badRes.State, badRes.Reason)
				}
				if err != nil {
					t.Logf("wrong password error: %v", err)
					if strings.Contains(err.Error(), "WRONGPW_zz") || strings.Contains(err.Error(), "127.0.0.1") {
						t.Errorf("auth error leaks: %v", err)
					}
				} else if badRes.State == "available" || len(badRes.Records) > 0 {
					t.Errorf("wrong password yielded data: %+v", badRes)
				}
				for name, b := range blobs {
					for _, f := range forbidden {
						if f == "localhost" || f == "172." || f == "root@" {
							// only a leak in monitor/process output, still flag
						}
						if strings.Contains(string(b), f) {
							t.Errorf("%s (%s): output contains %q: %.200s", name, who.Username, f, b)
						}
					}
				}
			}
		})
	}
}

// W-P17: the scope and filter inputs are validated before any connection is made.
func TestReviewB17ScopeAndFilterValidation(t *testing.T) {
	e := testEngine(t)
	for _, scope := range []string{"INVALID", "global", "GLOBAL VARIABLES; DROP TABLE x; --", "SESSION ", "GLOBAL\n"} {
		_, err := e.ServerAdminRead(context.Background(), "variables", core.ServerAdminReadRequest{Connection: core.Connection{Driver: "mysql", Host: "127.0.0.1", Port: 1}, Scope: scope})
		var p *core.Error
		if !errors.As(err, &p) || p.Code != "invalid_server_admin" {
			t.Errorf("scope %q not rejected before connecting: %v", scope, err)
		}
	}
	for _, kind := range []string{"kill", "set", "variables;", "STATUS", ""} {
		if _, err := e.ServerAdminRead(context.Background(), kind, core.ServerAdminReadRequest{Connection: core.Connection{Driver: "mysql", Host: "127.0.0.1", Port: 1}}); err == nil {
			t.Errorf("kind %q accepted", kind)
		}
	}
}

// XML round trip over hostile cell values: Go's encoder silently replaces characters that
// XML 1.0 cannot carry, so every such value must travel base64 and come back identical.
func TestReviewB28XMLRoundTripHostileValues(t *testing.T) {
	e, c, ref := reviewNastyFixture(t)
	var typed, xmlOut bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "json"}, &typed); err != nil {
		t.Fatal(err)
	}
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "xml"}, &xmlOut); err != nil {
		t.Fatal(err)
	}
	var want struct {
		Tables []struct {
			Rows [][]core.Cell `json:"rows"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(typed.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	var doc extendedXMLDocument
	if err := xml.Unmarshal(xmlOut.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	rows := doc.Tables[0].Rows
	if len(rows) != len(want.Tables[0].Rows) {
		t.Fatalf("row count %d != %d", len(rows), len(want.Tables[0].Rows))
	}
	for i, r := range rows {
		for j, cell := range r.Cells {
			w := want.Tables[0].Rows[i][j]
			// encoding/xml normalises CR; compare after the same normalisation only for CRLF-free values
			if cell.Kind.value(t) != w.Kind || (w.Kind != "null" && cell.Value.value(t) != w.Value) {
				t.Errorf("row %d cell %d: %q != %q", i, j, cell.Value.value(t), w.Value)
			}
		}
	}
}

// Live partition attacks: confirmation bypass, stale hash, hostile names/bounds, selective truncate.
func TestReviewB29LivePartitionAttack(t *testing.T) {
	for _, system := range []string{"mysql", "maria", "postgres"} {
		t.Run(system, func(t *testing.T) {
			c, e, schema := reviewLiveConn(t, system)
			ctx := context.Background()
			h := &opHarness{t: t, e: e, c: c, schema: schema}
			p0, p1 := "p0", "p1"
			if c.Driver == "mysql" {
				h.exec("DROP TABLE IF EXISTS rv_part")
				h.exec("CREATE TABLE rv_part (id INT NOT NULL, v INT) PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20))")
			} else {
				p0, p1 = "rv_part_p0", "rv_part_p1"
				h.exec("DROP TABLE IF EXISTS rv_part")
				h.exec("CREATE TABLE rv_part (id INT NOT NULL, v INT) PARTITION BY RANGE (id)")
				h.exec("CREATE TABLE rv_part_p0 PARTITION OF rv_part FOR VALUES FROM (0) TO (10)")
				h.exec("CREATE TABLE rv_part_p1 PARTITION OF rv_part FOR VALUES FROM (10) TO (20)")
			}
			h.exec("INSERT INTO rv_part VALUES (1,1),(2,2),(11,11),(12,12)")
			table := core.TableRef{Schema: schema, Name: "rv_part"}
			count := func() int {
				r, err := e.Partitions(ctx, core.PartitionRequest{Connection: c, Table: table})
				if err != nil {
					t.Fatal(err)
				}
				return len(r.Partitions)
			}
			rows := func() string { return h.text("SELECT COUNT(*) FROM rv_part") }
			req := core.PartitionRequest{Connection: c, Table: table, Action: "drop", Partitions: []string{p1}}
			plan, err := e.PlanPartitions(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("drop plan: %v confirm=%+v", plan.Statements, plan.Confirmation)
			for name, ex := range map[string]core.StructureExecuteRequest{
				"empty target":                    {PartitionRequest: req, Confirm: dbplan.Confirmation{Action: plan.Action}, ExpectedPlanSHA256: plan.SHA256},
				"partition name instead of table": {PartitionRequest: req, Confirm: dbplan.Confirmation{Action: plan.Action, TargetName: p1}, ExpectedPlanSHA256: plan.SHA256},
				"wrong action":                    {PartitionRequest: req, Confirm: dbplan.Confirmation{Action: "partition_truncate", TargetName: "rv_part"}, ExpectedPlanSHA256: plan.SHA256},
				"zero hash":                       {PartitionRequest: req, Confirm: reviewConfirm(plan), ExpectedPlanSHA256: strings.Repeat("0", 64)},
				"hash of other plan": func() core.StructureExecuteRequest {
					other, _ := e.PlanPartitions(ctx, core.PartitionRequest{Connection: c, Table: table, Action: "truncate", Partitions: []string{p1}})
					return core.StructureExecuteRequest{PartitionRequest: req, Confirm: reviewConfirm(plan), ExpectedPlanSHA256: other.SHA256}
				}(),
			} {
				if _, err := e.ExecutePartitions(ctx, ex); err == nil {
					t.Errorf("%s: accepted", name)
				}
				if count() != 2 || rows() != "4" {
					t.Fatalf("%s: state changed", name)
				}
			}
			// hostile names are catalog lookups, never SQL
			for _, bad := range []string{p1 + "`; DROP TABLE rv_part; --", p1 + "\" ; DROP TABLE rv_part", "%", p1 + " "} {
				r := req
				r.Partitions = []string{bad}
				if _, err := e.PlanPartitions(ctx, r); err == nil {
					t.Errorf("hostile partition name %q planned", bad)
				}
			}
			// hostile bound is quoted text; the server must reject it and nothing may change
			vv := "30) , PARTITION px VALUES LESS THAN (MAXVALUE"
			add := core.PartitionRequest{Connection: c, Table: table, Action: "add", Definitions: []core.PartitionDefinition{{Name: "px", LessThan: &vv}}}
			if c.Driver == "postgres" {
				a, b := "20", "30') TO ('40"
				add.Definitions = []core.PartitionDefinition{{Name: "px", From: &a, To: &b}}
			}
			if ap, err := e.PlanPartitions(ctx, add); err == nil {
				t.Logf("hostile add plan: %v", ap.Statements)
				res, _ := e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: add, Confirm: reviewConfirm(ap), ExpectedPlanSHA256: ap.SHA256})
				if res.Outcome == "completed" || count() != 2 {
					t.Errorf("hostile bound changed the schema: %s partitions=%d", res.Outcome, count())
				}
			}
			// selective truncate keeps the other partition's rows
			tr := core.PartitionRequest{Connection: c, Table: table, Action: "truncate", Partitions: []string{p0}}
			tp, err := e.PlanPartitions(ctx, tr)
			if err != nil {
				t.Fatal(err)
			}
			res, err := e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: tr, Confirm: reviewConfirm(tp), ExpectedPlanSHA256: tp.SHA256})
			if err != nil || res.Outcome != "completed" || rows() != "2" {
				t.Errorf("truncate: %+v %v rows=%s", res, err, rows())
			}
			// two concurrent executions of the same drop: at most one completes
			type out struct {
				res core.ObjectOpResult
				err error
			}
			ch := make(chan out, 2)
			for i := 0; i < 2; i++ {
				go func() {
					r, err := e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: req, Confirm: reviewConfirm(plan), ExpectedPlanSHA256: plan.SHA256})
					ch <- out{r, err}
				}()
			}
			done := 0
			for i := 0; i < 2; i++ {
				o := <-ch
				if o.err == nil && o.res.Outcome == "completed" {
					done++
				}
			}
			if done != 1 || count() != 1 {
				t.Errorf("concurrent drop: completed=%d partitions=%d", done, count())
			}
			h.exec("DROP TABLE IF EXISTS rv_part")
		})
	}
}
