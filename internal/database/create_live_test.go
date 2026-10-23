// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func liveCreateBoundaries(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	if c.Driver == "mysql" {
		liveMySQLCreateBoundaries(t, e, c)
		liveMySQLCreateMDL(t, e, c)
		return
	}
	// A competing uncommitted CREATE owns the name; another CREATE must wait/cancel.
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("CREATE TABLE public.b4a_name_race(original BIGINT)"); err != nil {
		t.Fatal(err)
	}
	r := createFixture(c, "b4a_name_race")
	r.TimeoutMS = 100
	_, err = e.CreateTable(context.Background(), r)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DDL name lock did not cancel: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	_, err = e.CreateTable(context.Background(), r)
	requireMutationCode(t, err, "create_conflict")
	d, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: r.Table})
	if err != nil || len(d.Columns) != 1 || d.Columns[0].Name != "original" {
		t.Fatalf("race replaced winner %#v %v", d, err)
	}
	execute(t, e, c, "CREATE ROLE b4a_no_create LOGIN PASSWORD 'public-b4a-fixture'")
	execute(t, e, c, "GRANT USAGE ON SCHEMA public TO b4a_no_create")
	denied := c
	denied.Username = "b4a_no_create"
	denied.Password = "public-b4a-fixture"
	_, err = e.CreateTable(context.Background(), createFixture(denied, "b4a_denied"))
	requireMutationCode(t, err, "database_error")
	// A real deferred check introduced by a DDL event trigger rejects COMMIT.
	execute(t, e, c, "CREATE TABLE b4a_event_parent(id INTEGER PRIMARY KEY)")
	execute(t, e, c, "CREATE TABLE b4a_event_audit(id INTEGER REFERENCES b4a_event_parent(id) DEFERRABLE INITIALLY DEFERRED)")
	execute(t, e, c, `CREATE FUNCTION b4a_fail_commit() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS(SELECT 1 FROM pg_event_trigger_ddl_commands() WHERE object_identity='public.b4a_commit_failure') THEN INSERT INTO b4a_event_audit VALUES(99); END IF; END $$`)
	execute(t, e, c, "CREATE EVENT TRIGGER b4a_commit_guard ON ddl_command_end WHEN TAG IN ('CREATE TABLE') EXECUTE FUNCTION b4a_fail_commit()")
	_, err = e.CreateTable(context.Background(), createFixture(c, "b4a_commit_failure"))
	requireMutationCode(t, err, "outcome_unknown")
	execute(t, e, c, "DROP EVENT TRIGGER b4a_commit_guard")
	_, err = e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b4a_commit_failure"}})
	requireMutationCode(t, err, "table_not_found")
}
func liveMySQLCreateBoundaries(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, "CREATE USER 'b4a_create_only'@'%' IDENTIFIED BY 'public-b4a-fixture'")
	execute(t, e, c, "GRANT CREATE ON probe.* TO 'b4a_create_only'@'%'")
	limited := c
	limited.Username = "b4a_create_only"
	limited.Password = "public-b4a-fixture"
	result, err := e.CreateTable(context.Background(), createFixture(limited, "b4a_unavailable"))
	if err != nil || result.Outcome != "created" || result.StructureCheck != "unavailable" || result.SnapshotAtomic {
		t.Fatalf("CREATE ack incorrectly lost after denied SELECT %#v %v", result, err)
	}
	if _, err = e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b4a_unavailable"}}); err != nil {
		t.Fatal("ack did not correspond to actual CREATE", err)
	}
	// This lock affects only the disposable fixture and is released on every path.
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	control, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	if _, err = control.ExecContext(context.Background(), "FLUSH TABLES WITH READ LOCK"); err != nil {
		t.Fatal(err)
	}
	defer control.ExecContext(context.Background(), "UNLOCK TABLES")
	r := createFixture(c, "b4a_lock_cancel")
	r.TimeoutMS = 100
	started := time.Now()
	_, err = e.CreateTable(context.Background(), r)
	requireMutationCode(t, err, "outcome_unknown")
	if time.Since(started) > 3*time.Second {
		t.Fatal("blocked CREATE did not cancel promptly")
	}
	if _, err = control.ExecContext(context.Background(), "UNLOCK TABLES"); err != nil {
		t.Fatal(err)
	}
	if _, err = e.CreateTable(context.Background(), createFixture(c, "b4a_after_cancel")); err != nil {
		t.Fatal("connection/slot not released", err)
	}
}

type createObservationRace struct {
	*createSession
	change func() error
}

func (s *createObservationRace) observeCreate(ctx context.Context, r core.CreateRequest) (core.TableDescription, error) {
	if err := s.change(); err != nil {
		return core.TableDescription{}, err
	}
	return s.createSession.observeCreate(ctx, r)
}
func liveCreateSchemaAndObservation(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	schema := `b4a"schema`
	prefix := "CREATE SCHEMA "
	if c.Driver == "mysql" {
		prefix = "CREATE DATABASE "
	}
	execute(t, e, c, prefix+quoteIdentifier(c.Driver, schema))
	r := createFixture(c, "b4a_schema_table")
	r.Table.Schema = schema
	if result, err := e.CreateTable(context.Background(), r); err != nil || result.ActualDescription == nil || result.ActualDescription.Table.Schema != schema {
		t.Fatalf("explicit schema %#v %v", result, err)
	}
	missing := createFixture(c, "absent")
	missing.Table.Schema = "b4a_missing_schema"
	_, err := e.CreateTable(context.Background(), missing)
	requireMutationCode(t, err, "database_error")
	if c.Driver != "mysql" {
		return
	}
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	r = createFixture(c, "b4a_observation_race")
	r.Columns = r.Columns[:1]
	r.PrimaryKey = []string{}
	r.Table, _ = createRef(c, r.Table)
	statement, err := createSQL(r)
	if err != nil {
		t.Fatal(err)
	}
	s := &createObservationRace{createSession: &createSession{session: &mutationSession{conn: conn, driver: "mysql"}}, change: func() error {
		_, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "ALTER TABLE probe.b4a_observation_race ADD COLUMN changed_after_ack BIGINT"})
		return err
	}}
	result, err := executeCreate(context.Background(), r, statement, s, false)
	if err != nil || result.Outcome != "created" || result.StructureCheck != "different" || result.SnapshotAtomic || result.ActualDescription == nil || len(result.ActualDescription.Columns) != 2 {
		t.Fatalf("separate real observation %#v %v", result, err)
	}
	d, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: r.Table})
	if err != nil || len(d.Columns) != 2 {
		t.Fatalf("race table repaired/dropped %#v %v", d, err)
	}
}

func liveMySQLCreateMDL(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	r := createFixture(c, "b4a_mdl")
	r.Columns = r.Columns[:1]
	r.PrimaryKey = []string{}
	if _, err := e.CreateTable(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT 1 FROM probe.b4a_mdl LIMIT 0")
	if err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = e.Query(ctx, core.QueryRequest{Connection: c, SQL: "ALTER TABLE probe.b4a_mdl ADD COLUMN raced BIGINT"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DDL not blocked by separate observation MDL: %v", err)
	}
	// No claim about the pending DDL after this read transaction releases the lock.
	d, err := describe(context.Background(), tx, c, core.TableRef{Schema: "probe", Name: "b4a_mdl"})
	if err != nil || len(d.Columns) != 1 {
		t.Fatalf("locked observation changed %#v %v", d, err)
	}
}
