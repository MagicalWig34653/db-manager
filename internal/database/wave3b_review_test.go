// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// Independent review probes for W-P09 (indexes) and W-P14 (search/replace).
// Live variants need the dbfix environment (DBM_TEST_*_PORT); SQLite always runs.

type w3Rig struct {
	t *testing.T
	e *Engine
	c core.Connection
}

func (r w3Rig) q(n string) string { return quoteIdentifier(r.c.Driver, n) }
func (r w3Rig) exec(s string)     { r.t.Helper(); execute(r.t, r.e, r.c, s) }
func (r w3Rig) tryExec(s string) error {
	_, err := r.e.Query(context.Background(), core.QueryRequest{Connection: r.c, SQL: s})
	return err
}
func (r w3Rig) text(s string) string { r.t.Helper(); return queryText(r.t, r.e, r.c, s) }

func w3bLive(t *testing.T, fn func(w3Rig)) {
	for _, test := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.env)
			if raw == "" {
				t.Skip("fixture required")
			}
			port, _ := strconv.Atoi(raw)
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			fn(w3Rig{t, e, c})
		})
	}
}

func (r w3Rig) idxPlan(req core.IndexesRequest) (core.IndexesPlan, error) {
	req.Connection = r.c
	return r.e.PlanIndexes(context.Background(), req)
}
func (r w3Rig) idxRun(req core.IndexesRequest, p core.IndexesPlan, action, target string) (core.IndexesResult, error) {
	req.Connection = r.c
	req.Confirm = &dbplan.Confirmation{Action: action, TargetName: target}
	req.ExpectedPlanSHA256 = p.SHA256
	return r.e.ExecuteIndexes(context.Background(), req)
}

func w3NoLeak(t *testing.T, what string, texts ...string) {
	t.Helper()
	for _, s := range texts {
		for _, bad := range []string{"ERROR", "Error 1", "SQLSTATE", "pq:", "pgx", "mysql:", "sqlite", "/Users/", "/tmp", "constraint failed", "Cannot drop", "violates", "Duplicate entry"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s: leaks %q in %q", what, bad, s)
			}
		}
	}
}

