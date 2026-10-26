// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

var extendedFormats = []string{"xml", "latex", "latex-longtable", "mediawiki", "yaml", "php-array", "texy", "codegen"}
var extendedMarkers = map[string]string{"xml": "<complete>true</complete></export>\n", "latex": "% DBMANAGER EXPORT COMPLETE\n", "latex-longtable": "% DBMANAGER EXPORT COMPLETE\n", "mediawiki": "<!-- DBMANAGER EXPORT COMPLETE -->\n", "yaml": "],\"complete\":true}\n", "php-array": "),'complete'=>true);\n", "texy": "<!-- DBMANAGER EXPORT COMPLETE -->\n", "codegen": "{\"complete\":true}\n"}

type extendedXMLField struct {
	Encoding string `xml:"encoding,attr"`
	Text     string `xml:",chardata"`
}

func (v extendedXMLField) value(t *testing.T) string {
	t.Helper()
	if v.Encoding == "base64" {
		b, err := base64.StdEncoding.DecodeString(v.Text)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if v.Encoding != "" {
		t.Fatal(v.Encoding)
	}
	return v.Text
}

type extendedXMLDocument struct {
	XMLName xml.Name `xml:"export"`
	Tables  []struct {
		Database extendedXMLField `xml:"database"`
		Name     extendedXMLField `xml:"name"`
		Columns  []struct {
			Name extendedXMLField `xml:"name"`
		} `xml:"columns>column"`
		Rows []struct {
			Cells []struct {
				Kind  extendedXMLField `xml:"kind"`
				Value extendedXMLField `xml:"value"`
			} `xml:"cell"`
		} `xml:"rows>row"`
	} `xml:"table"`
	Complete string `xml:"complete"`
}

func checkExtendedExport(t *testing.T, e *Engine, c core.Connection, ref core.TableRef) {
	t.Helper()
	var typed bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "json"}, &typed); err != nil {
		t.Fatal(err)
	}
	var want struct {
		Tables []struct {
			Rows [][]core.Cell `json:"rows"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(typed.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	for _, format := range extendedFormats {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			request := core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}
			if err := e.Export(context.Background(), request, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(out.String(), extendedMarkers[format]) {
				t.Fatalf("missing completion: %s", out.String())
			}
			if dir := os.Getenv("DBM_EXTENDED_EXPORT_DUMP"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, "placeholder-"+c.Driver+"."+format), out.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch format {
			case "xml":
				var doc extendedXMLDocument
				if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
					t.Fatal(err)
				}
				if doc.Complete != "true" || len(doc.Tables) != 1 || doc.Tables[0].Name.value(t) != ref.Name {
					t.Fatal("XML metadata")
				}
				got := [][]core.Cell{}
				for _, r := range doc.Tables[0].Rows {
					cells := []core.Cell{}
					for _, v := range r.Cells {
						cells = append(cells, core.Cell{Kind: v.Kind.value(t), Value: v.Value.value(t)})
					}
					got = append(got, cells)
				}
				if !reflect.DeepEqual(got, want.Tables[0].Rows) {
					t.Fatalf("XML roundtrip: %#v %#v", got, want.Tables[0].Rows)
				}
			case "yaml":
				// Strict parser for our YAML 1.2 JSON subset. No tags, anchors, aliases,
				// implicit scalar typing or duplicate-name maps are generated.
				var doc struct {
					Format  string `json:"format"`
					Version int    `json:"version"`
					Tables  []struct {
						Database string        `json:"database"`
						Table    core.TableRef `json:"table"`
						Columns  []core.Column `json:"columns"`
						Rows     [][]core.Cell `json:"rows"`
					} `json:"tables"`
					Complete bool `json:"complete"`
				}
				dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&doc); err != nil {
					t.Fatal(err)
				}
				if dec.Decode(new(any)) != io.EOF {
					t.Fatal("extra document")
				}
				if !doc.Complete || !reflect.DeepEqual(doc.Tables[0].Rows, want.Tables[0].Rows) {
					t.Fatal("YAML subset roundtrip")
				}
			case "codegen":
				lines := strings.Split(strings.TrimSpace(out.String()), "\n")
				if len(lines) != len(want.Tables[0].Rows)+3 {
					t.Fatal("record count")
				}
				for _, line := range lines {
					if !json.Valid([]byte(line)) {
						t.Fatal("invalid text record")
					}
				}
			case "php-array":
				if strings.Contains(out.String(), "?>") || strings.Contains(out.String(), "${") {
					t.Fatal("PHP injection")
				}
			case "mediawiki", "texy":
				if strings.Contains(out.String(), "{{") || strings.Contains(out.String(), "<script") {
					t.Fatal("markup injection")
				}
			}
			request.MaxRows = 1
			out.Reset()
			err := e.Export(context.Background(), request, &out)
			if err == nil || strings.HasSuffix(out.String(), extendedMarkers[format]) {
				t.Fatal("row limit completed", err)
			}
			request.MaxRows = 0
			request.MaxBytes = 16
			out.Reset()
			if e.Export(context.Background(), request, &out) == nil || strings.HasSuffix(out.String(), extendedMarkers[format]) {
				t.Fatal("byte limit completed")
			}
			request.MaxBytes = 0
			if e.Export(context.Background(), request, &exportFailWriter{}) == nil {
				t.Fatal("disconnect completed")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if e.Export(ctx, request, io.Discard) == nil {
				t.Fatal("cancellation completed")
			}
		})
	}
	if err := e.Test(context.Background(), c); err != nil {
		t.Fatal("leaked export slot or connection", err)
	}
}
func TestSQLiteExtendedExportFormats(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ref := exportFixture(t, e, c)
	execute(t, e, c, "INSERT INTO "+quoteIdentifier(c.Driver, ref.Name)+" VALUES (2,CAST(X'0001090a0d1b' AS TEXT),'',NULL,X'', '0','2026')")
	checkExtendedExport(t, e, c, ref)
}
func TestExtendedExportReferenceEscapes(t *testing.T) {
	if got := phpExportString("\x00'\"$\\\n雪"); got != `"\x00\x27\x22\x24\x5C\x0A\xE9\x9B\xAA"` {
		t.Fatal(got)
	}
	if got := latexExportString("\\{}$&#%_^~\n"); got != `\textbackslash{}\{\}\$\&\#\%\_\textasciicircum{}\textasciitilde{}\textbackslash{}u000A` {
		t.Fatal(got)
	}
	if got := wikiExportString("|{}<'&\n"); got != "&#124;&#123;&#125;&#60;&#39;&#38;&#92;&#117;&#48;&#48;&#48;&#65;" {
		t.Fatal(got)
	}
	if got := xmlExportField("value", "<>&\"'\r"); got != "<value>&lt;&gt;&amp;&#34;&#39;&#xD;</value>" {
		t.Fatal(got)
	}
	for _, format := range extendedFormats {
		r, _ := core.NormalizeExtendedExport(core.ExportRequest{Connection: sqliteConnection(), Format: format, Tables: []core.TableRef{{Name: "placeholder"}}})
		var out bytes.Buffer
		f := newExportFormat(&out, r)
		if err := f.textRow([]core.Cell{{Kind: "text", Value: string([]byte{255})}}); err == nil {
			t.Fatal("invalid UTF-8 accepted", format)
		}
		if err := f.textTable(r.Connection, core.TableDescription{Table: core.TableRef{Name: string([]byte{255})}}); err == nil {
			t.Fatal("invalid name accepted", format)
		}
	}
}
func TestSQLiteExtendedDumpDetails(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, `CREATE TABLE source (id INTEGER, value TEXT)`)
	execute(t, e, c, `INSERT INTO source VALUES (9007199254740993,CAST(X'00275c' AS TEXT)),(2,'{{x}}|$\\input{bad}')`)
	viewName := "view \";--"
	execute(t, e, c, "CREATE VIEW "+quoteIdentifier(c.Driver, viewName)+" AS SELECT id,value FROM source")
	r := core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: viewName}}, ViewsAsTables: true, Definer: "omit", Comments: true}
	var out bytes.Buffer
	if err := e.Export(context.Background(), r, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "CREATE VIEW") || strings.Contains(out.String(), "DEFINER") || !strings.Contains(out.String(), "-- Catalog table:") {
		t.Fatal(out.String())
	}
	target := c
	target.Database = "materialized"
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
	query := `SELECT id,value,typeof(id),typeof(value) FROM ` + quoteIdentifier(c.Driver, r.Tables[0].Name) + ` ORDER BY id`
	if !reflect.DeepEqual(execute(t, e, c, query).Results[0].Rows, execute(t, e, target, query).Results[0].Rows) {
		t.Fatal("materialized view storage classes changed")
	}
	execute(t, e, c, `CREATE TABLE "target;--" (id INTEGER,value TEXT)`)
	r = core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "source"}}, Compatibility: "data-only", Aliases: []core.ExportAlias{{Source: core.TableRef{Name: "source"}, Target: core.TableRef{Name: "target;--"}}}}
	out.Reset()
	if err := e.Export(context.Background(), r, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "CREATE TABLE") || !strings.Contains(out.String(), `INSERT INTO "main"."target;--"`) {
		t.Fatal(out.String())
	}
	for _, stmt := range splitExportTestSQL(out.String()) {
		execute(t, e, c, stmt)
	}
	if !reflect.DeepEqual(execute(t, e, c, `SELECT * FROM source ORDER BY id`).Results, execute(t, e, c, `SELECT * FROM "target;--" ORDER BY id`).Results) {
		t.Fatal("alias changed values")
	}
	r.Aliases[0].Target.Name = "missing"
	out.Reset()
	if e.Export(context.Background(), r, &out) == nil || out.Len() != 0 {
		t.Fatal("uncatalogued alias accepted")
	}
	r = core.ExportRequest{Connection: c, Scope: "database", ViewsAsTables: true, Format: "xml"}
	out.Reset()
	if err := e.Export(context.Background(), r, &out); err != nil {
		t.Fatal(err)
	}
	var doc extendedXMLDocument
	if err := xml.Unmarshal(out.Bytes(), &doc); err != nil || len(doc.Tables) != 3 {
		t.Fatal("view discovery", err, len(doc.Tables))
	}
}
func TestSQLiteExtendedExportInvalidUTF8AndTimeout(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for _, format := range extendedFormats {
		var out bytes.Buffer
		err := e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "query", Format: format, SQL: "SELECT CAST(X'FF' AS TEXT)"}, &out)
		var p *core.Error
		if !errors.As(err, &p) || p.Code != "unsupported_export" || strings.HasSuffix(out.String(), extendedMarkers[format]) {
			t.Fatal("invalid UTF-8", format, err)
		}
		out.Reset()
		err = e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "query", Format: format, SQL: "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000000) SELECT x FROM n", TimeoutMS: 1}, &out)
		if !errors.Is(err, context.DeadlineExceeded) || strings.HasSuffix(out.String(), extendedMarkers[format]) {
			t.Fatal("timeout", format, err)
		}
	}
}
func TestLiveExtendedExport(t *testing.T) {
	for _, system := range []struct{ name, driver, key string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.key)
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
			c := core.Connection{Driver: system.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			ref := exportFixture(t, e, c)
			execute(t, e, c, "INSERT INTO "+quoteIdentifier(c.Driver, ref.Name)+" SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name))
			checkExtendedExport(t, e, c, ref)
			view := "placeholder_view"
			execute(t, e, c, "CREATE VIEW "+quoteIdentifier(c.Driver, view)+" AS SELECT id,text_value FROM "+quoteIdentifier(c.Driver, ref.Name))
			var out bytes.Buffer
			if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: view}}, ViewsAsTables: true, Definer: "omit"}, &out); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "DEFINER") || strings.Contains(out.String(), "CREATE VIEW") {
				t.Fatal("view was not materialized")
			}
			// Restore into an independent disposable dbfix database, then read back.
			target := c
			target.Database = "extended_restored"
			execute(t, e, c, "CREATE DATABASE "+quoteIdentifier(c.Driver, target.Database))
			db, cleanup, openErr := e.open(context.Background(), target)
			if openErr != nil {
				t.Fatal(openErr)
			}
			defer cleanup()
			conn, connErr := db.Conn(context.Background())
			if connErr != nil {
				t.Fatal(connErr)
			}
			for _, stmt := range splitExportTestSQL(out.String()) {
				if _, execErr := conn.ExecContext(context.Background(), stmt); execErr != nil {
					t.Fatal(execErr)
				}
			}
			conn.Close()
			query := "SELECT id,text_value FROM " + quoteIdentifier(c.Driver, view) + " ORDER BY id"
			if !reflect.DeepEqual(execute(t, e, c, query).Results[0].Rows, execute(t, e, target, query).Results[0].Rows) {
				t.Fatal("live view restore mismatch")
			}
		})
	}
}

