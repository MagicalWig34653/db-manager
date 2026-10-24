// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (h *opHarness) wantFail(err error, res core.ObjectOpResult, what string) {
	h.t.Helper()
	if err == nil && res.Outcome == "completed" {
		h.t.Fatalf("%s: action succeeded", what)
	}
}

// reviewConfirmation tries to bypass every confirmation rule of destructive plans.
func reviewConfirmation(t *testing.T, h *opHarness) {
	t.Helper()
	for _, n := range []string{"rv_victim", "rv_other", "Rv_Victim_é"} {
		h.exec("CREATE TABLE " + h.q(n) + "(id INTEGER PRIMARY KEY, v TEXT)")
		h.exec("INSERT INTO " + h.q(n) + " VALUES(1,'a'),(2,'b')")
	}
	for _, action := range []string{core.OpTableDrop, core.OpTableTruncate, core.OpTableRename} {
		req := core.ObjectOpRequest{Table: h.ref("rv_victim")}
		if action == core.OpTableRename {
			req.NewName = sp("rv_renamed")
		}
		p := h.mustPlan(action, req)
		other := core.ObjectOpRequest{Table: h.ref("rv_other")}
		if action == core.OpTableRename {
			other.NewName = sp("rv_renamed2")
		}
		otherPlan := h.mustPlan(action, other)
		good := core.ObjectOpConfirm{Action: action, TargetName: "rv_victim"}
		for name, try := range map[string]struct {
			confirm core.ObjectOpConfirm
			sha     string
			code    string
		}{
			"no name":        {core.ObjectOpConfirm{Action: action}, p.PlanSHA256, "confirmation_mismatch"},
			"upper":          {core.ObjectOpConfirm{Action: action, TargetName: "RV_VICTIM"}, p.PlanSHA256, "confirmation_mismatch"},
			"trailing space": {core.ObjectOpConfirm{Action: action, TargetName: "rv_victim "}, p.PlanSHA256, "confirmation_mismatch"},
			"leading space":  {core.ObjectOpConfirm{Action: action, TargetName: " rv_victim"}, p.PlanSHA256, "confirmation_mismatch"},
			"other target":   {core.ObjectOpConfirm{Action: action, TargetName: "rv_other"}, p.PlanSHA256, "confirmation_mismatch"},
			"wrong action":   {core.ObjectOpConfirm{Action: core.OpTableOptions, TargetName: "rv_victim"}, p.PlanSHA256, "confirmation_mismatch"},
			"swapped plan":   {good, otherPlan.PlanSHA256, "plan_mismatch"},
			"zero hash":      {good, strings.Repeat("0", 64), "plan_mismatch"},
			"upper hash":     {good, strings.ToUpper(p.PlanSHA256), "invalid_object_op"},
			"short hash":     {good, p.PlanSHA256[:63], "invalid_object_op"},
			"empty hash":     {good, "", "invalid_object_op"},
		} {
			r := req
			r.Connection = h.c
			res, err := h.e.ObjectOpExecute(context.Background(), action, core.ObjectOpExecuteRequest{ObjectOpRequest: r, Confirm: try.confirm, ExpectedPlanSHA256: try.sha})
			h.wantFail(err, res, action+"/"+name)
			h.wantCode(err, try.code)
		}
		if !h.has("rv_victim") || h.count("rv_victim") != "2" || h.has("rv_renamed") {
			t.Fatalf("%s: refused confirmation changed state", action)
		}
	}
	// Unicode normalization: a decomposed typed name must not equal the composed catalog name.
	composed := "Rv_Victim_é"
	decomposed := "Rv_Victim_é"
	p := h.mustPlan(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref(composed)})
	req := core.ObjectOpRequest{Table: h.ref(composed), Connection: h.c}
	res, err := h.e.ObjectOpExecute(context.Background(), core.OpTableDrop, execWith(req, core.ObjectOpConfirm{Action: core.OpTableDrop, TargetName: decomposed}, p.PlanSHA256))
	h.wantFail(err, res, "NFD typed name")
	h.wantCode(err, "confirmation_mismatch")
	// A request naming the table with a different spelling is not resolved (no case folding).
	for _, spelling := range []string{"RV_VICTIM", "rv_victim ", "rv_victim​"} {
		if _, err := h.plan(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref(spelling)}); err == nil {
			t.Fatalf("spelling %q resolved to a table", spelling)
		}
	}
	// Correct run, then replay of the identical confirmation.
	p, res2 := h.run(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("rv_victim")})
	h.completed(res2)
	if h.has("rv_victim") || !h.has("rv_other") || h.count("rv_other") != "2" {
		t.Fatal("drop radius")
	}
	r := core.ObjectOpRequest{Table: h.ref("rv_victim"), Connection: h.c}
	_, err = h.e.ObjectOpExecute(context.Background(), core.OpTableDrop, execWith(r, core.ObjectOpConfirm{Action: core.OpTableDrop, TargetName: "rv_victim"}, p.PlanSHA256))
	h.wantCode(err, "object_not_found")
	// Truncate replay: documented behavior (hash is stateless), pinned here as a finding.
	h.exec("CREATE TABLE " + h.q("rv_replay") + "(id INTEGER PRIMARY KEY)")
	h.exec("INSERT INTO " + h.q("rv_replay") + " VALUES(1)")
	tp, tres := h.run(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("rv_replay")})
	h.completed(tres)
	h.exec("INSERT INTO " + h.q("rv_replay") + " VALUES(7)")
	rr := core.ObjectOpRequest{Table: h.ref("rv_replay"), Connection: h.c}
	again, err := h.e.ObjectOpExecute(context.Background(), core.OpTableTruncate, execWith(rr, core.ObjectOpConfirm{Action: core.OpTableTruncate, TargetName: "rv_replay"}, tp.PlanSHA256))
	t.Logf("truncate replay after refill: err=%v outcome=%s rows=%s (stateless digest, see finding M1)", err, again.Outcome, h.count("rv_replay"))
}

