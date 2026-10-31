// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
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
	"github.com/MagicalWig34653/db-manager/internal/sqlsplit"
)

// Live DUMP-001 proof. The disposable fixture is created and destroyed by
// scripts/test-dump-objects.py (dbfix); without it the tests skip. The test builds
// a hostile catalog, dumps it, drops and recreates the database, restores the dump
// with the project splitter and compares the catalogs read with independent queries.
// DBM_DUMP_OBJECTS_DIR receives the dumps for the independent client restore.

type liveObjectSystem struct{ name, driver, key string }

var liveObjectSystems = []liveObjectSystem{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}}

func liveObjectEngine(t *testing.T, system liveObjectSystem) (*Engine, core.Connection) {
	t.Helper()
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
	return e, c
}

// liveStatements runs the statements on one pinned connection (session state matters).
func liveStatements(t *testing.T, e *Engine, c core.Connection, statements ...string) {
	t.Helper()
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
	for _, statement := range statements {
		if _, err = conn.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("%v\n%.300s", err, statement)
		}
	}
}

func liveRecreate(t *testing.T, e *Engine, c core.Connection, name string) core.Connection {
	t.Helper()
	admin := c
	admin.Database = "probe"
	q := quoteIdentifier(c.Driver, name)
	liveStatements(t, e, admin, "DROP DATABASE IF EXISTS "+q, "CREATE DATABASE "+q)
	c.Database = name
	return c
}

func liveMySQLFixture(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	big := "BEGIN SET @a = 0; " + strings.Repeat("SET @a = @a + 1; ", 120000) + "END"
	liveStatements(t, e, c,
		"CREATE TABLE t (id INT PRIMARY KEY, note VARCHAR(100)) ENGINE=InnoDB",
		"CREATE TABLE audit (id INT AUTO_INCREMENT PRIMARY KEY, msg TEXT) ENGINE=InnoDB",
		"INSERT INTO t VALUES (1,'a;b'),(2,'$$ ;; // -- /* x */ `q`')",
		"CREATE VIEW z_base AS SELECT id,note FROM t",
		"CREATE VIEW `a_top``;--` AS SELECT id,CONCAT(note,';') AS n FROM z_base",
		"CREATE VIEW `m_mid` AS SELECT id FROM `a_top``;--`",
		// body holds every delimiter candidate up to the 8th: the dump must escalate
		"CREATE FUNCTION `f``x;--`(a INT) RETURNS INT DETERMINISTIC COMMENT 'it''s; \"q\" -- c' BEGIN DECLARE s TEXT; SET s = ';; $$ // %% @@ !! ^^ ~~ delimiter'; /* ;; $$ */ -- ;; line\n RETURN a + CHAR_LENGTH(s); END",
		"CREATE FUNCTION simple_f() RETURNS INT DETERMINISTIC RETURN 7",
		"CREATE VIEW uses_f AS SELECT simple_f() AS v",
		"CREATE PROCEDURE p_cursor(IN x INT, OUT y INT) BEGIN DECLARE done INT DEFAULT 0; DECLARE cur CURSOR FOR SELECT id FROM t; DECLARE CONTINUE HANDLER FOR NOT FOUND SET done=1; OPEN cur; SET y = x; CLOSE cur; END",
		"CREATE PROCEDURE `big_p`() "+big,
		"CREATE TRIGGER `trg``;--` AFTER INSERT ON t FOR EACH ROW BEGIN INSERT INTO audit(msg) VALUES (CONCAT('ins;', NEW.note)); END",
		"CREATE TRIGGER trg_second AFTER INSERT ON t FOR EACH ROW FOLLOWS `trg``;--` INSERT INTO audit(msg) VALUES ('second')",
		"SET time_zone='+05:30'",
		"CREATE EVENT `ev``;--` ON SCHEDULE EVERY 1 YEAR STARTS '2099-01-01 00:00:00' DISABLE DO BEGIN INSERT INTO audit(msg) VALUES('ev;'); END",
		"CREATE EVENT ev_at ON SCHEDULE AT '2099-01-01 10:00:00' ON COMPLETION PRESERVE DISABLE DO SELECT 1",
		"SET time_zone='+00:00'",
		"SET SESSION sql_mode='ANSI_QUOTES,NO_BACKSLASH_ESCAPES'",
		`CREATE PROCEDURE ansi_p() SELECT "note" FROM t WHERE note <> 'a\b'`,
		"SET SESSION sql_mode=DEFAULT",
	)
}

