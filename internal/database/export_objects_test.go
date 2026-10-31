// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func objectsRequest(c core.Connection, objects ...string) core.ExportRequest {
	return core.ExportRequest{Connection: c, Scope: "database", Objects: objects, Comments: true}
}

func sqliteCatalog(t *testing.T, e *Engine, c core.Connection) [][]string {
	t.Helper()
	var out [][]string
	for _, row := range execute(t, e, c, "SELECT type,name,tbl_name,sql FROM sqlite_schema WHERE type IN ('view','trigger') ORDER BY type,name").Results[0].Rows {
		var line []string
		for _, cell := range row {
			line = append(line, cell.Value)
		}
		out = append(out, line)
	}
	return out
}

// TestSQLiteExportObjectsRoundTrip dumps views and triggers with hostile names and
// bodies, restores into a fresh database and compares the catalog and the behavior.
func TestSQLiteExportObjectsRoundTrip(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for i, statement := range []string{
		"CREATE TABLE base (id INTEGER PRIMARY KEY, note TEXT)",
		"CREATE TABLE audit (n TEXT)",
		"INSERT INTO base VALUES (1,'a;b'),(2,'$$ -- /* \\')",
		// view reading a view that sorts later by name: creation order must not matter
		"CREATE VIEW \"a_view`;--\" AS SELECT id,note || ';' AS note FROM \"z_view\"",
		"CREATE VIEW z_view AS SELECT id,note FROM base -- trailing comment",
		"CREATE TRIGGER \"trg\"\";DROP TABLE base;--\" AFTER INSERT ON base BEGIN INSERT INTO audit VALUES ('x;' || new.note || '$$ /* \"'); END",
	} {
		if _, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: statement}); err != nil {
			t.Fatal(i, statement, err)
		}
	}
	before := sqliteCatalog(t, e, c)
	var dump bytes.Buffer
	if err := e.Export(context.Background(), objectsRequest(c, "views", "triggers"), &dump); err != nil {
		t.Fatal(err)
	}
	text := dump.String()
	if !strings.HasSuffix(text, "-- DBMANAGER EXPORT COMPLETE\n") || !strings.Contains(text, "-- Catalog view ") || strings.Index(text, "CREATE TABLE") > strings.Index(text, "CREATE VIEW") || strings.Index(text, "INSERT INTO") > strings.Index(text, "CREATE TRIGGER") {
		t.Fatal(text)
	}
	restore := func(script string) core.Connection {
		target := c
		target.Database = "restored" + string(rune('a'+len(script)%20))
		db, cleanup, err := e.open(context.Background(), target)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		err = conn.Raw(func(raw any) error { return raw.(sqliteDriver.Conn).Raw().Exec(script) })
		conn.Close()
		cleanup()
		if err != nil {
			t.Fatal(err)
		}
		return target
	}
	target := restore(text)
	if after := sqliteCatalog(t, e, target); !reflect.DeepEqual(before, after) {
		t.Fatalf("catalog mismatch\n%v\n%v", before, after)
	}
	// Behavior: triggers were created after the rows, so the restore did not fire them.
	if n := execute(t, e, target, "SELECT count(*) FROM audit").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("trigger fired during restore", n)
	}
	execute(t, e, target, "INSERT INTO base VALUES (3,'new')")
	if n := execute(t, e, target, "SELECT count(*) FROM audit").Results[0].Rows[0][0].Value; n != "1" {
		t.Fatal("restored trigger did not fire", n)
	}
	// DROP and IF NOT EXISTS variants restore twice into the same database.
	var again bytes.Buffer
	r := objectsRequest(c, "views", "triggers")
	r.Drop = true
	if err := e.Export(context.Background(), r, &again); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(again.String(), "DROP VIEW IF EXISTS") || strings.Index(again.String(), "DROP TRIGGER") > strings.Index(again.String(), "DROP TABLE") {
		t.Fatal(again.String())
	}
	var idempotent bytes.Buffer
	r = objectsRequest(c, "views", "triggers")
	r.IfNotExists = true
	if err := e.Export(context.Background(), r, &idempotent); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(idempotent.String(), "CREATE VIEW IF NOT EXISTS") || !strings.Contains(idempotent.String(), "CREATE TRIGGER IF NOT EXISTS") {
		t.Fatal(idempotent.String())
	}
	restore(idempotent.String() + "  ")
}

