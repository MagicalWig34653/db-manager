// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func columnDef(name string) *core.ColumnDefinition {
	nullable := true
	return &core.ColumnDefinition{Name: name, Type: dbplan.ColumnType{Kind: "text", Length: 40}, Nullable: &nullable, Default: &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "q'\\;DROP TABLE neighbor;--"}}}
}
func dumpColumns(t *testing.T, component string, value any) {
	t.Helper()
	dir := os.Getenv("DBM_COLUMNS_DUMP")
	if dir == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	name := strings.ReplaceAll(t.Name(), "/", "_") + "-" + strconv.Itoa(len(entries)) + "." + component + ".json"
	if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func columnCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	name := "col \"`' ;DROP TABLE neighbor"
	q := func(n string) string { return columnQuote(c.Driver, n) }
	execute(t, e, c, "CREATE TABLE neighbor (id BIGINT)")
	execute(t, e, c, "INSERT INTO neighbor VALUES (9)")
	execute(t, e, c, "CREATE TABLE "+q(name)+" (id BIGINT PRIMARY KEY, old_value VARCHAR(80))")
	execute(t, e, c, "INSERT INTO "+q(name)+" VALUES (1,NULL)")
	r := core.ColumnsRequest{Connection: c, Table: core.TableRef{Name: name}, Changes: []core.ColumnChange{{Action: "add", Definition: columnDef("new \"` value")}}}
	p, err := e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	dumpColumns(t, "ColumnsPlan", p)
	p2, err := e.PlanColumns(ctx, r)
	if err != nil || p.SHA256 != p2.SHA256 {
		t.Fatal("unstable plan", err)
	}
	if queryText(t, e, c, "SELECT COUNT(*) FROM "+q(name)) != "1" {
		t.Fatal("planning changed data")
	}
	r.Confirm = &dbplan.Confirmation{Action: "columns_alter"}
	r.ExpectedPlanSHA256 = p.SHA256
	r.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	_, err = e.ExecuteColumns(ctx, r)
	requireMutationCode(t, err, "plan_mismatch")
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm.Action = "drop"
	_, err = e.ExecuteColumns(ctx, r)
	requireMutationCode(t, err, "confirmation_mismatch")
	r.Confirm.Action = "columns_alter"
	result, err := e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusCompleted {
		t.Fatalf("add %#v %v", result, err)
	}
	dumpColumns(t, "ColumnsResult", result)
	if queryText(t, e, c, "SELECT "+q("new \"` value")+" FROM "+q(name)) != "q'\\;DROP TABLE neighbor;--" {
		t.Fatal("literal roundtrip")
	}
	// Read independently from catalog rather than relying on the execution answer.
	var catalog string
	switch c.Driver {
	case "sqlite":
		catalog = "SELECT type FROM pragma_table_xinfo(" + columnLiteral(name) + ") WHERE name=" + columnLiteral("new \"` value")
	case "mysql":
		catalog = "SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='probe' AND TABLE_NAME=" + columnLiteral(name) + " AND COLUMN_NAME=" + columnLiteral("new \"` value")
	case "postgres":
		catalog = "SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name=" + columnLiteral(name) + " AND column_name=" + columnLiteral("new \"` value")
	}
	if !strings.Contains(strings.ToLower(queryText(t, e, c, catalog)), "var") {
		t.Fatal("catalog type")
	}
	if c.Driver != "sqlite" {
		def := columnDef("old_value")
		nullable := false
		def.Nullable = &nullable
		def.Default = &core.CreateDefault{Mode: "none"}
		comment := ""
		def.Comment = &comment
		if c.Driver == "mysql" {
			collation := "utf8mb4_bin"
			def.Collation = &collation
		}
		nullRequest := r
		nullRequest.Changes = []core.ColumnChange{{Action: "change", Column: "old_value", Definition: def}}
		nullPlan, err := e.PlanColumns(ctx, nullRequest)
		if err != nil {
			t.Fatal(err)
		}
		if !nullPlan.Destructive || !strings.Contains(strings.Join(nullPlan.Warnings, " "), "NULL-Werte") {
			t.Fatal("missing NULL warning", nullPlan)
		}
		nullRequest.ExpectedPlanSHA256 = nullPlan.SHA256
		nullRequest.Confirm = &dbplan.Confirmation{Action: "columns_alter", TargetName: nullPlan.TargetName}
		nullResult, err := e.ExecuteColumns(ctx, nullRequest)
		if err != nil || nullResult.Outcome != dbplan.StatusRejected {
			t.Fatal("NOT NULL changed NULL data", nullResult, err)
		}
	}
	if c.Driver != "sqlite" {
		def := columnDef("new \"` value")
		def.Type.Length = 12
		def.Default = &core.CreateDefault{Mode: "none"}
		comment := "q'\\ DB comment"
		def.Comment = &comment
		if c.Driver == "mysql" {
			collation := "utf8mb4_bin"
			def.Collation = &collation
		}
		r.Changes = []core.ColumnChange{{Action: "change", Column: def.Name, Definition: def}}
		p, err = e.PlanColumns(ctx, r)
		if err != nil || !p.Destructive || len(p.Warnings) < 3 {
			t.Fatal(p, err)
		}
		r.ExpectedPlanSHA256 = p.SHA256
		r.Confirm.TargetName = "wrong"
		_, err = e.ExecuteColumns(ctx, r)
		requireMutationCode(t, err, "confirmation_mismatch")
		r.Confirm.TargetName = p.Table.Name
		// Shrink would lose data: strict MySQL or PostgreSQL cast can reject/truncate;
		// remove the long value explicitly in this disposable fixture before ALTER.
		execute(t, e, c, "UPDATE "+q(name)+" SET "+q(def.Name)+"='short'")
		result, err = e.ExecuteColumns(ctx, r)
		if err != nil || result.Outcome != dbplan.StatusCompleted {
			t.Fatalf("change %#v %v", result, err)
		}
		var attributes string
		if c.Driver == "mysql" {
			attributes = "SELECT CONCAT(CHARACTER_MAXIMUM_LENGTH,':',IS_NULLABLE,':',IF(COLUMN_DEFAULT IS NULL,'none','value')) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='probe' AND TABLE_NAME=" + columnLiteral(name) + " AND COLUMN_NAME=" + columnLiteral(def.Name)
			if queryText(t, e, c, attributes) != "12:YES:none" {
				t.Fatal("changed attributes")
			}
			if queryText(t, e, c, "SELECT COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='probe' AND TABLE_NAME="+columnLiteral(name)+" AND COLUMN_NAME="+columnLiteral(def.Name)) != comment {
				t.Fatal("comment roundtrip")
			}
		} else {
			attributes = "SELECT character_maximum_length::text||':'||is_nullable||':'||CASE WHEN column_default IS NULL THEN 'none' ELSE 'value' END FROM information_schema.columns WHERE table_schema='public' AND table_name=" + columnLiteral(name) + " AND column_name=" + columnLiteral(def.Name)
			if queryText(t, e, c, attributes) != "12:YES:none" {
				t.Fatal("changed attributes")
			}
			if queryText(t, e, c, "SELECT pg_catalog.col_description(c.oid,a.attnum) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid WHERE c.relname="+columnLiteral(name)+" AND a.attname="+columnLiteral(def.Name)) != comment {
				t.Fatal("comment roundtrip")
			}
		}
		if c.Driver == "mysql" {
			def.Type = dbplan.ColumnType{Kind: "enum", Values: []string{"short", "a,b", "q'\\"}}
			def.Collation = nil
			def.Default = &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "a,b"}}
			r.Changes = []core.ColumnChange{{Action: "change", Column: def.Name, Definition: def, Position: &core.ColumnPosition{Mode: "before", Column: "id"}}}
			p, err = e.PlanColumns(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			r.ExpectedPlanSHA256 = p.SHA256
			result, err = e.ExecuteColumns(ctx, r)
			if err != nil || result.Outcome != dbplan.StatusCompleted {
				t.Fatalf("enum %#v %v", result, err)
			}
			if queryText(t, e, c, "SELECT ORDINAL_POSITION FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='probe' AND TABLE_NAME="+columnLiteral(name)+" AND COLUMN_NAME="+columnLiteral(def.Name)) != "1" {
				t.Fatal("independent position")
			}
			if result.Description.Columns[0].Name != def.Name {
				t.Fatal("position")
			}
			def.Type = dbplan.ColumnType{Kind: "set", Values: []string{"short", "a", "b", "q'\\"}}
			def.Default = &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "a,b"}}
			r.Changes = []core.ColumnChange{{Action: "reorder", Column: def.Name, Definition: def, Position: &core.ColumnPosition{Mode: "after", Column: "id"}}}
			p, err = e.PlanColumns(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			r.ExpectedPlanSHA256 = p.SHA256
			result, err = e.ExecuteColumns(ctx, r)
			if err != nil || result.Outcome != dbplan.StatusCompleted {
				t.Fatalf("set/reorder %#v %v", result, err)
			}
			if !strings.HasPrefix(strings.ToLower(queryText(t, e, c, "SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='probe' AND TABLE_NAME="+columnLiteral(name)+" AND COLUMN_NAME="+columnLiteral(def.Name))), "set(") {
				t.Fatal("SET catalog")
			}

		}
	} else {
		unsupported := r
		unsupported.Changes = []core.ColumnChange{{Action: "change", Column: "old_value", Definition: columnDef("old_value")}}
		_, err = e.PlanColumns(ctx, unsupported)
		requireMutationCode(t, err, "unsupported_action")
	}
	r.Changes = []core.ColumnChange{{Action: "rename", Column: "old_value", NewName: "renamed \"`"}}
	p, err = e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	result, err = e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusCompleted {
		t.Fatal(result, err)
	}
	r.Changes = []core.ColumnChange{{Action: "drop", Column: "renamed \"`"}}
	p, err = e.PlanColumns(ctx, r)
	if err != nil || !p.Destructive || len(p.Dependencies) == 0 {
		t.Fatal(p, err)
	}
	dumpColumns(t, "ColumnsPlan", p)
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm.TargetName = "wrong"
	_, err = e.ExecuteColumns(ctx, r)
	requireMutationCode(t, err, "confirmation_mismatch")
	r.Confirm.TargetName = p.Table.Name
	result, err = e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusCompleted {
		t.Fatal(result, err)
	}
	dumpColumns(t, "ColumnsResult", result)
	if queryText(t, e, c, "SELECT COUNT(*) FROM neighbor") != "1" {
		t.Fatal("neighbor changed")
	}
	execute(t, e, c, "CREATE TABLE col_child(ref BIGINT REFERENCES "+q(name)+"(id))")
	if c.Driver == "mysql" {
		// Inline REFERENCES is ignored by some MySQL versions; install a real FK.
		execute(t, e, c, "ALTER TABLE col_child ADD CONSTRAINT col_child_fk FOREIGN KEY(ref) REFERENCES "+q(name)+"(id)")
	}
	r.Changes = []core.ColumnChange{{Action: "drop", Column: "id"}}
	p, err = e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	// PK-dependent drop must be rejected; no CASCADE and no data loss.
	{
		result, err = e.ExecuteColumns(ctx, r)
		if err != nil || result.Outcome != dbplan.StatusRejected {
			t.Fatal(result, err)
		}
		dumpColumns(t, "ColumnsResult", result)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = e.ExecuteColumns(canceled, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("canceled", err)
	}
}
func TestSQLiteColumns(t *testing.T) {
	e := testEngine(t)
	c := core.Connection{Driver: "sqlite", Database: "columns"}
	t.Log("SQLite", queryText(t, e, c, "SELECT sqlite_version()"))
	columnCases(t, e, c)
}
func TestLiveColumns(t *testing.T) {
	for _, test := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.env)
			if raw == "" {
				t.Skip("W-P08 disposable TLS fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD")}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			columnCases(t, e, c)
			columnsDenied(t, e, c)
		})
	}
}
func TestColumnsValidationAndFailureClassification(t *testing.T) {
	for _, action := range []string{"generated", "reorder", "change"} {
		r := core.ColumnsRequest{Connection: sqliteConnection(), Table: core.TableRef{Name: "x"}, Changes: []core.ColumnChange{{Action: action}}}
		if validateColumns(r) == nil {
			t.Fatal(action)
		}
	}
	for _, d := range []*core.ColumnDefinition{{Name: "x"}, {Name: "x", Type: dbplan.ColumnType{Kind: "text);DROP TABLE x"}}} {
		r := core.ColumnsRequest{Connection: sqliteConnection(), Table: core.TableRef{Name: "x"}, Changes: []core.ColumnChange{{Action: "add", Definition: d}}}
		if validateColumns(r) == nil {
			t.Fatal(d)
		}
	}
	for _, test := range []struct {
		err      error
		reason   string
		definite bool
	}{{errors.New("password /private/path SQL"), "connection_lost", false}, {&mysql.MySQLError{Number: 1142}, "permission_denied", true}, {&mysql.MySQLError{Number: 1205}, "lock_timeout", true}, {&pgconn.PgError{Code: "2BP01"}, "dependency_blocks", true}, {&pgconn.PgError{Code: "42501"}, "permission_denied", true}} {
		reason, definite := columnsFailure(test.err)
		if reason != test.reason || definite != test.definite {
			t.Fatal(reason, definite)
		}
	}
}

