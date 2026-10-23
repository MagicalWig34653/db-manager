// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func createFixture(c core.Connection, name string) core.CreateRequest {
	text := core.Cell{Kind: "text", Value: "O'Reilly \\ '; DROP TABLE untouched;--"}
	decimal := core.Cell{Kind: "decimal", Value: "1234567890123456.1200"}
	counter := core.Cell{Kind: "integer", Value: "9223372036854775807"}
	return core.CreateRequest{Connection: c, Table: core.TableRef{Name: name}, Confirm: true, PrimaryKey: []string{"part", "id"}, Columns: []core.CreateColumn{
		{Name: "id", Type: core.CreateType{Kind: "int64"}, Default: core.CreateDefault{Mode: "none"}},
		{Name: "part", Type: core.CreateType{Kind: "text", Length: 80}, Default: core.CreateDefault{Mode: "none"}},
		{Name: "odd\"column", Type: core.CreateType{Kind: "text", Length: 100}, Nullable: true, Default: core.CreateDefault{Mode: "value", Value: &text}},
		{Name: "amount", Type: core.CreateType{Kind: "decimal", Precision: 20, Scale: 4}, Nullable: true, Default: core.CreateDefault{Mode: "value", Value: &decimal}},
		{Name: "payload", Type: core.CreateType{Kind: "binary"}, Nullable: true, Default: core.CreateDefault{Mode: "null"}},
		{Name: "optional", Type: core.CreateType{Kind: "text", Length: 20}, Nullable: true, Default: core.CreateDefault{Mode: "none"}},
		{Name: "counter", Type: core.CreateType{Kind: "int64"}, Default: core.CreateDefault{Mode: "value", Value: &counter}},
	}}
}
func createCases(t *testing.T, e *Engine, c core.Connection, maria bool) {
	t.Helper()
	name := "b4a\";DROP TABLE untouched;--'`"
	r := createFixture(c, name)
	execute(t, e, c, "CREATE TABLE untouched(id INTEGER)")
	result, err := e.CreateTable(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "created" || result.ActualDescription == nil {
		t.Fatalf("create result %#v", result)
	}
	expected := "transactional"
	if c.Driver == "mysql" {
		expected = "observed"
	}
	if result.StructureCheck != expected || result.SnapshotAtomic != (c.Driver != "mysql") {
		encoded, _ := json.Marshal(result)
		t.Fatalf("verification %s", encoded)
	}
	ref, _ := createRef(c, r.Table)
	r.Table = ref
	if !createMatches(r, *result.ActualDescription, maria) {
		t.Fatalf("actual metadata %#v", result.ActualDescription)
	}
	q := func(s string) string { return quoteIdentifier(c.Driver, s) }
	execute(t, e, c, "INSERT INTO "+q(ref.Schema)+"."+q(name)+"("+q("id")+","+q("part")+") VALUES(7,'key')")
	data, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT " + q("odd\"column") + "," + q("payload") + "," + q("optional") + " FROM " + q(ref.Schema) + "." + q(name)})
	if err != nil || len(data.Results) != 1 || len(data.Results[0].Rows) != 1 || data.Results[0].Rows[0][0].Value != r.Columns[2].Default.Value.Value || data.Results[0].Rows[0][1].Kind != "null" {
		t.Fatalf("literal defaults %#v %v", data, err)
	}
	binarySQL := "X'00FF'"
	if c.Driver == "postgres" {
		binarySQL = "decode('00ff','hex')"
	}
	execute(t, e, c, "INSERT INTO "+q(ref.Schema)+"."+q(name)+"("+q("id")+","+q("part")+","+q("payload")+") VALUES(8,'binary',"+binarySQL+")")
	stored, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT " + q("counter") + "," + q("payload") + " FROM " + q(ref.Schema) + "." + q(name) + " WHERE " + q("id") + "=8"})
	if err != nil || len(stored.Results) != 1 || len(stored.Results[0].Rows) != 1 || stored.Results[0].Rows[0][0] != (core.Cell{Kind: "integer", Value: "9223372036854775807"}) || stored.Results[0].Rows[0][1] != (core.Cell{Kind: "binary", Value: "AP8="}) {
		t.Fatalf("created type/default storage %#v %v", stored, err)
	}
	_, err = e.CreateTable(context.Background(), r)
	requireMutationCode(t, err, "create_conflict")
	if _, err = e.Describe(context.Background(), core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "untouched"}}); err != nil {
		t.Fatal("default executed SQL", err)
	}
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: ref})
	if err != nil || len(page.Result.Rows) != 2 {
		t.Fatalf("existing table changed %#v %v", page, err)
	}
	plain := createFixture(c, "b4a_plain")
	plain.PrimaryKey = []string{}
	plain.Columns = plain.Columns[:1]
	if _, err = e.CreateTable(context.Background(), plain); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = e.CreateTable(ctx, createFixture(c, "b4a_cancel"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
}
func TestSQLiteCreate(t *testing.T) {
	e, _ := New(t.TempDir(), nil)
	createCases(t, e, core.Connection{Driver: "sqlite", Database: "create"}, false)
}
func TestLiveCreate(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("B4a disposable TLS fixture required")
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
			createCases(t, e, c, test.name == "maria")
			liveCreateBoundaries(t, e, c)
			liveCreateSchemaAndObservation(t, e, c)
		})
	}
}
func TestCreateValidation(t *testing.T) {
	c := core.Connection{Driver: "sqlite", Database: "create"}
	for _, change := range []func(*core.CreateRequest){
		func(r *core.CreateRequest) { r.Confirm = false }, func(r *core.CreateRequest) { r.Columns = nil }, func(r *core.CreateRequest) { r.Columns[0].Type.Kind = "BIGINT); DROP TABLE x;--" },
		func(r *core.CreateRequest) { r.Columns[1].Name = r.Columns[0].Name }, func(r *core.CreateRequest) { r.PrimaryKey = []string{"missing"} }, func(r *core.CreateRequest) { r.Columns[0].Nullable = true },
		func(r *core.CreateRequest) { r.Table.Schema = "temp" }, func(r *core.CreateRequest) { r.Table.Name = "sqlite_system" }, func(r *core.CreateRequest) { r.Columns[2].Type.Length = 1025 },
		func(r *core.CreateRequest) { r.Columns[2].Default.Value = &core.Cell{Kind: "text", Value: "a\x00b"} }, func(r *core.CreateRequest) {
			r.Columns[3].Default.Value = &core.Cell{Kind: "decimal", Value: "1.2);DROP TABLE x"}
		},
		func(r *core.CreateRequest) {
			r.Columns[0].Default = core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "integer", Value: "9223372036854775808"}}
		},
	} {
		r := createFixture(c, "test")
		change(&r)
		if _, err := createSQL(r); err == nil {
			t.Fatalf("accepted invalid request %#v", r)
		}
	}
}