func w3Indexes(t *testing.T, r w3Rig) {
	ctx := context.Background()
	_ = ctx
	d := r.c.Driver
	// Unicode/case table name.
	tbl := "Üser Tab"
	r.exec("CREATE TABLE " + r.q(tbl) + " (id BIGINT NOT NULL, v VARCHAR(40), w VARCHAR(40))")
	r.exec("INSERT INTO " + r.q(tbl) + " VALUES (1,'a','x'),(2,'b','y')")
	create := core.IndexesRequest{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "ix_ü", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v"}}}}
	p, err := r.idxPlan(create)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := r.idxRun(create, p, "index_create", ""); err != nil || res.Outcome != dbplan.StatusCompleted {
		t.Fatalf("create %#v %v", res, err)
	}
	// Replay of create: must be object_exists (409), not success.
	if _, err := r.idxPlan(create); err == nil {
		t.Error("replayed create plan succeeded")
	} else {
		requireMutationCode(t, err, "object_exists")
	}
	drop := core.IndexesRequest{Table: core.TableRef{Name: tbl}, Action: "index_drop", Name: "ix_ü"}
	dp, err := r.idxPlan(drop)
	if err != nil {
		t.Fatal(err)
	}
	if !dp.Destructive {
		t.Error("drop not destructive")
	}
	for _, bad := range []string{"üser tab", "ÜSER TAB", "Üser Tab ", "", "ix_ü"} {
		_, err = r.idxRun(drop, dp, "index_drop", bad)
		var pe *core.Error
		if !errors.As(err, &pe) || pe.Code != "confirmation_mismatch" {
			t.Errorf("confirm %q accepted: %v", bad, err)
		}
	}
	// wrong action with right name
	_, err = r.idxRun(drop, dp, "index_create", tbl)
	requireMutationCode(t, err, "confirmation_mismatch")
	// case-variant table ref in request
	alt := drop
	alt.Table = core.TableRef{Name: "üSER tAB"}
	if _, err = r.idxPlan(alt); err == nil && d != "mysql" {
		t.Errorf("case-variant table accepted on %s", d)
	}
	res, err := r.idxRun(drop, dp, "index_drop", tbl)
	if err != nil || res.Outcome != dbplan.StatusCompleted {
		t.Fatalf("drop %#v %v", res, err)
	}
	// Replay of drop: object_not_found
	_, err = r.e.ExecuteIndexes(ctx, core.IndexesRequest{Connection: r.c, Table: drop.Table, Action: "index_drop", Name: "ix_ü", Confirm: &dbplan.Confirmation{Action: "index_drop", TargetName: tbl}, ExpectedPlanSHA256: dp.SHA256})
	requireMutationCode(t, err, "object_not_found")

	// Injection attempts through names / expression / direction / referenced names.
	for _, bad := range []core.IndexesRequest{
		{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "x`; DROP TABLE a;--", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v\"; DROP"}}}},
		{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "ok1", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v", Expression: "lower(v)); DROP TABLE x; --"}}}},
		{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "ok2", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v", Direction: "asc; DROP"}}}},
		{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "ok3", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v", PrefixLength: 99999}}}},
		{Table: core.TableRef{Name: tbl}, Action: "index_create", Index: &core.IndexDefinition{Name: "ok4", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "v", PrefixLength: -1}}}},
		{Table: core.TableRef{Name: tbl}, Action: "foreign_key_create", ForeignKey: &core.ForeignKeyDefinition{Name: "fkx", Columns: []string{"id"}, ReferencedTable: core.TableRef{Name: "nonexistent"}, ReferencedColumns: []string{"id"}, OnDelete: "cascade; DROP", OnUpdate: "restrict"}},
		{Table: core.TableRef{Name: tbl}, Action: "index_rename", Name: "ix", NewName: "a\x00b"},
		{Table: core.TableRef{Name: "sqlite_master"}, Action: "index_create", Index: &core.IndexDefinition{Name: "zz", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "name"}}}},
	} {
		if _, err := r.idxPlan(bad); err == nil {
			t.Errorf("accepted %#v", bad.Index)
		} else {
			var pe *core.Error
			if !errors.As(err, &pe) {
				t.Errorf("non-public error %v", err)
			} else {
				w3NoLeak(t, "inject", pe.Message)
			}
		}
	}
	// System tables / schemas.
	sys := []core.TableRef{}
	switch d {
	case "sqlite":
		sys = append(sys, core.TableRef{Name: "sqlite_master"}, core.TableRef{Name: "sqlite_schema"}, core.TableRef{Name: "SQLITE_MASTER"})
	case "mysql":
		sys = append(sys, core.TableRef{Schema: "mysql", Name: "user"}, core.TableRef{Schema: "MYSQL", Name: "user"}, core.TableRef{Schema: "sys", Name: "sys_config"}, core.TableRef{Schema: "performance_schema", Name: "threads"}, core.TableRef{Schema: "information_schema", Name: "TABLES"})
	case "postgres":
		sys = append(sys, core.TableRef{Schema: "pg_catalog", Name: "pg_class"}, core.TableRef{Schema: "information_schema", Name: "tables"}, core.TableRef{Schema: "pg_toast", Name: "x"})
	}
	for _, ref := range sys {
		for _, req := range []core.IndexesRequest{
			{Table: ref, Action: "index_create", Index: &core.IndexDefinition{Name: "rv_sys", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "name"}}}},
			{Table: ref, Action: "primary_drop"},
		} {
			if _, err := r.idxPlan(req); err == nil {
				t.Errorf("system table %v accepted for %s", ref, req.Action)
			}
		}
	}
	if d == "sqlite" {
		// Auto indexes: unique-owned index must not be droppable.
		r.exec("CREATE TABLE uq (id INTEGER, v TEXT UNIQUE, PRIMARY KEY (id))")
		_, err := r.idxPlan(core.IndexesRequest{Table: core.TableRef{Name: "uq"}, Action: "index_drop", Name: "sqlite_autoindex_uq_1"})
		if err == nil {
			t.Error("sqlite autoindex droppable")
		}
		_, err = r.idxPlan(core.IndexesRequest{Table: core.TableRef{Name: "uq"}, Action: "index_create", Index: &core.IndexDefinition{Name: "SQLITE_x", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}})
		if err == nil {
			t.Error("sqlite_ index name accepted")
		}
		return
	}

	// TOCTOU: unique plan clean, duplicate inserted before execution.
	r.exec("CREATE TABLE tt (id BIGINT NOT NULL, v VARCHAR(10))")
	r.exec("INSERT INTO tt VALUES (1,'a'),(2,'b')")
	uq := core.IndexesRequest{Table: core.TableRef{Name: "tt"}, Action: "index_create", Index: &core.IndexDefinition{Name: "uq_v", Kind: "unique", Terms: []core.IndexDefinitionTerm{{Column: "v"}}}}
	up, err := r.idxPlan(uq)
	if err != nil {
		t.Fatal(err)
	}
	r.exec("INSERT INTO tt VALUES (3,'a')")
	res, err = r.idxRun(uq, up, "index_create", "")
	if err != nil || res.Outcome == dbplan.StatusCompleted {
		t.Errorf("unique over duplicates executed: %#v %v", res, err)
	}
	w3NoLeak(t, "dup", res.Message, res.Reason)
	if got := r.text("SELECT COUNT(*) FROM " + r.q("tt")); got != "3" {
		t.Errorf("rows changed %s", got)
	}

	// FK creation over violating data.
	r.exec("CREATE TABLE par (id BIGINT PRIMARY KEY)")
	r.exec("CREATE TABLE chi (pid BIGINT)")
	r.exec("INSERT INTO chi VALUES (99)")
	fk := core.IndexesRequest{Table: core.TableRef{Name: "chi"}, Action: "foreign_key_create", ForeignKey: &core.ForeignKeyDefinition{Name: "fk_chi", Columns: []string{"pid"}, ReferencedTable: core.TableRef{Name: "par"}, ReferencedColumns: []string{"id"}, OnDelete: "restrict", OnUpdate: "restrict"}}
	fp, err := r.idxPlan(fk)
	if err != nil {
		t.Fatalf("fk plan: %v", err)
	}
	if !strings.Contains(strings.Join(fp.Warnings, " "), "ohne passende Elternzeile") {
		t.Errorf("fk plan lacks the orphan pre-check warning: %v", fp.Warnings)
	}
	res, err = r.idxRun(fk, fp, "foreign_key_create", "")
	if res.Outcome == dbplan.StatusCompleted {
		t.Error("fk created over violating data")
	}
	w3NoLeak(t, "fk", res.Message, res.Reason)

	// PK drop with FK dependency + auto-increment.
	if d == "mysql" {
		r.exec("CREATE TABLE ai (id BIGINT AUTO_INCREMENT PRIMARY KEY, v INT)")
		pd := core.IndexesRequest{Table: core.TableRef{Name: "ai"}, Action: "primary_drop"}
		pp, err := r.idxPlan(pd)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(pp.Warnings, " "), "AUTO_INCREMENT") {
			t.Errorf("auto-increment pk drop not warned: %v", pp.Warnings)
		}
		res, err = r.idxRun(pd, pp, "primary_drop", "ai")
		t.Logf("auto-inc pk drop: %#v err=%v", res, err)
		if res.Outcome == dbplan.StatusCompleted {
			t.Error("auto-increment PK dropped")
		}
		w3NoLeak(t, "ai", res.Message, res.Reason)
		// Index carrying an FK.
		r.exec("CREATE TABLE fp (id BIGINT PRIMARY KEY)")
		r.exec("CREATE TABLE fc (pid BIGINT, KEY ix_pid (pid), CONSTRAINT fc_fk FOREIGN KEY (pid) REFERENCES fp (id))")
		di := core.IndexesRequest{Table: core.TableRef{Name: "fc"}, Action: "index_drop", Name: "ix_pid"}
		dpl, err := r.idxPlan(di)
		if err != nil {
			t.Logf("drop fk-index plan: %v", err)
		} else {
			res, err = r.idxRun(di, dpl, "index_drop", "fc")
			t.Logf("drop fk-carrying index: %#v err=%v warnings=%v", res, err, dpl.Warnings)
			w3NoLeak(t, "fkindex", res.Message, res.Reason)
		}
		// MyISAM table
		r.exec("CREATE TABLE my (id INT) ENGINE=MyISAM")
		if _, err := r.idxPlan(core.IndexesRequest{Table: core.TableRef{Name: "my"}, Action: "index_create", Index: &core.IndexDefinition{Name: "ix", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}}); err == nil {
			t.Error("MyISAM accepted")
		}
	}
	if d == "postgres" {
		r.exec("CREATE TABLE pk1 (id BIGINT PRIMARY KEY, v TEXT, n INT)")
		r.exec("CREATE TABLE pk2 (pid BIGINT REFERENCES pk1(id))")
		pd := core.IndexesRequest{Table: core.TableRef{Name: "pk1"}, Action: "primary_drop"}
		pp, err := r.idxPlan(pd)
		if err != nil {
			t.Fatal(err)
		}
		res, err = r.idxRun(pd, pp, "primary_drop", "pk1")
		t.Logf("pk drop with fk: %#v err=%v", res, err)
		if res.Outcome == dbplan.StatusCompleted {
			t.Error("pk dropped despite FK")
		}
		w3NoLeak(t, "pkfk", res.Message, res.Reason)
		// expression on non-text
		if _, err := r.idxPlan(core.IndexesRequest{Table: core.TableRef{Name: "pk1"}, Action: "index_create", Index: &core.IndexDefinition{Name: "e1", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "n", Expression: "lower"}}}}); err == nil {
			t.Error("expression on int accepted")
		}
		// expression unique duplicate groups by lower()
		r.exec("INSERT INTO pk1 VALUES (1,'Abc',1),(2,'aBC',2)")
		ep := core.IndexesRequest{Table: core.TableRef{Name: "pk1"}, Action: "index_create", Index: &core.IndexDefinition{Name: "e2", Kind: "unique", Terms: []core.IndexDefinitionTerm{{Column: "v", Expression: "lower"}}}}
		pl, err := r.idxPlan(ep)
		if err != nil || pl.DuplicateGroups == nil || *pl.DuplicateGroups != 1 {
			t.Errorf("expr unique duplicates not detected: %#v %v", pl.DuplicateGroups, err)
		}
	}
}

