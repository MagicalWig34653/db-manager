// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func exportBool(v bool) *bool { return &v }
func exportFixture(t *testing.T, e *Engine, c core.Connection) core.TableRef {
	t.Helper()
	name := "ex\"` port';--"
	q := quoteIdentifier(c.Driver, name)
	binary, large, decimal := "BLOB", "INTEGER", "TEXT"
	timestamp := "TEXT"
	if c.Driver == "mysql" {
		binary, large, decimal = "BLOB", "BIGINT", "DECIMAL(40,20)"
		timestamp = "TIMESTAMP(6)"
	}
	if c.Driver == "postgres" {
		binary, large, decimal = "BYTEA", "BIGINT", "NUMERIC(40,20)"
		timestamp = "TIMESTAMPTZ"
	}
	// Several live tests share one fixture database; start from a clean table each time.
	drop := "DROP TABLE IF EXISTS " + q
	if c.Driver == "postgres" {
		drop += " CASCADE" // other live tests attach dependent views
	}
	execute(t, e, c, drop)
	execute(t, e, c, "CREATE TABLE "+q+" (id "+large+", text_value TEXT, empty_value TEXT, nil_value TEXT, bytes_value "+binary+", exact_value "+decimal+", utc_value "+timestamp+")")
	bin := "X'00ff27225c'"
	if c.Driver == "postgres" {
		bin = "decode('00ff27225c','hex')"
	}
	execute(t, e, c, "INSERT INTO "+q+" VALUES (9007199254740993, 'Grüße ''雪, line\n=1+1', '', NULL, "+bin+", '12345678901234567890.12345678901234567890','2026-01-02 03:04:05.123456')")
	execute(t, e, c, "CREATE INDEX "+quoteIdentifier(c.Driver, "export index")+" ON "+q+" (id)")
	return core.TableRef{Name: name}
}

func TestSQLiteExportRoundTrip(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := exportFixture(t, e, c)
	// Embedded NUL, backslash and storage classes must survive SQL restoration.
	execute(t, e, c, "INSERT INTO "+quoteIdentifier(c.Driver, ref.Name)+" VALUES (2,CAST(X'6100625c27' AS TEXT),'',NULL,X'', '0', '2026-01-02T03:04:05.123456Z')")
	for _, extended := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "extended"}[extended], func(t *testing.T) {
			var out bytes.Buffer
			r := core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Extended: extended, Transaction: true}
			if err := e.Export(context.Background(), r, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(out.String(), "-- DBMANAGER EXPORT COMPLETE\n") {
				t.Fatal("missing completion")
			}
			target := c
			target.Database = map[bool]string{false: "restored", true: "restoredextended"}[extended]
			db, cleanup, err := e.open(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			conn, err := db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			err = conn.Raw(func(raw any) error { return raw.(sqliteDriver.Conn).Raw().Exec(out.String()) })
			conn.Close()
			cleanup()
			if err != nil {
				t.Fatal(err)
			}
			source := execute(t, e, c, "SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name)+" ORDER BY id")
			restored := execute(t, e, target, "SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name)+" ORDER BY id")
			if !reflect.DeepEqual(source.Results, restored.Results) {
				t.Fatalf("roundtrip mismatch: %#v %#v", source, restored)
			}
			sourceDDL := queryText(t, e, c, "SELECT group_concat(sql,';') FROM (SELECT sql FROM sqlite_schema WHERE sql IS NOT NULL AND type IN ('table','index') ORDER BY type,name)")
			restoredDDL := queryText(t, e, target, "SELECT group_concat(sql,';') FROM (SELECT sql FROM sqlite_schema WHERE sql IS NOT NULL AND type IN ('table','index') ORDER BY type,name)")
			if sourceDDL != restoredDDL {
				t.Fatal("schema mismatch")
			}
		})
	}
}

