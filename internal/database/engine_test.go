// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func sqliteConnection() core.Connection { return core.Connection{Driver: "sqlite", Database: "probe"} }
func execute(t *testing.T, e *Engine, c core.Connection, sql string) core.QueryResult {
	t.Helper()
	r, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: sql})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// dropTable removes a table left behind by another live test that shares the fixture database.
func dropTable(t *testing.T, e *Engine, c core.Connection, name string) {
	t.Helper()
	statement := "DROP TABLE IF EXISTS " + quoteIdentifier(c.Driver, name)
	if c.Driver == "postgres" {
		statement += " CASCADE"
	}
	execute(t, e, c, statement)
}
func TestSQLiteRoundTripAndCatalog(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, `CREATE TABLE "odd'name"(value TEXT, empty TEXT, nil TEXT, binary BLOB, large INTEGER)`)
	execute(t, e, c, `INSERT INTO "odd'name" VALUES('Grüße','',NULL,X'00FF',9007199254740993)`)
	r := execute(t, e, c, `SELECT * FROM "odd'name"`)
	want := []core.Cell{{Kind: "text", Value: "Grüße"}, {Kind: "text", Value: ""}, {Kind: "null"}, {Kind: "binary", Value: "AP8="}, {Kind: "integer", Value: "9007199254740993"}}
	got, _ := json.Marshal(r.Results[0].Rows[0])
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Fatalf("cells %s want %s", got, expected)
	}
	tables, err := e.Catalog(context.Background(), c)
	if err != nil || len(tables) != 1 || tables[0].Name != "odd'name" {
		t.Fatalf("catalog %#v %v", tables, err)
	}
	if err := e.Test(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}
func TestBoundsAndCancellation(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	r, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<20) SELECT x FROM n", MaxRows: 3})
	if err != nil || len(r.Results[0].Rows) != 3 || !r.Results[0].Truncated {
		t.Fatalf("row bound %#v %v", r, err)
	}
	for _, sql := range []string{
		"SELECT zeroblob(1048577)",
		"WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<8) SELECT zeroblob(1048576) FROM n",
		"WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<4) SELECT replace(hex(zeroblob(262144)), '0', char(1)) FROM n",
	} {
		_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: sql})
		var sizeError *core.Error
		if !errors.As(err, &sizeError) || sizeError.Code != "result_too_large" {
			t.Fatalf("size bound error for %q: %v", sql, err)
		}
	}
	start := time.Now()
	_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT sum(x) FROM n", TimeoutMS: 30})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("cancel: %v", err)
	}
	execute(t, e, c, "SELECT 1")
}
func TestSingleStatementAndSafeErrors(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for _, sql := range []string{"CREATE TABLE secret(value TEXT); INSERT INTO secret VALUES('unexpected')", "ATTACH DATABASE '/outside' AS forbidden", "VACUUM INTO '/outside'", "SELECT password_super_secret FROM no_table"} {
		_, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: sql})
		if err == nil {
			t.Fatalf("accepted %q", sql)
		}
		if strings.Contains(err.Error(), "password_super_secret") || strings.Contains(err.Error(), "/outside") {
			t.Fatal("raw error exposed")
		}
	}
	tables, err := e.Catalog(context.Background(), c)
	if err != nil || len(tables) != 0 {
		t.Fatalf("batch was partly executed %#v %v", tables, err)
	}
}
func TestConfigurationAndPolicy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "data")
	e, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Test(context.Background(), sqliteConnection()); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(root, "probe.sqlite", "child"), nil); err == nil {
		t.Fatal("invalid root accepted")
	}
	for _, c := range []core.Connection{
		{Driver: "sqlite", Database: "../outside"}, {Driver: "sqlite", Database: "file:probe?mode=memory"}, {Driver: "mysql", Host: "169.254.169.254"}, {Driver: "postgres", Host: "public.example"}, {Driver: "mysql", Host: "/unix/socket"}, {Driver: "mysql", Port: 65536}, {Driver: "postgres", TLSMode: "prefer"},
	} {
		if err := e.Test(context.Background(), c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
	for _, ip := range []string{"169.254.169.254", "0.0.0.0", "224.0.0.1", "fe80::1", "::"} {
		allowed, err := New(t.TempDir(), []string{ip})
		if err != nil {
			t.Fatal(err)
		}
		if err := allowed.Test(context.Background(), core.Connection{Driver: "mysql", Host: ip}); err == nil {
			t.Fatalf("unsafe IP accepted %s", ip)
		}
	}
}

func TestGlobalConnectionLimitIsCancelable(t *testing.T) {
	e := testEngine(t)
	for i := 0; i < MaxConcurrentRequests; i++ {
		_, closeDB, err := e.open(context.Background(), sqliteConnection())
		if err != nil {
			t.Fatal(err)
		}
		defer closeDB()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := e.open(ctx, sqliteConnection()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("connection limit: %v", err)
	}
}

func TestCellTypeAndRawLimits(t *testing.T) {
	for _, tc := range []struct {
		value           any
		typ, kind, want string
	}{
		{"123456789.123456789", "NUMERIC", "decimal", "123456789.123456789"},
		{"{1,2}", "_INT4", "text", "{1,2}"},
		{"1 day", "INTERVAL", "text", "1 day"},
		{true, "BOOL", "boolean", "true"},
		{time.Date(2026, 1, 2, 3, 4, 5, 123456000, time.UTC), "TIMESTAMP", "timestamp", "2026-01-02T03:04:05.123456"},
		{[]byte{}, "BYTEA", "binary", ""},
	} {
		got, err := cell(tc.value, tc.typ)
		if err != nil || got.Kind != tc.kind || got.Value != tc.want {
			t.Fatalf("%s => %#v %v", tc.typ, got, err)
		}
	}
	if c, err := cell(strings.Repeat("a", MaxCellBytes), "TEXT"); err != nil || len(c.Value) != MaxCellBytes {
		t.Fatal("exact raw limit rejected")
	}
	if _, err := cell(strings.Repeat("a", MaxCellBytes+1), "TEXT"); err == nil {
		t.Fatal("oversized raw value accepted")
	}
	if c, err := cell(make([]byte, MaxCellBytes), "BYTEA"); err != nil || len(c.Value) <= MaxCellBytes {
		t.Fatal("base64 overhead treated as raw-byte overflow")
	}
}

func TestEncodedBinaryResponseWithinLimit(t *testing.T) {
	e := testEngine(t)
	result := execute(t, e, sqliteConnection(), "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<5) SELECT zeroblob(1048576) FROM n")
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes || len(result.Results[0].Rows) != 5 {
		t.Fatalf("bounded positive response %d %v", len(encoded), err)
	}
}
