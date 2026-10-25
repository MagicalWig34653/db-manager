// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func maintenanceDump(t *testing.T, component, label string, value any) {
	t.Helper()
	if dir := os.Getenv("DBM_MAINTENANCE_DUMP"); dir != "" {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.ReplaceAll(t.Name(), "/", "_") + "." + component + "." + label + ".json"
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func maintenancePlan(t *testing.T, e *Engine, c core.Connection, item core.MaintenanceItem) core.MaintenancePreview {
	t.Helper()
	result, err := e.PlanMaintenance(context.Background(), core.MaintenancePlanRequest{Connection: c, Items: []core.MaintenanceItem{item}})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("plan: %#v %v", result, err)
	}
	maintenanceDump(t, "MaintenancePlanResult", item.Action+strconv.FormatBool(item.Full), result)
	return result.Items[0]
}
func confirmedMaintenance(item core.MaintenanceItem, p core.MaintenancePreview) core.MaintenanceExecuteItem {
	return core.MaintenanceExecuteItem{MaintenanceItem: item, ExpectedPlanSHA256: p.Plan.SHA256, Confirm: dbplan.Confirmation{Action: item.Action, TargetName: p.TargetName}}
}
func maintenanceCases(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	ctx := context.Background()
	odd := "maint \"`'; SELECT 1--"
	name, _ := dbplan.QualifiedName(c.Driver, schema, odd)
	sentinel, _ := dbplan.QualifiedName(c.Driver, schema, "maint_sentinel")
	execute(t, e, c, "CREATE TABLE "+name+" (id INTEGER PRIMARY KEY, value TEXT)")
	execute(t, e, c, "CREATE TABLE "+sentinel+" (id INTEGER PRIMARY KEY)")
	execute(t, e, c, "INSERT INTO "+name+" VALUES (1,'preserved')")
	execute(t, e, c, "INSERT INTO "+sentinel+" VALUES (7)")
	view, _ := dbplan.QualifiedName(c.Driver, schema, "maint_view")
	execute(t, e, c, "CREATE VIEW "+view+" AS SELECT * FROM "+name)
	table := core.TableRef{Schema: schema, Name: odd}
	actions := []string{"check", "analyze", "optimize", "repair", "checksum", "vacuum", "reindex"}
	for _, action := range actions {
		item := core.MaintenanceItem{Table: table, Action: action}
		p := maintenancePlan(t, e, c, item)
		if !core.MaintenanceCapabilities().Allows(c.Driver, action) || c.Driver == "mysql" && action == "repair" {
			if p.State != "rejected" || p.Plan != nil {
				t.Fatalf("unsupported %s: %#v", action, p)
			}
			continue
		}
		if p.State != "planned" || p.Plan == nil || p.TargetName != odd || len(p.Plan.Warnings) < 2 {
			t.Fatalf("%s preview %#v", action, p)
		}
		again := maintenancePlan(t, e, c, item)
		if again.Plan.SHA256 != p.Plan.SHA256 {
			t.Fatal("unstable plan")
		}
		request := confirmedMaintenance(item, p)
		for _, change := range []string{"hash", "action", "target"} {
			if change == "target" && !p.TypedConfirmation {
				continue
			}
			bad := request
			switch change {
			case "hash":
				bad.ExpectedPlanSHA256 = strings.Repeat("0", 64)
			case "action":
				bad.Confirm.Action = "check"
				if action == "check" {
					bad.Confirm.Action = "analyze"
				}
			case "target":
				bad.Confirm.TargetName = "wrong"
			}
			result, err := e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{bad}})
			if err != nil || result.Items[0].Outcome != dbplan.StatusRejected {
				t.Fatalf("%s %s: %#v %v", action, change, result, err)
			}
		}
		result, err := e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{request}})
		if err != nil || result.Items[0].Outcome != dbplan.StatusCompleted {
			t.Fatalf("%s execute: %#v %v", action, result, err)
		}
		if action == "checksum" && result.Items[0].Checksum == nil {
			t.Fatal("missing checksum")
		}
		maintenanceDump(t, "MaintenanceResult", action, result)
	}
	if c.Driver == "postgres" {
		for _, full := range []bool{false, true} {
			item := core.MaintenanceItem{Table: table, Action: "vacuum", Full: full, Analyze: true}
			p := maintenancePlan(t, e, c, item)
			if p.Plan == nil || p.TypedConfirmation != full {
				t.Fatalf("vacuum: %#v", p)
			}
			r, err := e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{confirmedMaintenance(item, p)}})
			if err != nil || r.Items[0].Outcome != dbplan.StatusCompleted {
				t.Fatalf("vacuum %#v %v", r, err)
			}
			maintenanceDump(t, "MaintenanceResult", "vacuum-full-"+strconv.FormatBool(full), r)
		}
		parent, _ := dbplan.QualifiedName(c.Driver, schema, "maint_parent")
		child, _ := dbplan.QualifiedName(c.Driver, schema, "maint_child")
		execute(t, e, c, "CREATE TABLE "+parent+" (id int)")
		execute(t, e, c, "CREATE TABLE "+child+" () INHERITS ("+parent+")")
		p := maintenancePlan(t, e, c, core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "maint_parent"}, Action: "vacuum", Full: true})
		if p.Reason != "unsupported_table" {
			t.Fatalf("inheritance %#v", p)
		}
	}
	for _, ref := range []core.TableRef{{Schema: schema, Name: "maint_view"}, {Schema: schema, Name: "missing"}, {Schema: schema, Name: odd + "x"}, {Schema: "pg_catalog", Name: "pg_class"}, {Schema: "mysql", Name: "user"}, {Schema: "main", Name: "sqlite_schema"}} {
		p := maintenancePlan(t, e, c, core.MaintenanceItem{Table: ref, Action: "analyze"})
		if p.State != "rejected" || p.Plan != nil {
			t.Fatalf("unsafe target %#v", p)
		}
	}
	// An early failure does not prevent later entries; all outcomes retain order.
	item := core.MaintenanceItem{Table: table, Action: "analyze"}
	p := maintenancePlan(t, e, c, item)
	valid := confirmedMaintenance(item, p)
	missing := valid
	missing.Table.Name = "missing"
	r, err := e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{missing, valid}})
	if err != nil || !r.Partial || r.Items[0].Outcome != dbplan.StatusRejected || r.Items[1].Outcome != dbplan.StatusCompleted {
		t.Fatalf("batch %#v %v", r, err)
	}
	maintenanceDump(t, "MaintenanceResult", "batch", r)
	if queryText(t, e, c, "SELECT value FROM "+name) != "preserved" || queryText(t, e, c, "SELECT id FROM "+sentinel) != "7" {
		t.Fatal("unrelated data changed")
	}
	if c.Driver == "sqlite" && queryText(t, e, c, "SELECT COUNT(*) FROM sqlite_stat1 WHERE tbl='"+strings.ReplaceAll(odd, "'", "''")+"'") != "1" {
		t.Fatal("ANALYZE statistics missing")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = e.ExecuteMaintenance(canceled, core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{valid}})
	if err == nil {
		t.Fatal("canceled request accepted")
	}
}

