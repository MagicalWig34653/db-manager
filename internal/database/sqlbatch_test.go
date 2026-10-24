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
)

func batchRequest(c core.Connection, query string) core.SQLBatchRequest {
	return core.SQLBatchRequest{Connection: c, SQL: query, Confirm: true}
}
func dumpSQLBatch(t *testing.T, kind string, v any) {
	t.Helper()
	if dir := os.Getenv("DBM_SQLBATCH_DUMP"); dir != "" {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.ReplaceAll(t.Name(), "/", "_") + "." + kind + ".json"
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			name = strings.ReplaceAll(t.Name(), "/", "_") + "-explain." + kind + ".json"
		}
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func checkSQLBatch(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE p15_rows (id INTEGER PRIMARY KEY, value VARCHAR(30))")
	request := batchRequest(c, "INSERT INTO p15_rows VALUES (1, 'one');INSERT INTO p15_rows VALUES (2, 'two');INSERT INTO p15_rows VALUES (1, 'duplicate');INSERT INTO p15_rows VALUES (3, 'three');SELECT * FROM p15_rows ORDER BY id;")
	got, err := e.SQLBatch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Statements) != 5 || got.Statements[0].Outcome != "completed" || got.Statements[1].Outcome != "completed" || got.Statements[2].Outcome != "outcome_unknown" || got.Statements[3].Outcome != "not_executed" || got.Statements[4].Outcome != "not_executed" {
		t.Fatalf("stop: %#v", got)
	}
	if got.Statements[0].RowsAffected == nil || *got.Statements[0].RowsAffected != "1" {
		t.Fatalf("affected: %#v", got)
	}
	dumpSQLBatch(t, "SQLBatchResult", got)
	if count := execute(t, e, c, "SELECT count(*) FROM p15_rows").Results[0].Rows[0][0].Value; count != "2" {
		t.Fatal(count)
	}
	request.SQL = "INSERT INTO p15_rows VALUES (1, 'duplicate');INSERT INTO p15_rows VALUES (3, 'three');SELECT id FROM p15_rows ORDER BY id"
	request.Mode = "continue"
	got, err = e.SQLBatch(ctx, request)
	if err != nil || got.Statements[1].Outcome != "completed" || len(got.Statements[2].Results[0].Rows) != 3 {
		t.Fatalf("continue %#v %v", got, err)
	}
	got, err = e.SQLExplain(ctx, batchRequest(c, "SELECT * FROM p15_rows WHERE id=1"))
	if err != nil || got.Statements[0].Outcome != "completed" || len(got.Statements[0].Results[0].Rows) == 0 {
		t.Fatalf("explain %#v %v", got, err)
	}
	dumpSQLBatch(t, "SQLBatchResult", got)
	sim, err := e.SQLSimulate(ctx, batchRequest(c, "UPDATE p15_rows SET value='preview' WHERE id=1"))
	if c.Driver == "sqlite" {
		if err != nil || sim.Outcome != "rolled_back" || sim.RowsAffected == nil || *sim.RowsAffected != "1" {
			t.Fatalf("simulate %#v %v", sim, err)
		}
		dumpSQLBatch(t, "SQLSimulationResult", sim)
	} else if err == nil {
		t.Fatal("unsafe simulation accepted")
	}
	if value := execute(t, e, c, "SELECT value FROM p15_rows WHERE id=1").Results[0].Rows[0][0].Value; value != "one" {
		t.Fatal("simulation persisted", value)
	}
	update, err := e.SQLBatch(ctx, batchRequest(c, "UPDATE p15_rows SET value='RETURNING' WHERE id=1 /* RETURNING */"))
	if err != nil || update.Statements[0].RowsAffected == nil || *update.Statements[0].RowsAffected != "1" {
		t.Fatalf("quoted RETURNING lost affected count: %v", err)
	}

	request = batchRequest(c, "SELECT id FROM p15_rows ORDER BY id; INSERT INTO p15_rows VALUES (4,'four')")
	request.MaxRows = 1
	got, err = e.SQLBatch(ctx, request)
	if err != nil || !got.Statements[0].Results[0].Truncated || got.Statements[1].Outcome != "not_executed" {
		t.Fatalf("limit %#v %v", got, err)
	}
}
func TestSQLiteSQLBatch(t *testing.T) { checkSQLBatch(t, testEngine(t), sqliteConnection()) }
func TestSQLBatchValidation(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for _, request := range []core.SQLBatchRequest{
		{Connection: c, SQL: "CREATE TABLE forbidden(x)"}, batchRequest(c, "SELECT 1; SELECT 'unclosed"), batchRequest(c, "BEGIN;SELECT 1;COMMIT"), batchRequest(c, "SET autocommit=0"), batchRequest(c, "/*! BEGIN */"), batchRequest(c, "/* /* */ BEGIN -- */ SELECT"),
	} {
		if _, err := e.SQLBatch(context.Background(), request); err == nil {
			t.Fatal("invalid batch accepted", request.SQL)
		}
	}
	for _, query := range []string{"EXPLAIN ANALYZE SELECT 1", "SELECT 1;SELECT 2", "CREATE TABLE forbidden(x)", "ANALYZE SELECT 1"} {
		if _, err := e.SQLExplain(context.Background(), batchRequest(c, query)); err == nil {
			t.Fatal("unsafe explain", query)
		}
	}
	for _, query := range []string{"CREATE TABLE forbidden(x)", "WITH x AS (SELECT 1) DELETE FROM x", "INSERT INTO x VALUES(1) RETURNING *", "DELETE FROM x;COMMIT"} {
		if _, err := e.SQLSimulate(context.Background(), batchRequest(c, query)); err == nil {
			t.Fatal("unsafe simulate", query)
		}
	}
	pg := core.Connection{Driver: "postgres", Database: "probe"}
	if _, err := e.SQLBatch(context.Background(), batchRequest(pg, "/* outer /* inner */ SELECT */ BEGIN")); err == nil {
		t.Fatal("nested comment bypassed transaction check")
	}

	objects, err := e.Catalog(context.Background(), c)
	if err != nil || len(objects) != 0 {
		t.Fatalf("validation had effects %#v %v", objects, err)
	}
}
func TestSQLiteSQLBatchRollbackAndSandbox(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE target (id INTEGER PRIMARY KEY, v TEXT)")
	execute(t, e, c, "CREATE TABLE audit(v TEXT)")
	execute(t, e, c, "CREATE TRIGGER trace AFTER INSERT ON target BEGIN INSERT INTO audit VALUES(new.v); END")
	got, err := e.SQLSimulate(ctx, batchRequest(c, "INSERT INTO target VALUES(1,'preview')"))
	if err != nil || got.Outcome != "rolled_back" {
		t.Fatalf("%#v %v", got, err)
	}
	for _, table := range []string{"target", "audit"} {
		if execute(t, e, c, "SELECT count(*) FROM "+table).Results[0].Rows[0][0].Value != "0" {
			t.Fatal("rollback failed", table)
		}
	}
	for _, query := range []string{"ATTACH DATABASE '/outside' AS bad", "VACUUM INTO '/outside'", "PRAGMA journal_mode=WAL", "SELECT load_extension('/outside')"} {
		got, err := e.SQLBatch(ctx, batchRequest(c, query))
		if err != nil || got.Statements[0].ErrorCode == "" {
			t.Fatalf("sandbox bypass %#v %v", got, err)
		}
		encoded, _ := json.Marshal(got)
		if strings.Contains(string(encoded), "outside") {
			t.Fatal("path leaked")
		}
	}
	got, err = e.SQLSimulate(ctx, batchRequest(c, "INSERT INTO target VALUES(1,'first'),(1,'duplicate')"))
	if err != nil || got.Outcome != "rolled_back" || got.RowsAffected != nil {
		t.Fatalf("failed simulation %#v %v", got, err)
	}
	request := batchRequest(c, "INSERT INTO target WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT x,'preview' FROM n")
	request.TimeoutMS = 20
	canceled, err := e.SQLSimulate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.RowsAffected != nil {
		t.Fatal("canceled simulation supplied a count")
	}
	for _, table := range []string{"target", "audit"} {
		if execute(t, e, c, "SELECT count(*) FROM "+table).Results[0].Rows[0][0].Value != "0" {
			t.Fatal("canceled simulation persisted", table)
		}
	}

}
func TestSQLiteSQLBatchLimitsAndCancellation(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	request := batchRequest(c, "SELECT zeroblob(1048577); SELECT 2; SELECT 3")
	request.Mode = "continue"
	got, err := e.SQLBatch(ctx, request)
	if err != nil || got.Statements[0].ErrorCode != "result_too_large" {
		t.Fatalf("size outcome %s code %s: %v", got.Statements[0].Outcome, got.Statements[0].ErrorCode, err)
	}
	// Aggregate limit: each result fits individually, combined results do not.
	request = batchRequest(c, strings.Repeat("SELECT hex(zeroblob(400000));", 12))
	got, err = e.SQLBatch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	if len(encoded)+1 > MaxResponseBytes {
		t.Fatal("aggregate limit")
	}
	found := false
	for _, s := range got.Statements {
		if s.ErrorCode == "result_too_large" {
			found = true
		}
	}
	if !found {
		t.Fatal("aggregate limit not exercised")
	}
	request = batchRequest(c, "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT sum(x) FROM n;SELECT 2")
	request.TimeoutMS = 20
	start := time.Now()
	got, err = e.SQLBatch(ctx, request)
	if err != nil || got.Statements[0].ErrorCode != "query_canceled" || got.Statements[1].Outcome != "not_executed" || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel %#v %v", got, err)
	}
	execute(t, e, c, "SELECT 1")
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = e.SQLBatch(canceled, batchRequest(c, "SELECT 1")); err == nil {
		t.Fatal("canceled request accepted")
	}
}
func TestLiveSQLBatch(t *testing.T) {
	for _, system := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.env)
			if raw == "" {
				t.Skip("disposable fixture port required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: system.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			checkSQLBatch(t, e, c)
			sleep := "SELECT SLEEP(5)"
			if c.Driver == "postgres" {
				sleep = "SELECT pg_sleep(5)"
			}
			cancelRequest := batchRequest(c, sleep+";INSERT INTO p15_rows VALUES(99,'canceled')")
			cancelRequest.TimeoutMS = 30
			cancelResult, err := e.SQLBatch(context.Background(), cancelRequest)
			if err != nil || cancelResult.Statements[0].ErrorCode != "query_canceled" || cancelResult.Statements[1].Outcome != "not_executed" {
				t.Fatalf("deadline %#v %v", cancelResult, err)
			}
			if count := execute(t, e, c, "SELECT count(*) FROM p15_rows WHERE id=99").Results[0].Rows[0][0].Value; count != "0" {
				t.Fatal("canceled remainder executed")
			}
			if c.Driver == "mysql" {
				warningResult, err := e.SQLBatch(context.Background(), batchRequest(c, "SELECT CAST('not-a-number' AS UNSIGNED)"))
				if err != nil || len(warningResult.Statements[0].Warnings) == 0 {
					t.Fatalf("missing server warning %#v %v", warningResult, err)
				}
			}

			if c.Driver == "mysql" {
				request := batchRequest(c, "DELIMITER $$\nCREATE PROCEDURE p15_proc() BEGIN SELECT 1 AS a; SELECT 2 AS b; END$$\nDELIMITER ;\nCALL p15_proc();\nDROP PROCEDURE p15_proc;")
				result, err := e.SQLBatch(context.Background(), request)
				if err != nil || result.Statements[0].Outcome != "completed" || len(result.Statements[1].Results) < 2 || result.Statements[2].Outcome != "completed" {
					t.Fatalf("routine %#v %v", result, err)
				}
			} else {
				request := batchRequest(c, "CREATE FUNCTION p15_func() RETURNS integer LANGUAGE SQL AS $body$SELECT 42;$body$; SELECT p15_func();DROP FUNCTION p15_func();")
				result, err := e.SQLBatch(context.Background(), request)
				if err != nil || result.Statements[1].Outcome != "completed" {
					t.Fatalf("dollar routine %#v %v", result, err)
				}
			}
		})
	}
}
