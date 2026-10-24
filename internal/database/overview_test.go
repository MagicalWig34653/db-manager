// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func queryText(t *testing.T, e *Engine, c core.Connection, sql string) string {
	t.Helper()
	r := execute(t, e, c, sql)
	if len(r.Results) != 1 || len(r.Results[0].Rows) < 1 {
		t.Fatalf("no row for %s", sql)
	}
	return r.Results[0].Rows[0][0].Value
}

// dumpOverview writes real engine results for the contract checker when
// DBM_OVERVIEW_DUMP names a directory (scripts/test-overview.py).
func dumpOverview(t *testing.T, kind string, value any) {
	t.Helper()
	dir := os.Getenv("DBM_OVERVIEW_DUMP")
	if dir == "" {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "." + kind + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// overviewCases runs the overview contract against one live or sqlite database.
// schema is empty for SQLite and the isolated schema/database for the others.
func overviewCases(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	ctx := context.Background()
	q := func(name string) string { return quoteIdentifier(c.Driver, name) }
	qualified := func(name string) string {
		if schema == "" {
			return q(name)
		}
		return q(schema) + "." + q(name)
	}
	// Empty database/schema.
	empty, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: schema})
	if err != nil || empty.Total != 0 || len(empty.Objects) != 0 || empty.HasMore {
		t.Fatalf("empty %#v %v", empty, err)
	}
	// Many objects with paging.
	for i := 0; i < 25; i++ {
		execute(t, e, c, fmt.Sprintf("CREATE TABLE %s(id INTEGER PRIMARY KEY, v TEXT)", qualified(fmt.Sprintf("t_%03d", i))))
	}
	odd := "we\"ird `na'me;DROP TABLE x"
	execute(t, e, c, fmt.Sprintf("CREATE TABLE %s(id INTEGER PRIMARY KEY, v TEXT)", qualified(odd)))
	execute(t, e, c, fmt.Sprintf("INSERT INTO %s VALUES(1,'a'),(2,'b'),(3,'c')", qualified(odd)))
	execute(t, e, c, fmt.Sprintf("CREATE VIEW %s AS SELECT id FROM %s", qualified("v_view"), qualified("t_000")))
	seen := map[string]core.ObjectInfo{}
	for offset, pages := 0, 0; ; pages++ {
		page, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: schema, Limit: 10, Offset: offset})
		if err != nil || page.Total != 27 || page.Limit != 10 || page.Offset != offset {
			t.Fatalf("page %d %#v %v", pages, page, err)
		}
		for _, o := range page.Objects {
			if _, dup := seen[o.Name]; dup {
				t.Fatalf("duplicate %s across pages", o.Name)
			}
			seen[o.Name] = o
		}
		if !page.HasMore {
			if pages != 2 || len(page.Objects) != 7 {
				t.Fatalf("last page %d %d", pages, len(page.Objects))
			}
			break
		}
		offset += 10
	}
	if len(seen) != 27 || seen["v_view"].Kind != "view" || seen["t_000"].Kind != "table" || seen[odd].Name != odd {
		t.Fatalf("kinds %#v", seen[odd])
	}
	if seen["v_view"].Rows.State == "exact" || seen["v_view"].Rows.Value != nil {
		t.Fatalf("view row count %#v", seen["v_view"].Rows)
	}
	// Estimate versus exact.
	if st := seen[odd].Rows.State; st == "exact" {
		t.Fatalf("estimate presented as exact")
	}
	exact, err := e.Objects(ctx, core.ObjectsRequest{Connection: c, Schema: schema, ExactRows: true, Limit: 100})
	if err != nil || !exact.ExactRequested || exact.ExactIncomplete {
		t.Fatalf("exact %#v %v", exact, err)
	}
	for _, o := range exact.Objects {
		switch {
		case o.Name == odd && (o.Rows.State != "exact" || o.Rows.Value == nil || *o.Rows.Value != 3):
			t.Fatalf("exact odd %#v", o.Rows)
		case o.Kind == "table" && o.Rows.State != "exact":
			t.Fatalf("exact table %#v", o)
		case o.Kind == "view" && o.Rows.State == "exact":
			t.Fatalf("exact view %#v", o)
		}
	}
	dumpOverview(t, "objects", exact)
	for _, bad := range []core.ObjectsRequest{{Limit: 1001}, {Offset: -1}, {ExactRows: true, Limit: 101}, {TimeoutMS: 30001}} {
		bad.Connection = c
		bad.Schema = schema
		if _, err := e.Objects(ctx, bad); err == nil {
			t.Fatalf("accepted %#v", bad)
		} else {
			requireMutationCode(t, err, "invalid_overview")
		}
	}
	// Definitions against the direct query.
	for _, name := range []string{odd, "v_view"} {
		d, err := e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: core.TableRef{Schema: schema, Name: name}})
		if err != nil || d.Definition == "" || d.Object.Name != name {
			t.Fatalf("definition %s %#v %v", name, d, err)
		}
		dumpOverview(t, "definition-"+map[bool]string{true: "view", false: "table"}[name == "v_view"], d)
		switch c.Driver {
		case "sqlite":
			if want := queryText(t, e, c, "SELECT sql FROM sqlite_schema WHERE name='"+strings.ReplaceAll(name, "'", "''")+"'"); d.Definition != want || d.Derived {
				t.Fatalf("sqlite definition %q want %q", d.Definition, want)
			}
		case "mysql":
			direct := execute(t, e, c, "SHOW CREATE "+map[bool]string{true: "VIEW", false: "TABLE"}[name == "v_view"]+" "+qualified(name))
			if d.Definition != direct.Results[0].Rows[0][1].Value || d.Derived || d.Source != "show_create" {
				t.Fatalf("mysql definition mismatch %q", d.Definition)
			}
		case "postgres":
			if !d.Derived || d.Notice == nil || d.Source != "catalog_reconstruction" {
				t.Fatalf("postgres must be derived %#v", d)
			}
		}
	}
	for _, name := range []string{"absent", odd + "x", "t_000\"; DROP TABLE t_000;--", "sqlite_master"} {
		_, err := e.Definition(ctx, core.DefinitionRequest{Connection: c, Object: core.TableRef{Schema: schema, Name: name}})
		requireMutationCode(t, err, "object_not_found")
	}
	if got := queryText(t, e, c, "SELECT COUNT(*) FROM "+qualified("t_000")); got != "0" {
		t.Fatal("fixture object touched")
	}
	// Databases and server info against direct queries.
	dbs, err := e.Databases(ctx, core.DatabasesRequest{Connection: c, Limit: 1000})
	if err != nil || len(dbs.Databases) == 0 || dbs.Total != int64(len(dbs.Databases)) {
		t.Fatalf("databases %#v %v", dbs, err)
	}
	foundCurrent := false
	for _, d := range dbs.Databases {
		foundCurrent = foundCurrent || (d.Current && d.Name == c.Database)
	}
	if !foundCurrent {
		t.Fatalf("current database missing: %#v", dbs.Databases)
	}
	dumpOverview(t, "databases", dbs)
	one, err := e.Databases(ctx, core.DatabasesRequest{Connection: c, Limit: 1})
	if err != nil || len(one.Databases) != 1 || one.HasMore != (dbs.Total > 1) {
		t.Fatalf("db paging %#v %v", one, err)
	}
	info, err := e.ServerInfo(ctx, core.ServerInfoRequest{Connection: c})
	if err != nil || info.Version == "" || len(info.Collations) == 0 || len(info.CharacterSets) == 0 {
		t.Fatalf("server info %#v %v", info, err)
	}
	dumpOverview(t, "server-info", info)
	direct := map[string]string{"sqlite": "SELECT sqlite_version()", "mysql": "SELECT VERSION()", "postgres": "SHOW server_version"}[c.Driver]
	if want := queryText(t, e, c, direct); info.Version != want {
		t.Fatalf("version %q want %q", info.Version, want)
	}
	encoded := fmt.Sprintf("%+v", info)
	if c.Password != "" && strings.Contains(encoded, c.Password) {
		t.Fatal("password leaked in server info")
	}
	if info.Connection.Database != c.Database || info.Connection.Username != c.Username && c.Driver != "sqlite" {
		t.Fatalf("connection echo %#v", info.Connection)
	}
	if c.Driver != "sqlite" {
		if info.UptimeSeconds == nil || len(info.Engines) == 0 || len(info.Plugins) == 0 {
			t.Fatalf("server info incomplete %#v", info)
		}
	}
	_ = strconv.Itoa
}