func TestSQLiteExportReaders(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := exportFixture(t, e, c)
	for _, format := range []string{"json", "csv", "excel-csv"} {
		var out bytes.Buffer
		if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}, &out); err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("DBM_EXPORT_DUMP"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, "sqlite."+format), out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if format == "json" {
			var result struct {
				Complete bool `json:"complete"`
				Tables   []struct {
					Rows [][]core.Cell `json:"rows"`
				} `json:"tables"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			row := result.Tables[0].Rows[0]
			if !result.Complete || row[0].Value != "9007199254740993" || row[2].Kind != "text" || row[2].Value != "" || row[3].Kind != "null" || row[4].Value != "AP8nIlw=" {
				t.Fatalf("typed JSON: %#v", row)
			}
		} else {
			text := strings.TrimPrefix(out.String(), "\ufeff")
			reader := csv.NewReader(strings.NewReader(text))
			if format == "excel-csv" {
				reader.Comma = ';'
			}
			records, err := reader.ReadAll()
			if err != nil || len(records) != 3 {
				t.Fatalf("CSV reader: %#v %v", records, err)
			}
			prefix := ""
			if format == "excel-csv" {
				prefix = "'"
			}
			if records[1][4] != prefix+"\\N" || records[1][5] != prefix+"base64:AP8nIlw=" {
				t.Fatal(records)
			}
		}
	}
}

type exportFailWriter struct{ count int }

func (w *exportFailWriter) Write(p []byte) (int, error) {
	w.count++
	if w.count > 2 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func TestSQLiteExportLimitsAndAbort(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := exportFixture(t, e, c)
	execute(t, e, c, "INSERT INTO "+quoteIdentifier(c.Driver, ref.Name)+" SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name))
	for _, r := range []core.ExportRequest{
		{Connection: c, Tables: []core.TableRef{ref}, MaxRows: 1},
		{Connection: c, Tables: []core.TableRef{ref}, MaxBytes: 10},
	} {
		var out bytes.Buffer
		err := e.Export(context.Background(), r, &out)
		var p *core.Error
		if !errors.As(err, &p) || (p.Code != "export_row_limit" && p.Code != "export_byte_limit") || strings.Contains(out.String(), "EXPORT COMPLETE") {
			t.Fatalf("limit: %v %s", err, out.String())
		}
	}
	w := &exportFailWriter{}
	if e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}}, w) == nil {
		t.Fatal("disconnect accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e.Export(ctx, core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}}, io.Discard) == nil {
		t.Fatal("cancel accepted")
	}
	if err := e.Test(context.Background(), c); err != nil {
		t.Fatal("connection leaked", err)
	}
}

func TestSQLiteExportQueryReadOnly(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE guard (id INTEGER)")
	for _, query := range []string{"DELETE FROM guard RETURNING id", "SELECT 1; DELETE FROM guard", "PRAGMA query_only=OFF", "ATTACH ':memory:' AS bad"} {
		var out bytes.Buffer
		if e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "query", Format: "json", SQL: query}, &out) == nil {
			t.Fatal("unsafe query accepted", query)
		}
	}
	var out bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "query", Format: "json", SQL: "SELECT 9007199254740993 AS x, NULL AS x"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"complete":true`) {
		t.Fatal(out.String())
	}
}

func TestExportLiteral(t *testing.T) {
	tests := []struct {
		driver string
		cell   core.Cell
		want   string
	}{
		{"sqlite", core.Cell{Kind: "text", Value: "a'b"}, "'a''b'"},
		{"sqlite", core.Cell{Kind: "text", Value: "a\x00b"}, "CAST(X'610062' AS TEXT)"},
		{"postgres", core.Cell{Kind: "text", Value: "'\\"}, `E'''\\'`},
		{"mysql", core.Cell{Kind: "text", Value: "'\\"}, "CONVERT(X'275c' USING utf8mb4)"},
		{"postgres", core.Cell{Kind: "binary", Value: "AP8="}, "decode('00ff','hex')"},
		{"sqlite", core.Cell{Kind: "float", Value: "1"}, "1.0"},
	}
	for _, tt := range tests {
		got, err := exportLiteral(tt.driver, tt.cell)
		if err != nil || got != tt.want {
			t.Errorf("%s: %q != %q (%v)", tt.driver, got, tt.want, err)
		}
	}
	for _, kind := range []string{"integer", "decimal", "float"} {
		if _, err := exportLiteral("mysql", core.Cell{Kind: kind, Value: "0);DROP TABLE x"}); err == nil {
			t.Fatal("numeric injection")
		}
	}
}

type exportCancelWriter struct {
	cancel context.CancelFunc
	bytes  int
}