type createFault struct {
	execErr, observeErr, commitErr error
	description                    core.TableDescription
	execs, commits                 int
}

func (s *createFault) execCreate(context.Context, string) error { s.execs++; return s.execErr }
func (s *createFault) observeCreate(context.Context, core.CreateRequest) (core.TableDescription, error) {
	return s.description, s.observeErr
}
func (s *createFault) commitCreate(context.Context) error { s.commits++; return s.commitErr }
func TestCreateOutcomeBoundaries(t *testing.T) {
	for _, driver := range []string{"mysql", "sqlite", "postgres"} {
		r := createFixture(core.Connection{Driver: driver, Database: "probe"}, "new")
		r.Table, _ = createRef(r.Connection, r.Table)
		fault := &createFault{observeErr: errors.New("read failed")}
		result, err := executeCreate(context.Background(), r, "CREATE", fault, false)
		if driver == "mysql" {
			if err != nil || result.Outcome != "created" || result.StructureCheck != "unavailable" {
				t.Fatalf("known ack lost %#v %v", result, err)
			}
		} else if err == nil {
			t.Fatal("transaction read failure returned success")
		}
		if fault.execs != 1 || fault.commits != 0 {
			t.Fatal("unexpected retry/commit")
		}
	}
	r := createFixture(core.Connection{Driver: "mysql", Database: "probe"}, "new")
	fault := &createFault{execErr: errors.New("response lost")}
	_, err := executeCreate(context.Background(), r, "CREATE", fault, false)
	requireMutationCode(t, err, "outcome_unknown")
	if fault.execs != 1 {
		t.Fatal("retried unknown CREATE")
	}
	// A confirmed CREATE can be followed by a different separately observed object.
	fault = &createFault{}
	result, err := executeCreate(context.Background(), r, "CREATE", fault, false)
	if err != nil || result.StructureCheck != "different" || result.Outcome != "created" {
		t.Fatalf("different observation %#v %v", result, err)
	}
}