func TestExtendedExportNamesAndDataStatements(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	name := "header\n\t\x01|{}\\input{bad}<!--"
	for _, format := range extendedFormats {
		var out bytes.Buffer
		r := core.ExportRequest{Connection: c, Scope: "query", Format: format, SQL: "SELECT 1 AS " + quoteIdentifier(c.Driver, name) + ",NULL AS " + quoteIdentifier(c.Driver, name)}
		if err := e.Export(context.Background(), r, &out); err != nil {
			t.Fatal(err)
		}
		if format == "xml" {
			var doc extendedXMLDocument
			if err := xml.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Tables[0].Columns) != 2 || doc.Tables[0].Columns[0].Name.value(t) != name || doc.Tables[0].Columns[1].Name.value(t) != name {
				t.Fatal("column names changed")
			}
		}
	}
	execute(t, e, c, `CREATE TABLE conflict(id INTEGER PRIMARY KEY, value TEXT)`)
	execute(t, e, c, `INSERT INTO conflict VALUES(1,'exported')`)
	for _, statement := range []string{"insert-ignore", "replace"} {
		var out bytes.Buffer
		r := core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "conflict"}}, Structure: exportBool(false), DataStatement: statement}
		if err := e.Export(context.Background(), r, &out); err != nil {
			t.Fatal(err)
		}
		execute(t, e, c, `UPDATE conflict SET value='existing'`)
		for _, stmt := range splitExportTestSQL(out.String()) {
			execute(t, e, c, stmt)
		}
		want := "existing"
		if statement == "replace" {
			want = "exported"
		}
		if got := queryText(t, e, c, `SELECT value FROM conflict`); got != want {
			t.Fatal(statement, got)
		}
		execute(t, e, c, `UPDATE conflict SET value='exported'`)
	}
	for _, tt := range []struct{ driver, statement, prefix string }{{"mysql", "insert-ignore", "INSERT IGNORE INTO "}, {"mysql", "replace", "REPLACE INTO "}, {"sqlite", "insert-ignore", "INSERT OR IGNORE INTO "}} {
		f := newExportFormat(io.Discard, core.ExportRequest{DataStatement: tt.statement})
		if !strings.HasPrefix(f.insertPrefix(tt.driver), tt.prefix) {
			t.Fatal(tt)
		}
	}
}

func TestExtendedExportMaterializedTypeAllowlist(t *testing.T) {
	for _, typ := range []string{"numeric(40,20)", "timestamp with time zone", "varchar(100)", "bigint unsigned", "text"} {
		if !exportViewType.MatchString(typ) {
			t.Fatal("catalog type rejected", typ)
		}
	}
	for _, typ := range []string{"text);DROP TABLE victim;--", "enum('x')", "public.domain", "text[]", "text/*x*/"} {
		if exportViewType.MatchString(typ) {
			t.Fatal("unsafe or unsupported type accepted", typ)
		}
	}
}
