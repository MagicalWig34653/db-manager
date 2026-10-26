// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func TestLiveImport(t *testing.T) {
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
			checkImportRoundTrip(t, e, c)
			ctx := context.Background()
			r := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
			// Both a foreign synthetic dump and the real W-P11 same-dialect output.
			got, err := e.ExecuteImport(ctx, r, []byte("CREATE TABLE p12_sql(id INTEGER PRIMARY KEY,value VARCHAR(100));INSERT INTO p12_sql VALUES(1,'foreign');INSERT INTO missing_secret VALUES(1);INSERT INTO p12_sql VALUES(2,'not-run')"), nil)
			if err != nil || got.Statements[1].Outcome != "completed" || got.Statements[3].Outcome != "not_executed" {
				t.Fatalf("SQL %#v %v", got, err)
			}
			dumpImport(t, "ImportResult.real-"+system.name+"-sql", got)
			if n := execute(t, e, c, "SELECT count(*) FROM p12_sql").Results[0].Rows[0][0].Value; n != "1" {
				t.Fatal("SQL count", n)
			}
			var dump bytes.Buffer
			if err = e.Export(ctx, core.ExportRequest{Connection: c, Format: "sql", Tables: []core.TableRef{{Name: "p12_sql"}}, Transaction: true}, &dump); err != nil {
				t.Fatal(err)
			}
			execute(t, e, c, "DROP TABLE p12_sql")
			got, err = e.ExecuteImport(ctx, r, dump.Bytes(), nil)
			if err != nil || got.Outcome != "completed" {
				t.Fatalf("export restore %#v %v", got, err)
			}
			if n := execute(t, e, c, "SELECT count(*) FROM p12_sql WHERE value='foreign'").Results[0].Rows[0][0].Value; n != "1" {
				t.Fatal("restore readback", n)
			}
			routine := "DELIMITER //\nCREATE PROCEDURE p12_proc() BEGIN INSERT INTO p12_sql VALUES(2,'routine'); END//\nDELIMITER ;\nCALL p12_proc();"
			sleep := "SELECT SLEEP(5)"
			if c.Driver == "postgres" {
				routine = "CREATE FUNCTION p12_fn() RETURNS void LANGUAGE plpgsql AS $body$ BEGIN INSERT INTO p12_sql VALUES(2,'routine'); END $body$; SELECT p12_fn();"
				sleep = "SELECT pg_sleep(5)"
			}
			got, err = e.ExecuteImport(ctx, r, []byte(routine), nil)
			if err != nil || got.Outcome != "completed" {
				t.Fatalf("routine %#v %v", got, err)
			}
			if n := execute(t, e, c, "SELECT count(*) FROM p12_sql WHERE id=2").Results[0].Rows[0][0].Value; n != "1" {
				t.Fatal("routine readback", n)
			}
			timed := r
			timed.TimeoutMS = 30
			got, err = e.ExecuteImport(ctx, timed, []byte(sleep+";INSERT INTO p12_sql VALUES(3,'canceled')"), nil)
			if err != nil || got.Statements[0].ErrorCode != "query_canceled" || got.Statements[1].Outcome != "not_executed" {
				t.Fatalf("timeout %#v %v", got, err)
			}
			if n := execute(t, e, c, "SELECT count(*) FROM p12_sql WHERE id=3").Results[0].Rows[0][0].Value; n != "0" {
				t.Fatal("canceled SQL persisted", n)
			}
			// Conflicts in skip mode must not abort PostgreSQL's whole transaction.
			csv := importRequest(c)
			csv.Mode = "skip"
			file := []byte("1,conflict\n4,new\n5,new")
			p, err := e.PlanImport(ctx, csv, file)
			if err != nil {
				t.Fatal(err)
			}
			result, err := e.ExecuteImport(ctx, importConfirm(csv, p), file, nil)
			if err != nil || result.Outcome != "completed" || result.Skipped != 1 || result.Inserted != 2 {
				t.Fatalf("savepoints %#v %v", result, err)
			}
			if n := execute(t, e, c, "SELECT count(*) FROM p12_rows WHERE id IN(4,5)").Results[0].Rows[0][0].Value; n != "2" {
				t.Fatal("skip readback", n)
			}
			// A read-only database role can preview, but cannot execute the CSV plan.
			if c.Driver == "postgres" {
				execute(t, e, c, "CREATE ROLE p12_reader LOGIN PASSWORD 'public-import-fixture'")
				execute(t, e, c, "GRANT CONNECT ON DATABASE probe TO p12_reader")
				execute(t, e, c, "GRANT USAGE ON SCHEMA public TO p12_reader")
				execute(t, e, c, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO p12_reader")
			} else {
				execute(t, e, c, "CREATE USER 'p12_reader'@'%' IDENTIFIED BY 'public-import-fixture'")
				execute(t, e, c, "GRANT SELECT ON probe.* TO 'p12_reader'@'%'")
			}
			restricted := csv
			restricted.Connection.Username = "p12_reader"
			restricted.Connection.Password = "public-import-fixture"
			p, err = e.PlanImport(ctx, restricted, []byte("6,forbidden"))
			if err != nil {
				t.Fatal("reader preview", err)
			}
			denied, err := e.ExecuteImport(ctx, importConfirm(restricted, p), []byte("6,forbidden"), nil)
			if err == nil && denied.Inserted != 0 {
				t.Fatal("read-only role inserted")
			}
			if n := execute(t, e, c, "SELECT count(*) FROM p12_rows WHERE id=6").Results[0].Rows[0][0].Value; n != "0" {
				t.Fatal("permission failure persisted")
			}
			if c.Driver == "mysql" {
				execute(t, e, c, "CREATE TABLE p12_nontransactional(id INTEGER,value TEXT) ENGINE=MyISAM")
				unsupported := csv
				unsupported.Table.Name = "p12_nontransactional"
				if _, err = e.PlanImport(ctx, unsupported, []byte("1,unsafe")); err == nil {
					t.Fatal("MyISAM accepted")
				}
			}
		})
	}
}