func TestSQLiteMaintenance(t *testing.T) {
	e := testEngine(t)
	maintenanceCases(t, e, core.Connection{Driver: "sqlite", Database: "maintenance"}, "main")
}
func TestMaintenanceValidationAndStatements(t *testing.T) {
	for _, n := range []int{0, 51} {
		if _, err := maintenanceTimeout(n, 1); err == nil {
			t.Fatal("batch limit")
		}
	}
	for _, ms := range []int{-1, 60001} {
		if _, err := maintenanceTimeout(1, ms); err == nil {
			t.Fatal("time limit")
		}
	}
	if d, err := maintenanceTimeout(1, 0); err != nil || d != 10*time.Second {
		t.Fatal("default timeout")
	}
	for _, test := range []struct {
		driver, engine, action, want string
		full, analyze, typed         bool
	}{
		{"mysql", "InnoDB", "repair", "", false, false, false},
		{"mysql", "MyISAM", "repair", "REPAIR TABLE `s`.`a``b`", false, false, true},
		{"mysql", "Aria", "repair", "REPAIR TABLE `s`.`a``b`", false, false, true},
		{"mysql", "InnoDB", "optimize", "OPTIMIZE TABLE `s`.`a``b`", false, false, true},
		{"postgres", "postgres", "vacuum", `VACUUM (FULL, ANALYZE) "s"."a` + "`" + `b"`, true, true, true},
	} {
		stmt, typed, reason := maintenanceStatement(test.driver, test.engine, core.TableRef{Schema: "s", Name: "a`b"}, core.MaintenanceItem{Action: test.action, Full: test.full, Analyze: test.analyze})
		if stmt != test.want || typed != test.typed || (reason != "") != (test.want == "") {
			t.Fatalf("statement %#v: %q %v %s", test, stmt, typed, reason)
		}
	}
}
func TestLiveMaintenance(t *testing.T) {
	for _, system := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.env)
			if raw == "" {
				t.Skip("disposable dbfix TLS fixture required")
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
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			schema := "p10_fixture"
			execute(t, e, c, "CREATE SCHEMA "+quoteIdentifier(c.Driver, schema))
			maintenanceCases(t, e, c, schema)
			maintenanceRights(t, e, c, schema)
			maintenanceLockLimit(t, e, c, schema)
			if c.Driver == "mysql" {
				name, _ := dbplan.QualifiedName(c.Driver, schema, "maint_myisam")
				execute(t, e, c, "CREATE TABLE "+name+" (id int) ENGINE=MyISAM")
				item := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "maint_myisam"}, Action: "repair"}
				p := maintenancePlan(t, e, c, item)
				if p.Plan == nil || !p.TypedConfirmation {
					t.Fatalf("repair %#v", p)
				}
				r, err := e.ExecuteMaintenance(context.Background(), core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{confirmedMaintenance(item, p)}})
				if err != nil || r.Items[0].Outcome != dbplan.StatusCompleted {
					t.Fatalf("repair %#v %v", r, err)
				}
				maintenanceDump(t, "MaintenanceResult", "repair-myisam", r)
			}
		})
	}
}
func maintenanceRights(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	limited := c
	limited.Username = "p10_reader"
	limited.Password = "public-p10-fixture"
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE USER 'p10_reader'@'%' IDENTIFIED BY 'public-p10-fixture'")
		execute(t, e, c, "GRANT SELECT ON "+quoteIdentifier(c.Driver, schema)+".* TO 'p10_reader'@'%'")
		limited.Database = schema
	} else {
		execute(t, e, c, "CREATE ROLE p10_reader LOGIN PASSWORD 'public-p10-fixture'")
		execute(t, e, c, "GRANT USAGE ON SCHEMA "+quoteIdentifier(c.Driver, schema)+" TO p10_reader")
		execute(t, e, c, "GRANT SELECT ON ALL TABLES IN SCHEMA "+quoteIdentifier(c.Driver, schema)+" TO p10_reader")
	}
	item := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "maint_sentinel"}, Action: "analyze"}
	p := maintenancePlan(t, e, limited, item)
	if c.Driver == "postgres" {
		if p.Reason != "permission_denied" {
			t.Fatalf("permission %#v", p)
		}
		return
	}
	if p.Plan == nil {
		t.Fatalf("mysql visible table %#v", p)
	}
	r, err := e.ExecuteMaintenance(context.Background(), core.MaintenanceExecuteRequest{Connection: limited, Items: []core.MaintenanceExecuteItem{confirmedMaintenance(item, p)}})
	if err != nil || r.Items[0].Outcome == dbplan.StatusCompleted {
		t.Fatalf("permission %#v %v", r, err)
	}
}

// Lock contention must finish within the request budget. The lock belongs to
// this disposable fixture and is always released, including on assertion errors.
func maintenanceLockLimit(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	if c.Driver != "postgres" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, cleanup, err := e.open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	name, _ := dbplan.QualifiedName(c.Driver, schema, "maint_sentinel")
	if _, err = tx.ExecContext(ctx, "LOCK TABLE "+name+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	item := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "maint_sentinel"}, Action: "vacuum", Full: true}
	// Plan catalog reads do not acquire a conflicting table lock.
	p := maintenancePlan(t, e, c, item)
	if p.Plan == nil {
		t.Fatalf("locked preview %#v", p)
	}
	start := time.Now()
	r, err := e.ExecuteMaintenance(context.Background(), core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{confirmedMaintenance(item, p)}, TimeoutMS: 100})
	if err != nil || r.Items[0].Outcome == dbplan.StatusCompleted || time.Since(start) > 3*time.Second {
		t.Fatalf("lock limit %#v %v", r, err)
	}
	maintenanceDump(t, "MaintenanceResult", "lock-limit", r)
}
