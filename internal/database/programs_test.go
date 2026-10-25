// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func dumpProgram(t *testing.T, component string, value any) {
	t.Helper()
	if dir := os.Getenv("DBM_PROGRAMS_DUMP"); dir != "" {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.ReplaceAll(t.Name(), "/", "_") + "." + component + ".json"
		if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func programCode(t *testing.T, err error, code string) {
	t.Helper()
	var p *core.Error
	if !errors.As(err, &p) || p.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
func programApply(t *testing.T, e *Engine, r core.ProgramChangeRequest) core.ProgramPlan {
	t.Helper()
	ctx := context.Background()
	p, err := e.ProgramPlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.ProgramPlan(ctx, r)
	if err != nil || again.SHA256 != p.SHA256 {
		t.Fatalf("unstable plan: %v", err)
	}
	dumpProgram(t, "ProgramPlan", p)
	req := core.ProgramExecuteRequest{ProgramChangeRequest: r, Confirm: dbplan.Confirmation{Action: r.Action, TargetName: p.Object.Name}, ExpectedPlanSHA256: p.SHA256}
	bad := req
	bad.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	_, err = e.ProgramExecute(ctx, bad)
	programCode(t, err, "plan_mismatch")
	bad = req
	bad.Confirm.Action = "other"
	_, err = e.ProgramExecute(ctx, bad)
	programCode(t, err, "confirmation_mismatch")
	if p.Destructive {
		bad = req
		bad.Confirm.TargetName = "wrong"
		_, err = e.ProgramExecute(ctx, bad)
		programCode(t, err, "confirmation_mismatch")
	}
	result, err := e.ProgramExecute(ctx, req)
	if err != nil || result.Outcome != "completed" {
		t.Fatalf("execute %#v %v; statements=%q", result, err, p.Statements)
	}
	dumpProgram(t, "ProgramResult", result)
	return p
}
func programReadOne(t *testing.T, e *Engine, c core.Connection, ref core.ProgramRef) core.Program {
	t.Helper()
	result, err := e.Programs(context.Background(), core.ProgramsRequest{Connection: c, Object: &ref})
	if err != nil || len(result.Objects) != 1 {
		t.Fatalf("describe %#v %v", result, err)
	}
	dumpProgram(t, "ProgramsResult", result)
	return result.Objects[0]
}
func TestProgramsSQLite(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE source(value INTEGER)`)
	execute(t, e, c, `INSERT INTO source VALUES (4)`)
	execute(t, e, c, `CREATE TABLE neighbor(value TEXT)`)
	execute(t, e, c, `INSERT INTO neighbor VALUES ('keep')`)
	execute(t, e, c, `CREATE TABLE audit(value INTEGER)`)
	view := core.ProgramRef{Kind: "view", Name: "v odd\"`.;DROP TABLE neighbor--"}
	request := core.ProgramChangeRequest{Connection: c, Action: "create", Object: view, Input: &core.ProgramInput{Body: `SELECT value*2 AS doubled FROM source`}}
	p := programApply(t, e, request)
	if !p.UserSQL || p.Destructive {
		t.Fatal("flags")
	}
	got := programReadOne(t, e, c, view)
	if got.Definition == nil || !strings.Contains(*got.Definition, "SELECT") || len(got.Dependencies) == 0 {
		t.Fatalf("definition %#v", got)
	}
	if queryText(t, e, c, `SELECT doubled FROM `+quoteIdentifier("sqlite", view.Name)) != "8" {
		t.Fatal("view result")
	}
	request.Action = "replace"
	request.Input.Body = `SELECT value*3 AS doubled FROM source`
	oldPlan, err := e.ProgramPlan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	changed := request
	other := *request.Input
	other.Body = `SELECT 42 AS doubled`
	changed.Input = &other
	_, err = e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: changed, Confirm: dbplan.Confirmation{Action: "replace", TargetName: view.Name}, ExpectedPlanSHA256: oldPlan.SHA256})
	programCode(t, err, "plan_mismatch")
	p = programApply(t, e, request)
	if len(p.Statements) != 2 || !p.Destructive || !p.Transactional {
		t.Fatal("replace")
	}
	if queryText(t, e, c, `SELECT doubled FROM `+quoteIdentifier("sqlite", view.Name)) != "12" {
		t.Fatal("changed view")
	}
	trigger := core.ProgramRef{Kind: "trigger", Name: "t odd\"`.;DROP TABLE neighbor--", Table: &core.TableRef{Name: "source"}}
	request = core.ProgramChangeRequest{Connection: c, Action: "create", Object: trigger, Input: &core.ProgramInput{Body: `BEGIN INSERT INTO audit VALUES(NEW.value); INSERT INTO audit VALUES(NEW.value+1); END`, Timing: "after", Event: "insert"}}
	p = programApply(t, e, request)
	if len(p.Statements) != 1 {
		t.Fatal("compound body split")
	}
	execute(t, e, c, `INSERT INTO source VALUES(7)`)
	if queryText(t, e, c, `SELECT sum(value) FROM audit`) != "15" {
		t.Fatal("trigger effect")
	}
	got = programReadOne(t, e, c, trigger)
	if got.Object.Table == nil || got.Object.Table.Name != "source" {
		t.Fatal("trigger table")
	}
	request.Action = "replace"
	request.Input.Body = `BEGIN INSERT INTO audit VALUES(100); END`
	programApply(t, e, request)
	execute(t, e, c, `INSERT INTO source VALUES(9)`)
	if queryText(t, e, c, `SELECT sum(value) FROM audit`) != "115" {
		t.Fatal("replaced trigger")
	}
	// A failed replacement must roll back its earlier DROP.
	request.Input.Body = `BEGIN INVALID SQL; END`
	p, err = e.ProgramPlan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: request, Confirm: dbplan.Confirmation{Action: "replace", TargetName: trigger.Name}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || result.Outcome != "rejected" {
		t.Fatalf("failed replace %#v %v", result, err)
	}
	programReadOne(t, e, c, trigger)
	// Trailing SQL is rejected by the driver parser, including after a valid trigger body.
	for _, body := range []string{`BEGIN SELECT 1; END; DROP TABLE neighbor`, `BEGIN SELECT 1; END; ATTACH DATABASE '/private/forbidden' AS outside`} {
		request.Action = "create"
		request.Object.Name = "injected"
		request.Input.Body = body
		p, err = e.ProgramPlan(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		result, err = e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: request, Confirm: dbplan.Confirmation{Action: "create"}, ExpectedPlanSHA256: p.SHA256})
		if err != nil || result.Outcome != "rejected" {
			t.Fatalf("batch %#v %v", result, err)
		}
		_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &request.Object})
		programCode(t, err, "object_not_found")
	}
	resultList, err := e.Programs(ctx, core.ProgramsRequest{Connection: c, Kind: "trigger", Table: &core.TableRef{Name: "source"}, Limit: 1})
	if err != nil || len(resultList.Objects) != 1 {
		t.Fatalf("table filter %v", err)
	}
	programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: trigger})
	execute(t, e, c, `INSERT INTO source VALUES(10)`)
	if queryText(t, e, c, `SELECT sum(value) FROM audit`) != "115" {
		t.Fatal("dropped trigger still active")
	}
	programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: view})
	_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &view})
	programCode(t, err, "object_not_found")
	if queryText(t, e, c, `SELECT value FROM neighbor`) != "keep" {
		t.Fatal("neighbor changed")
	}
	if queryText(t, e, c, `SELECT count(*) FROM source`) != "4" {
		t.Fatal("base data changed")
	}
	for _, kind := range []string{"function", "procedure", "event"} {
		_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Kind: kind})
		programCode(t, err, "unsupported_action")
	}
	for _, schema := range []string{"temp", "other", "main; DROP TABLE neighbor"} {
		_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Schema: schema})
		programCode(t, err, "invalid_program")
	}
}
func TestProgramsValidation(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	base := core.ProgramChangeRequest{Connection: c, Action: "create", Object: core.ProgramRef{Kind: "view", Name: "v"}, Input: &core.ProgramInput{Body: "SELECT 1"}}
	cases := []core.ProgramChangeRequest{}
	for _, action := range []string{"call", "execute", "scheduler", "alter", ""} {
		r := base
		r.Action = action
		cases = append(cases, r)
	}
	for _, name := range []string{"", "sqlite_bad", "bad\x00name", strings.Repeat("x", 256)} {
		r := base
		r.Object.Name = name
		cases = append(cases, r)
	}
	for _, input := range []*core.ProgramInput{nil, {Body: ""}, {Body: strings.Repeat("x", core.ProgramsMaxBodyBytes+1)}, {Body: "SELECT 1", Timing: "after"}, {Body: "SELECT 1", Security: "definer"}, {Body: "SELECT 1", Definer: &core.ProgramDefiner{Name: "root"}}} {
		r := base
		r.Input = input
		cases = append(cases, r)
	}
	r := base
	r.Action = "drop"
	cases = append(cases, r)
	for i, r := range cases {
		if _, err := e.ProgramPlan(ctx, r); err == nil {
			t.Fatalf("invalid case %d accepted", i)
		}
	}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		caps := core.ProgramsCapabilities()
		if caps.Get(driver, "routine_execute").Support != dbplan.Unsupported || caps.Get(driver, "scheduler_write").Support != dbplan.Unsupported {
			t.Fatal("unsafe capability")
		}
	}
	for _, schedule := range []*core.ProgramSchedule{nil, {Every: 1, Unit: "second;DROP"}, {At: "2026-99-99 00:00:00"}, {Every: -1, Unit: "day"}, {At: "2026-01-01 00:00:00", Every: 1}, {Every: 1, Unit: "day", Starts: "2026-02-01 00:00:00", Ends: "2026-01-01 00:00:00"}} {
		if _, err := programSchedule(schedule); err == nil {
			t.Fatal("invalid schedule")
		}
	}
}

