// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func searchDump(t *testing.T, kind string, v any) {
	t.Helper()
	if dir := os.Getenv("DBM_SEARCH_DUMP"); dir != "" {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+"."+kind+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestSQLiteSearch(t *testing.T) { searchCases(t, testEngine(t), sqliteConnection()) }
func searchCases(t *testing.T, e *Engine, c core.Connection) {
	ctx := context.Background()
	q := func(s string) string { return quoteIdentifier(c.Driver, s) }
	name := "p14 odd'`\" --"
	ref := core.TableRef{Name: name}
	col := "value'`\""
	bin := "BLOB"
	if c.Driver == "postgres" {
		bin = "BYTEA"
	}
	execute(t, e, c, "CREATE TABLE "+q(name)+" (id INTEGER NOT NULL PRIMARY KEY,"+q(col)+" TEXT,other TEXT,raw "+bin+")")
	for _, sql := range []string{"(1,'100%_! fox','red',NULL)", "(2,'fox fox','blue',NULL)", "(3,NULL,'fox',NULL)", "(4,'unchanged','red',NULL)"} {
		execute(t, e, c, "INSERT INTO "+q(name)+" VALUES "+sql)
	}
	request := core.SearchRequest{Connection: c, Targets: []core.SearchTarget{{Table: ref, Columns: []string{col, "other"}}}, Mode: "substring", Term: "fox"}
	result, err := e.Search(ctx, request)
	if err != nil || result.Tables[0].Count != 3 || !result.Tables[0].CountExact {
		t.Fatalf("search %#v %v", result, err)
	}
	searchDump(t, "search", result)
	for _, term := range []string{"%_!", "100%_!"} {
		request.Term = term
		result, err = e.Search(ctx, request)
		if err != nil || result.Tables[0].Count != 1 {
			t.Fatalf("escaped %#v %v", result, err)
		}
	}
	request.Term = "red fox"
	request.Mode = "all_words"
	result, err = e.Search(ctx, request)
	if err != nil || result.Tables[0].Count != 1 {
		t.Fatalf("words %#v %v", result, err)
	}
	request.Term = "fox fox"
	request.Mode = "exact"
	result, err = e.Search(ctx, request)
	if err != nil || result.Tables[0].Count != 1 {
		t.Fatalf("exact %#v %v", result, err)
	}
	request.Term = "' OR 1=1 --"
	result, err = e.Search(ctx, request)
	if err != nil || result.Tables[0].Count != 0 {
		t.Fatalf("injection %#v %v", result, err)
	}
	request.Mode = "substring"
	request.Term = "fox"
	request.MaxRows = 1
	result, err = e.Search(ctx, request)
	if err != nil || result.Tables[0].Count != 1 || result.Tables[0].CountExact {
		t.Fatalf("limit %#v %v", result, err)
	}
	request.Targets[0].Columns = []string{"raw"}
	_, err = e.Search(ctx, request)
	requireMutationCode(t, err, "unsupported_search")
	request.Targets[0].Columns = []string{col}
	request.Targets[0].Table.Name = "absent; DROP TABLE x"
	_, err = e.Search(ctx, request)
	requireMutationCode(t, err, "table_not_found")
	request.Targets[0].Table = ref
	request.Mode = "regex"
	request.Term = "fo[x]"
	result, err = e.Search(ctx, request)
	if c.Driver == "sqlite" {
		requireMutationCode(t, err, "unsupported_search")
	} else if err != nil || result.Tables[0].Count != 1 {
		t.Fatalf("regex %#v %v", result, err)
	}
	// Native literal replacement, read-only preview, full-row conflicts and confirmation.
	replacement := "wolf"
	r := core.ReplaceRequest{Connection: c, Table: ref, Column: &col, Term: "fox", Replacement: &replacement, Mode: "substring"}
	before := execute(t, e, c, "SELECT * FROM "+q(name)+" ORDER BY id")
	p, err := e.ReplacePlan(ctx, r)
	if err != nil || p.AffectedRows != 2 || len(p.Examples) != 2 || p.Examples[1].After.Value != "wolf wolf" {
		t.Fatalf("plan %#v %v", p, err)
	}
	searchDump(t, "plan", p)
	after := execute(t, e, c, "SELECT * FROM "+q(name)+" ORDER BY id")
	if !reflect.DeepEqual(before.Results, after.Results) {
		t.Fatal("preview changed data")
	}
	run := func(p core.ReplacePlan) (core.ReplaceResult, error) {
		return e.ReplaceExecute(ctx, core.ReplaceExecuteRequest{ReplaceRequest: r, Confirm: core.ReplaceConfirm{Action: "replace", TargetName: name, AffectedRows: p.AffectedRows}, ExpectedPlanSHA256: p.SHA256})
	}
	bad := p
	bad.SHA256 = strings.Repeat("0", 64)
	_, err = run(bad)
	requireMutationCode(t, err, "plan_mismatch")
	execute(t, e, c, "UPDATE "+q(name)+" SET other='changed' WHERE id=4")
	_, err = run(p)
	requireMutationCode(t, err, "plan_mismatch")
	p, err = e.ReplacePlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	res, err := run(p)
	if err != nil || res.Outcome != "completed" || res.AffectedRows != 2 {
		t.Fatalf("execute %#v %v", res, err)
	}
	searchDump(t, "result", res)
	if got := execute(t, e, c, "SELECT "+q(col)+" FROM "+q(name)+" WHERE id=2").Results[0].Rows[0][0].Value; got != "wolf wolf" {
		t.Fatal(got)
	}
	_, err = run(p)
	requireMutationCode(t, err, "plan_mismatch")
	r.MaxRows = 2
	_, err = e.ReplacePlan(ctx, r)
	requireMutationCode(t, err, "replace_limit")
	r.MaxRows = 0
	for _, badCol := range []string{"id", "raw"} {
		r.Column = &badCol
		_, err = e.ReplacePlan(ctx, r)
		requireMutationCode(t, err, "unsupported_search")
	}
	r.Column = &col
	if c.Driver != "sqlite" {
		replacement = "lynx"
		r.Mode = "regex"
		r.Term = "wo[l]f"
		p, err = e.ReplacePlan(ctx, r)
		if err != nil || p.AffectedRows != 2 {
			t.Fatalf("regex plan %#v %v", p, err)
		}
		res, err = run(p)
		if err != nil || res.Outcome != "completed" || res.AffectedRows != 2 {
			t.Fatalf("regex replacement %#v %v", res, err)
		}
	}
	// Extended operators and unchanged v2 rejection.
	browse := core.BrowseRequest{Connection: c, Table: ref, PageSize: 1, Filters: []core.TableFilter{{Column: "id", Operator: "gte", Value: &core.Cell{Kind: "integer", Value: "2"}}}}
	page, err := e.SearchBrowse(ctx, browse)
	if err != nil || len(page.Result.Rows) != 1 || page.Result.Rows[0][0].Value != "2" || !page.HasMore {
		t.Fatalf("browse %#v %v", page, err)
	}
	searchDump(t, "browse", page)
	if _, err = e.Browse(ctx, browse); err == nil {
		t.Fatal("v2 accepted new operator")
	}
	for _, test := range []struct {
		op, value string
		want      int
	}{{"lt", "3", 2}, {"lte", "3", 3}, {"gt", "3", 1}, {"ne", "3", 3}, {"eq", "3", 1}} {
		browse.PageSize = 10
		browse.Filters[0].Operator = test.op
		browse.Filters[0].Value.Value = test.value
		got, err := e.SearchBrowse(ctx, browse)
		if err != nil || len(got.Result.Rows) != test.want {
			t.Fatalf("operator %s %#v %v", test.op, got, err)
		}
	}
	browse.Filters = []core.TableFilter{{Column: col, Operator: "contains", Value: &core.Cell{Kind: "text", Value: "%_!"}}}
	page, err = e.SearchBrowse(ctx, browse)
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("contains %#v %v", page, err)
	}
	browse.Filters[0].Operator = "like"
	browse.Filters[0].Value.Value = "100!%!_!!%"
	page, err = e.SearchBrowse(ctx, browse)
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("like %#v %v", page, err)
	}
}
func TestLiveSearch(t *testing.T) {
	for _, test := range []struct{ name, driver, port string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.port)
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
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture", TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			searchCases(t, e, c)
			liveSearchLock(t, e, c)
			liveSearchRestrictions(t, e, c)
		})
	}
}
func TestSQLiteReplaceSafety(t *testing.T) {
	ctx := context.Background()
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE safety (id INTEGER PRIMARY KEY, value TEXT UNIQUE ON CONFLICT REPLACE, other TEXT)")
	execute(t, e, c, "INSERT INTO safety VALUES (1,'fox','one'),(2,'wolf','two')")
	col := "value"
	replacement := "wolf"
	r := core.ReplaceRequest{Connection: c, Table: core.TableRef{Name: "safety"}, Column: &col, Term: "fox", Replacement: &replacement, Mode: "exact"}
	run := func(p core.ReplacePlan) (core.ReplaceResult, error) {
		return e.ReplaceExecute(ctx, core.ReplaceExecuteRequest{ReplaceRequest: r, Confirm: core.ReplaceConfirm{Action: "replace", TargetName: "safety", AffectedRows: p.AffectedRows}, ExpectedPlanSHA256: p.SHA256})
	}
	p, err := e.ReplacePlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	res, err := run(p)
	if err != nil || res.Outcome != "rejected" {
		t.Fatalf("constraint %#v %v", res, err)
	}
	searchDump(t, "rejected", res)
	if got := execute(t, e, c, "SELECT count(*) FROM safety").Results[0].Rows[0][0].Value; got != "2" {
		t.Fatal("REPLACE deleted neighbour")
	}
	replacement = "cat"
	execute(t, e, c, "CREATE TRIGGER bad AFTER UPDATE ON safety BEGIN UPDATE safety SET other='triggered' WHERE id=2; END")
	p, err = e.ReplacePlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	res, err = run(p)
	if err != nil || res.Outcome != "rejected" || res.Reason != "row_conflict" {
		t.Fatalf("trigger %#v %v", res, err)
	}
	if got := execute(t, e, c, "SELECT other FROM safety WHERE id=2").Results[0].Rows[0][0].Value; got != "two" {
		t.Fatal("trigger not rolled back")
	}
	execute(t, e, c, "DROP TRIGGER bad")
	// Two concurrent confirmations cannot both apply the original snapshot.
	p, err = e.ReplacePlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan core.ReplaceResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); res, _ := run(p); results <- res }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for res := range results {
		if res.Outcome == "completed" {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("success count %d", successes)
	}
	// Blob storage in a declared text column must not be searched/replaced.
	execute(t, e, c, "UPDATE safety SET value=X'666F78' WHERE id=1")
	request := core.SearchRequest{Connection: c, Targets: []core.SearchTarget{{Table: r.Table, Columns: []string{col}}}, Term: "fox", Mode: "substring"}
	found, err := e.Search(ctx, request)
	if err != nil || found.Tables[0].Count != 0 {
		t.Fatalf("runtime blob %#v %v", found, err)
	}
	p, err = e.ReplacePlan(ctx, r)
	if err != nil || p.AffectedRows != 0 {
		t.Fatalf("blob plan %#v %v", p, err)
	}
	// Deferred FK commit errors have an honest unknown outcome.
	execute(t, e, c, "CREATE TABLE parent (value TEXT PRIMARY KEY)")
	execute(t, e, c, "INSERT INTO parent VALUES ('fox')")
	execute(t, e, c, "CREATE TABLE child (id INTEGER PRIMARY KEY,value TEXT REFERENCES parent(value) DEFERRABLE INITIALLY DEFERRED)")
	execute(t, e, c, "INSERT INTO child VALUES (1,'fox')")
	r.Table.Name = "child"
	p, err = e.ReplacePlan(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	res, err = e.ReplaceExecute(ctx, core.ReplaceExecuteRequest{ReplaceRequest: r, Confirm: core.ReplaceConfirm{Action: "replace", TargetName: "child", AffectedRows: p.AffectedRows}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || res.Outcome != "outcome_unknown" {
		t.Fatalf("commit %#v %v", res, err)
	}
	searchDump(t, "unknown", res)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = e.Search(cancelled, request); err == nil {
		t.Fatal("cancellation ignored")
	}
	// A long read is canceled, even with a result row limit.
	execute(t, e, c, "CREATE TABLE slow (value TEXT)")
	execute(t, e, c, "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000) INSERT INTO slow SELECT 'x' FROM n")
	request.Targets = []core.SearchTarget{{Table: core.TableRef{Name: "slow"}, Columns: []string{"value"}}}
	request.Term = "missing"
	request.TimeoutMS = 1
	start := time.Now()
	_, err = e.Search(ctx, request)
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout %v", err)
	}
}

func TestSQLiteSearchBoundaries(t *testing.T) {
	ctx := context.Background()
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE boundary (id TEXT NOT NULL PRIMARY KEY, value TEXT, calc TEXT GENERATED ALWAYS AS (value||'!') STORED)")
	execute(t, e, c, "INSERT INTO boundary(id,value) VALUES ('a','fox'),('A','FOX')")
	col := "value"
	replacement := "cat"
	r := core.ReplaceRequest{Connection: c, Table: core.TableRef{Name: "boundary"}, Column: &col, Term: "fox", Replacement: &replacement, Mode: "substring"}
	p, err := e.ReplacePlan(ctx, r)
	if err != nil || p.AffectedRows != 1 {
		t.Fatalf("case %#v %v", p, err)
	}
	for _, column := range []string{"calc", "id"} {
		bad := r
		bad.Column = &column
		_, err = e.ReplacePlan(ctx, bad)
		requireMutationCode(t, err, "unsupported_search")
	}
	res, err := e.ReplaceExecute(ctx, core.ReplaceExecuteRequest{ReplaceRequest: r, Confirm: core.ReplaceConfirm{Action: "replace", TargetName: "boundary", AffectedRows: 1}, ExpectedPlanSHA256: p.SHA256})
	if err != nil || res.Outcome != "completed" {
		t.Fatalf("generated %#v %v", res, err)
	}
	r.Term = "FOX"
	replacement = ""
	p, err = e.ReplacePlan(ctx, r)
	if err != nil || p.AffectedRows != 1 || p.Examples[0].After.Kind != "text" || p.Examples[0].After.Value != "" {
		t.Fatalf("empty %#v %v", p, err)
	}
	r.Term = "none"
	p, err = e.ReplacePlan(ctx, r)
	if err != nil || p.AffectedRows != 0 {
		t.Fatalf("no-op %#v %v", p, err)
	}
	request := core.SearchRequest{Connection: c, Targets: []core.SearchTarget{{Table: r.Table, Columns: []string{col}}}, Mode: "substring", Term: "x"}
	for _, mutate := range []func(*core.SearchRequest){func(r *core.SearchRequest) { r.MaxRows = 1001 }, func(r *core.SearchRequest) { r.PreviewRows = 21 }, func(r *core.SearchRequest) { r.TimeoutMS = 60001 }, func(r *core.SearchRequest) { r.Term = "" }, func(r *core.SearchRequest) { r.Term = "\x00" }, func(r *core.SearchRequest) { r.Term = strings.Repeat("x", 4097) }, func(r *core.SearchRequest) { r.Targets = append(r.Targets, r.Targets[0]) }} {
		bad := request
		mutate(&bad)
		if _, err = e.Search(ctx, bad); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	// The total budget is shared by tables, with an explicit inexact zero after exhaustion.
	execute(t, e, c, "CREATE TABLE second (value TEXT)")
	execute(t, e, c, "INSERT INTO second VALUES ('x')")
	request.Term = "cat"
	request.MaxRows = 1
	request.Targets = append(request.Targets, core.SearchTarget{Table: core.TableRef{Name: "second"}, Columns: []string{"value"}})
	execute(t, e, c, "UPDATE second SET value='cat'")
	got, err := e.Search(ctx, request)
	if err != nil || got.Tables[1].Count != 0 || got.Tables[1].CountExact {
		t.Fatalf("shared budget %#v %v", got, err)
	}
	// Preview truncation happens inside SQL; large cells are not transferred.
	execute(t, e, c, "UPDATE second SET value=replace(hex(zeroblob(50000)),'0','x')")
	request.Targets = request.Targets[1:]
	request.Term = "x"
	got, err = e.Search(ctx, request)
	if err != nil || len(got.Tables[0].Rows[0][0].Value) != 256 {
		t.Fatalf("large preview %#v %v", got, err)
	}
	execute(t, e, c, "CREATE VIEW boundary_view AS SELECT * FROM boundary")
	request.Targets[0].Table.Name = "boundary_view"
	if _, err = e.Search(ctx, request); err == nil {
		t.Fatal("view searched")
	}
	execute(t, e, c, "CREATE TABLE no_key (value TEXT)")
	r.Table.Name = "no_key"
	_, err = e.ReplacePlan(ctx, r)
	requireMutationCode(t, err, "unsupported_search")
	r.Table.Name = "second"
	r.Term = "x"
	_, err = e.ReplacePlan(ctx, r)
	if err == nil {
		t.Fatal("large no-key replacement accepted")
	}
}

func liveSearchLock(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	db, closeDB, err := e.open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer closeDB()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s := &mutationSession{conn: conn, driver: c.Driver}
	if err = s.begin(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.rollback()
	name := quoteIdentifier(c.Driver, "p14 odd'`\" --")
	rows, err := s.QueryContext(ctx, "SELECT id FROM "+name+" WHERE id=1 FOR UPDATE")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	col := "value'`\""
	replacement := "cat"
	r := core.ReplaceRequest{Connection: c, Table: core.TableRef{Name: "p14 odd'`\" --"}, Column: &col, Term: "wolf", Replacement: &replacement, Mode: "substring", TimeoutMS: 50}
	start := time.Now()
	_, err = e.ReplacePlan(ctx, r)
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("held row lock deadline: %v", err)
	}
}

func liveSearchRestrictions(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	col := "value"
	replacement := "cat"
	name := "p14_restricted"
	ddl := "CREATE TABLE " + name + " (id INTEGER NOT NULL PRIMARY KEY,value TEXT)"
	if c.Driver == "mysql" {
		ddl += " ENGINE=MyISAM"
	}
	execute(t, e, c, ddl)
	if c.Driver == "postgres" {
		execute(t, e, c, "CREATE TABLE p14_inherited () INHERITS (p14_restricted)")
	}
	r := core.ReplaceRequest{Connection: c, Table: core.TableRef{Name: name}, Column: &col, Term: "fox", Replacement: &replacement, Mode: "substring"}
	_, err := e.ReplacePlan(context.Background(), r)
	requireMutationCode(t, err, "unsupported_search")
	r.Table.Name = "p14 odd'`\" --"
	col = "value'`\""
	r.Mode = "regex"
	r.Term = "["
	_, err = e.ReplacePlan(context.Background(), r)
	if err == nil || strings.Contains(err.Error(), "REGEXP") || strings.Contains(err.Error(), "syntax") {
		t.Fatalf("raw regex error: %v", err)
	}
}

func TestSearchBrowseSQL(t *testing.T) {
	// SQL keywords and PostgreSQL parameter tokens inside quoted catalog names
	// must survive predicate insertion and pagination binding unchanged.
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			column := "value ORDER BY $1 LIMIT $2 "
			d := core.TableDescription{Table: core.TableRef{Schema: "schema$2", Name: "table LIMIT $1 ORDER BY "}, Columns: []core.TableColumn{{Name: column}, {Name: "id"}}, PrimaryKey: []string{"id"}}
			r := core.BrowseRequest{Connection: core.Connection{Driver: driver}, PageSize: 7, Offset: 3, Sort: []core.TableSort{{Column: column, Direction: "desc"}}, Filters: []core.TableFilter{{Column: column, Operator: "contains", Value: &core.Cell{Kind: "text", Value: "%_!"}}, {Column: "id", Operator: "gte", Value: &core.Cell{Kind: "integer", Value: "2"}}}}
			query, args, err := searchBrowseSQL(r, d)
			if err != nil {
				t.Fatal(err)
			}
			name := quoteIdentifier(driver, column)
			p := []string{"?", "?", "?", "?"}
			if driver == "postgres" {
				p = []string{"$1", "$2", "$3", "$4"}
			}
			want := "SELECT " + name + "," + quoteIdentifier(driver, "id") + " FROM " + searchTable(d, driver) + " WHERE " + name + " LIKE " + p[0] + " ESCAPE '!' AND " + quoteIdentifier(driver, "id") + " >= " + p[1] + " ORDER BY " + name + " DESC," + quoteIdentifier(driver, "id")
			if driver == "sqlite" {
				want += " COLLATE BINARY"
			}
			want += " ASC LIMIT " + p[2] + " OFFSET " + p[3]
			if query != want || !reflect.DeepEqual(args, []any{"%!%!_!!%", int64(2), int64(8), int64(3)}) {
				t.Fatalf("query %s args %#v; want %s", query, args, want)
			}
		})
	}
}

func TestSQLiteSearchBrowseQuotedKeywords(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	name, column := "table ORDER BY LIMIT $1 ", "value LIMIT $2 ORDER BY "
	q := func(s string) string { return quoteIdentifier(c.Driver, s) }
	execute(t, e, c, "CREATE TABLE "+q(name)+" (id INTEGER PRIMARY KEY,"+q(column)+" TEXT)")
	execute(t, e, c, "INSERT INTO "+q(name)+" VALUES (1,'fox'),(2,'cat')")
	r := core.BrowseRequest{Connection: c, Table: core.TableRef{Name: name}, Filters: []core.TableFilter{{Column: column, Operator: "contains", Value: &core.Cell{Kind: "text", Value: "fox"}}}}
	result, err := e.SearchBrowse(context.Background(), r)
	if err != nil || len(result.Result.Rows) != 1 || result.Result.Rows[0][0].Value != "1" {
		t.Fatalf("quoted catalog names %#v %v", result, err)
	}
}
