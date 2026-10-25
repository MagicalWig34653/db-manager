// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// splitExportTestSQL is a test-only reader for our table dumps, not an import
// parser. Routine bodies and client delimiters belong to W-P12/W-P19.
func splitExportTestSQL(s string) []string {
	var result []string
	start := 0
	var quote byte
	comment := false
	for i := 0; i < len(s); i++ {
		if comment {
			if s[i] == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			if s[i] == '\\' && quote == '\'' {
				i++
				continue
			}
			if s[i] == quote {
				if i+1 < len(s) && s[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if s[i] == '-' && i+1 < len(s) && s[i+1] == '-' {
			comment = true
			i++
			continue
		}
		if s[i] == '\'' || s[i] == '"' || s[i] == '`' {
			quote = s[i]
			continue
		}
		if s[i] == ';' {
			result = append(result, s[start:i+1])
			start = i + 1
		}
	}
	if strings.TrimSpace(s[start:]) != "" && !strings.HasPrefix(strings.TrimSpace(s[start:]), "--") {
		result = append(result, s[start:])
	}
	return result
}

func TestLiveExportRoundTrip(t *testing.T) {
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
			name := "export_restored"
			execute(t, e, c, "CREATE DATABASE "+quoteIdentifier(c.Driver, name))
			target := c
			target.Database = name
			var dump bytes.Buffer
			if err = e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Extended: true}, &dump); err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("DBM_EXPORT_DUMP"); dir != "" {
				if err = os.WriteFile(filepath.Join(dir, "live-"+system.name+".sql"), dump.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			db, cleanup, err := e.open(context.Background(), target)
			if err != nil {
				t.Fatal(err)
			}
			timeZoneSQL := "SET time_zone='+05:30'"
			if c.Driver == "postgres" {
				timeZoneSQL = "SET TIME ZONE 'Asia/Kolkata'"
			}
			if _, err = db.ExecContext(context.Background(), timeZoneSQL); err != nil {
				cleanup()
				t.Fatal(err)
			}
			for _, statement := range splitExportTestSQL(dump.String()) {
				if _, err = db.ExecContext(context.Background(), statement); err != nil {
					cleanup()
					t.Fatal("restore failed", err)
				}
			}
			// Idempotent structure restore: a second pass of the structure-only dump with IF NOT EXISTS must succeed.
			var again bytes.Buffer
			if err = e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Data: exportBool(false), IfNotExists: true}, &again); err != nil {
				cleanup()
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				for _, statement := range splitExportTestSQL(again.String()) {
					if _, err = db.ExecContext(context.Background(), statement); err != nil {
						cleanup()
						t.Fatalf("idempotent restore pass %d failed: %v\n%s", pass, err, statement)
					}
				}
			}
			cleanup()
			if c.Driver == "mysql" {
				exportBackslashRestore(t, e, c)
			}
			source := execute(t, e, c, "SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name))
			restored := execute(t, e, target, "SELECT * FROM "+quoteIdentifier(c.Driver, ref.Name))
			if !reflect.DeepEqual(source.Results, restored.Results) {
				t.Fatalf("restored values differ: %#v %#v", source, restored)
			}
			d1, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: ref})
			if err != nil {
				t.Fatal(err)
			}
			d2, err := e.Describe(context.Background(), core.DescribeRequest{Connection: target, Table: ref})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(d1.Columns, d2.Columns) {
				t.Fatal("restored columns differ")
			}
			structure1, err := e.Structure(context.Background(), core.DescribeRequest{Connection: c, Table: ref})
			if err != nil {
				t.Fatal(err)
			}
			structure2, err := e.Structure(context.Background(), core.DescribeRequest{Connection: target, Table: ref})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(structure1.Indexes, structure2.Indexes) {
				t.Fatal("restored index metadata differs")
			}
			for _, format := range []string{"json", "csv"} {
				var out bytes.Buffer
				if err = e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: format}, &out); err != nil {
					t.Fatal(err)
				}
				if dir := os.Getenv("DBM_EXPORT_DUMP"); dir != "" {
					if err = os.WriteFile(filepath.Join(dir, "live-"+system.name+"."+format), out.Bytes(), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if format == "json" {
					var value any
					if err = json.Unmarshal(out.Bytes(), &value); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err = csv.NewReader(&out).ReadAll(); err != nil {
						t.Fatal(err)
					}
				}
			}

			snapshot := &exportChangeWriter{change: func() error {
				_, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "UPDATE " + quoteIdentifier(c.Driver, ref.Name) + " SET text_value='independent writer'"})
				return err
			}}
			if err = e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{ref}, Format: "json"}, snapshot); err != nil {
				t.Fatal("snapshot export", err)
			}
			if strings.Contains(snapshot.String(), "independent writer") {
				t.Fatal("export did not retain its repeatable-read snapshot")
			}
			if queryText(t, e, c, "SELECT text_value FROM "+quoteIdentifier(c.Driver, ref.Name)) != "independent writer" {
				t.Fatal("independent writer did not commit")
			}
			execute(t, e, c, "CREATE USER "+quoteIdentifier(c.Driver, "export_denied")+map[bool]string{true: " IDENTIFIED BY 'synthetic-export-password'", false: " WITH PASSWORD 'synthetic-export-password'"}[c.Driver == "mysql"])
			restricted := c
			restricted.Username = "export_denied"
			restricted.Password = "synthetic-export-password"
			var restrictedOut bytes.Buffer
			if err = e.Export(context.Background(), core.ExportRequest{Connection: restricted, Tables: []core.TableRef{ref}}, &restrictedOut); err == nil {
				t.Fatal("missing SELECT rights accepted")
			}
			if c.Driver == "mysql" {
				execute(t, e, c, "CREATE TABLE export_nontransactional(id INTEGER) ENGINE=MyISAM")
				var unsupported bytes.Buffer
				if err = e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "export_nontransactional"}}}, &unsupported); err == nil {
					t.Fatal("nontransactional table accepted")
				}
			}
			denied := c
			denied.Password = "synthetic-invalid-password"
			var out bytes.Buffer
			if err = e.Export(context.Background(), core.ExportRequest{Connection: denied, Tables: []core.TableRef{ref}}, &out); err == nil || strings.Contains(out.String(), denied.Password) {
				t.Fatal("credential failure leaked or accepted")
			}
		})
	}
}

