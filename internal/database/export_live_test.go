// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
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
			cleanup()
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