func TestExportObjectsOptionValidation(t *testing.T) {
	c := sqliteConnection()
	bad := []core.ExportRequest{
		{Connection: c, Scope: "database", Objects: []string{"routines"}},
		{Connection: c, Scope: "database", Objects: []string{"events"}},
		{Connection: c, Scope: "database", Objects: []string{"views", "views"}},
		{Connection: c, Scope: "database", Objects: []string{"tables"}},
		{Connection: c, Scope: "tables", Tables: []core.TableRef{{Name: "t"}}, Objects: []string{"views"}},
		{Connection: c, Scope: "database", Objects: []string{"views"}, Structure: exportBool(false)},
		{Connection: c, Scope: "database", Objects: []string{"views"}, Compatibility: "data-only"},
		{Connection: c, Scope: "database", Objects: []string{"views"}, ViewsAsTables: true},
		{Connection: c, Scope: "database", Objects: []string{"views"}, Format: "json"},
		{Connection: c, Scope: "database", Objects: []string{"views"}, Definer: "preserve"},
		{Connection: c, Scope: "database", Definer: "preserve"},
		{Connection: core.Connection{Driver: "postgres"}, Scope: "database", Objects: []string{"events"}},
		{Connection: core.Connection{Driver: "postgres"}, Scope: "database", Objects: []string{"views"}, IfNotExists: true},
		{Connection: core.Connection{Driver: "mysql"}, Scope: "database", Objects: []string{"views"}, IfNotExists: true},
		{Connection: core.Connection{Driver: "postgres"}, Scope: "database", Objects: []string{"views"}, Definer: "preserve"},
	}
	for i, r := range bad {
		if _, err := core.NormalizeExtendedExport(r); err == nil {
			t.Fatal("accepted", i)
		}
	}
	r, err := core.NormalizeExtendedExport(core.ExportRequest{Connection: core.Connection{Driver: "mysql"}, Scope: "database", Objects: []string{"events", "views", "routines"}, Definer: "preserve", IfNotExists: false})
	if err != nil || !reflect.DeepEqual(r.Objects, []string{"views", "routines", "events"}) {
		t.Fatal(r.Objects, err)
	}
	if _, err = core.NormalizeExport(core.ExportRequest{Connection: c, Scope: "database", Objects: []string{"views"}}); err == nil {
		t.Fatal("v11 accepted objects")
	}
}

func TestMySQLRewriteCreate(t *testing.T) {
	cases := []struct {
		in, kind, want string
		preserve, ine  bool
	}{
		{"CREATE ALGORITHM=UNDEFINED DEFINER=`ro``ot`@`%` SQL SECURITY DEFINER VIEW `v` AS select 1", "view", "CREATE ALGORITHM=UNDEFINED SQL SECURITY DEFINER VIEW `v` AS select 1", false, false},
		{"CREATE DEFINER=`a b`@`h@x` PROCEDURE `p`() SELECT 'DEFINER=`x`@`y` '", "procedure", "CREATE PROCEDURE `p`() SELECT 'DEFINER=`x`@`y` '", false, false},
		{"CREATE DEFINER=`role` EVENT `e` ON SCHEDULE AT now() DO SELECT 1", "event", "CREATE EVENT IF NOT EXISTS `e` ON SCHEDULE AT now() DO SELECT 1", false, true},
		{"CREATE DEFINER=`r`@`%` TRIGGER `t` BEFORE INSERT ON `x` FOR EACH ROW SET @a=1", "trigger", "CREATE DEFINER=`r`@`%` TRIGGER `t` BEFORE INSERT ON `x` FOR EACH ROW SET @a=1", true, false},
	}
	for _, c := range cases {
		got, err := mysqlRewriteCreate(c.in, c.kind, c.preserve, c.ine)
		if err != nil || got != c.want {
			t.Fatalf("%q -> %q (%v)", c.in, got, err)
		}
	}
	for _, in := range []string{"ALTER VIEW x", "CREATE DEFINER=root@localhost PROCEDURE p() SELECT 1", "CREATE DEFINER=`a`@`b`PROCEDURE `p`() SELECT 1", "CREATE UNKNOWN=1 PROCEDURE `p`() SELECT 1", "CREATE DEFINER=`a"} {
		if _, err := mysqlRewriteCreate(in, "procedure", false, false); err == nil {
			t.Fatal("accepted", in)
		}
	}
}

func TestMySQLDelimiterAndViewReferences(t *testing.T) {
	d, err := mysqlDelimiter("BEGIN SELECT ';;', '$$'; END")
	if err != nil || d != "//" {
		t.Fatal(d, err)
	}
	if _, err = mysqlDelimiter("BEGIN\n  Delimiter //\nEND"); err == nil {
		t.Fatal("delimiter line accepted")
	}
	all := strings.Join(delimiterCandidates, " ")
	if _, err = mysqlDelimiter(all); err == nil {
		t.Fatal("no delimiter left")
	}
	refs := mysqlViewReferences("select `d`.`a``b`.`x`,'`d`.`s`' from `d`.`a``b` join `d`.`c` /* `d`.`n` */ join `other`.`o` -- `d`.`m`\n", "d")
	if !reflect.DeepEqual(refs, []string{"a`b", "a`b", "c"}) {
		t.Fatal(refs)
	}
	order, err := dependencyOrder(3, [][]int{{1}, {2}, nil})
	if err != nil || !reflect.DeepEqual(order, []int{2, 1, 0}) {
		t.Fatal(order, err)
	}
	if _, err = dependencyOrder(2, [][]int{{1}, {0}}); err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestSQLiteExportObjectsFailureHasNoMarker(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE base (id INTEGER)")
	execute(t, e, c, "CREATE VIEW v AS SELECT id FROM base")
	var out bytes.Buffer
	// A byte limit smaller than the view section ends the dump without the marker.
	r := objectsRequest(c, "views")
	var full bytes.Buffer
	if err := e.Export(context.Background(), r, &full); err != nil {
		t.Fatal(err)
	}
	r.MaxBytes = int64(full.Len() - 30)
	err := e.Export(context.Background(), r, &out)
	var p *core.Error
	if !errors.As(err, &p) || p.Code != "export_byte_limit" || strings.Contains(out.String(), "DBMANAGER EXPORT COMPLETE") {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "CREATE TABLE") {
		t.Fatal("limit hit too early", out.String())
	}
}