func TestSQLiteOverview(t *testing.T) {
	e := testEngine(t)
	overviewCases(t, e, core.Connection{Driver: "sqlite", Database: "overview"}, "")
}

func TestOverviewValidation(t *testing.T) {
	e := testEngine(t)
	c := core.Connection{Driver: "sqlite", Database: "overviewvalid"}
	_, err := e.Definition(context.Background(), core.DefinitionRequest{Connection: c, Object: core.TableRef{}})
	requireMutationCode(t, err, "invalid_overview")
	_, err = e.Objects(context.Background(), core.ObjectsRequest{Connection: c, Schema: "temp"})
	requireMutationCode(t, err, "invalid_overview")
}

func TestLiveOverview(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("W-P06 disposable TLS fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			schema := "p06_main"
			create := "CREATE SCHEMA "
			if c.Driver == "mysql" {
				create = "CREATE DATABASE "
			}
			execute(t, e, c, create+quoteIdentifier(c.Driver, schema))
			overviewCases(t, e, c, schema)
			liveOverviewRights(t, e, c, schema)
		})
	}
}

// liveOverviewRights proves that a role without privileges sees no objects.
func liveOverviewRights(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	limited := c
	limited.Username, limited.Password = "p06_nobody", "public-p06-fixture"
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE USER 'p06_nobody'@'%' IDENTIFIED BY 'public-p06-fixture'")
		limited.Database = "" // the role holds no right on the fixture database
	} else {
		execute(t, e, c, "CREATE ROLE p06_nobody LOGIN PASSWORD 'public-p06-fixture'")
	}
	objects, err := e.Objects(context.Background(), core.ObjectsRequest{Connection: limited, Schema: schema})
	if err != nil || objects.Total != 0 || len(objects.Objects) != 0 {
		t.Fatalf("limited role sees objects %#v %v", objects, err)
	}
	_, err = e.Definition(context.Background(), core.DefinitionRequest{Connection: limited, Object: core.TableRef{Schema: schema, Name: "t_000"}})
	requireMutationCode(t, err, "object_not_found")
	if _, err = e.ServerInfo(context.Background(), core.ServerInfoRequest{Connection: limited}); err != nil {
		t.Fatalf("limited server info: %v", err)
	}
	// A wrong password never succeeds and never echoes the secret.
	limited.Password = "wrong"
	if _, err = e.ServerInfo(context.Background(), core.ServerInfoRequest{Connection: limited}); err == nil || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("bad credentials handling: %v", err)
	}
}
