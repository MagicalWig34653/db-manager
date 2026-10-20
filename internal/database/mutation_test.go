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

func mutationSnapshot(t *testing.T, e *Engine, c core.Connection, ref core.TableRef) core.RowSnapshot {
	t.Helper()
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: ref, PageSize: 1})
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("snapshot: %#v %v", page, err)
	}
	columns := []string{}
	for _, column := range page.Description.Columns {
		columns = append(columns, column.Name)
	}
	return core.RowSnapshot{Columns: columns, Values: page.Result.Rows[0]}
}
func requireMutationCode(t *testing.T, err error, code string) {
	t.Helper()
	var p *core.Error
	if !errors.As(err, &p) || p.Code != code {
		t.Fatalf("want %s got %v", code, err)
	}
}
func TestSQLiteMutations(t *testing.T) { mutationCases(t, testEngine(t), sqliteConnection()) }
func mutationCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	name := "edit\"row'`"
	quote := func(v string) string { return quoteIdentifier(c.Driver, v) }
	ref := core.TableRef{Name: name}
	blob := "BLOB"
	if c.Driver == "postgres" {
		blob = "BYTEA"
	}
	execute(t, e, c, "CREATE TABLE "+quote(name)+"(a INTEGER NOT NULL,b INTEGER NOT NULL,value TEXT,raw "+blob+",amount DECIMAL(30,5) DEFAULT 1.25,calc INTEGER GENERATED ALWAYS AS(a+b) STORED,PRIMARY KEY(b,a))")
	execute(t, e, c, "INSERT INTO "+quote(name)+"(a,b,value,raw,amount) VALUES(1,2,'initial',"+binaryLiteral(c.Driver)+",12345678901234567890.12345)")
	eligible, err := e.Eligibility(ctx, core.DescribeRequest{Connection: c, Table: ref})
	if err != nil || !eligible.Eligible || len(eligible.WritableColumns) != 3 {
		t.Fatalf("eligibility %#v %v", eligible, err)
	}
	original := mutationSnapshot(t, e, c, ref)
	request := core.MutationRequest{Connection: c, Table: ref, Expected: original, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "updated'; DROP TABLE x;--"}}}, Confirm: true}
	result, err := e.Update(ctx, request)
	if err != nil || result.Outcome != "committed" || !result.Changed || result.Row.Values[2] != request.Changes[0].Value {
		t.Fatalf("update %#v %v", result, err)
	}
	if result.Row.Values[3] != original.Values[3] || result.Row.Values[4] != original.Values[4] || result.Row.Values[5] != original.Values[5] {
		t.Fatal("unrelated precision/binary/generated changed")
	}
	_, err = e.Update(ctx, request)
	requireMutationCode(t, err, "row_conflict")
	request.Expected = *result.Row
	noop, err := e.Update(ctx, request)
	if err != nil || noop.Changed {
		t.Fatalf("noop %#v %v", noop, err)
	}
	for _, column := range []string{"a", "calc", "not-real"} {
		bad := request
		bad.Changes = []core.RowChange{{Column: column, Value: core.Cell{Kind: "integer", Value: "9"}}}
		_, err = e.Update(ctx, bad)
		requireMutationCode(t, err, "invalid_mutation")
	}
	bad := request
	bad.Confirm = false
	_, err = e.Update(ctx, bad)
	requireMutationCode(t, err, "invalid_mutation")
	bad = request
	bad.Expected.Columns = append([]string{}, bad.Expected.Columns...)
	bad.Expected.Columns[0] = "wrong"
	_, err = e.Update(ctx, bad)
	requireMutationCode(t, err, "row_conflict")
	// NULL and empty blobs retain distinct canonical storage representations.
	request.Changes = []core.RowChange{{Column: "value", Value: core.Cell{Kind: "null"}}, {Column: "raw", Value: core.Cell{Kind: "binary", Value: ""}}}
	result, err = e.Update(ctx, request)
	if err != nil || result.Row.Values[2].Kind != "null" || result.Row.Values[3] != (core.Cell{Kind: "binary", Value: ""}) {
		t.Fatalf("null/blob %#v %v", result, err)
	}
	request.Expected = *result.Row
	request.Changes = []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: ""}}}
	result, err = e.Update(ctx, request)
	if err != nil || result.Row.Values[2] != (core.Cell{Kind: "text", Value: ""}) {
		t.Fatalf("empty text %#v %v", result, err)
	}
	request.Expected = *result.Row
	// A concurrent writer can commit once; the second stale expectation cannot overwrite it.
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, text := range []string{"writer-one", "writer-two"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			r := request
			r.Changes = []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: text}}}
			_, err := e.Update(ctx, r)
			outcomes <- err
		}(text)
	}
	wg.Wait()
	close(outcomes)
	success := 0
	for err := range outcomes {
		if err == nil {
			success++
		} else {
			var p *core.Error
			if !errors.As(err, &p) || (p.Code != "row_conflict" && p.Code != "database_busy") {
				t.Fatalf("parallel update: %v", err)
			}
		}
	}
	if success != 1 {
		t.Fatalf("parallel success count %d", success)
	}
	request.Expected = mutationSnapshot(t, e, c, ref)
	request.Changes = nil
	result, err = e.Delete(ctx, request)
	if err != nil || !result.Changed || result.Row != nil {
		t.Fatalf("delete %#v %v", result, err)
	}
	_, err = e.Delete(ctx, request)
	requireMutationCode(t, err, "row_conflict")
	execute(t, e, c, "CREATE TABLE edit_no_key(value TEXT)")
	eligible, err = e.Eligibility(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "edit_no_key"}})
	if err != nil || eligible.Eligible {
		t.Fatalf("noPK %#v %v", eligible, err)
	}
	execute(t, e, c, "CREATE VIEW edit_view AS SELECT * FROM "+quote(name))
	eligible, err = e.Eligibility(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "edit_view"}})
	if err != nil || eligible.Eligible {
		t.Fatalf("view %#v %v", eligible, err)
	}
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE TABLE edit_myisam(id INTEGER PRIMARY KEY,value TEXT) ENGINE=MyISAM")
		eligible, err = e.Eligibility(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "edit_myisam"}})
		if err != nil || eligible.Eligible {
			t.Fatalf("myisam %#v %v", eligible, err)
		}
	}
}
func TestSQLiteMutationCollationAndRuntimeConflict(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := core.TableRef{Name: "collated_edit"}
	execute(t, e, c, `CREATE TABLE collated_edit(a TEXT COLLATE NOCASE NOT NULL,value TEXT,PRIMARY KEY(a COLLATE BINARY))`)
	execute(t, e, c, `INSERT INTO collated_edit VALUES('a','one'),('A','two')`)
	snapshot := mutationSnapshot(t, e, c, ref)
	r := core.MutationRequest{Connection: c, Table: ref, Expected: snapshot, Confirm: true, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "changed"}}}}
	got, err := e.Update(context.Background(), r)
	if err != nil || got.Row.Values[0].Value != "A" {
		t.Fatalf("collated identity %#v %v", got, err)
	}
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: ref})
	if err != nil || page.Result.Rows[1][1].Value != "one" {
		t.Fatalf("other key modified %#v %v", page, err)
	}
	r.Expected = *got.Row
	execute(t, e, c, `UPDATE collated_edit SET value=X'6368616e676564' WHERE a COLLATE BINARY='A'`)
	_, err = e.Update(context.Background(), r)
	requireMutationCode(t, err, "row_conflict")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = e.Update(ctx, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
}
func TestLiveMutations(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("B2b disposable fixture required")
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
			mutationCases(t, e, c)
			liveMutationLock(t, e, c)
			liveMutationStatementTimeout(t, e, c)
			if c.Driver == "postgres" {
				liveDeferredCommitFailure(t, e, c)
				livePostgresInheritance(t, e, c)
			}
		})
	}
}
func liveMutationLock(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, "CREATE TABLE edit_locked(id INTEGER PRIMARY KEY,value TEXT)")
	execute(t, e, c, "INSERT INTO edit_locked VALUES(1,'kept')")
	ref := core.TableRef{Name: "edit_locked"}
	snapshot := mutationSnapshot(t, e, c, ref)
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
	if _, err = tx.ExecContext(context.Background(), "UPDATE edit_locked SET value='locked' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = e.Update(context.Background(), core.MutationRequest{Connection: c, Table: ref, Expected: snapshot, Confirm: true, TimeoutMS: 30, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "blocked"}}}})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatalf("lock timeout %v", err)
	}
	tx.Rollback()
	if err = e.Test(context.Background(), c); err != nil {
		t.Fatalf("post timeout connection %v", err)
	}
}