func TestCreateCommitFailureAndOversizeObservation(t *testing.T) {
	r := createFixture(core.Connection{Driver: "sqlite", Database: "create"}, "new")
	r.Table, _ = createRef(r.Connection, r.Table)
	d := core.TableDescription{Table: r.Table, Kind: "table", PrimaryKey: r.PrimaryKey}
	for _, column := range r.Columns {
		typ, _ := createTypeSQL("sqlite", column.Type)
		literal, _ := createDefaultSQL(column)
		actual := core.TableColumn{Name: column.Name, DatabaseType: typ, Nullable: column.Nullable, Generated: "none"}
		if column.Default.Mode != "none" {
			actual.DefaultValue = &literal
		}
		d.Columns = append(d.Columns, actual)
	}
	fault := &createFault{description: d, commitErr: errors.New("commit response lost")}
	_, err := executeCreate(context.Background(), r, "CREATE", fault, false)
	requireMutationCode(t, err, "outcome_unknown")
	if fault.commits != 1 || fault.execs != 1 {
		t.Fatal("commit not attempted exactly once")
	}
	r.Connection.Driver = "mysql"
	d.Columns[0].DatabaseType = strings.Repeat("x", MaxResponseBytes)
	fault = &createFault{description: d}
	result, err := executeCreate(context.Background(), r, "CREATE", fault, false)
	if err != nil || result.Outcome != "created" || result.StructureCheck != "unavailable" || result.ActualDescription != nil {
		t.Fatalf("oversize after ack %#v %v", result, err)
	}
}

func TestSQLiteCreateBusyAndDefinitionBounds(t *testing.T) {
	e, _ := New(t.TempDir(), nil)
	c := core.Connection{Driver: "sqlite", Database: "busy_create"}
	opened, cleanup, err := e.open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if err = opened.PingContext(context.Background()); err != nil {
		cleanup()
		t.Fatal(err)
	}
	_, err = e.CreateTable(context.Background(), createFixture(c, "blocked"))
	requireMutationCode(t, err, "database_busy")
	cleanup()
	if _, err = e.CreateTable(context.Background(), createFixture(c, "released")); err != nil {
		t.Fatal("busy connection not released", err)
	}
	r := createFixture(c, "many")
	r.Columns = nil
	r.PrimaryKey = []string{}
	for i := 0; i < MaxCreateColumns+1; i++ {
		r.Columns = append(r.Columns, core.CreateColumn{Name: "c" + strconv.Itoa(i), Type: core.CreateType{Kind: "int64"}, Default: core.CreateDefault{Mode: "none"}})
	}
	if _, err = createSQL(r); err == nil {
		t.Fatal("column bound ignored")
	}
	r = createFixture(c, "types")
	r.Columns[2].Type = core.CreateType{Kind: "decimal", Precision: 4, Scale: 5}
	if _, err = createSQL(r); err == nil {
		t.Fatal("invalid decimal scale accepted")
	}
	column := core.CreateColumn{Type: core.CreateType{Kind: "decimal", Precision: 5, Scale: 2}, Default: core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "decimal", Value: "0.5"}}}
	expression := "1/2"
	if createDefaultMatches("postgres", column, &expression, false) {
		t.Fatal("catalog expression treated as constant")
	}
}
