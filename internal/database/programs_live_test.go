// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// TestLivePrograms uses only disposable dbfix fixtures. It never invokes a
// routine or enables a scheduler. Trigger effects use an ordinary table insert.
func TestLivePrograms(t *testing.T) {
	for _, system := range []struct{ name, driver, port string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.port)
			if raw == "" {
				t.Skip("disposable dbfix fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: system.driver, Database: "probe", Host: "localhost", Port: port, Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Password == "" {
				c.Password = "public-b1-fixture"
			}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			schedulerBefore := ""
			if c.Driver == "mysql" {
				schedulerBefore = queryText(t, e, c, "SELECT @@GLOBAL.event_scheduler")
			}
			schema := "probe"
			if c.Driver == "postgres" {
				schema = "public"
			}
			ctx := context.Background()
			execute(t, e, c, `CREATE TABLE p19_source(value INTEGER)`)
			execute(t, e, c, `INSERT INTO p19_source VALUES (4)`)
			execute(t, e, c, `CREATE TABLE p19_audit(value INTEGER)`)
			execute(t, e, c, `CREATE TABLE p19_neighbor(value TEXT)`)
			execute(t, e, c, `INSERT INTO p19_neighbor VALUES ('keep')`)
			view := core.ProgramRef{Kind: "view", Schema: schema, Name: "p19 odd`view\".;DROP TABLE p19_neighbor--"}
			r := core.ProgramChangeRequest{Connection: c, Action: "create", Object: view, Input: &core.ProgramInput{Body: "SELECT value*2 AS doubled FROM p19_source", Security: "invoker"}}
			if c.Driver == "mysql" {
				current := queryText(t, e, c, "SELECT CURRENT_USER()")
				split := strings.LastIndex(current, "@")
				r.Input.Definer = &core.ProgramDefiner{Name: current[:split], Host: current[split+1:]}
			} else {
				r.Input.Definer = &core.ProgramDefiner{Name: "postgres"}
			}
			programApply(t, e, r)
			one := programReadOne(t, e, c, view)
			if one.Definition == nil || one.Security == nil || *one.Security != "invoker" || one.Definer == nil {
				t.Fatalf("view details %#v", one)
			}
			q, _ := opQualified(c.Driver, schema, view.Name)
			if queryText(t, e, c, "SELECT doubled FROM "+q) != "8" {
				t.Fatal("view effect")
			}
			r.Action = "replace"
			r.Input.Body = "SELECT value*3 AS doubled FROM p19_source"
			programApply(t, e, r)
			if queryText(t, e, c, "SELECT doubled FROM "+q) != "12" {
				t.Fatal("view replacement")
			}
			// Independent dependency and RESTRICT behavior: a referencing view blocks
			// PostgreSQL DROP, while MySQL reports the textual dependency.
			dq, _ := opQualified(c.Driver, schema, "p19_dependent")
			execute(t, e, c, "CREATE VIEW "+dq+" AS SELECT * FROM "+q)
			one = programReadOne(t, e, c, view)
			found := false
			for _, d := range one.Dependencies {
				found = found || d.Direction == "incoming" && d.Object.Name == "p19_dependent"
			}
			if !found {
				t.Fatalf("missing incoming dependency in %#v (object %#v)", one.Dependencies, one.Object)
			}
			if c.Driver == "postgres" {
				plan, err := e.ProgramPlan(ctx, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: view})
				if err != nil {
					t.Fatal(err)
				}
				result, err := e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: core.ProgramChangeRequest{Connection: c, Action: "drop", Object: view}, Confirm: dbplan.Confirmation{Action: "drop", TargetName: view.Name}, ExpectedPlanSHA256: plan.SHA256})
				if err != nil || result.Outcome != "rejected" || result.Reason != "dependency_blocks" {
					t.Fatalf("restrict %#v %v", result, err)
				}
			}
			execute(t, e, c, "DROP VIEW p19_dependent")
			// A hidden schema must not leak through dependency responses, even if
			// the caller has SELECT on an object inside it.
			if c.Driver == "postgres" {
				execute(t, e, c, "CREATE ROLE p19_dependency_reader LOGIN PASSWORD 'public-p19-dependency-fixture'")
				execute(t, e, c, "CREATE SCHEMA p19_private")
				execute(t, e, c, "CREATE VIEW p19_private.hidden_view AS SELECT * FROM "+q)
				execute(t, e, c, "GRANT USAGE ON SCHEMA public TO p19_dependency_reader")
				execute(t, e, c, "GRANT SELECT ON "+q+", p19_private.hidden_view TO p19_dependency_reader")
				reader := c
				reader.Username, reader.Password = "p19_dependency_reader", "public-p19-dependency-fixture"
				details := programReadOne(t, e, reader, view)
				for _, dep := range details.Dependencies {
					if dep.Object.Schema == "p19_private" {
						t.Fatal("dependency disclosed an inaccessible schema")
					}
				}
				execute(t, e, c, "GRANT USAGE ON SCHEMA p19_private TO p19_dependency_reader")
				details = programReadOne(t, e, reader, view)
				visible := false
				for _, dep := range details.Dependencies {
					visible = visible || dep.Object.Schema == "p19_private" && dep.Object.Name == "hidden_view"
				}
				if !visible {
					t.Fatal("visible dependency missing")
				}
				execute(t, e, c, "DROP VIEW p19_private.hidden_view")
				execute(t, e, c, "DROP SCHEMA p19_private")
			}
			// Both routine kinds: metadata, native export, structured replace and DROP.
			// Their bodies are never invoked, including during validation.
			for _, kind := range []string{"function", "procedure"} {
				input := &core.ProgramInput{Body: "BEGIN SELECT 1; END", Parameters: []core.ProgramParameter{{Name: "p", Mode: "in", Type: "integer"}}, Security: "invoker", Deterministic: true}
				if kind == "function" {
					input.Returns = "integer"
					input.Body = "BEGIN RETURN p+1; END"
				}
				if kind == "procedure" {
					input.Parameters = append(input.Parameters, core.ProgramParameter{Name: "answer", Mode: "inout", Type: "integer"})
					input.Body = "BEGIN SET answer=p+1; END"
				}
				if c.Driver == "postgres" {
					input.Language = "plpgsql"
					input.Deterministic = false
					if kind == "function" {
						input.Body = "BEGIN RETURN p+1; END"
					} else {
						input.Body = "BEGIN answer := p+1; END"
					}
				}
				ref := core.ProgramRef{Kind: kind, Schema: schema, Name: "p19_" + kind}
				req := core.ProgramChangeRequest{Connection: c, Action: "create", Object: ref, Input: input}
				programApply(t, e, req)
				list, err := e.Programs(ctx, core.ProgramsRequest{Connection: c, Schema: schema, Kind: kind})
				if err != nil {
					t.Fatal(err)
				}
				for _, item := range list.Objects {
					if item.Object.Name == ref.Name {
						ref = item.Object
					}
				}
				details := programReadOne(t, e, c, ref)
				if len(details.Parameters) != len(input.Parameters) || details.Parameters[0].Name != "p" || details.Definition == nil || details.Definer == nil {
					t.Fatalf("routine metadata %#v", details)
				}
				req.Object = ref
				req.Action = "replace"
				if kind == "function" {
					input.Body = "BEGIN RETURN p+2; END"
				} else if c.Driver == "mysql" {
					input.Body = "BEGIN SET answer=p+2; END"
				}
				programApply(t, e, req)
				details = programReadOne(t, e, c, ref)
				if details.Body == nil || !strings.Contains(*details.Body, input.Body) {
					t.Fatalf("routine body %#v", details)
				}
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: ref})
				_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &ref})
				programCode(t, err, "object_not_found")
			}
			var triggerFunction *core.ProgramRef
			if c.Driver == "postgres" {
				ref := core.ProgramRef{Kind: "function", Schema: schema, Name: "p19_trigger_fn"}
				triggerFunction = &ref
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "create", Object: ref, Input: &core.ProgramInput{Body: "BEGIN INSERT INTO p19_audit VALUES(NEW.value); RETURN NEW; END", Returns: "trigger", Language: "plpgsql", Security: "invoker"}})
			}
			trigger := core.ProgramRef{Kind: "trigger", Schema: schema, Name: "p19 odd`trig\".;", Table: &core.TableRef{Schema: schema, Name: "p19_source"}}
			input := &core.ProgramInput{Timing: "after", Event: "insert", Body: "BEGIN INSERT INTO p19_audit VALUES(NEW.value); END"}
			if c.Driver == "postgres" {
				input.Body = ""
				input.Function = triggerFunction
			}
			req := core.ProgramChangeRequest{Connection: c, Action: "create", Object: trigger, Input: input}
			programApply(t, e, req)
			programReadOne(t, e, c, trigger)
			execute(t, e, c, "INSERT INTO p19_source VALUES(7)")
			if queryText(t, e, c, "SELECT sum(value) FROM p19_audit") != "7" {
				t.Fatal("trigger effect")
			}
			req.Action = "replace"
			programApply(t, e, req)
			execute(t, e, c, "INSERT INTO p19_source VALUES(8)")
			if queryText(t, e, c, "SELECT sum(value) FROM p19_audit") != "15" {
				t.Fatal("replace trigger effect")
			}
			list, err := e.Programs(ctx, core.ProgramsRequest{Connection: c, Kind: "trigger", Schema: schema, Table: trigger.Table})
			if err != nil || len(list.Objects) != 1 {
				t.Fatalf("table trigger list %v", err)
			}
			if c.Driver == "mysql" {
				req.Input.Body = "BEGIN INVALID SQL; END"
				plan, err := e.ProgramPlan(ctx, req)
				if err != nil {
					t.Fatal(err)
				}
				result, err := e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: req, Confirm: dbplan.Confirmation{Action: "replace", TargetName: trigger.Name}, ExpectedPlanSHA256: plan.SHA256})
				if err != nil || result.Outcome != "outcome_unknown" || result.StatementsExecuted != 1 {
					t.Fatalf("partial DDL %#v %v", result, err)
				}
				_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &trigger})
				programCode(t, err, "object_not_found")
				req.Action = "create"
				req.Input.Body = "BEGIN INSERT INTO p19_audit VALUES(NEW.value); END"
				programApply(t, e, req)
			}
			if c.Driver == "postgres" {
				replacement := core.ProgramRef{Kind: "function", Schema: schema, Name: "p19_trigger_fn_new"}
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "create", Object: replacement, Input: &core.ProgramInput{Body: "BEGIN RETURN NEW; END", Returns: "trigger", Language: "plpgsql"}})
				req.Action, req.Input.Function = "replace", &replacement
				plan, err := e.ProgramPlan(ctx, req)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, dep := range plan.Dependencies {
					found = found || dep.Direction == "outgoing" && dep.Object.Name == replacement.Name
				}
				if !found {
					t.Fatal("planned trigger function missing from dependencies")
				}
				programApply(t, e, req)
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: trigger})
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: replacement})
			} else {
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: trigger})
			}
			// Driver parsing must reject appended statements without applying either.
			injection := core.ProgramChangeRequest{Connection: c, Action: "create", Object: core.ProgramRef{Kind: "view", Schema: schema, Name: "p19_injected"}, Input: &core.ProgramInput{Body: "SELECT 1 AS value; DROP TABLE p19_neighbor"}}
			badPlan, err := e.ProgramPlan(ctx, injection)
			if err != nil {
				t.Fatal(err)
			}
			result, err := e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: injection, Confirm: dbplan.Confirmation{Action: "create"}, ExpectedPlanSHA256: badPlan.SHA256})
			if err != nil || result.Outcome != "rejected" {
				t.Fatalf("trailing statement %#v %v", result, err)
			}
			_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &injection.Object})
			programCode(t, err, "object_not_found")
			execute(t, e, c, "INSERT INTO p19_source VALUES(9)")
			if queryText(t, e, c, "SELECT sum(value) FROM p19_audit") != "15" {
				t.Fatal("trigger remained")
			}
			if triggerFunction != nil {
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: *triggerFunction})
			}
			if c.Driver == "mysql" {
				event := core.ProgramRef{Kind: "event", Schema: schema, Name: "p19_event"}
				req := core.ProgramChangeRequest{Connection: c, Action: "create", Object: event, Input: &core.ProgramInput{Body: "BEGIN INSERT INTO p19_audit VALUES(99); END", Schedule: &core.ProgramSchedule{Every: 1, Unit: "day", Starts: "2030-01-01 00:00:00", Preserve: true}, Status: "disabled"}}
				programApply(t, e, req)
				details := programReadOne(t, e, c, event)
				if details.Properties["status"] != "DISABLED" || details.Properties["interval_unit"] != "DAY" || details.Definition == nil {
					t.Fatalf("event details %#v", details)
				}
				req.Action = "replace"
				req.Input.Schedule.Every = 2
				programApply(t, e, req)
				programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: event})
			} else {
				_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Kind: "event"})
				programCode(t, err, "unsupported_action")
			}
			if c.Driver == "mysql" && queryText(t, e, c, "SELECT @@GLOBAL.event_scheduler") != schedulerBefore {
				t.Fatal("scheduler changed")
			}
			programApply(t, e, core.ProgramChangeRequest{Connection: c, Action: "drop", Object: view})
			if queryText(t, e, c, "SELECT value FROM p19_neighbor") != "keep" {
				t.Fatal("neighbor changed")
			}
			// A SELECT-only fixture role must not acquire DDL privileges through plans.
			limited := c
			limited.Username = "p19_reader"
			limited.Password = "public-p19-reader-fixture"
			if c.Driver == "mysql" {
				execute(t, e, c, "CREATE USER 'p19_reader'@'%' IDENTIFIED BY 'public-p19-reader-fixture'")
				execute(t, e, c, "GRANT SELECT ON probe.* TO 'p19_reader'@'%'")
			} else {
				execute(t, e, c, "CREATE ROLE p19_reader LOGIN PASSWORD 'public-p19-reader-fixture'")
				execute(t, e, c, "GRANT USAGE ON SCHEMA public TO p19_reader")
				execute(t, e, c, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO p19_reader")
			}
			request := core.ProgramChangeRequest{Connection: limited, Action: "create", Object: core.ProgramRef{Kind: "view", Schema: schema, Name: "p19_forbidden"}, Input: &core.ProgramInput{Body: "SELECT * FROM p19_source"}}
			plan, err := e.ProgramPlan(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			result, err = e.ProgramExecute(ctx, core.ProgramExecuteRequest{ProgramChangeRequest: request, Confirm: dbplan.Confirmation{Action: "create"}, ExpectedPlanSHA256: plan.SHA256})
			if err != nil || result.Outcome != "rejected" || result.Reason != "permission_denied" {
				t.Fatalf("privileges %#v %v", result, err)
			}
			_, err = e.Programs(ctx, core.ProgramsRequest{Connection: c, Object: &request.Object})
			programCode(t, err, "object_not_found")
		})
	}
}
