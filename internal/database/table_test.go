// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestTableDescribeAndBrowse(t *testing.T) {
	tableCases(t, testEngine(t), sqliteConnection())
}
func tableCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	driver := c.Driver
	name := "odd\"table'`"
	q := func(value string) string { return quoteIdentifier(driver, value) }
	ref := core.TableRef{Name: name}
	if driver == "mysql" {
		ref.Schema = c.Database
	} else if driver == "postgres" {
		ref.Schema = "public"
	}
	definition := "CREATE TABLE " + q(name) + " (a INTEGER NOT NULL,b INTEGER NOT NULL,value TEXT,raw "
	if driver == "postgres" {
		definition += "BYTEA"
	} else {
		definition += "BLOB"
	}
	definition += ",amount DECIMAL(30,5) DEFAULT 1.25,calc INTEGER GENERATED ALWAYS AS (a+b) STORED,PRIMARY KEY(b,a))"
	execute(t, e, c, definition)
	for _, sql := range []string{
		"INSERT INTO " + q(name) + "(a,b,value,raw,amount) VALUES (2,1,'second'," + binaryLiteral(driver) + ",12345678901234567890.12345)",
		"INSERT INTO " + q(name) + "(a,b,value,raw) VALUES (1,1,'first',NULL)",
		"INSERT INTO " + q(name) + "(a,b,value,raw) VALUES (1,2,NULL,NULL)",
	} {
		execute(t, e, c, sql)
	}
	description, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: ref})
	if err != nil {
		t.Fatal(err)
	}
	if len(description.Columns) != 6 || len(description.PrimaryKey) != 2 || description.PrimaryKey[0] != "b" || description.PrimaryKey[1] != "a" || !description.StableOrder || !description.ReadOnly {
		t.Fatalf("description %#v", description)
	}
	if description.Columns[5].Generated != "stored" || description.Columns[4].DefaultValue == nil || !description.Columns[2].Nullable || description.Columns[0].Nullable {
		t.Fatalf("column flags %#v", description.Columns)
	}
	browse := core.BrowseRequest{Connection: c, Table: ref, PageSize: 1}
	page, err := e.Browse(context.Background(), browse)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || !page.StableOrder || len(page.Result.Rows) != 1 || page.Result.Rows[0][2].Value != "first" {
		t.Fatalf("first page %#v", page)
	}
	browse.Offset = 1
	page, err = e.Browse(context.Background(), browse)
	if err != nil || page.Result.Rows[0][2].Value != "second" || page.Result.Rows[0][3].Kind != "binary" || page.Result.Rows[0][3].Value != "AP8=" {
		t.Fatalf("offset page %#v %v", page, err)
	}
	if driver != "sqlite" && page.Result.Rows[0][4].Value != "12345678901234567890.12345" {
		t.Fatalf("numeric precision %#v", page.Result.Rows[0][4])
	}
	browse.Offset = 0
	browse.Sort = []core.TableSort{{Column: "a", Direction: "desc"}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || page.Result.Rows[0][0].Value != "2" {
		t.Fatalf("sort %#v %v", page, err)
	}
	browse.Sort = nil
	browse.Filters = []core.TableFilter{{Column: "value", Operator: "eq", Value: &core.Cell{Kind: "text", Value: "first' OR 1=1 --"}}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || len(page.Result.Rows) != 0 {
		t.Fatalf("bound injection %#v %v", page, err)
	}
	browse.Filters = []core.TableFilter{{Column: "value", Operator: "is_null"}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || len(page.Result.Rows) != 1 || page.Result.Rows[0][2].Kind != "null" {
		t.Fatalf("null filter %#v %v", page, err)
	}
	browse.Filters = []core.TableFilter{{Column: "raw", Operator: "eq", Value: &core.Cell{Kind: "binary", Value: "AP8="}}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("binary filter %#v %v", page, err)
	}
	browse.Filters = []core.TableFilter{{Column: "a", Operator: "eq", Value: &core.Cell{Kind: "integer", Value: "2"}}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("integer filter %#v %v", page, err)
	}
	browse.Filters = []core.TableFilter{{Column: "amount", Operator: "eq", Value: &core.Cell{Kind: "decimal", Value: "12345678901234567890.12345"}}}
	page, err = e.Browse(context.Background(), browse)
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("decimal filter %#v %v", page, err)
	}
	for _, bad := range []core.TableFilter{{Column: "value; DROP TABLE x", Operator: "eq", Value: &core.Cell{Kind: "text", Value: "x"}}, {Column: "value", Operator: "eq OR 1=1"}, {Column: "value", Operator: "eq", Value: &core.Cell{Kind: "null"}}, {Column: "raw", Operator: "eq", Value: &core.Cell{Kind: "binary", Value: "?invalid"}}} {
		browse.Filters = []core.TableFilter{bad}
		if _, err := e.Browse(context.Background(), browse); err == nil {
			t.Fatalf("bad filter %#v accepted", bad)
		}
	}
	browse.Filters = nil
	browse.Sort = []core.TableSort{{Column: "value", Direction: "asc;DROP TABLE x"}}
	if _, err := e.Browse(context.Background(), browse); err == nil {
		t.Fatal("sort injection accepted")
	}
	badRef := ref
	badRef.Name += "; DROP TABLE x"
	if _, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: badRef}); err == nil {
		t.Fatal("missing injected table accepted")
	}
	specialColumn := "odd\"column'`"
	execute(t, e, c, "CREATE TABLE "+q("identifier_fixture")+" (id INTEGER PRIMARY KEY,"+q(specialColumn)+" TEXT)")
	execute(t, e, c, "INSERT INTO "+q("identifier_fixture")+" VALUES(1,'kept')")
	identifierPage, identifierErr := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Schema: ref.Schema, Name: "identifier_fixture"}, Sort: []core.TableSort{{Column: specialColumn, Direction: "asc"}}, Filters: []core.TableFilter{{Column: specialColumn, Operator: "eq", Value: &core.Cell{Kind: "text", Value: "kept"}}}})
	if identifierErr != nil || len(identifierPage.Result.Rows) != 1 {
		t.Fatalf("quoted column %#v %v", identifierPage, identifierErr)
	}
	execute(t, e, c, "CREATE TABLE "+q("without_key")+" (value TEXT)")
	noKey, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Schema: ref.Schema, Name: "without_key"}})
	if err != nil || noKey.StableOrder || !noKey.ReadOnly {
		t.Fatalf("no key %#v %v", noKey, err)
	}
	execute(t, e, c, "INSERT INTO "+q("without_key")+" VALUES('duplicate'),('duplicate')")
	noKeyPage, noKeyErr := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Schema: ref.Schema, Name: "without_key"}, PageSize: 1})
	if noKeyErr != nil || noKeyPage.StableOrder || !noKeyPage.HasMore {
		t.Fatalf("unstable pagination %#v %v", noKeyPage, noKeyErr)
	}
	blobType := "BLOB"
	if driver == "postgres" {
		blobType = "BYTEA"
	}
	execute(t, e, c, "CREATE TABLE "+q("empty_blob_fixture")+"(value "+blobType+")")
	emptyBlob := "X''"
	if driver == "postgres" {
		emptyBlob = "decode('','hex')"
	}
	execute(t, e, c, "INSERT INTO "+q("empty_blob_fixture")+" VALUES(NULL),("+emptyBlob+")")
	emptyPage, emptyErr := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Schema: ref.Schema, Name: "empty_blob_fixture"}, Filters: []core.TableFilter{{Column: "value", Operator: "eq", Value: &core.Cell{Kind: "binary", Value: ""}}}})
	if emptyErr != nil || len(emptyPage.Result.Rows) != 1 || emptyPage.Result.Rows[0][0] != (core.Cell{Kind: "binary", Value: ""}) {
		t.Fatalf("empty blob filter %#v %v", emptyPage, emptyErr)
	}
	execute(t, e, c, "CREATE VIEW "+q("visible_view")+" AS SELECT value FROM "+q(name))
	view, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Schema: ref.Schema, Name: "visible_view"}})
	if err != nil || view.Kind != "view" || !view.ReadOnly || view.StableOrder {
		t.Fatalf("view %#v %v", view, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Browse(canceled, core.BrowseRequest{Connection: c, Table: ref}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled browse: %v", err)
	}
	encoded, _ := json.Marshal(page)
	if len(encoded)+1 > MaxResponseBytes {
		t.Fatal("oversized browse response")
	}
}
func binaryLiteral(driver string) string {
	if driver == "postgres" {
		return "decode('00ff','hex')"
	}
	return "X'00FF'"
}
func TestSQLiteNullablePrimaryKeyAndBounds(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	for _, sql := range []string{"CREATE TABLE alias(id INTEGER PRIMARY KEY,value)", "CREATE TABLE descending(id INTEGER PRIMARY KEY DESC,value)", "CREATE TABLE nullable(id TEXT PRIMARY KEY,value)", "CREATE TABLE large(id INTEGER PRIMARY KEY,value TEXT)", "INSERT INTO large VALUES(1,'x')"} {
		execute(t, e, c, sql)
	}
	for _, tc := range []struct {
		name   string
		stable bool
	}{{"alias", true}, {"descending", false}, {"nullable", false}} {
		d, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: tc.name}})
		if err != nil || d.StableOrder != tc.stable {
			t.Fatalf("%s %#v %v", tc.name, d, err)
		}
	}
	for _, r := range []core.BrowseRequest{{Connection: c, Table: core.TableRef{Name: "large"}, PageSize: 1001}, {Connection: c, Table: core.TableRef{Name: "large"}, Offset: -1}, {Connection: c, Table: core.TableRef{Name: "large"}, TimeoutMS: 60001}} {
		if _, err := e.Browse(context.Background(), r); err == nil {
			t.Fatal("bad boundary accepted")
		}
	}
	// Views are read-only, but remain subject to execution deadlines and output limits.
	execute(t, e, c, "CREATE VIEW slow AS WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT sum(x) AS value FROM n")
	_, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "slow"}, TimeoutMS: 20})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("view timeout: %v", err)
	}
	execute(t, e, c, "CREATE VIEW huge AS SELECT zeroblob(1048577) AS value")
	_, err = e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "huge"}})
	var public *core.Error
	if !errors.As(err, &public) || public.Code != "result_too_large" {
		t.Fatalf("view cell limit: %v", err)
	}
}
func TestLiveTableBrowse(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("B2a disposable fixture port required")
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
			tableCases(t, e, c)
			quote := func(value string) string { return quoteIdentifier(c.Driver, value) }
			slow := "SELECT SLEEP(5) AS value"
			large := "SELECT REPEAT('x',1048577) AS value"
			if c.Driver == "postgres" {
				slow = "SELECT 1 AS value FROM pg_sleep(5)"
				large = "SELECT repeat('x',1048577) AS value"
			}
			execute(t, e, c, "CREATE VIEW "+quote("slow_table")+" AS "+slow)
			execute(t, e, c, "CREATE VIEW "+quote("large_table")+" AS "+large)
			start := time.Now()
			_, err = e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "slow_table"}, TimeoutMS: 30})
			if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
				t.Fatalf("live browse timeout: %v", err)
			}
			_, err = e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "large_table"}})
			var sizeError *core.Error
			if !errors.As(err, &sizeError) || sizeError.Code != "result_too_large" {
				t.Fatalf("live browse cell limit: %v", err)
			}

			if c.Driver == "mysql" {
				execute(t, e, c, "CREATE TABLE nontransactional(value TEXT) ENGINE=MyISAM")
				d, err := e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "nontransactional"}})
				if err != nil || d.Transactional != "no" || !d.ReadOnly {
					t.Fatalf("nontransactional %#v %v", d, err)
				}
			}
		})
	}
}

func TestBrowseLookaheadDoesNotConsumeResponseBudget(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE lookahead(id INTEGER PRIMARY KEY,value BLOB)")
	execute(t, e, c, "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<6) INSERT INTO lookahead SELECT x,zeroblob(1048500) FROM n")
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "lookahead"}, PageSize: 5})
	if err != nil || len(page.Result.Rows) != 5 || !page.HasMore || page.Result.Truncated {
		t.Fatalf("lookahead %#v %v", page, err)
	}
}