type columnsFault struct {
	columnsExecution
	execErr    error
	commitErr  error
	observeErr error
	count      int
}

func (f *columnsFault) exec(context.Context, string, ...any) (sql.Result, error) {
	f.count++
	return nil, f.execErr
}
func (f *columnsFault) commit(context.Context) error { return f.commitErr }
func (f *columnsFault) observeColumns(context.Context, core.Connection, core.TableRef) (core.TableDescription, error) {
	return core.TableDescription{}, f.observeErr
}
func TestColumnsExecutionOutcomes(t *testing.T) {
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		r := core.ColumnsRequest{Connection: core.Connection{Driver: driver}}
		p := core.ColumnsPlan{Plan: dbplan.Plan{Statements: []string{"ALTER TABLE fixture DROP COLUMN value"}}}
		f := &columnsFault{execErr: errors.New("private password SQL")}
		result := runColumns(context.Background(), r, p, f)
		if result.Outcome != dbplan.StatusOutcomeUnknown || strings.Contains(result.Message, "password") {
			t.Fatal(driver, result)
		}
		f = &columnsFault{execErr: &pgconn.PgError{Code: "42501"}}
		result = runColumns(context.Background(), r, p, f)
		if result.Outcome != dbplan.StatusRejected || result.Reason != "permission_denied" {
			t.Fatal(driver, result)
		}
		f = &columnsFault{observeErr: errors.New("observation lost")}
		result = runColumns(context.Background(), r, p, f)
		want := dbplan.StatusRejected
		if driver == "mysql" {
			want = dbplan.StatusCompleted
		}
		if result.Outcome != want || result.Observation != "unavailable" {
			t.Fatal(driver, result)
		}
		f = &columnsFault{commitErr: errors.New("commit lost")}
		result = runColumns(context.Background(), r, p, f)
		if driver != "mysql" && (result.Outcome != dbplan.StatusOutcomeUnknown || result.Description != nil || result.Atomic) {
			t.Fatal(driver, result)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		f = &columnsFault{}
		result = runColumns(ctx, r, p, f)
		if result.Outcome != dbplan.StatusRejected || result.Reason != "canceled" || f.count != 0 {
			t.Fatal(driver, result)
		}
	}
}