func livePostgresFixture(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	big := "BEGIN\n" + strings.Repeat("  PERFORM 1;\n", 100000) + "END"
	liveStatements(t, e, c,
		"CREATE SCHEMA app",
		"CREATE TABLE public.t (id int PRIMARY KEY, note text)",
		"CREATE TABLE app.audit (msg text)",
		`INSERT INTO t VALUES (1,'a;b'),(2,E'$$ $f$ $function$ -- /* \\''')`,
		"CREATE VIEW public.z_base AS SELECT id,note FROM public.t",
		`CREATE FUNCTION public."f""x;--"(a int) RETURNS int LANGUAGE sql IMMUTABLE AS $q$ SELECT a + 1 /* $$ */ -- $function$ ; $q$`,
		// body contains the default delimiter: pg_get_functiondef must pick another
		`CREATE FUNCTION app.tg() RETURNS trigger LANGUAGE plpgsql SET search_path = pg_catalog, public AS $outer$ BEGIN INSERT INTO app.audit VALUES ('ins;' || NEW.note || ' $function$ '); RETURN NEW; END $outer$`,
		`CREATE TRIGGER "trg"";--" AFTER INSERT ON public.t FOR EACH ROW EXECUTE FUNCTION app.tg()`,
		`CREATE TRIGGER trg_disabled BEFORE UPDATE ON public.t FOR EACH ROW EXECUTE FUNCTION app.tg()`,
		`ALTER TABLE public.t DISABLE TRIGGER trg_disabled`,
		// view reads a function and a view; sorts before z_base by name
		`CREATE VIEW app.a_view WITH (security_barrier=true) AS SELECT id, public."f""x;--"(id) AS n FROM public.z_base`,
		"CREATE VIEW public.v_check WITH (check_option=local) AS SELECT * FROM public.t WHERE id > 0",
		// function depends on the row type of a view
		"CREATE FUNCTION app.rows_of() RETURNS SETOF app.a_view LANGUAGE sql AS 'SELECT * FROM app.a_view'",
		"CREATE FUNCTION app.rows_of(x int) RETURNS int LANGUAGE sql AS 'SELECT x'",
		"CREATE MATERIALIZED VIEW app.mv AS SELECT id FROM public.t",
		"CREATE INDEX mv_idx ON app.mv (id)",
		"CREATE PROCEDURE app.proc(x int) LANGUAGE plpgsql AS $$ BEGIN INSERT INTO app.audit VALUES ('p' || x); END $$",
		"CREATE FUNCTION app.big() RETURNS void LANGUAGE plpgsql AS $body$ "+big+" $body$",
	)
}

