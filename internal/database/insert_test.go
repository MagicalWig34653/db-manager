// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func insertCell(column, kind, value string) core.RowChange {
	return core.RowChange{Column: column, Value: core.Cell{Kind: kind, Value: value}}
}
func TestSQLiteInsert(t *testing.T) { insertCases(t, testEngine(t), sqliteConnection()) }
func insertCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	quote := func(v string) string { return quoteIdentifier(c.Driver, v) }
	name := "insert\"odd'`"
	ref := core.TableRef{Name: name}
	blob := "BLOB"
	if c.Driver == "postgres" {
		blob = "BYTEA"
	}
	execute(t, e, c, "CREATE TABLE "+quote(name)+"(a INTEGER NOT NULL,b INTEGER NOT NULL,value VARCHAR(255) DEFAULT 'from-db',raw "+blob+",amount DECIMAL(30,5) DEFAULT 1.25,calc INTEGER GENERATED ALWAYS AS(a+b) STORED,PRIMARY KEY(b,a))")
	plan, err := e.InsertEligibility(ctx, core.DescribeRequest{Connection: c, Table: ref})
	if err != nil || !plan.Eligible || len(plan.RequiredPK) != 2 || len(plan.InsertableColumns) != 5 {
		t.Fatalf("plan %#v %v", plan, err)
	}
	request := core.InsertRequest{Connection: c, Table: ref, Confirm: true, Values: []core.RowChange{insertCell("a", "integer", "1"), insertCell("b", "integer", "2")}}
	result, err := e.Insert(ctx, request)
	if err != nil || result.Outcome != "committed" || result.Row.Values[2] != (core.Cell{Kind: "text", Value: "from-db"}) || result.Row.Values[3].Kind != "null" || result.Row.Values[5].Value != "3" {
		t.Fatalf("default/generated insert %#v %v", result, err)
	}
	_, err = e.Insert(ctx, request)
	requireMutationCode(t, err, "insert_conflict")
	page, err := e.Browse(ctx, core.BrowseRequest{Connection: c, Table: ref})
	if err != nil || len(page.Result.Rows) != 1 || page.Result.Rows[0][2].Value != "from-db" {
		t.Fatal("duplicate overwrote or inserted row")
	}
	request.Values = []core.RowChange{insertCell("a", "integer", "3"), insertCell("b", "integer", "2"), insertCell("value", "null", ""), insertCell("raw", "binary", ""), insertCell("amount", "decimal", "12345678901234567890.12345")}
	result, err = e.Insert(ctx, request)
	if err != nil || result.Row.Values[2].Kind != "null" || result.Row.Values[3] != (core.Cell{Kind: "binary", Value: ""}) {
		t.Fatalf("null/emptyblob %#v %v", result, err)
	}
	if c.Driver != "sqlite" && result.Row.Values[4] != (core.Cell{Kind: "decimal", Value: "12345678901234567890.12345"}) {
		t.Fatalf("decimal precision %#v", result.Row.Values[4])
	}
	request.Values = []core.RowChange{insertCell("a", "integer", "4"), insertCell("b", "integer", "2"), insertCell("amount", "decimal", "1.234567")}
	rounded, err := e.Insert(ctx, request)
	if err != nil {
		t.Fatalf("server decimal conversion %v", err)
	}
	if c.Driver != "sqlite" && rounded.Row.Values[4] != (core.Cell{Kind: "decimal", Value: "1.23457"}) {
		t.Fatalf("actual server-rounded value %#v", rounded.Row.Values[4])
	}
	for _, values := range [][]core.RowChange{
		{insertCell("a", "integer", "5")},
		{insertCell("a", "integer", "5"), insertCell("b", "null", "")},
		{insertCell("a", "integer", "5"), insertCell("b", "integer", "2"), insertCell("a", "integer", "6")},
		{insertCell("a", "integer", "5"), insertCell("b", "integer", "2"), insertCell("calc", "integer", "7")},
		{insertCell("a", "integer", "5"), insertCell("b", "integer", "2"), insertCell("missing'; DROP TABLE x;--", "text", "x")},
	} {
		bad := request
		bad.Values = values
		_, err = e.Insert(ctx, bad)
		requireMutationCode(t, err, "invalid_insert")
	}
	request.Confirm = false
	_, err = e.Insert(ctx, request)
	requireMutationCode(t, err, "invalid_insert")
	request.Confirm = true
	// Competing inserts at an identical PK never overwrite the successful row.
	request.Values = []core.RowChange{insertCell("a", "integer", "8"), insertCell("b", "integer", "2"), insertCell("value", "text", "bound'; DROP TABLE x;--")}
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := e.Insert(ctx, request); outcomes <- err }()
	}
	wg.Wait()
	close(outcomes)
	success := 0
	for err := range outcomes {
		if err == nil {
			success++
		} else {
			var p *core.Error
			if !errors.As(err, &p) || (p.Code != "insert_conflict" && p.Code != "database_busy") {
				t.Fatalf("parallel insert %v", err)
			}
		}
	}
	if success != 1 {
		t.Fatalf("parallel successes %d", success)
	}
	execute(t, e, c, "CREATE TABLE insert_no_key(value TEXT)")
	plan, err = e.InsertEligibility(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "insert_no_key"}})
	if err != nil || plan.Eligible {
		t.Fatalf("no key %#v %v", plan, err)
	}
	execute(t, e, c, "CREATE VIEW insert_view AS SELECT * FROM "+quote(name))
	plan, err = e.InsertEligibility(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "insert_view"}})
	if err != nil || plan.Eligible {
		t.Fatalf("view %#v %v", plan, err)
	}
	insertAutoCases(t, e, c)
}
func insertAutoCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	definition := "id INTEGER PRIMARY KEY"
	if c.Driver == "mysql" {
		definition = "id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY"
	} else if c.Driver == "postgres" {
		definition = "id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY"
	}
	execute(t, e, c, "CREATE TABLE insert_auto("+definition+",value VARCHAR(255) DEFAULT 'automatic')")
	if c.Driver == "mysql" {
		execute(t, e, c, "ALTER TABLE insert_auto AUTO_INCREMENT=9223372036854775811")
	}
	ref := core.TableRef{Name: "insert_auto"}
	plan, err := e.InsertEligibility(ctx, core.DescribeRequest{Connection: c, Table: ref})
	if err != nil || !plan.Eligible || plan.AutoPK != "id" || len(plan.RequiredPK) != 0 {
		t.Fatalf("auto plan %#v %v", plan, err)
	}
	r := core.InsertRequest{Connection: c, Table: ref, Values: []core.RowChange{}, Confirm: true}
	result, err := e.Insert(ctx, r)
	if err != nil || result.Row.Values[1].Value != "automatic" {
		t.Fatalf("auto insert %#v %v", result, err)
	}
	if c.Driver == "mysql" && result.Row.Values[0] != (core.Cell{Kind: "integer", Value: "9223372036854775811"}) {
		t.Fatalf("uint64 autoID %#v", result.Row.Values[0])
	}
	r.Values = []core.RowChange{insertCell("value", "text", "")}
	second, err := e.Insert(ctx, r)
	if err != nil || second.Row.Values[0] == result.Row.Values[0] || second.Row.Values[1] != (core.Cell{Kind: "text", Value: ""}) {
		t.Fatalf("second auto %#v %v", second, err)
	}
	if c.Driver == "postgres" {
		execute(t, e, c, "CREATE TABLE insert_serial(id BIGSERIAL PRIMARY KEY,value TEXT DEFAULT 'serial')")
		r.Table = core.TableRef{Name: "insert_serial"}
		r.Values = []core.RowChange{}
		result, err = e.Insert(ctx, r)
		if err != nil || result.Row.Values[0].Value != "1" || result.Row.Values[1].Value != "serial" {
			t.Fatalf("serial %#v %v", result, err)
		}
	}
}
func TestSQLiteInsertCollationAndCommitFailure(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE insert_collated(id TEXT COLLATE NOCASE NOT NULL,value TEXT,PRIMARY KEY(id COLLATE BINARY))`)
	for _, key := range []string{"a", "A"} {
		r := core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_collated"}, Confirm: true, Values: []core.RowChange{insertCell("id", "text", key)}}
		result, err := e.Insert(ctx, r)
		if err != nil || result.Row.Values[0].Value != key {
			t.Fatalf("collated %#v %v", result, err)
		}
	}
	insertCommitFailure(t, e, c)
}
func insertCommitFailure(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, `CREATE TABLE insert_parent(id INTEGER PRIMARY KEY)`)
	execute(t, e, c, `CREATE TABLE insert_commit(id INTEGER PRIMARY KEY,parent INTEGER REFERENCES insert_parent(id) DEFERRABLE INITIALLY DEFERRED)`)
	result, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_commit"}, Confirm: true, Values: []core.RowChange{insertCell("id", "integer", "1"), insertCell("parent", "integer", "99")}})
	requireMutationCode(t, err, "outcome_unknown")
	if result.Outcome != "" {
		t.Fatal("commit failure returned success")
	}
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "insert_commit"}})
	if err != nil || len(page.Result.Rows) != 0 {
		t.Fatalf("commit rejected but row persisted %#v %v", page, err)
	}
}
func TestLiveInserts(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("B2c disposable fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			insertCases(t, e, c)
			insertTriggerCases(t, e, c)
			if c.Driver == "postgres" {
				insertCommitFailure(t, e, c)
				liveInsertRuleAndSuppression(t, e, c)
			}
			liveInsertLock(t, e, c)
			liveInsertResponseLimit(t, e, c)
		})
	}
}
func liveInsertLock(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, "CREATE TABLE insert_locked(id INTEGER PRIMARY KEY,value TEXT)")
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(context.Background(), "INSERT INTO insert_locked VALUES(1,'blocker')"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_locked"}, Confirm: true, TimeoutMS: 30, Values: []core.RowChange{insertCell("id", "integer", "1")}})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("insert lock timeout %v", err)
	}
	tx.Rollback()
	if err = e.Test(context.Background(), c); err != nil {
		t.Fatal("connection not released")
	}
}

func TestSQLiteInsertTriggers(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	insertTriggerCases(t, e, c)
	execute(t, e, c, `CREATE TABLE insert_ignored(id INTEGER PRIMARY KEY,value TEXT)`)
	execute(t, e, c, `CREATE TRIGGER insert_ignored BEFORE INSERT ON insert_ignored BEGIN SELECT RAISE(IGNORE); END`)
	_, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_ignored"}, Confirm: true, Values: []core.RowChange{insertCell("id", "integer", "1")}})
	requireMutationCode(t, err, "insert_not_applied")
}
func fixtureInsertDDL(t *testing.T, e *Engine, c core.Connection, query string) {
	t.Helper()
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err = db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("fixture DDL failed: %v", err)
	}
}
func countInsertFixture(t *testing.T, e *Engine, c core.Connection, name string, want string) {
	t.Helper()
	result, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT count(*) FROM " + quoteIdentifier(c.Driver, name)})
	if err != nil || result.Results[0].Rows[0][0].Value != want {
		t.Fatalf("count %s %#v %v", name, result, err)
	}
}
func insertTriggerCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	definition := "id INTEGER PRIMARY KEY"
	if c.Driver == "mysql" {
		definition = "id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY"
	} else if c.Driver == "postgres" {
		definition = "id BIGSERIAL PRIMARY KEY"
	}
	execute(t, e, c, "CREATE TABLE insert_relocated("+definition+",value TEXT)")
	execute(t, e, c, "CREATE TABLE insert_move_audit(id BIGINT)")
	switch c.Driver {
	case "sqlite":
		execute(t, e, c, `CREATE TRIGGER insert_relocate AFTER INSERT ON insert_relocated BEGIN INSERT INTO insert_move_audit VALUES(NEW.id); UPDATE insert_relocated SET id=NEW.id+100 WHERE id=NEW.id; END`)
	case "postgres":
		execute(t, e, c, `CREATE FUNCTION insert_relocate_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO insert_move_audit VALUES(NEW.id); UPDATE insert_relocated SET id=NEW.id+100 WHERE id=NEW.id; RETURN NULL; END $$`)
		execute(t, e, c, `CREATE TRIGGER insert_relocate AFTER INSERT ON insert_relocated FOR EACH ROW EXECUTE FUNCTION insert_relocate_fn()`)
	case "mysql":
		fixtureInsertDDL(t, e, c, `CREATE TRIGGER insert_relocate BEFORE INSERT ON insert_relocated FOR EACH ROW BEGIN SET NEW.id=999; INSERT INTO insert_move_audit VALUES(NEW.id); END`)
	}
	_, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_relocated"}, Values: []core.RowChange{}, Confirm: true})
	requireMutationCode(t, err, "insert_identity_unknown")
	countInsertFixture(t, e, c, "insert_relocated", "0")
	countInsertFixture(t, e, c, "insert_move_audit", "0")
	// Audit insertion with its own auto-key cannot replace the outer row's identity.
	execute(t, e, c, "CREATE TABLE insert_audit_owner("+definition+",value TEXT)")
	execute(t, e, c, "CREATE TABLE insert_auto_audit("+definition+",owner BIGINT)")
	switch c.Driver {
	case "sqlite":
		execute(t, e, c, `CREATE TRIGGER insert_audit AFTER INSERT ON insert_audit_owner BEGIN INSERT INTO insert_auto_audit(id,owner) VALUES(700,NEW.id); UPDATE insert_audit_owner SET value='after-trigger' WHERE id=NEW.id; END`)
	case "postgres":
		execute(t, e, c, `CREATE FUNCTION insert_audit_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO insert_auto_audit(owner) VALUES(NEW.id); UPDATE insert_audit_owner SET value='after-trigger' WHERE id=NEW.id; RETURN NULL; END $$`)
		execute(t, e, c, `CREATE TRIGGER insert_audit AFTER INSERT ON insert_audit_owner FOR EACH ROW EXECUTE FUNCTION insert_audit_fn()`)
	case "mysql":
		execute(t, e, c, `ALTER TABLE insert_auto_audit AUTO_INCREMENT=700`)
		fixtureInsertDDL(t, e, c, `CREATE TRIGGER insert_audit BEFORE INSERT ON insert_audit_owner FOR EACH ROW SET NEW.value='after-trigger'`)
		fixtureInsertDDL(t, e, c, `CREATE TRIGGER insert_audit_after AFTER INSERT ON insert_audit_owner FOR EACH ROW INSERT INTO insert_auto_audit(owner) VALUES(NEW.id)`)
	}
	result, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_audit_owner"}, Values: []core.RowChange{}, Confirm: true})
	if err != nil || result.Row.Values[0].Value != "1" || result.Row.Values[1].Value != "after-trigger" {
		t.Fatalf("final trigger row %#v %v", result, err)
	}
	countInsertFixture(t, e, c, "insert_auto_audit", "1")
}

func liveInsertResponseLimit(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	largeType := "LONGTEXT"
	if c.Driver == "postgres" {
		largeType = "TEXT"
	}
	definition := "CREATE TABLE insert_large(id INTEGER PRIMARY KEY"
	for i := range 6 {
		definition += ",value" + strconv.Itoa(i) + " " + largeType + ` GENERATED ALWAYS AS (repeat('"',900000)) STORED`
	}
	definition += ")"
	execute(t, e, c, definition)
	_, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_large"}, Confirm: true, Values: []core.RowChange{insertCell("id", "integer", "1")}})
	requireMutationCode(t, err, "result_too_large")
	countInsertFixture(t, e, c, "insert_large", "0")
}
func TestSQLiteInsertBoundsAndTypeRollback(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE insert_integer(id INTEGER PRIMARY KEY,value TEXT)`)
	r := core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_integer"}, Confirm: true, Values: []core.RowChange{insertCell("id", "text", "1")}}
	_, err := e.Insert(ctx, r)
	requireMutationCode(t, err, "insert_identity_unknown")
	countInsertFixture(t, e, c, "insert_integer", "0")
	r.Values = []core.RowChange{insertCell("id", "integer", "1")}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = e.Insert(canceled, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("insert cancel %v", err)
	}
	r.TimeoutMS = 60001
	_, err = e.Insert(ctx, r)
	requireMutationCode(t, err, "invalid_insert")
	execute(t, e, c, `CREATE TABLE insert_huge(id INTEGER PRIMARY KEY,value BLOB GENERATED ALWAYS AS (zeroblob(1048577)) STORED)`)
	r.Table = core.TableRef{Name: "insert_huge"}
	r.TimeoutMS = 0
	_, err = e.Insert(ctx, r)
	requireMutationCode(t, err, "result_too_large")
	countInsertFixture(t, e, c, "insert_huge", "0")
}