// reviewInjection feeds hostile new names to rename/copy and checks the exact result.
func reviewInjection(t *testing.T, h *opHarness) {
	t.Helper()
	h.exec("CREATE TABLE " + h.q("rv_src") + "(id INTEGER PRIMARY KEY, v TEXT)")
	h.exec("INSERT INTO " + h.q("rv_src") + " VALUES(1,'a')")
	h.exec("CREATE TABLE " + h.q("rv_bystander") + "(id INTEGER PRIMARY KEY)")
	max := map[string]int{"mysql": 64, "postgres": 63, "sqlite": 255}[h.c.Driver]
	names := []string{"a`b", `a"b`, "a'b", "x;DROP TABLE rv_bystander;--", "`; DROP TABLE rv_bystander; --", "ünï-名前", "a b", strings.Repeat("n", max), "%_"}
	if h.c.Driver != "mysql" { // the shared test literal helper does not escape backslashes for MySQL
		names = append(names, "a\\b")
	}
	bad := []string{"", " lead", "trail ", "nul\x00x", "ctrl\x07x", "new\nline", strings.Repeat("n", max+1), "\xff\xfe"}
	if h.c.Driver == "sqlite" {
		bad = append(bad, "sqlite_x", "SQLITE_y")
	}
	for _, n := range bad {
		for _, action := range []string{core.OpTableRename, core.OpTableCopy} {
			if _, err := h.plan(action, core.ObjectOpRequest{Table: h.ref("rv_src"), NewName: sp(n)}); err == nil {
				t.Fatalf("%s accepted bad name %q", action, n)
			}
		}
	}
	for i, n := range names {
		_, res := h.run(core.OpTableCopy, core.ObjectOpRequest{Table: h.ref("rv_src"), NewName: sp(n)})
		h.completed(res)
		if !h.has(n) || h.count(n) != "1" {
			t.Fatalf("copy %d %q missing", i, n)
		}
		renamed := n + "_r"
		if len(renamed) > max {
			renamed = "r" + strconv.Itoa(i)
		}
		_, res = h.run(core.OpTableRename, core.ObjectOpRequest{Table: h.ref(n), NewName: sp(renamed)})
		h.completed(res)
		if h.has(n) || !h.has(renamed) {
			t.Fatalf("rename %q", n)
		}
	}
	if !h.has("rv_bystander") || !h.has("rv_src") || h.count("rv_src") != "1" {
		t.Fatal("injection changed other tables")
	}
	// Option strings: comment and unknown engine/collation never reach SQL unchecked.
	if h.c.Driver == "mysql" {
		for _, bad := range []string{"x'; DROP TABLE rv_bystander;--", "utf8mb4_bin; DROP TABLE rv_bystander", "no`such"} {
			if _, err := h.plan(core.OpTableOptions, core.ObjectOpRequest{Table: h.ref("rv_src"), Collation: sp(bad)}); err == nil {
				t.Fatalf("collation %q accepted", bad)
			}
			if _, err := h.plan(core.OpTableOptions, core.ObjectOpRequest{Table: h.ref("rv_src"), Engine: sp(bad)}); err == nil {
				t.Fatalf("engine %q accepted", bad)
			}
		}
	}
	if h.c.Driver != "sqlite" {
		hostile := "it's \\ `x` \"y\"; DROP TABLE rv_bystander; --\n\tend"
		_, res := h.run(core.OpTableOptions, core.ObjectOpRequest{Table: h.ref("rv_src"), Comment: sp(hostile)})
		h.completed(res)
		query := "SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_NAME='rv_src' AND TABLE_SCHEMA=" + sqlText(h.schema)
		if h.c.Driver == "postgres" {
			query = "SELECT obj_description(c.oid,'pg_class') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relname='rv_src' AND n.nspname=" + sqlText(h.schema)
		}
		if got := h.text(query); got != hostile || !h.has("rv_bystander") {
			t.Fatalf("comment round trip %q", got)
		}
	}
}

