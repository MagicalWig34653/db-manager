// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// attackNames returns hostile but creatable identifiers for the driver.
func attackNames(driver string) []string {
	long := 300
	switch driver {
	case "mysql":
		long = 64
	case "postgres":
		long = 63
	}
	names := []string{
		"a`b", "a``b", `a"b`, `a""b`, "a'b", "x;DROP TABLE y;--", "`; DROP TABLE t_atk; --", `"; DROP TABLE t_atk; --`,
		"ünï-名前-é", "a b", "a\nb", "a\\b", "%_", strings.Repeat("l", long), strings.Repeat("é", long/2-1),
	}
	if driver == "sqlite" {
		names = append(names, "Sqlite", strings.Repeat("é", 200))
	}
	return names
}

func attackCases(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	ctx := context.Background()
	q := func(n string) string { return quoteIdentifier(c.Driver, n) }
	qual := func(n string) string {
		if schema == "" {
			return q(n)
		}
		return q(schema) + "." + q(n)
	}
	execute(t, e, c, "CREATE TABLE "+qual("t_atk")+"(id INTEGER PRIMARY KEY)")
	execute(t, e, c, "INSERT INTO "+qual("t_atk")+" VALUES(1)")
	names := attackNames(c.Driver)
	for i, n := range names {
		execute(t, e, c, fmt.Sprintf("CREATE TABLE %s(id INTEGER PRIMARY KEY, v TEXT)", qual(n)))
		execute(t, e, c, fmt.Sprintf("INSERT INTO %s VALUES(1,'x'),(2,'y')", qual(n)))
		view := fmt.Sprintf("view_%d_%s", i, n)
		if len(view) > 60 {
			view = fmt.Sprintf("view_%d", i)
		}
		execute(t, e, c, fmt.Sprintf("CREATE VIEW %s AS SELECT id FROM %s", qual(view), qual(n)))
	}
	page, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: schema, ExactRows: true, Limit: 100})
	if err != nil || page.ExactIncomplete {
		t.Fatalf("objects %v incomplete=%v", err, page.ExactIncomplete)
	}
	got := map[string]core.ObjectInfo{}
	for _, o := range page.Objects {
		got[o.Name] = o
	}
	for _, n := range names {
		o, ok := got[n]
		if !ok || o.Kind != "table" || o.Rows.State != "exact" || o.Rows.Value == nil || *o.Rows.Value != 2 {
			t.Fatalf("name %q: %#v ok=%v", n, o, ok)
		}
		d, err := e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: core.TableRef{Schema: schema, Name: n}})
		if err != nil || d.Object.Name != n || d.Definition == "" {
			t.Fatalf("definition %q: %v", n, err)
		}
		// Hostile suffix/prefix variants must be unknown, never matched or executed.
		for _, bad := range []string{n + "`", n + `"`, n + ";", "`" + n} {
			// PostgreSQL truncates a bound `name` parameter to 63 bytes, so an over-long
			// variant of a 63-byte name resolves to that same catalog object (harmless: the
			// response carries the real catalog name; documented as review finding I1).
			if c.Driver == "postgres" && len(n) >= 62 {
				continue
			}
			if _, err := e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: core.TableRef{Schema: schema, Name: bad}}); err == nil {
				t.Fatalf("variant %q accepted", bad)
			} else {
				requireMutationCode(t, err, "object_not_found")
			}
		}
	}
	// Injection attempts via schema and name fields.
	for _, bad := range []core.TableRef{
		{Schema: schema, Name: "t_atk` ; DROP TABLE t_atk;--"}, {Schema: schema, Name: "t_atk\" ; DROP TABLE t_atk;--"},
		{Schema: schema + "`", Name: "t_atk"}, {Schema: schema + `"`, Name: "t_atk"}, {Schema: "x; DROP TABLE t_atk", Name: "t_atk"},
		{Schema: schema, Name: strings.Repeat("z", 1025)}, {Schema: schema, Name: "a\x00b"}, {Schema: "a\x00b", Name: "t"},
	} {
		_, err := e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: bad})
		if err == nil {
			t.Fatalf("accepted %#v", bad)
		}
		if ce, ok := err.(*core.Error); !ok || (ce.Code != "object_not_found" && ce.Code != "invalid_overview" && ce.Code != "database_error") {
			t.Fatalf("%#v: %v", bad, err)
		}
	}
	for _, bad := range []string{schema + "`", schema + `"`, "x; DROP TABLE t_atk", "a\x00b"} {
		r, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: bad, ExactRows: true})
		if err == nil && len(r.Objects) != 0 {
			t.Fatalf("schema %q listed %d objects", bad, len(r.Objects))
		}
	}
	if got := queryText(t, e, c, "SELECT COUNT(*) FROM "+qual("t_atk")); got != "1" {
		t.Fatal("fixture table damaged by injection attempt")
	}
	// Errors must not echo SQL, paths or credentials.
	_, err = e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: core.TableRef{Schema: schema, Name: "absent"}})
	for _, leak := range []string{"SELECT", "information_schema", "pg_class", "sqlite_schema", c.Password, "/tmp"} {
		if leak != "" && err != nil && strings.Contains(err.Error(), leak) {
			t.Fatalf("error leaks %q: %v", leak, err)
		}
	}
	// Exact counting is off by default.
	plain, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range plain.Objects {
		if o.Rows.State == "exact" {
			t.Fatalf("exact count without exact_rows: %#v", o)
		}
	}
}