// liveCatalog reads views, routines, triggers and events with independent catalog
// queries (information_schema / pg_catalog), one canonical line per property.
func liveCatalog(t *testing.T, e *Engine, c core.Connection) []string {
	t.Helper()
	db, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var queries []string
	if c.Driver == "mysql" {
		queries = []string{
			"SELECT 'view',TABLE_NAME,VIEW_DEFINITION,CHECK_OPTION,IS_UPDATABLE,SECURITY_TYPE,CHARACTER_SET_CLIENT,COLLATION_CONNECTION FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME",
			"SELECT 'routine',ROUTINE_TYPE,ROUTINE_NAME,DTD_IDENTIFIER,ROUTINE_BODY,SHA2(ROUTINE_DEFINITION,256),CHAR_LENGTH(ROUTINE_DEFINITION),IS_DETERMINISTIC,SQL_DATA_ACCESS,SECURITY_TYPE,SQL_MODE,ROUTINE_COMMENT,CHARACTER_SET_CLIENT,COLLATION_CONNECTION FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() ORDER BY ROUTINE_TYPE,ROUTINE_NAME",
			"SELECT 'param',SPECIFIC_NAME,ORDINAL_POSITION,PARAMETER_MODE,PARAMETER_NAME,DTD_IDENTIFIER FROM information_schema.PARAMETERS WHERE SPECIFIC_SCHEMA=DATABASE() ORDER BY SPECIFIC_NAME,ORDINAL_POSITION",
			"SELECT 'trigger',TRIGGER_NAME,EVENT_MANIPULATION,EVENT_OBJECT_TABLE,ACTION_ORDER,ACTION_STATEMENT,ACTION_TIMING,SQL_MODE,CHARACTER_SET_CLIENT,COLLATION_CONNECTION FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() ORDER BY TRIGGER_NAME",
			"SELECT 'event',EVENT_NAME,EVENT_DEFINITION,EVENT_TYPE,EXECUTE_AT,INTERVAL_VALUE,INTERVAL_FIELD,STATUS,ON_COMPLETION,SQL_MODE,TIME_ZONE,STARTS,ENDS,CHARACTER_SET_CLIENT,COLLATION_CONNECTION FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE() ORDER BY EVENT_NAME",
		}
	} else {
		filter := "n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg\\_toast%'"
		queries = []string{
			"SELECT 'view',n.nspname,c.relname,c.relkind,pg_get_viewdef(c.oid,true),COALESCE(array_to_string(c.reloptions,','),'') FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('v','m') AND " + filter + " ORDER BY 2,3",
			"SELECT 'index',indexname,indexdef FROM pg_indexes WHERE schemaname='app' ORDER BY 2",
			"SELECT 'routine',n.nspname,p.proname,pg_get_function_identity_arguments(p.oid),p.prokind,pg_get_functiondef(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE " + filter + " ORDER BY 2,3,4",
			"SELECT 'trigger',t.tgname,c.relname,t.tgenabled,pg_get_triggerdef(t.oid,true) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND " + filter + " ORDER BY 2,3",
		}
	}
	var lines []string
	for _, query := range queries {
		rows, err := db.QueryContext(context.Background(), query)
		if err != nil {
			t.Fatal(err, query)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			values := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err = rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			parts := make([]string, len(cols))
			for i, v := range values {
				parts[i] = v.String
				if !v.Valid {
					parts[i] = "<NULL>"
				}
			}
			lines = append(lines, strings.Join(parts, "|"))
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return lines
}

func liveRestore(t *testing.T, e *Engine, c core.Connection, dump string) {
	t.Helper()
	statements, err := sqlsplit.SplitLimited(dump, ";", c.Driver, 64<<20, 100000)
	if err != nil {
		t.Fatal(err)
	}
	liveStatements(t, e, c, statements...)
}

func liveWriteDump(t *testing.T, name, dump string) {
	t.Helper()
	if dir := os.Getenv("DBM_DUMP_OBJECTS_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(dump), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLiveExportObjects(t *testing.T) {
	for _, system := range liveObjectSystems {
		t.Run(system.name, func(t *testing.T) {
			e, admin := liveObjectEngine(t, system)
			fixture := func(c core.Connection) {
				if c.Driver == "mysql" {
					liveMySQLFixture(t, e, c)
				} else {
					livePostgresFixture(t, e, c)
				}
			}
			objects := []string{"views", "routines", "triggers", "events"}
			if system.driver == "postgres" {
				objects = []string{"views", "routines", "triggers"}
			}
			// Source database for the independent client restore (kept intact).
			src := liveRecreate(t, e, admin, "objsrc")
			fixture(src)
			var dump bytes.Buffer
			request := core.ExportRequest{Connection: src, Scope: "database", Objects: objects, Comments: true, Extended: true}
			if err := e.Export(context.Background(), request, &dump); err != nil {
				t.Fatal(err)
			}
			liveWriteDump(t, system.name+"-objects.sql", dump.String())
			request.Drop = true
			var withDrop bytes.Buffer
			if err := e.Export(context.Background(), request, &withDrop); err != nil {
				t.Fatal(err)
			}
			liveWriteDump(t, system.name+"-objects-drop.sql", withDrop.String())
			request.Drop = false
			if system.driver == "mysql" {
				request.IfNotExists = true
				request.Data = exportBool(false) // data inserts are never idempotent
				request.Objects = []string{"routines", "triggers", "events"}
				var idem bytes.Buffer
				if err := e.Export(context.Background(), request, &idem); err != nil {
					t.Fatal(err)
				}
				liveWriteDump(t, system.name+"-objects-ine.sql", idem.String())
				if !strings.Contains(idem.String(), "TRIGGER IF NOT EXISTS") || !strings.Contains(idem.String(), "EVENT IF NOT EXISTS") {
					t.Fatal("IF NOT EXISTS missing")
				}
				request.IfNotExists = false
				request.Data = nil
			}
			text := dump.String()
			if !strings.HasSuffix(text, "-- DBMANAGER EXPORT COMPLETE\n") {
				t.Fatal("no marker")
			}
			if strings.Contains(text, "DEFINER=") && system.driver == "mysql" {
				// SQL SECURITY DEFINER is a view attribute; the account clause must be gone.
				t.Fatal("DEFINER leaked")
			}
			// Order: tables, data, views, routines, triggers, events.
			last := -1
			markers := []string{"CREATE TABLE", "INSERT INTO", "VIEW", "FUNCTION", "TRIGGER", "EVENT"}
			if system.driver == "mysql" {
				markers = []string{"CREATE TABLE", "INSERT INTO", "FUNCTION", "VIEW", "TRIGGER", "EVENT"} // views may call functions
			}
			for _, marker := range markers {
				if system.driver == "postgres" && marker == "EVENT" {
					continue
				}
				idx := strings.Index(text, marker)
				if marker == "VIEW" {
					idx = strings.Index(text, "CREATE VIEW")
					if system.driver == "mysql" {
						idx = strings.Index(text, "VIEW `z_base`")
					}
				}
				if marker == "TRIGGER" && system.driver == "postgres" {
					idx = strings.Index(text, "CREATE TRIGGER")
				}
				if idx < 0 || idx < last {
					t.Fatalf("order of %s: %d after %d", marker, idx, last)
				}
				last = idx
			}
			// In-process proof on its own database: dump, drop, recreate, restore, compare.
			goDB := liveRecreate(t, e, admin, "objgo")
			fixture(goDB)
			before := liveCatalog(t, e, goDB)
			if len(before) < 10 {
				t.Fatal("fixture catalog too small", len(before))
			}
			var goDump bytes.Buffer
			goRequest := core.ExportRequest{Connection: goDB, Scope: "database", Objects: objects, Comments: true}
			if err := e.Export(context.Background(), goRequest, &goDump); err != nil {
				t.Fatal(err)
			}
			goDB = liveRecreate(t, e, admin, "objgo")
			liveRestore(t, e, goDB, goDump.String())
			after := liveCatalog(t, e, goDB)
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Fatalf("catalog mismatch\nBEFORE\n%s\nAFTER\n%s", firstDiff(before, after), "")
			}
			// Restored objects work, and the restore did not fire triggers.
			q := func(query string) string {
				r := execute(t, e, goDB, query)
				return r.Results[0].Rows[0][0].Value
			}
			audit := "audit"
			if system.driver == "postgres" {
				audit = "app.audit"
			}
			if n := q("SELECT count(*) FROM " + audit); n != "0" {
				t.Fatal("trigger fired during restore", n)
			}
			execute(t, e, goDB, "INSERT INTO t VALUES (3,'new')")
			want := "2"
			if system.driver == "postgres" {
				want = "1"
			}
			if n := q("SELECT count(*) FROM " + audit); n != want {
				t.Fatal("restored trigger did not fire", n, want)
			}
			// Re-running the DROP variant on the restored database is idempotent and complete.
			liveRestore(t, e, goDB, withDrop.String())
			if system.driver == "mysql" {
				// IF NOT EXISTS variant: objects exist already, nothing fails.
				idem, _ := os.ReadFile(filepath.Join(os.Getenv("DBM_DUMP_OBJECTS_DIR"), system.name+"-objects-ine.sql"))
				if len(idem) > 0 {
					liveRestore(t, e, goDB, string(idem))
				}
			}
		})
	}
}

func firstDiff(a, b []string) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return "line " + strconv.Itoa(i) + "\n  before: " + trim(x) + "\n  after:  " + trim(y)
		}
	}
	return "equal"
}

