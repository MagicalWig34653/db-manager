// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func TestSQLitePartitionsUnsupported(t *testing.T) {
	e := testEngine(t)
	r := core.PartitionRequest{Connection: sqliteConnection(), Table: core.TableRef{Name: "x"}}
	_, err := e.Partitions(context.Background(), r)
	requireMutationCode(t, err, "unsupported_action")
	for _, a := range []string{"define", "add", "drop", "remove", "truncate", "analyze", "attach", "detach"} {
		r.Action = a
		_, err = e.PlanPartitions(context.Background(), r)
		requireMutationCode(t, err, "unsupported_action")
	}
}
func TestPartitionDefinitionsAreData(t *testing.T) {
	for _, driver := range []string{"mysql", "postgres"} {
		defs, err := partitionDefinitions(driver, "LIST", []core.PartitionDefinition{{Name: "odd`\"name", Values: []string{"1", "x'); DROP TABLE keep; --"}}})
		if err != nil || len(defs) != 1 || !strings.Contains(defs[0], "'x''); DROP TABLE keep; --'") {
			t.Fatalf("unsafe bound: %v %v", defs, err)
		}
		_, err = partitionDefinitions(driver, "LIST", []core.PartitionDefinition{{Name: "x", Values: []string{"1"}, LessThan: sp("2")}})
		if err == nil {
			t.Fatal("foreign bound accepted")
		}
	}
	_, err := partitionDefinitions("mysql", "RANGE", []core.PartitionDefinition{{Name: "x", LessThan: sp("10")}, {Name: "x", LessThan: sp("MAXVALUE")}})
	if err == nil {
		t.Fatal("duplicate partition accepted")
	}
}
func TestSQLitePrefixBatch(t *testing.T) { prefixCases(t, testEngine(t), sqliteConnection(), "main") }
func TestSQLitePrefixPartialFailureReport(t *testing.T) {
	prefixPartialCase(t, testEngine(t), sqliteConnection(), "main")
}