func TestSQLiteMutationRejectsAmbiguousTextStorage(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := core.TableRef{Name: "invalid_text"}
	execute(t, e, c, `CREATE TABLE invalid_text(id INTEGER PRIMARY KEY,value TEXT)`)
	execute(t, e, c, `INSERT INTO invalid_text VALUES(1,CAST(X'FF' AS TEXT))`)
	r := core.MutationRequest{Connection: c, Table: ref, Expected: mutationSnapshot(t, e, c, ref), Confirm: true}
	_, err := e.Delete(context.Background(), r)
	requireMutationCode(t, err, "mutation_not_supported")
}

func liveDeferredCommitFailure(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, `CREATE TABLE edit_parent(id INTEGER PRIMARY KEY)`)
	execute(t, e, c, `INSERT INTO edit_parent VALUES(1)`)
	execute(t, e, c, `CREATE TABLE edit_commit(id INTEGER PRIMARY KEY,parent INTEGER REFERENCES edit_parent(id) DEFERRABLE INITIALLY DEFERRED)`)
	execute(t, e, c, `INSERT INTO edit_commit VALUES(1,1)`)
	ref := core.TableRef{Name: "edit_commit"}
	before := mutationSnapshot(t, e, c, ref)
	result, err := e.Update(context.Background(), core.MutationRequest{Connection: c, Table: ref, Expected: before, Confirm: true, Changes: []core.RowChange{{Column: "parent", Value: core.Cell{Kind: "integer", Value: "99"}}}})
	requireMutationCode(t, err, "outcome_unknown")
	if result.Outcome != "" {
		t.Fatalf("commit error returned success %#v", result)
	}
	after := mutationSnapshot(t, e, c, ref)
	if after.Values[1] != before.Values[1] {
		t.Fatal("deferred constraint failure persisted")
	}
}