func (w *exportCancelWriter) Write(p []byte) (int, error) {
	w.bytes += len(p)
	w.cancel()
	return len(p), nil
}

func TestSQLiteExportScopesAndOptions(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, `CREATE TABLE generated (id INTEGER PRIMARY KEY, value TEXT, computed INTEGER GENERATED ALWAYS AS (id+2) STORED)`)
	execute(t, e, c, `INSERT INTO generated(id,value) VALUES(1,'=1+1')`)
	for _, r := range []core.ExportRequest{
		{Connection: c, Scope: "database", Format: "json"},
		{Connection: c, Scope: "server", Format: "json", Databases: []string{c.Database}},
		{Connection: c, Tables: []core.TableRef{{Name: "generated"}}, Structure: exportBool(true), Data: exportBool(false), Drop: true, IfNotExists: true},
		{Connection: c, Tables: []core.TableRef{{Name: "generated"}}, Structure: exportBool(false), Charset: "utf-8-bom"},
	} {
		var out bytes.Buffer
		if err := e.Export(context.Background(), r, &out); err != nil {
			t.Fatal(err)
		}
		if r.Format == "json" && !json.Valid(out.Bytes()) {
			t.Fatal("invalid scope JSON")
		}
		if r.IfNotExists && (!strings.Contains(out.String(), "DROP TABLE IF EXISTS") || !strings.Contains(out.String(), "CREATE TABLE IF NOT EXISTS")) {
			t.Fatal(out.String())
		}
		if r.Charset == "utf-8-bom" && !bytes.HasPrefix(out.Bytes(), []byte{0xef, 0xbb, 0xbf}) {
			t.Fatal("missing BOM")
		}
	}
	var dump bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "generated"}}}, &dump); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), `INSERT INTO "main"."generated" ("id","value")`) {
		t.Fatal("generated column inserted", dump.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &exportCancelWriter{cancel: cancel}
	if err := e.Export(ctx, core.ExportRequest{Connection: c, Scope: "database"}, writer); !errors.Is(err, context.Canceled) {
		t.Fatal("midstream cancellation was lost", err)
	}
	if writer.bytes == 0 {
		t.Fatal("cancellation did not happen inside stream")
	}
	// A long recursive result must stop at the duration budget and never finish.
	var out bytes.Buffer
	err := e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "query", Format: "json", SQL: "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000000) SELECT x FROM n", TimeoutMS: 1}, &out)
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(out.String(), `"complete":true`) {
		t.Fatal("duration limit", err)
	}
	if err = e.Test(context.Background(), c); err != nil {
		t.Fatal("cancelled export leaked its connection", err)
	}
}

func TestSQLiteExportStatementLimit(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE statement_size(value TEXT)")
	execute(t, e, c, "INSERT INTO statement_size VALUES('"+strings.Repeat("x", 1200)+"')")
	var out bytes.Buffer
	err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "statement_size"}}, MaxStatementBytes: 1024}, &out)
	var p *core.Error
	if !errors.As(err, &p) || p.Code != "export_statement_limit" || strings.Contains(out.String(), "EXPORT COMPLETE") {
		t.Fatal("oversized single INSERT accepted", err)
	}
}

func TestSQLiteExportBatchBoundary(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE batch_size(value TEXT)")
	execute(t, e, c, "INSERT INTO batch_size VALUES('"+strings.Repeat("x", 400)+"'),('"+strings.Repeat("y", 400)+"'),('"+strings.Repeat("z", 400)+"')")
	var out bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "batch_size"}}, Extended: true, MaxStatementBytes: 1024}, &out); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "INSERT INTO") {
			count++
			if len(line)+1 > 1024 {
				t.Fatal("INSERT batch exceeded its bound")
			}
		}
	}
	if count != 2 {
		t.Fatal("batch boundary not exercised", count)
	}
}

func TestSQLiteExportIdempotentStructure(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := exportFixture(t, e, c)
	var out bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Data: exportBool(false), IfNotExists: true}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CREATE INDEX IF NOT EXISTS") {
		t.Fatalf("index not idempotent: %s", out.String())
	}
	target := core.Connection{Driver: "sqlite", Database: "exportidem"}
	for pass := 0; pass < 2; pass++ {
		for _, statement := range splitExportTestSQL(out.String()) {
			execute(t, e, target, statement)
		}
	}
}