func TestProgramDialectStatements(t *testing.T) {
	ctx := context.Background()
	for _, driver := range []string{"mysql", "postgres"} {
		env := &opEnv{c: core.Connection{Driver: driver}}
		in := &core.ProgramInput{Body: "BEGIN RETURN 1; END", Returns: "integer", Parameters: []core.ProgramParameter{{Name: "arg`\";", Mode: "in", Type: "integer"}}}
		if driver == "postgres" {
			in.Language = "plpgsql"
		}
		req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "function", Schema: "quoted`\"schema", Name: "fun`\";name"}, Input: in}
		statements, err := programCreate(ctx, env, req, nil)
		if err != nil || len(statements) != 1 || !strings.Contains(statements[0], "BEGIN RETURN 1; END") {
			t.Fatalf("%s compound %#v %v", driver, statements, err)
		}
		bad := *in
		bad.Parameters = []core.ProgramParameter{{Name: "p", Mode: "in", Type: "integer); DROP TABLE secret--"}}
		req.Input = &bad
		if _, err = programCreate(ctx, env, req, nil); err == nil {
			t.Fatal("type injection")
		}
		req.Input = in
		req.Action = "replace"
		if driver == "mysql" {
			existing := newProgram(req.Object)
			statements, err = programCreate(ctx, env, req, &existing)
			if err != nil || len(statements) != 2 || !strings.HasPrefix(statements[0], "DROP FUNCTION ") {
				t.Fatalf("mysql replace %#v %v", statements, err)
			}
		}
	}
	env := &opEnv{c: core.Connection{Driver: "postgres"}}
	ref := core.ProgramRef{Kind: "trigger", Schema: "public", Name: "a.b\";DROP", Table: &core.TableRef{Schema: "public", Name: "t.x\""}}
	drop, err := programDrop(env, ref)
	if err != nil || drop != `DROP TRIGGER "a.b"";DROP" ON "public"."t.x"""` {
		t.Fatalf("trigger quoting %q %v", drop, err)
	}
	schedule, err := programSchedule(&core.ProgramSchedule{At: "2099-01-01 00:00:00", Preserve: true})
	if err != nil || schedule != "AT '2099-01-01 00:00:00' ON COMPLETION PRESERVE" {
		t.Fatal(schedule, err)
	}
}