// reviewRadius checks that DROP/TRUNCATE touch only the target.
func reviewRadius(t *testing.T, h *opHarness, otherSchema string) {
	t.Helper()
	h.exec("CREATE TABLE " + h.q("rv_t") + "(id INTEGER PRIMARY KEY)")
	h.exec("INSERT INTO " + h.q("rv_t") + " VALUES(1),(2)")
	h.exec("CREATE VIEW " + h.q("rv_view") + " AS SELECT id FROM " + h.q("rv_t"))
	h.exec("CREATE TABLE " + h.q("rv_child") + "(id INTEGER PRIMARY KEY, p INTEGER, FOREIGN KEY (p) REFERENCES " + h.q("rv_t") + "(id))")
	h.exec("INSERT INTO " + h.q("rv_child") + " VALUES(1,1)")
	if otherSchema != "" {
		h.exec("CREATE TABLE " + quoteIdentifier(h.c.Driver, otherSchema) + "." + quoteIdentifier(h.c.Driver, "rv_t") + "(id INTEGER PRIMARY KEY)")
	}
	// FK child blocks both; nothing changes (no CASCADE).
	for _, action := range []string{core.OpTableTruncate, core.OpTableDrop} {
		_, res := h.run(action, core.ObjectOpRequest{Table: h.ref("rv_t")})
		if res.Outcome == "completed" && h.c.Driver != "sqlite" {
			t.Fatalf("%s succeeded despite FK child", action)
		}
		if h.c.Driver != "sqlite" && (h.count("rv_t") != "2" || h.count("rv_child") != "1") {
			t.Fatalf("%s changed data despite refusal: %#v", action, res)
		}
	}
	if h.c.Driver == "sqlite" {
		return
	}
	if otherSchema != "" {
		if _, ok := h.hasOther(otherSchema, "rv_t"); !ok {
			t.Fatal("same-named table in other schema lost")
		}
	}
}

func (h *opHarness) hasOther(schema, name string) (bool, bool) {
	r := h.hasIn(schema, name)
	return r, r
}

// reviewPartitionRadius: PostgreSQL TRUNCATE/DROP on a parent also hits children.
func reviewPartitionRadius(t *testing.T, h *opHarness) {
	t.Helper()
	h.exec("CREATE TABLE " + h.q("rv_parent") + "(id INTEGER)")
	h.exec("CREATE TABLE " + h.q("rv_inh_child") + "() INHERITS (" + h.q("rv_parent") + ")")
	h.exec("INSERT INTO " + h.q("rv_inh_child") + " VALUES(1),(2),(3)")
	p, err := h.plan(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("rv_parent")})
	if err != nil {
		t.Logf("inheritance parent plan refused: %v", err)
		return
	}
	mentions := strings.Contains(strings.Join(p.Warnings, " "), "rv_inh_child") || strings.Contains(strings.Join(p.AffectedObjects, " "), "rv_inh_child")
	res, err := h.execute(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("rv_parent")}, p, core.ObjectOpConfirm{Action: core.OpTableTruncate, TargetName: "rv_parent"})
	if err != nil || res.Outcome != "completed" || !mentions || h.count("rv_inh_child") != "3" || !strings.Contains(p.Statements[0], "TRUNCATE TABLE ONLY") {
		t.Fatalf("inheritance: outcome=%s err=%v child rows=%s mentions=%v statements=%v", res.Outcome, err, h.count("rv_inh_child"), mentions, p.Statements)
	}
	h.exec("CREATE TABLE " + h.q("rv_part") + "(id INTEGER, k INTEGER) PARTITION BY LIST (k)")
	h.exec("CREATE TABLE " + h.q("rv_part_1") + " PARTITION OF " + h.q("rv_part") + " FOR VALUES IN (1)")
	h.exec("INSERT INTO " + h.q("rv_part") + " VALUES(1,1)")
	pp, err := h.plan(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("rv_part")})
	if err != nil {
		t.Logf("partitioned plan refused: %v", err)
		return
	}
	res, err = h.execute(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("rv_part")}, pp, core.ObjectOpConfirm{Action: core.OpTableDrop, TargetName: "rv_part"})
	if !strings.Contains(strings.Join(pp.AffectedObjects, " "), "rv_part_1") || !strings.Contains(strings.Join(pp.Warnings, " "), "rv_part_1") {
		t.Fatalf("partition not named in plan: affected=%v warnings=%v", pp.AffectedObjects, pp.Warnings)
	}
	_ = res
}