func TestSQLiteInsertOverridesTableReplacePolicy(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, `CREATE TABLE insert_replace(id INTEGER PRIMARY KEY ON CONFLICT REPLACE,value TEXT UNIQUE ON CONFLICT REPLACE)`)
	execute(t, e, c, `INSERT INTO insert_replace VALUES(1,'kept')`)
	for _, values := range [][]core.RowChange{
		{insertCell("id", "integer", "1"), insertCell("value", "text", "overwrite")},
		{insertCell("id", "integer", "2"), insertCell("value", "text", "kept")},
	} {
		_, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_replace"}, Confirm: true, Values: values})
		requireMutationCode(t, err, "insert_conflict")
	}
	row := mutationSnapshot(t, e, c, core.TableRef{Name: "insert_replace"})
	if row.Values[0].Value != "1" || row.Values[1].Value != "kept" {
		t.Fatalf("table replace policy overwrote row %#v", row)
	}
	countInsertFixture(t, e, c, "insert_replace", "1")
}

func liveInsertRuleAndSuppression(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, `CREATE TABLE insert_rule(id INTEGER PRIMARY KEY,value TEXT)`)
	execute(t, e, c, `INSERT INTO insert_rule VALUES(1,'kept')`)
	execute(t, e, c, `CREATE RULE insert_rule AS ON INSERT TO insert_rule DO INSTEAD UPDATE insert_rule SET value=NEW.value WHERE id=NEW.id`)
	_, err := e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_rule"}, Confirm: true, Values: []core.RowChange{insertCell("id", "integer", "1"), insertCell("value", "text", "overwrite")}})
	requireMutationCode(t, err, "insert_not_supported")
	row := mutationSnapshot(t, e, c, core.TableRef{Name: "insert_rule"})
	if row.Values[1].Value != "kept" {
		t.Fatal("rewrite rule overwrote existing row")
	}
	execute(t, e, c, `CREATE TABLE insert_suppressed(id BIGSERIAL PRIMARY KEY,value TEXT)`)
	execute(t, e, c, `CREATE FUNCTION insert_suppressed_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$`)
	execute(t, e, c, `CREATE TRIGGER insert_suppressed BEFORE INSERT ON insert_suppressed FOR EACH ROW EXECUTE FUNCTION insert_suppressed_fn()`)
	_, err = e.Insert(context.Background(), core.InsertRequest{Connection: c, Table: core.TableRef{Name: "insert_suppressed"}, Values: []core.RowChange{}, Confirm: true})
	requireMutationCode(t, err, "insert_not_applied")
	countInsertFixture(t, e, c, "insert_suppressed", "0")
}
