// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func findStructureIndex(t *testing.T, result core.TableStructure, name string) core.StructureIndex {
	t.Helper()
	for _, index := range result.Indexes {
		if index.Name == name {
			return index
		}
	}
	t.Fatalf("index %q missing in %#v", name, result.Indexes)
	return core.StructureIndex{}
}
func findStructureConstraint(t *testing.T, result core.TableStructure, kind string) core.StructureConstraint {
	t.Helper()
	for _, constraint := range result.Constraints {
		if constraint.Kind == kind {
			return constraint
		}
	}
	t.Fatalf("constraint kind %q missing", kind)
	return core.StructureConstraint{}
}
func TestSQLiteStructure(t *testing.T) { structureCases(t, testEngine(t), sqliteConnection()) }
func structureCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	q := func(v string) string { return quoteIdentifier(c.Driver, v) }
	name := "structure\"odd'`"
	ref := core.TableRef{Name: name}
	execute(t, e, c, "CREATE TABLE b3a_parent(a INTEGER NOT NULL,b INTEGER NOT NULL,PRIMARY KEY(a,b))")
	execute(t, e, c, "CREATE TABLE "+q(name)+"(a INTEGER NOT NULL,b INTEGER NOT NULL,value VARCHAR(80),peer INTEGER,PRIMARY KEY(b,a),CONSTRAINT b3a_unique UNIQUE(value),CONSTRAINT b3a_foreign FOREIGN KEY(a,b) REFERENCES b3a_parent(a,b) ON DELETE CASCADE ON UPDATE RESTRICT,CONSTRAINT b3a_check CHECK(a>0))")
	execute(t, e, c, "CREATE INDEX b3a_order ON "+q(name)+"(value DESC,a ASC)")
	execute(t, e, c, "CREATE INDEX b3a_primary_like ON "+q(name)+"(peer)")
	if c.Driver == "sqlite" || c.Driver == "postgres" {
		execute(t, e, c, "CREATE INDEX b3a_expression ON "+q(name)+"(lower(value)) WHERE value IS NOT NULL")
	}
	if c.Driver == "postgres" {
		execute(t, e, c, "CREATE INDEX b3a_include ON "+q(name)+"(value) INCLUDE(peer)")
	}
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE INDEX b3a_prefix ON "+q(name)+"(value(3))")
	}
	result, err := e.Structure(ctx, core.DescribeRequest{Connection: c, Table: ref})
	if err != nil {
		t.Fatal(err)
	}
	if result.Table.Name != name || result.Coverage.Indexes != "catalog" {
		t.Fatalf("structure %#v", result)
	}
	primary := findStructureConstraint(t, result, "primary_key")
	if !primary.ColumnsKnown || strings.Join(primary.Columns, ",") != "b,a" {
		t.Fatalf("PK order %#v", primary)
	}
	foreign := findStructureConstraint(t, result, "foreign_key")
	if foreign.ReferencedTable == nil || foreign.ReferencedTable.Name != "b3a_parent" || strings.Join(foreign.Columns, ",") != "a,b" || !foreign.ReferencedColumnsKnown || strings.Join(foreign.ReferencedColumns, ",") != "a,b" || foreign.OnDelete != "cascade" || foreign.OnUpdate != "restrict" {
		t.Fatalf("FK %#v", foreign)
	}
	ordered := findStructureIndex(t, result, "b3a_order")
	if len(ordered.Terms) < 2 || ordered.Terms[0].Direction != "desc" || ordered.Terms[0].Column == nil || *ordered.Terms[0].Column != "value" || ordered.Terms[1].Direction != "asc" {
		t.Fatalf("index order %#v", ordered)
	}
	if findStructureIndex(t, result, "b3a_primary_like").Primary != "no" {
		t.Fatal("PK inferred from index name")
	}
	unique := findStructureConstraint(t, result, "unique")
	if !unique.ColumnsKnown || strings.Join(unique.Columns, ",") != "value" {
		t.Fatalf("unique %#v", unique)
	}
	if c.Driver == "sqlite" {
		if result.Coverage.Checks != "unavailable" || primary.NameKnown || foreign.NameKnown || unique.NameKnown || foreign.Deferrable != "unknown" {
			t.Fatal("invented SQLite metadata")
		}
	} else {
		check := findStructureConstraint(t, result, "check")
		if !check.NameKnown || check.Name != "b3a_check" || result.Coverage.Checks != "catalog" {
			t.Fatalf("check %#v", check)
		}
	}
	if c.Driver == "sqlite" || c.Driver == "postgres" {
		index := findStructureIndex(t, result, "b3a_expression")
		if index.Partial != "yes" || index.Terms[0].Kind != "expression" || index.Terms[0].Column != nil {
			t.Fatalf("expression %#v", index)
		}
	}
	if c.Driver == "postgres" {
		index := findStructureIndex(t, result, "b3a_include")
		if len(index.Terms) != 2 || index.Terms[1].Role != "include" || index.Terms[1].Direction != "not_applicable" {
			t.Fatalf("include %#v", index)
		}
	}
	if c.Driver == "mysql" {
		index := findStructureIndex(t, result, "b3a_prefix")
		if index.Terms[0].PrefixLength == nil || *index.Terms[0].PrefixLength != 3 || index.Terms[0].PrefixState != "present" {
			t.Fatalf("prefix %#v", index)
		}
	}
	execute(t, e, c, "CREATE TABLE b3a_plain(value INTEGER)")
	empty, err := e.Structure(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b3a_plain"}})
	if err != nil || len(empty.Indexes) != 0 {
		t.Fatalf("plain %#v %v", empty, err)
	}
	execute(t, e, c, "CREATE VIEW b3a_view AS SELECT value FROM b3a_plain")
	_, err = e.Structure(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b3a_view"}})
	requireMutationCode(t, err, "unsupported_table")
	_, err = e.Structure(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "'; DROP TABLE b3a_plain;--"}})
	requireMutationCode(t, err, "table_not_found")
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = e.Structure(canceled, core.DescribeRequest{Connection: c, Table: ref})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("structure cancellation %v", err)
	}
}
func TestSQLiteStructureEmptyNamesAndImplicitFK(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE "other.parent"("" INTEGER PRIMARY KEY)`)
	execute(t, e, c, `CREATE TABLE b3a_empty("" INTEGER PRIMARY KEY,peer INTEGER,UNIQUE(peer),FOREIGN KEY(peer) REFERENCES "other.parent"(""))`)
	result, err := e.Structure(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b3a_empty"}})
	if err != nil {
		t.Fatal(err)
	}
	pk := findStructureConstraint(t, result, "primary_key")
	if pk.NameKnown || len(pk.Columns) != 1 || pk.Columns[0] != "" {
		t.Fatalf("empty PK %#v", pk)
	}
	for _, index := range result.Indexes {
		if index.Primary == "yes" {
			t.Fatal("invented rowid PK index")
		}
	}
	fk := findStructureConstraint(t, result, "foreign_key")
	if fk.ReferencedTable.Schema != "main" || fk.ReferencedTable.Name != "other.parent" || !fk.ReferencedColumnsKnown || len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "" {
		t.Fatalf("empty FK %#v", fk)
	}
	execute(t, e, c, `CREATE TABLE b3a_implicit(id INTEGER PRIMARY KEY,peer INTEGER REFERENCES b3a_implicit)`)
	result, err = e.Structure(ctx, core.DescribeRequest{Connection: c, Table: core.TableRef{Name: "b3a_implicit"}})
	if err != nil {
		t.Fatal(err)
	}
	fk = findStructureConstraint(t, result, "foreign_key")
	if fk.ReferencedTable.Name != "b3a_implicit" || fk.ReferencedColumnsKnown || len(fk.ReferencedColumns) != 0 {
		t.Fatalf("implicit self FK %#v", fk)
	}
}
func TestStructureBudget(t *testing.T) {
	base := core.TableStructure{Indexes: []core.StructureIndex{}, Constraints: []core.StructureConstraint{}}
	objects := base
	objects.Indexes = make([]core.StructureIndex, MaxStructureObjects+1)
	requireMutationCode(t, structureBudget(objects), "result_too_large")
	escaped := base
	for range 3 {
		escaped.Indexes = append(escaped.Indexes, core.StructureIndex{Name: strings.Repeat("\x01", 600000)})
	}
	requireMutationCode(t, structureBudget(escaped), "result_too_large")
	if err := structureBudget(base); err != nil {
		t.Fatal(err)
	}
}
func TestLiveStructure(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("B3a disposable TLS fixture required")
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
			structureCases(t, e, c)
		})
	}
}