func TestSQLiteColumnsBatchRollback(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE fixture(id INTEGER PRIMARY KEY, value TEXT)")
	r := core.ColumnsRequest{Connection: c, Table: core.TableRef{Name: "fixture"}, Changes: []core.ColumnChange{{Action: "add", Definition: columnDef("extra")}, {Action: "drop", Column: "id"}}}
	p, err := e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm = &dbplan.Confirmation{Action: "columns_alter", TargetName: p.TargetName}
	result, err := e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusRejected || result.StatementsExecuted != 0 {
		t.Fatal(result, err)
	}
	if queryText(t, e, c, "SELECT COUNT(*) FROM pragma_table_xinfo('fixture') WHERE name='extra'") != "0" {
		t.Fatal("partial DDL escaped rollback")
	}
	execute(t, e, c, "CREATE VIEW fixture_view AS SELECT value FROM fixture")
	r.Changes = []core.ColumnChange{{Action: "drop", Column: "value"}}
	p, err = e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(p.Dependencies, " "), "fixture_view") {
		t.Fatal("missing view preview", p.Dependencies)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	result, err = e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusRejected {
		t.Fatal(result, err)
	}
	if queryText(t, e, c, "SELECT COUNT(*) FROM pragma_table_xinfo('fixture') WHERE name='value'") != "1" {
		t.Fatal("dependency drop changed table")
	}
}

