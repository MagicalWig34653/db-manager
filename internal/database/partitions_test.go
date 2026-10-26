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
	dumpPartition(t, "StructureResult", result)
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