func trim(s string) string {
	if len(s) > 600 {
		return s[:600] + "..."
	}
	return s
}

// TestLiveExportObjectsFailures proves the honest failure behavior: no marker.
func TestLiveExportObjectsFailures(t *testing.T) {
	for _, system := range liveObjectSystems {
		t.Run(system.name, func(t *testing.T) {
			e, admin := liveObjectEngine(t, system)
			c := liveRecreate(t, e, admin, "objfail")
			var stmts []string
			if system.driver == "mysql" {
				stmts = []string{"CREATE TABLE t (id INT PRIMARY KEY) ENGINE=InnoDB", "CREATE VIEW v AS SELECT id FROM t",
					"CREATE PROCEDURE huge() BEGIN SET @a=0; " + strings.Repeat("SET @a = @a + 1; ", 260000) + "END"}
			} else {
				stmts = []string{"CREATE TABLE t (id int PRIMARY KEY)", "CREATE VIEW v AS SELECT id FROM t",
					"CREATE FUNCTION huge() RETURNS void LANGUAGE plpgsql AS $b$ BEGIN\n" + strings.Repeat("  PERFORM 1;\n", 500000) + "END $b$"}
			}
			liveStatements(t, e, c, stmts...)
			// An object over the definition limit aborts the dump without the marker.
			var out bytes.Buffer
			err := e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "database", Objects: []string{"routines"}}, &out)
			var p *core.Error
			if !errors.As(err, &p) || p.Code != "export_object_too_large" || strings.Contains(out.String(), "DBMANAGER EXPORT COMPLETE") {
				t.Fatal(err, len(out.String()))
			}
			if !strings.Contains(out.String(), "CREATE TABLE") {
				t.Fatal("tables were not written before the failure")
			}
			// A byte limit inside the object section also ends without the marker.
			out.Reset()
			err = e.Export(context.Background(), core.ExportRequest{Connection: c, Scope: "database", Objects: []string{"views"}, MaxBytes: 330}, &out)
			if !errors.As(err, &p) || p.Code != "export_byte_limit" || strings.Contains(out.String(), "DBMANAGER EXPORT COMPLETE") {
				t.Fatal(err, out.String())
			}
			// Cancellation in the middle likewise.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			out.Reset()
			if err = e.Export(ctx, core.ExportRequest{Connection: c, Scope: "database", Objects: []string{"views"}}, &out); err == nil || strings.Contains(out.String(), "DBMANAGER EXPORT COMPLETE") {
				t.Fatal(err)
			}
			if system.driver == "mysql" {
				// A restricted account cannot prove read access to routines/triggers/events: no output at all.
				liveStatements(t, e, admin, "DROP USER IF EXISTS 'objlimited'@'%'", "CREATE USER 'objlimited'@'%' IDENTIFIED BY 'limited-pass-1'", "GRANT SELECT ON objfail.* TO 'objlimited'@'%'")
				limited := c
				limited.Username, limited.Password = "objlimited", "limited-pass-1"
				out.Reset()
				err = e.Export(context.Background(), core.ExportRequest{Connection: limited, Scope: "database", Objects: []string{"views"}}, &out)
				if !errors.As(err, &p) || p.Code != "export_object_privilege" || out.Len() != 0 {
					t.Fatal(err, out.String())
				}
				// Tables alone remain exportable for that account.
				out.Reset()
				if err = e.Export(context.Background(), core.ExportRequest{Connection: limited, Scope: "database"}, &out); err != nil {
					t.Fatal(err)
				}
				liveStatements(t, e, admin, "DROP USER 'objlimited'@'%'")
			}
		})
	}
}

// TestLiveExportObjectsCatalogDump writes the catalog before the client restore.
func TestLiveExportObjectsCatalogDump(t *testing.T) {
	dir := os.Getenv("DBM_DUMP_OBJECTS_DIR")
	for _, system := range liveObjectSystems {
		t.Run(system.name, func(t *testing.T) {
			e, admin := liveObjectEngine(t, system)
			if dir == "" {
				t.Skip("DBM_DUMP_OBJECTS_DIR required")
			}
			c := admin
			c.Database = "objsrc"
			data, err := json.Marshal(liveCatalog(t, e, c))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, system.name+"-catalog.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