// exportChangeWriter performs an independent committed update after the snapshot
// is established but before its first data row is streamed.
type exportChangeWriter struct {
	bytes.Buffer
	change  func() error
	changed bool
}

func (w *exportChangeWriter) Write(p []byte) (int, error) {
	if !w.changed {
		w.changed = true
		if err := w.change(); err != nil {
			return 0, err
		}
	}
	return w.Buffer.Write(p)
}

// exportBackslashRestore restores a dump with backslashes in default and comment
// literals on a session that runs with NO_BACKSLASH_ESCAPES.
func exportBackslashRestore(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	execute(t, e, c, "CREATE TABLE export_bs (id INT PRIMARY KEY, v VARCHAR(10) DEFAULT 'a\\\\b' COMMENT 'c\\\\d')")
	var dump bytes.Buffer
	if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "export_bs"}}, Data: exportBool(false), Drop: true}, &dump); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dump.String(), "sql_mode") {
		t.Fatal("dump header does not neutralize sql_mode")
	}
	target := c
	target.Database = "export_restored"
	db, cleanup, err := e.open(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err = db.ExecContext(context.Background(), "SET SESSION sql_mode=CONCAT(@@sql_mode,',NO_BACKSLASH_ESCAPES')"); err != nil {
		t.Fatal(err)
	}
	for _, statement := range splitExportTestSQL(dump.String()) {
		if _, err = db.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("restore under NO_BACKSLASH_ESCAPES failed: %v\n%s", err, statement)
		}
	}
	var mode string
	if err = db.QueryRowContext(context.Background(), "SELECT @@SESSION.sql_mode").Scan(&mode); err != nil || !strings.Contains(mode, "NO_BACKSLASH_ESCAPES") {
		t.Fatalf("sql_mode not restored after the dump: %q %v", mode, err)
	}
	var def, comment string
	query := "SELECT COLUMN_DEFAULT, COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME='export_bs' AND COLUMN_NAME='v'"
	if err = db.QueryRowContext(context.Background(), query, "export_restored").Scan(&def, &comment); err != nil {
		t.Fatal(err)
	}
	var wantDef, wantComment string
	source, sourceCleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	defer sourceCleanup()
	if err = source.QueryRowContext(context.Background(), query, c.Database).Scan(&wantDef, &wantComment); err != nil {
		t.Fatal(err)
	}
	if def != wantDef || comment != wantComment || !strings.Contains(comment, "\\") {
		t.Fatalf("backslashes changed: default %q (source %q) comment %q (source %q)", def, wantDef, comment, wantComment)
	}
}