// prefixPartialCase drops a batch whose second table is referenced from outside
// the batch. Transactional systems must report a clean rollback; MySQL/MariaDB
// keep the first drop and must report exactly which table is applied or unknown.
func prefixPartialCase(t *testing.T, e *Engine, c core.Connection, schema string) {
	ctx := context.Background()
	h := &opHarness{t: t, e: e, c: c, schema: schema}
	h.exec("CREATE TABLE " + h.q("pp_a") + " (id INTEGER PRIMARY KEY)")
	h.exec("CREATE TABLE " + h.q("pp_b") + " (id INTEGER PRIMARY KEY)")
	h.exec("INSERT INTO " + h.q("pp_b") + " VALUES (1)")
	h.exec("CREATE TABLE " + h.q("qq_child") + " (id INTEGER PRIMARY KEY, b INTEGER, FOREIGN KEY (b) REFERENCES " + quoteIdentifier(c.Driver, "pp_b") + "(id))")
	h.exec("INSERT INTO " + h.q("qq_child") + " VALUES (1,1)")
	r := core.PrefixRequest{Connection: c, Schema: schema, Prefix: "pp_", Action: "drop"}
	p, err := e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: p.SHA256})
	if err != nil {
		t.Fatal(err)
	}
	dumpPartition(t, "PrefixStructureResult", res)
	if len(res.Tables) != 2 || res.Outcome == "completed" {
		t.Fatalf("expected a failed two-table batch: %+v", res)
	}
	if c.Driver == "mysql" {
		if res.Outcome != "outcome_unknown" || res.Tables[0].Status != "applied" || res.Tables[1].Status != "unknown" || h.has("pp_a") {
			t.Fatalf("mysql partial state: %+v", res)
		}
		if !strings.Contains(res.Message, "Teilzustand") {
			t.Fatalf("partial state not announced: %s", res.Message)
		}
	} else if res.Outcome != "rejected" || res.Tables[0].Status != "not_applied" || res.Tables[1].Status != "not_applied" || !h.has("pp_a") || !h.has("pp_b") {
		t.Fatalf("transactional batch not rolled back: %+v", res)
	}
	if h.count("qq_child") != "1" {
		t.Fatal("unrelated table changed")
	}
}
func prefixCases(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	ctx := context.Background()
	h := &opHarness{t: t, e: e, c: c, schema: schema}
	for _, name := range []string{"p29_%one", "p29_%two", "p29_neighbor", "p29_keep"} {
		h.exec("CREATE TABLE " + h.q(name) + " (id INTEGER)")
		h.exec("INSERT INTO " + h.q(name) + " VALUES (7)")
	}
	r := core.PrefixRequest{Connection: c, Schema: schema, Prefix: "p29_%"}
	list, err := e.PrefixTables(ctx, r)
	if err != nil || len(list.Tables) != 2 {
		t.Fatalf("literal prefix: %v %v", list, err)
	}
	r.Action = "copy"
	r.NewPrefix = "p29_copy_"
	p, err := e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	dumpPartition(t, "StructurePlan", p)
	result, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || result.Outcome != "completed" {
		t.Fatalf("copy: %+v %v", result, err)
	}
	if h.count("p29_copy_one") != "1" || h.count("p29_copy_two") != "1" {
		t.Fatal("copy data mismatch")
	}
	for _, action := range []string{"truncate", "replace", "add", "remove", "drop"} {
		r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "p29_%", Action: action}
		if action == "replace" {
			r.NewPrefix = "p29_new_"
		}
		if action == "add" {
			r.NewPrefix = "extra_"
		}
		p, err = e.PlanPrefix(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: "wrong"}, ExpectedPlanSHA256: p.SHA256})
		requireMutationCode(t, err, "confirmation_mismatch")
		_, err = e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: strings.Repeat("0", 64)})
		requireMutationCode(t, err, "plan_mismatch")
	}

	// Apply a sequence to copies and verify each step independently in the catalog.
	for _, step := range []struct{ prefix, action, newPrefix, expected string }{
		{"p29_copy_", "replace", "p29_ren_", "p29_ren_one"},
		{"p29_ren_", "add", "z_", "z_p29_ren_one"},
		{"z_", "remove", "", "p29_ren_one"},
		{"p29_ren_", "truncate", "", "p29_ren_one"},
	} {
		request := core.PrefixRequest{Connection: c, Schema: schema, Prefix: step.prefix, Action: step.action, NewPrefix: step.newPrefix}
		plan, err := e.PlanPrefix(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		result, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: request, Confirm: dbplan.Confirmation{Action: plan.Action, TargetName: plan.Confirmation.TargetName}, ExpectedPlanSHA256: plan.SHA256})
		if err != nil || result.Outcome != "completed" {
			t.Fatalf("%s: %+v %v", step.action, result, err)
		}
		if !h.has(step.expected) {
			t.Fatalf("missing %s", step.expected)
		}
		want := "1"
		if step.action == "truncate" {
			want = "0"
		}
		if h.count(step.expected) != want {
			t.Fatalf("row mismatch after %s", step.action)
		}
	}
	r = core.PrefixRequest{Connection: c, Schema: schema, Prefix: "p29_%", Action: "drop"}
	p, err = e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	// Set drift changes the single batch hash and must not touch any old table.
	h.exec("CREATE TABLE " + h.q("p29_%three") + " (id INTEGER)")
	_, err = e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: p.SHA256})
	requireMutationCode(t, err, "plan_mismatch")
	if h.count("p29_%one") != "1" {
		t.Fatal("rejected batch changed data")
	}
	p, err = e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	result, err = e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || result.Outcome != "completed" {
		t.Fatalf("drop %+v %v", result, err)
	}
	dumpPartition(t, "PrefixStructureResult", result)
	if len(result.Tables) == 0 {
		t.Fatal("prefix result without per-table states")
	}
	for _, item := range result.Tables {
		if item.Status != "applied" || item.StatementsExecuted != item.StatementsTotal {
			t.Fatalf("table state: %+v", item)
		}
	}
	if h.has("p29_%one") || h.count("p29_neighbor") != "1" || h.count("p29_keep") != "1" {
		t.Fatal("unrelated table changed")
	}
}
func TestSQLitePrefixLimit(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for i := 0; i < 101; i++ {
		execute(t, e, c, fmt.Sprintf("CREATE TABLE limit_%03d (id INTEGER)", i))
	}
	r := core.PrefixRequest{Connection: c, Prefix: "limit_", Action: "drop"}
	_, err := e.PlanPrefix(context.Background(), r)
	requireMutationCode(t, err, "result_too_large")
}
func TestSQLitePrefixTransactionalFailure(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	for _, sql := range []string{"CREATE TABLE fail_a(id INTEGER PRIMARY KEY)", "INSERT INTO fail_a VALUES(1)", "CREATE TABLE fail_b(id INTEGER PRIMARY KEY)", "INSERT INTO fail_b VALUES(1)", "CREATE TABLE child(id INTEGER REFERENCES fail_b(id))", "INSERT INTO child VALUES(1)"} {
		execute(t, e, c, sql)
	}
	r := core.PrefixRequest{Connection: c, Prefix: "fail_", Action: "drop"}
	p, err := e.PlanPrefix(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.ExecutePrefix(ctx, core.PrefixExecuteRequest{PrefixRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || result.Outcome != "rejected" {
		t.Fatalf("%+v %v", result, err)
	}
	if queryText(t, e, c, "SELECT count(*) FROM fail_a") != "1" {
		t.Fatal("batch did not roll back")
	}
	if len(result.Tables) != 2 || result.Tables[0].Status != "not_applied" || result.Tables[1].Status != "not_applied" {
		t.Fatalf("per-table state after rollback: %+v", result.Tables)
	}
}

func TestPrefixTableResultsPartialStates(t *testing.T) {
	groups := []prefixGroup{{table: core.TableRef{Name: "a"}, count: 1}, {table: core.TableRef{Name: "b"}, count: 2}, {table: core.TableRef{Name: "c"}, count: 1}}
	statuses := func(items []core.PrefixTableResult) string {
		out := []string{}
		for _, i := range items {
			out = append(out, i.Status)
		}
		return strings.Join(out, ",")
	}
	cases := []struct {
		name, driver string
		res          core.ObjectOpResult
		want         string
	}{
		{"completed", "mysql", core.ObjectOpResult{Outcome: "completed", StatementsExecuted: 4}, "applied,applied,applied"},
		{"mysql stops in the second table", "mysql", core.ObjectOpResult{Outcome: "outcome_unknown", StatementsExecuted: 2}, "applied,unknown,not_applied"},
		{"mysql stops at a table boundary", "mysql", core.ObjectOpResult{Outcome: "outcome_unknown", StatementsExecuted: 1}, "applied,unknown,not_applied"},
		{"mysql first statement rejected", "mysql", core.ObjectOpResult{Outcome: "rejected"}, "not_applied,not_applied,not_applied"},
		{"transactional rollback", "postgres", core.ObjectOpResult{Outcome: "rejected", StatementsExecuted: 0}, "not_applied,not_applied,not_applied"},
		{"transactional unknown commit", "postgres", core.ObjectOpResult{Outcome: "outcome_unknown", StatementsExecuted: 4}, "unknown,unknown,unknown"},
	}
	for _, c := range cases {
		if got := statuses(prefixTableResults(c.driver, c.res, groups)); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
	// Partly executed multi-statement table is never reported as applied.
	items := prefixTableResults("mysql", core.ObjectOpResult{Outcome: "outcome_unknown", StatementsExecuted: 2}, groups)
	if items[1].StatementsExecuted != 1 || items[1].StatementsTotal != 2 {
		t.Fatalf("partial table counters: %+v", items[1])
	}
}

func TestStructureOutcomes(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE outcome_fixture(id INTEGER)")
	env, closeAll, err := e.openOp(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := structurePlan("sqlite", "prefix_drop", "outcome_", []string{"DELETE FROM outcome_fixture", "not valid SQL"}, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	// Reuse a real SQLite connection to inject a nontransactional second failure.
	env.c.Driver = "mysql"
	res := executeStructure(ctx, env, p)
	if res.Outcome != "outcome_unknown" || res.StatementsExecuted != 1 {
		t.Fatalf("partial: %+v", res)
	}
	env.sess.conn.Close()
	res = executeStructure(ctx, env, p)
	closeAll()
	if res.Outcome != "outcome_unknown" {
		t.Fatalf("closed connection: %+v", res)
	}
	dumpPartition(t, "StructureResult", res)
}