func TestReviewAttackSQLite(t *testing.T) {
	attackCases(t, testEngine(t), core.Connection{Driver: "sqlite", Database: "attack"}, "")
}

func TestReviewAttackLive(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("disposable TLS fixture required")
			}
			port, _ := strconv.Atoi(raw)
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
			schema := "rv_atk"
			create := "CREATE SCHEMA "
			if c.Driver == "postgres" {
				c.Username = "postgres"
			} else {
				create = "CREATE DATABASE "
			}
			execute(t, e, c, create+quoteIdentifier(c.Driver, schema))
			attackCases(t, e, c, schema)
		})
	}
}

// TestReviewExactCountBudget proves the exact-count budget: a slow COUNT ends at
// the request timeout, is reported as unknown/incomplete and never as exact.
func TestReviewExactCountBudget(t *testing.T) {
	for _, test := range []struct{ name, driver, variable, fill string }{
		{"postgres", "postgres", "DBM_TEST_POSTGRES_PORT", "CREATE TABLE rv_big AS SELECT g AS id FROM generate_series(1,12000000) g"},
	} {
		raw := os.Getenv(test.variable)
		if raw == "" {
			t.Skip("fixture required")
		}
		port, _ := strconv.Atoi(raw)
		e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
		if err != nil {
			t.Fatal(err)
		}
		c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "postgres", Password: "public-b1-fixture"}
		execute(t, e, c, test.fill)
		start := time.Now()
		r, err := e.Objects(context.Background(), core.ObjectsRequest{Connection: c, Schema: "public", ExactRows: true, TimeoutMS: 40})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		if elapsed > 3*time.Second || !r.ExactIncomplete {
			t.Fatalf("elapsed=%v incomplete=%v", elapsed, r.ExactIncomplete)
		}
		for _, o := range r.Objects {
			if o.Name == "rv_big" && (o.Rows.State == "exact") {
				t.Fatalf("timed-out count reported exact: %#v", o)
			}
		}
		// Request cancellation aborts the whole call.
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		if _, err = e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: "public", ExactRows: true, TimeoutMS: 30000}); err == nil {
			t.Fatal("canceled context must fail")
		}
		time.Sleep(500 * time.Millisecond)
		active := queryText(t, e, c, "SELECT COUNT(*) FROM pg_stat_activity WHERE query LIKE 'SELECT COUNT(*) FROM \"public\".\"rv_big\"%' AND state='active'")
		t.Logf("elapsed=%v, server-side counts still active after abort: %s", elapsed, active)
	}
}