func columnsDenied(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	denied := c
	denied.Username = "w_p08_reader"
	denied.Password = "public-w-p08-only"
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE USER 'w_p08_reader'@'%' IDENTIFIED BY 'public-w-p08-only'")
		execute(t, e, c, "GRANT SELECT ON probe.* TO 'w_p08_reader'@'%'")
	} else {
		execute(t, e, c, "CREATE ROLE w_p08_reader LOGIN PASSWORD 'public-w-p08-only'")
		execute(t, e, c, "GRANT USAGE ON SCHEMA public TO w_p08_reader")
		execute(t, e, c, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO w_p08_reader")
	}
	r := core.ColumnsRequest{Connection: denied, Table: core.TableRef{Name: "neighbor"}, Changes: []core.ColumnChange{{Action: "add", Definition: columnDef("denied")}}}
	p, err := e.PlanColumns(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm = &dbplan.Confirmation{Action: "columns_alter"}
	result, err := e.ExecuteColumns(ctx, r)
	if err != nil || result.Outcome != dbplan.StatusRejected || result.Reason != "permission_denied" {
		t.Fatal(result, err)
	}
	if queryText(t, e, c, "SELECT COUNT(*) FROM neighbor") != "1" {
		t.Fatal("permission denial changed fixture")
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), denied.Password) || strings.Contains(string(encoded), "ALTER TABLE") {
		t.Fatal("raw error leaked")
	}
}