func TestReviewW3BIndexesSQLite(t *testing.T) {
	w3Indexes(t, w3Rig{t, testEngine(t), sqliteConnection()})
}
func TestReviewW3BIndexesLive(t *testing.T) { w3bLive(t, func(r w3Rig) { w3Indexes(t, r) }) }

// ---------------- W-P14 ----------------

func (r w3Rig) search(table, term, mode string, cols ...string) (core.SearchResult, error) {
	return r.e.Search(context.Background(), core.SearchRequest{Connection: r.c, Targets: []core.SearchTarget{{Table: core.TableRef{Name: table}, Columns: cols}}, Term: term, Mode: mode, TimeoutMS: 5000})
}

func w3Search(t *testing.T, r w3Rig) {
	d := r.c.Driver
	ctx := context.Background()
	blob := "BLOB"
	if d == "postgres" {
		blob = "BYTEA"
	}
	r.exec("CREATE TABLE s1 (id INTEGER NOT NULL PRIMARY KEY, v VARCHAR(100), b " + blob + ")")
	vals := []string{"100%", "1000", "a_b", "axb", "a!b", "a!!b", "back\\slash", "backXslash", "back\\\\slash", "Straße", "STRASSE", "x'y", "\\_", "%"}
	for i, v := range vals {
		lit := "'" + strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "''") + "'"
		if d == "postgres" || d == "sqlite" {
			lit = "'" + strings.ReplaceAll(v, "'", "''") + "'"
		}
		r.exec("INSERT INTO s1 (id, v) VALUES (" + strconv.Itoa(i+1) + "," + lit + ")")
	}
	count := func(term, mode string) int {
		res, err := r.search("s1", term, mode, "v")
		if err != nil {
			t.Errorf("search %q: %v", term, err)
			return -1
		}
		return res.Tables[0].Count
	}
	for _, c := range []struct {
		term string
		want int
	}{{"%", 2}, {"_", 2}, {"!", 2}, {"\\", 3}, {"a_b", 1}, {"a!b", 1}, {"\\_", 1}, {"back\\slash", 1}, {"100%", 1}, {"strasse", 1}} {
		got := count(c.term, "substring")
		if d == "sqlite" && c.term == "strasse" {
			continue
		}
		if c.term == "strasse" {
			// Case handling is driver dependent and documented in SEARCH-API.md: PostgreSQL LIKE is case-sensitive.
			if d == "postgres" && got != 0 || d == "mysql" && got != 1 {
				t.Errorf("%s case behaviour changed: %d", d, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("%s substring %q = %d want %d", d, c.term, got, c.want)
		}
	}
	if d == "mysql" {
		// NO_BACKSLASH_ESCAPES session mode must not break escaping.
		r.exec("SET GLOBAL sql_mode = CONCAT(@@GLOBAL.sql_mode, ',NO_BACKSLASH_ESCAPES')")
		defer r.exec("SET GLOBAL sql_mode = REPLACE(@@GLOBAL.sql_mode, 'NO_BACKSLASH_ESCAPES', '')")
		e2, _ := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
		r2 := w3Rig{t, e2, r.c}
		for _, c := range []struct {
			term string
			want int
		}{{"%", 2}, {"_", 2}, {"\\", 3}, {"a_b", 1}} {
			res, err := r2.search("s1", c.term, "substring", "v")
			if err != nil || res.Tables[0].Count != c.want {
				t.Errorf("NO_BACKSLASH_ESCAPES %q: %#v %v want %d", c.term, res, err, c.want)
			}
		}
	}
	// Mode / limits
	if _, err := r.search("s1", "x", "bogus", "v"); err == nil {
		t.Error("bogus mode accepted")
	}
	if _, err := r.search("s1", "x", "substring", "b"); err == nil {
		t.Error("binary column searchable")
	}
	if _, err := r.search("s1", "x", "substring", "id"); err == nil {
		t.Error("int column searchable")
	}
	if _, err := r.search("s1", "x", "substring", "v\" OR 1=1 --"); err == nil {
		t.Error("unknown column accepted")
	}
	// Preview truncation
	r.exec("CREATE TABLE s2 (id INTEGER PRIMARY KEY, v TEXT)")
	r.exec("INSERT INTO s2 VALUES (1,'" + strings.Repeat("k", 5000) + "')")
	res, err := r.search("s2", "kk", "substring", "v")
	if err != nil || len(res.Tables[0].Rows[0][0].Value) > 300 {
		t.Errorf("preview not truncated: %v len=%d", err, len(res.Tables[0].Rows[0][0].Value))
	}
	// Term length cap and nul
	if _, err := r.search("s1", strings.Repeat("a", 5000), "substring", "v"); err == nil {
		t.Error("oversized term accepted")
	}
	// Regex error redaction and ReDoS
	if d != "sqlite" {
		_, err = r.search("s1", "([", "regex", "v")
		var pe *core.Error
		if err == nil || !errors.As(err, &pe) {
			t.Errorf("bad regex: %v", err)
		} else {
			w3NoLeak(t, "regex", pe.Message)
			t.Logf("bad regex error: %s / %s", pe.Code, pe.Message)
		}
		r.exec("CREATE TABLE s3 (id INTEGER PRIMARY KEY, v VARCHAR(100))")
		r.exec("INSERT INTO s3 VALUES (1,'" + strings.Repeat("a", 40) + "b')")
		start := time.Now()
		_, err = r.e.Search(ctx, core.SearchRequest{Connection: r.c, Targets: []core.SearchTarget{{Table: core.TableRef{Name: "s3"}, Columns: []string{"v"}}}, Term: "^(a+)+$", Mode: "regex", TimeoutMS: 2000})
		el := time.Since(start)
		t.Logf("ReDoS probe: elapsed=%v err=%v", el, err)
		if el > 8*time.Second {
			t.Errorf("ReDoS search not bounded: %v", el)
		}
		// server side still running?
		time.Sleep(2 * time.Second)
		var running string
		switch d {
		case "mysql":
			running = r.text("SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE INFO LIKE '%a+)+%' AND INFO NOT LIKE '%PROCESSLIST%'")
		case "postgres":
			running = r.text("SELECT COUNT(*) FROM pg_stat_activity WHERE state='active' AND query LIKE '%a+)+%' AND query NOT LIKE '%pg_stat_activity%'")
		}
		t.Logf("server-side still running after cancel: %s", running)
		if running != "0" {
			t.Errorf("search query still running on server after cancellation: %s", running)
		}
	}
}

func TestReviewW3BSearchSQLite(t *testing.T) {
	w3Search(t, w3Rig{t, testEngine(t), sqliteConnection()})
}
func TestReviewW3BSearchLive(t *testing.T) { w3bLive(t, func(r w3Rig) { w3Search(t, r) }) }

func (r w3Rig) replace(table, col, term, repl, mode string) (core.ReplacePlan, core.ReplaceRequest, error) {
	req := core.ReplaceRequest{Connection: r.c, Table: core.TableRef{Name: table}, Column: &col, Term: term, Replacement: &repl, Mode: mode, TimeoutMS: 10000}
	p, err := r.e.ReplacePlan(context.Background(), req)
	return p, req, err
}
func (r w3Rig) replaceRun(req core.ReplaceRequest, p core.ReplacePlan) (core.ReplaceResult, error) {
	return r.e.ReplaceExecute(context.Background(), core.ReplaceExecuteRequest{ReplaceRequest: req, Confirm: core.ReplaceConfirm{Action: "replace", TargetName: req.Table.Name, AffectedRows: p.AffectedRows}, ExpectedPlanSHA256: p.SHA256})
}

func w3Replace(t *testing.T, r w3Rig) {
	d := r.c.Driver
	r.exec("CREATE TABLE r1 (id INTEGER NOT NULL PRIMARY KEY, v VARCHAR(50) UNIQUE, w VARCHAR(50), n INTEGER)")
	r.exec("INSERT INTO r1 VALUES (1,'a','keep',1),(2,'b','keep',2),(3,'ab',NULL,3),(4,'cd','x',4)")
	state := func() string {
		return r.text("SELECT " + map[string]string{"sqlite": "group_concat(coalesce(v,'~') || ':' || coalesce(w,'~'), '|')", "mysql": "GROUP_CONCAT(CONCAT(IFNULL(v,'~'),':',IFNULL(w,'~')) ORDER BY id SEPARATOR '|')", "postgres": "string_agg(coalesce(v,'~') || ':' || coalesce(w,'~'), '|' ORDER BY id)"}[d] + " FROM r1")
	}
	before := state()

	// PK / non-text / unknown column refused.
	for _, col := range []string{"id", "n", "nope"} {
		if _, _, err := r.replace("r1", col, "1", "2", "substring"); err == nil {
			t.Errorf("replace in %s planned", col)
		}
	}
	// UNIQUE violation mid-way: a->b collides with existing b.
	p, req, err := r.replace("r1", "v", "a", "b", "exact")
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.replaceRun(req, p)
	t.Logf("unique collision: affected=%d res=%#v err=%v", p.AffectedRows, res, err)
	if res.Outcome == dbplan.StatusCompleted {
		t.Error("unique collision completed")
	}
	if state() != before {
		t.Errorf("partial state after collision: %s vs %s", state(), before)
	}
	// Substring with LIKE-case-insensitive collation: 'Ab' vs 'ab' - affected count must reflect real changes.
	p, req, err = r.replace("r1", "w", "KEEP", "z", "substring")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("case-insensitive substring KEEP->z affected=%d (db %s)", p.AffectedRows, d)
	// Injection through replacement text, regex backreferences.
	evil := "'); DROP TABLE r1; -- \\1 $1 %"
	p, req, err = r.replace("r1", "w", "keep", evil, "substring")
	if err != nil || p.AffectedRows != 2 {
		t.Fatalf("plan %#v %v", p, err)
	}
	w3NoLeak(t, "plan", strings.Join(p.Warnings, " "))
	// Stale: change an unrelated column after planning.
	r.exec("UPDATE r1 SET n = 77 WHERE id = 3")
	_, err = r.replaceRun(req, p)
	var pe *core.Error
	if !errors.As(err, &pe) || pe.Code != "plan_mismatch" {
		t.Errorf("stale plan executed: %v", err)
	}
	// Insert between plan and execute.
	p, req, _ = r.replace("r1", "w", "keep", evil, "substring")
	r.exec("INSERT INTO r1 VALUES (9,'zz','keep',9)")
	_, err = r.replaceRun(req, p)
	if !errors.As(err, &pe) || pe.Code != "plan_mismatch" {
		t.Errorf("phantom insert not detected: %v", err)
	}
	r.exec("DELETE FROM r1 WHERE id = 9")
	// Wrong confirmation variants.
	p, req, _ = r.replace("r1", "w", "keep", evil, "substring")
	for _, cf := range []core.ReplaceConfirm{{Action: "replace", TargetName: "R1", AffectedRows: p.AffectedRows}, {Action: "replace", TargetName: "r1", AffectedRows: p.AffectedRows + 1}, {Action: "update", TargetName: "r1", AffectedRows: p.AffectedRows}} {
		_, err = r.e.ReplaceExecute(context.Background(), core.ReplaceExecuteRequest{ReplaceRequest: req, Confirm: cf, ExpectedPlanSHA256: p.SHA256})
		if err == nil {
			t.Errorf("confirm %#v accepted", cf)
		}
	}
	res, err = r.replaceRun(req, p)
	if err != nil || res.Outcome != dbplan.StatusCompleted || res.AffectedRows != 2 {
		t.Fatalf("run %#v %v", res, err)
	}
	if got := r.text("SELECT w FROM r1 WHERE id = 1"); got != evil {
		t.Errorf("replacement stored as %q", got)
	}
	// Replay of the same plan after completion must not re-apply.
	_, err = r.replaceRun(req, p)
	if err == nil {
		t.Error("replay accepted")
	}
	// NULL untouched
	if got := r.text("SELECT COUNT(*) FROM r1 WHERE w IS NULL"); got != "1" {
		t.Errorf("NULL changed: %s", got)
	}
	if d == "sqlite" {
		return
	}
	// Regex backreference semantics.
	r.exec("CREATE TABLE r2 (id INTEGER PRIMARY KEY, v VARCHAR(50))")
	r.exec("INSERT INTO r2 VALUES (1,'foo123'),(2,'bar')")
	repl := "<$1>"
	if d == "postgres" || d == "mysql" && strings.Contains(r.text("SELECT VERSION()"), "Maria") {
		repl = "<\\1>"
	}
	p, req, err = r.replace("r2", "v", "([0-9]+)", repl, "regex")
	if err != nil {
		t.Fatalf("regex plan: %v", err)
	}
	t.Logf("regex backref plan: affected=%d examples=%+v", p.AffectedRows, p.Examples)
	if res, err = r.replaceRun(req, p); err != nil || res.Outcome != dbplan.StatusCompleted {
		t.Errorf("regex run %#v %v", res, err)
	}
	t.Logf("regex result stored: %s", r.text("SELECT v FROM r2 WHERE id = 1"))
	// invalid regex
	if _, _, err = r.replace("r2", "v", "([", "x", "regex"); err == nil {
		t.Error("invalid regex planned")
	} else if errors.As(err, &pe) {
		w3NoLeak(t, "regex plan", pe.Message)
	}
	// Time/other columns: on-update timestamp table.
	if d == "mysql" {
		r.exec("CREATE TABLE r3 (id INT PRIMARY KEY, v VARCHAR(20), upd TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)")
		r.exec("INSERT INTO r3 (id,v,upd) VALUES (1,'foo','2020-01-01 00:00:00')")
		_, _, err = r.replace("r3", "v", "foo", "bar", "exact")
		requireMutationCode(t, err, "unsupported_search")
		if r.text("SELECT v FROM r3 WHERE id=1") != "foo" {
			t.Error("data changed")
		}
		// generated column table
		r.exec("CREATE TABLE r4 (id INT PRIMARY KEY, v VARCHAR(20), g VARCHAR(30) AS (CONCAT(v,'!')) STORED)")
		r.exec("INSERT INTO r4 (id,v) VALUES (1,'foo')")
		if _, _, err = r.replace("r4", "g", "foo", "bar", "substring"); err == nil {
			t.Error("generated column replace planned")
		}
		p, req, err = r.replace("r4", "v", "foo", "bar", "exact")
		if err == nil {
			res, err = r.replaceRun(req, p)
			t.Logf("generated dependent replace: %#v err=%v", res, err)
		}
	}
	// ReDoS in replace plan
	r.exec("CREATE TABLE r5 (id INTEGER PRIMARY KEY, v VARCHAR(100))")
	r.exec("INSERT INTO r5 VALUES (1,'" + strings.Repeat("a", 40) + "b')")
	req = core.ReplaceRequest{Connection: r.c, Table: core.TableRef{Name: "r5"}, Column: ptr("v"), Term: "^(a+)+$", Replacement: ptr("x"), Mode: "regex", TimeoutMS: 2000}
	start := time.Now()
	_, err = r.e.ReplacePlan(context.Background(), req)
	t.Logf("replace ReDoS plan: elapsed=%v err=%v", time.Since(start), err)
	if time.Since(start) > 8*time.Second {
		t.Error("replace regex plan unbounded")
	}
}

func ptr(s string) *string { return &s }

func TestReviewW3BReplaceSQLite(t *testing.T) {
	w3Replace(t, w3Rig{t, testEngine(t), sqliteConnection()})
}
func TestReviewW3BReplaceLive(t *testing.T) { w3bLive(t, func(r w3Rig) { w3Replace(t, r) }) }

func TestReviewW3BCancelLive(t *testing.T) {
	w3bLive(t, func(r w3Rig) {
		if r.c.Driver == "sqlite" {
			return
		}
		r.exec("CREATE TABLE cx (id INTEGER PRIMARY KEY, v VARCHAR(100))")
		r.exec("INSERT INTO cx VALUES (1,'" + strings.Repeat("a", 30) + "')")
		for _, pat := range []string{"^(a*)*\\1b", "(a|aa)+$b", "^(a?){30}a{30}b", "^(a+)+\\1b", "(.*)(.*)(.*)(.*)(.*)(.*)(.*)(.*)b"} {
			start := time.Now()
			_, err := r.e.Search(context.Background(), core.SearchRequest{Connection: r.c, Targets: []core.SearchTarget{{Table: core.TableRef{Name: "cx"}, Columns: []string{"v"}}}, Term: pat, Mode: "regex", TimeoutMS: 1500})
			el := time.Since(start)
			var running string
			time.Sleep(1500 * time.Millisecond)
			if r.c.Driver == "mysql" {
				running = r.text("SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE INFO LIKE '%cx%' AND INFO NOT LIKE '%PROCESSLIST%'")
			} else {
				running = r.text("SELECT COUNT(*) FROM pg_stat_activity WHERE state='active' AND query LIKE '%FROM \"public\".\"cx\"%'")
			}
			t.Logf("pattern %q: elapsed=%v err=%v server_active=%s", pat, el, err != nil, running)
		}
	})
}

func TestReviewW3BScanCancelLive(t *testing.T) {
	w3bLive(t, func(r w3Rig) {
		r.exec("CREATE TABLE n10 (i INTEGER)")
		r.exec("INSERT INTO n10 VALUES (0),(1),(2),(3),(4),(5),(6),(7),(8),(9)")
		r.exec("CREATE TABLE big (id INTEGER PRIMARY KEY, v VARCHAR(200))")
		for batch := 0; batch < 8; batch++ {
			r.exec("INSERT INTO big SELECT a.i+10*b.i+100*c.i+1000*d.i+10000*e.i+" + strconv.Itoa(batch*100000) + ", " + map[string]string{"mysql": "REPEAT('abcdefghij', 15)", "postgres": "repeat('abcdefghij', 15)"}[r.c.Driver] + " FROM n10 a, n10 b, n10 c, n10 d, n10 e")
		}
		for _, mode := range []string{"substring", "regex"} {
			term := "zzzz"
			if mode == "regex" {
				term = "(j.*){14}k"
			}
			start := time.Now()
			_, err := r.e.Search(context.Background(), core.SearchRequest{Connection: r.c, Targets: []core.SearchTarget{{Table: core.TableRef{Name: "big"}, Columns: []string{"v"}}}, Term: term, Mode: mode, TimeoutMS: 300})
			el := time.Since(start)
			seen := []string{}
			for i := 0; i < 12; i++ {
				time.Sleep(500 * time.Millisecond)
				var n string
				if r.c.Driver == "mysql" {
					n = r.text("SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE COMMAND IN ('Query','Execute') AND ID<>CONNECTION_ID()")
				} else {
					n = r.text("SELECT COUNT(*) FROM pg_stat_activity WHERE state='active' AND query LIKE '%FROM \"public\".\"big\" WHERE%' AND pid<>pg_backend_pid()")
				}
				seen = append(seen, n)
			}
			for _, n := range seen[4:] {
				if n != "0" {
					t.Errorf("%s: server query still active after the client deadline (%v, err=%v): %v", mode, el, err != nil, seen)
					break
				}
			}
		}
	})
}