func liveMutationStatementTimeout(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, "CREATE TABLE edit_slow(id INTEGER PRIMARY KEY,value TEXT)")
	execute(t, e, c, "INSERT INTO edit_slow VALUES(1,'kept')")
	if c.Driver == "postgres" {
		execute(t, e, c, `CREATE FUNCTION edit_delay() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.3); RETURN NEW; END $$`)
		execute(t, e, c, `CREATE TRIGGER edit_delay BEFORE UPDATE ON edit_slow FOR EACH ROW EXECUTE FUNCTION edit_delay()`)
	} else {
		// MySQL does not prepare CREATE TRIGGER; setup uses fixture administration.
		db, cleanup, err := e.open(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.ExecContext(context.Background(), `CREATE TRIGGER edit_delay BEFORE UPDATE ON edit_slow FOR EACH ROW SET @b2b_delay=SLEEP(0.3)`)
		cleanup()
		if err != nil {
			t.Fatalf("fixture trigger: %v", err)
		}
	}
	ref := core.TableRef{Name: "edit_slow"}
	before := mutationSnapshot(t, e, c, ref)
	_, err := e.Update(context.Background(), core.MutationRequest{Connection: c, Table: ref, Expected: before, Confirm: true, TimeoutMS: 50, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "changed"}}}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("statement timeout: %v", err)
	}
	after := mutationSnapshot(t, e, c, ref)
	if after.Values[1] != before.Values[1] {
		t.Fatalf("timed out update persisted: %#v", after)
	}
}
func TestSQLiteMutationRollbackAndBusy(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := core.TableRef{Name: "rollback_edit"}
	execute(t, e, c, `CREATE TABLE rollback_edit(id INTEGER PRIMARY KEY,value TEXT NOT NULL CHECK(value<>'rejected'))`)
	execute(t, e, c, `INSERT INTO rollback_edit VALUES(1,'kept')`)
	before := mutationSnapshot(t, e, c, ref)
	r := core.MutationRequest{Connection: c, Table: ref, Expected: before, Confirm: true, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "rejected"}}}}
	if _, err := e.Update(context.Background(), r); err == nil {
		t.Fatal("constraint accepted")
	}
	after := mutationSnapshot(t, e, c, ref)
	if after.Values[1] != before.Values[1] {
		t.Fatal("failed statement persisted")
	}
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		conn.Close()
		cleanup()
		t.Fatal(err)
	}
	start := time.Now()
	_, err = e.Update(context.Background(), r)
	requireMutationCode(t, err, "database_busy")
	if time.Since(start) > time.Second {
		t.Fatal("busy lock blocked")
	}
	conn.ExecContext(context.Background(), "ROLLBACK")
	conn.Close()
	cleanup()
	if err = e.Test(context.Background(), c); err != nil {
		t.Fatalf("busy connection release %v", err)
	}
	execute(t, e, c, `CREATE TABLE nullable_edit(id TEXT PRIMARY KEY,value TEXT)`)
	eligible, err := e.Eligibility(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "nullable_edit"}})
	if err != nil || eligible.Eligible {
		t.Fatalf("nullable pk %#v %v", eligible, err)
	}
}

func TestSQLiteDeferredCommitFailure(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	liveDeferredCommitFailure(t, e, c)
}

func livePostgresInheritance(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, `CREATE TABLE edit_inherit_parent(id INTEGER PRIMARY KEY,value TEXT)`)
	execute(t, e, c, `CREATE TABLE edit_inherit_child() INHERITS(edit_inherit_parent)`)
	execute(t, e, c, `INSERT INTO edit_inherit_parent VALUES(1,'parent')`)
	execute(t, e, c, `INSERT INTO edit_inherit_child VALUES(1,'child')`)
	execute(t, e, c, `CREATE TABLE edit_partition_parent(id INTEGER PRIMARY KEY,value TEXT) PARTITION BY RANGE(id)`)
	for _, name := range []string{"edit_inherit_parent", "edit_inherit_child", "edit_partition_parent"} {
		eligible, err := e.Eligibility(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: name}})
		if err != nil || eligible.Eligible {
			t.Fatalf("inheritance %s %#v %v", name, eligible, err)
		}
	}
	_, err := e.Delete(context.Background(), core.MutationRequest{Connection: c, Table: core.TableRef{Name: "edit_inherit_parent"}, Expected: core.RowSnapshot{Columns: []string{"id", "value"}, Values: []core.Cell{{Kind: "integer", Value: "1"}, {Kind: "text", Value: "parent"}}}, Confirm: true})
	requireMutationCode(t, err, "mutation_not_supported")
}