func reviewAll(t *testing.T, e *Engine, c core.Connection, schema, other string) {
	h := &opHarness{t: t, e: e, c: c, schema: schema}
	reviewConfirmation(t, h)
	reviewInjection(t, h)
	reviewRadius(t, h, other)
	if c.Driver == "postgres" {
		reviewPartitionRadius(t, h)
	}
}

func TestReviewObjectOpsSQLite(t *testing.T) {
	reviewAll(t, testEngine(t), core.Connection{Driver: "sqlite", Database: "rvops"}, "", "")
}

func TestReviewObjectOpsLive(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("disposable TLS fixture required")
			}
			port, _ := strconv.Atoi(raw)
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
			create := "CREATE DATABASE "
			if c.Driver == "postgres" {
				c.Username, create = "postgres", "CREATE SCHEMA "
			}
			execute(t, e, c, create+quoteIdentifier(c.Driver, "rv_main"))
			execute(t, e, c, create+quoteIdentifier(c.Driver, "rv_other"))
			reviewAll(t, e, c, "rv_main", "rv_other")
			reviewCurrentDatabase(t, e, c)
		})
	}
}

// reviewCurrentDatabase: dropping the connection database needs the second acknowledgement.
func reviewCurrentDatabase(t *testing.T, e *Engine, c core.Connection) {
	if c.Driver == "postgres" {
		return
	}
	h := &opHarness{t: t, e: e, c: c}
	db := "rv_current"
	h.exec("CREATE DATABASE " + quoteIdentifier("mysql", db))
	cc := c
	cc.Database = db
	h = &opHarness{t: t, e: e, c: cc}
	req := core.ObjectOpRequest{Database: sp(db)}
	p := h.mustPlan(core.OpDatabaseDrop, req)
	if !p.Confirmation.CurrentDatabase {
		t.Fatal("current database not flagged")
	}
	req.Connection = cc
	res, err := h.e.ObjectOpExecute(context.Background(), core.OpDatabaseDrop, execWith(req, core.ObjectOpConfirm{Action: core.OpDatabaseDrop, TargetName: db}, p.PlanSHA256))
	h.wantFail(err, res, "no acknowledgement")
	h.wantCode(err, "confirmation_mismatch")
	res, err = h.e.ObjectOpExecute(context.Background(), core.OpDatabaseDrop, execWith(req, core.ObjectOpConfirm{Action: core.OpDatabaseDrop, TargetName: db, AcknowledgeCurrentDatabase: false}, p.PlanSHA256))
	h.wantCode(err, "confirmation_mismatch")
	// Case variant of the connection database spelling: must not skip the acknowledgement.
	up := cc
	up.Database = strings.ToUpper(db)
	pu, err := h.e.ObjectOpPlan(context.Background(), core.OpDatabaseDrop, core.ObjectOpRequest{Connection: up, Database: sp(db)})
	t.Logf("case-variant connection database: err=%v currentFlag=%v", err, pu.Confirmation.CurrentDatabase)
	if cnt := h.text("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=" + sqlText(db)); cnt != "1" {
		t.Fatal("database dropped without acknowledgement")
	}
	h.exec("DROP DATABASE " + quoteIdentifier("mysql", db))
}

func execWith(r core.ObjectOpRequest, confirm core.ObjectOpConfirm, sha string) core.ObjectOpExecuteRequest {
	return core.ObjectOpExecuteRequest{ObjectOpRequest: r, Confirm: confirm, ExpectedPlanSHA256: sha}
}
