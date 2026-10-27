// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func dumpIndexes(t *testing.T, component string, value any) {
	t.Helper()
	dir := os.Getenv("DBM_INDEXES_DUMP")
	if dir == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	name := strings.ReplaceAll(t.Name(), "/", "_") + "-" + strconv.Itoa(len(files)) + "." + component + ".json"
	if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func indexCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	q := func(n string) string { return columnQuote(c.Driver, n) }
	dropTable(t, e, c, "neighbor")
	execute(t, e, c, "CREATE TABLE neighbor (id BIGINT)")
	execute(t, e, c, "INSERT INTO neighbor VALUES (7)")
	name := "keys \"` ;DROP TABLE neighbor"
	execute(t, e, c, "CREATE TABLE "+q(name)+" (id BIGINT NOT NULL, value VARCHAR(40), other BIGINT)")
	execute(t, e, c, "INSERT INTO "+q(name)+" VALUES (1,'a',1),(2,'a',NULL),(3,NULL,1)")
	readStructure := func(table core.TableRef) core.TableStructure {
		t.Helper()
		got, err := e.Structure(ctx, core.DescribeRequest{Connection: c, Table: table})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	r := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: name}, Action: "index_create", Index: &core.IndexDefinition{Name: "idx \"` test", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "value", Direction: "desc"}}}}
	plan := func(r core.IndexesRequest) core.IndexesPlan {
		t.Helper()
		p, err := e.PlanIndexes(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		dumpIndexes(t, "IndexesPlan", p)
		return p
	}
	run := func(r core.IndexesRequest, p core.IndexesPlan) core.IndexesResult {
		t.Helper()
		r.Confirm = &dbplan.Confirmation{Action: r.Action, TargetName: p.TargetName}
		r.ExpectedPlanSHA256 = p.SHA256
		result, err := e.ExecuteIndexes(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		dumpIndexes(t, "IndexesResult", result)
		return result
	}
	p := plan(r)
	p2 := plan(r)
	if p.SHA256 != p2.SHA256 {
		t.Fatal("unstable plan")
	}
	r.Confirm = &dbplan.Confirmation{Action: r.Action}
	r.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	_, err := e.ExecuteIndexes(ctx, r)
	requireMutationCode(t, err, "plan_mismatch")
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm.Action = "index_drop"
	_, err = e.ExecuteIndexes(ctx, r)
	requireMutationCode(t, err, "confirmation_mismatch")
	if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
		t.Fatalf("create %#v", result)
	}
	// Read the existing v5 interface independently.
	structure, err := e.Structure(ctx, core.DescribeRequest{Connection: c, Table: r.Table})
	if err != nil {
		t.Fatal(err)
	}
	ix := findStructureIndex(t, structure, r.Index.Name)
	if ix.Terms[0].Direction != "desc" {
		t.Fatal(ix)
	}
	r.Confirm = nil
	r.ExpectedPlanSHA256 = ""
	r.Action = "index_drop"
	r.Name = r.Index.Name
	r.Index = nil
	p = plan(r)
	r.Confirm = &dbplan.Confirmation{Action: r.Action, TargetName: "neighbor"}
	r.ExpectedPlanSHA256 = p.SHA256
	_, err = e.ExecuteIndexes(ctx, r)
	requireMutationCode(t, err, "confirmation_mismatch")
	if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
		t.Fatalf("drop %#v", result)
	}
	r.Action = "index_create"
	r.Name = ""
	r.Index = &core.IndexDefinition{Name: "duplicate_unique", Kind: "unique", Terms: []core.IndexDefinitionTerm{{Column: "value"}}}
	p = plan(r)
	if p.DuplicateGroups == nil || *p.DuplicateGroups != 1 {
		t.Fatal("duplicate groups", p)
	}
	if result := run(r, p); result.Outcome != dbplan.StatusRejected || result.Reason != "invalid_value" {
		t.Fatal(result)
	}
	r.Index.Name = "unique_id"
	r.Index.Terms[0].Column = "id"
	p = plan(r)
	if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
		t.Fatal(result)
	}
	if c.Driver == "sqlite" {

		for _, bad := range []core.IndexesRequest{
			{Connection: c, Table: r.Table, Action: "primary_drop"},
			{Connection: c, Table: r.Table, Action: "index_alter", Name: "unique_id", Index: r.Index},
			{Connection: c, Table: r.Table, Action: "foreign_key_create", ForeignKey: &core.ForeignKeyDefinition{Name: "fk", Columns: []string{"id"}, ReferencedTable: r.Table, ReferencedColumns: []string{"id"}, OnDelete: "restrict", OnUpdate: "restrict"}},
		} {
			_, err = e.PlanIndexes(ctx, bad)
			requireMutationCode(t, err, "unsupported_action")
		}

	} else {

		identitySQL := "CREATE TABLE identity_keys (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY)"
		if c.Driver == "mysql" {
			identitySQL = "CREATE TABLE identity_keys (id BIGINT AUTO_INCREMENT PRIMARY KEY)"
		}
		execute(t, e, c, identitySQL)
		identityIndex := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: "identity_keys"}, Action: "index_create", Index: &core.IndexDefinition{Name: "identity_secondary", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}}
		if result := run(identityIndex, plan(identityIndex)); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		replacement := core.IndexesRequest{Connection: c, Table: r.Table, Action: "index_alter", Name: "unique_id", Index: &core.IndexDefinition{Name: "rejected_replacement", Kind: "unique", Terms: []core.IndexDefinitionTerm{{Column: "value"}}}}
		if result := run(replacement, plan(replacement)); result.Outcome != dbplan.StatusRejected {
			t.Fatal(result)
		}
		findStructureIndex(t, readStructure(r.Table), "unique_id")
		r.Action = "index_alter"
		r.Name = "unique_id"
		r.Index = &core.IndexDefinition{Name: "changed_id", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "id"}, {Column: "other"}}}
		p = plan(r)
		if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		altered := findStructureIndex(t, readStructure(r.Table), "changed_id")
		if len(altered.Terms) != 2 || altered.Terms[0].Column == nil || *altered.Terms[0].Column != "id" || altered.Terms[1].Column == nil || *altered.Terms[1].Column != "other" {
			t.Fatal(altered)
		}
		r.Action = "index_rename"
		r.Index = nil
		r.Name = "changed_id"
		r.NewName = "renamed_id"
		p = plan(r)
		if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		renamed := readStructure(r.Table)
		findStructureIndex(t, renamed, "renamed_id")
		for _, ix := range renamed.Indexes {
			if ix.Name == "changed_id" {
				t.Fatal("old name remains")
			}
		}
		r.Action = "primary_set"
		r.Name = ""
		r.NewName = ""
		r.Index = &core.IndexDefinition{Name: "pk_fixture", Kind: "primary", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}
		p = plan(r)
		if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		// Existing NULLs block a PK replacement before any DROP is sent.
		invalidPK := r
		invalidDefinition := *r.Index
		invalidDefinition.Terms = []core.IndexDefinitionTerm{{Column: "other"}}
		invalidPK.Index = &invalidDefinition
		invalidPlan := plan(invalidPK)
		if !invalidPlan.ContainsNull {
			t.Fatal("missing NULL warning")
		}
		if result := run(invalidPK, invalidPlan); result.Outcome != dbplan.StatusRejected {
			t.Fatal(result)
		}
		// Changing the PK is a replacement of its catalog-resolved old constraint.
		r.Index.Name = "pk_changed"
		p = plan(r)
		if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}
		alternate := core.IndexesRequest{Connection: c, Table: r.Table, Action: "index_drop", Name: "renamed_id"}
		if result := run(alternate, plan(alternate)); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		execute(t, e, c, "CREATE TABLE child_keys (parent_id BIGINT, text_id VARCHAR(10))")
		fk := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: "child_keys"}, Action: "foreign_key_create", ForeignKey: &core.ForeignKeyDefinition{Name: "fk \"` keys", Columns: []string{"parent_id"}, ReferencedTable: r.Table, ReferencedColumns: []string{"id"}, OnDelete: "cascade", OnUpdate: "restrict"}}
		fp := plan(fk)
		if result := run(fk, fp); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}
		structure, err = e.Structure(ctx, core.DescribeRequest{Connection: c, Table: fk.Table})
		if err != nil {
			t.Fatal(err)
		}

		constraint := findStructureConstraint(t, structure, "foreign_key")
		if constraint.Name != fk.ForeignKey.Name || constraint.OnDelete != "cascade" || constraint.OnUpdate != "restrict" || len(constraint.Columns) != 1 || constraint.Columns[0] != "parent_id" {
			t.Fatal(constraint)
		}
		drop := r
		drop.Action = "primary_drop"
		drop.Index = nil
		p = plan(drop)
		if len(p.Dependencies) == 0 {
			t.Fatal("missing FK dependency")
		}
		if result := run(drop, p); result.Outcome != dbplan.StatusRejected {
			t.Fatal("FK should block", result)
		}
		fk.Action = "foreign_key_alter"
		fk.Name = fk.ForeignKey.Name
		fk.ForeignKey.Name = "fk_replaced"
		fk.ForeignKey.OnDelete = "set_null"
		fp = plan(fk)
		if result := run(fk, fp); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		replaced := findStructureConstraint(t, readStructure(fk.Table), "foreign_key")
		if replaced.Name != "fk_replaced" || replaced.OnDelete != "set_null" {
			t.Fatal(replaced)
		}
		fk.Action = "foreign_key_drop"
		fk.Name = fk.ForeignKey.Name
		fk.ForeignKey = nil
		fp = plan(fk)
		if result := run(fk, fp); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		for _, con := range readStructure(fk.Table).Constraints {
			if con.Kind == "foreign_key" {
				t.Fatal("foreign key remains")
			}
		}
		if result := run(drop, plan(drop)); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}
		bad := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: "child_keys"}, Action: "foreign_key_create", ForeignKey: &core.ForeignKeyDefinition{Name: "bad_type", Columns: []string{"text_id"}, ReferencedTable: r.Table, ReferencedColumns: []string{"id"}, OnDelete: "restrict", OnUpdate: "restrict"}}
		_, err = e.PlanIndexes(ctx, bad)
		requireMutationCode(t, err, "invalid_indexes")
		r.Action = "index_create"
		r.Name = ""
		r.Index = &core.IndexDefinition{Name: "dialect_index", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "value"}}}
		if c.Driver == "mysql" {
			r.Index.Terms[0].PrefixLength = 8
		} else {
			r.Index.Terms[0].Expression = "lower"
			r.Index.Predicate = &core.IndexPredicate{Column: "value", Operator: "is_not_null"}
		}
		p = plan(r)
		if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
			t.Fatal(result)
		}

		dialect := findStructureIndex(t, readStructure(r.Table), "dialect_index")
		if c.Driver == "mysql" {
			if dialect.Terms[0].PrefixLength == nil || *dialect.Terms[0].PrefixLength != 8 {
				t.Fatal(dialect)
			}
		} else if dialect.Partial != "yes" || dialect.Terms[0].Kind != "expression" {
			t.Fatal(dialect)
		}
		if c.Driver == "mysql" {
			r.Index = &core.IndexDefinition{Name: "fulltext_index", Kind: "fulltext", Terms: []core.IndexDefinitionTerm{{Column: "value"}}}
			p = plan(r)
			if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
				t.Fatal(result)
			}

			fulltext := findStructureIndex(t, readStructure(r.Table), "fulltext_index")
			if !strings.EqualFold(fulltext.Method, "FULLTEXT") {
				t.Fatal(fulltext)
			}
			execute(t, e, c, "CREATE TABLE spatial_keys (point_value POINT NOT NULL)")
			r.Table = core.TableRef{Name: "spatial_keys"}
			r.Index = &core.IndexDefinition{Name: "spatial_index", Kind: "spatial", Terms: []core.IndexDefinitionTerm{{Column: "point_value"}}}
			p = plan(r)
			if result := run(r, p); result.Outcome != dbplan.StatusCompleted {
				t.Fatal(result)
			}
			spatial := findStructureIndex(t, readStructure(r.Table), "spatial_index")
			if !strings.EqualFold(spatial.Method, "RTREE") && !strings.EqualFold(spatial.Method, "SPATIAL") { // MySQL 8.4 reports SPATIAL, MariaDB RTREE
				t.Fatal(spatial)
			}
		}
	}
	if queryText(t, e, c, "SELECT id FROM neighbor") != "7" {
		t.Fatal("neighbor changed")
	}
}
func TestSQLiteIndexes(t *testing.T) { indexCases(t, testEngine(t), sqliteConnection()) }
func TestLiveIndexes(t *testing.T) {
	for _, test := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.env)
			if raw == "" {
				t.Skip("W-P09 disposable TLS fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD")}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			indexCases(t, e, c)
			indexesDenied(t, e, c)
		})
	}
}
func indexesDenied(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	if c.Driver == "mysql" {
		execute(t, e, c, "CREATE USER 'index_reader'@'%' IDENTIFIED BY 'fixture-only'")
		execute(t, e, c, "GRANT SELECT ON probe.* TO 'index_reader'@'%'")
	} else {
		execute(t, e, c, "CREATE ROLE index_reader LOGIN PASSWORD 'fixture-only'")
		execute(t, e, c, "GRANT USAGE ON SCHEMA public TO index_reader")
		execute(t, e, c, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO index_reader")
	}
	c.Username = "index_reader"
	c.Password = "fixture-only"
	r := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: "neighbor"}, Action: "index_create", Index: &core.IndexDefinition{Name: "denied_index", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}}
	p, err := e.PlanIndexes(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	r.Confirm = &dbplan.Confirmation{Action: r.Action}
	r.ExpectedPlanSHA256 = p.SHA256
	result, err := e.ExecuteIndexes(context.Background(), r)
	if err != nil || result.Outcome != dbplan.StatusRejected || result.Reason != "permission_denied" {
		t.Fatalf("denied %#v %v", result, err)
	}
	dumpIndexes(t, "IndexesResult", result)
}

func TestIndexesValidation(t *testing.T) {
	base := core.IndexesRequest{Connection: core.Connection{Driver: "postgres"}, Table: core.TableRef{Name: "fixture"}, Action: "index_create", Index: &core.IndexDefinition{Name: "idx", Kind: "index", Terms: []core.IndexDefinitionTerm{{Column: "value"}}}}
	for _, expression := range []string{"lower(value);DROP TABLE neighbor", "pg_sleep", "lower;--"} {
		r := base
		d := *base.Index
		d.Terms = []core.IndexDefinitionTerm{{Column: "value", Expression: expression}}
		r.Index = &d
		if validateIndexes(r) == nil {
			t.Fatal(expression)
		}
	}
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, `CREATE TABLE "fixture" (id BIGINT, value TEXT UNIQUE)`)
	for _, column := range []string{"absent", "value\";DROP TABLE fixture;--"} {
		r := base
		r.Connection = c
		d := *base.Index
		d.Terms = []core.IndexDefinitionTerm{{Column: column}}
		r.Index = &d
		_, err := e.PlanIndexes(context.Background(), r)
		requireMutationCode(t, err, "column_not_found")
	}
	r := core.IndexesRequest{Connection: c, Table: core.TableRef{Name: "fixture"}, Action: "index_drop", Name: "sqlite_autoindex_fixture_1"}
	_, err := e.PlanIndexes(context.Background(), r)
	requireMutationCode(t, err, "unsupported_action")
	r.Action = "index_create"
	r.Name = ""
	r.Index = &core.IndexDefinition{Name: "nullable_unique", Kind: "unique", Terms: []core.IndexDefinitionTerm{{Column: "id"}}}
	execute(t, e, c, `INSERT INTO fixture VALUES(NULL,'a'),(NULL,'b')`)
	p, err := e.PlanIndexes(context.Background(), r)
	if err != nil || p.DuplicateGroups == nil || *p.DuplicateGroups != 0 {
		t.Fatal(p, err)
	}
	r.ExpectedPlanSHA256 = p.SHA256
	r.Confirm = &dbplan.Confirmation{Action: r.Action}
	result, err := e.ExecuteIndexes(context.Background(), r)
	if err != nil || result.Outcome != dbplan.StatusCompleted {
		t.Fatal(result, err)
	}
	// Destructive confirmation must use the exact catalog table name.
	r.Action = "index_drop"
	r.Name = "nullable_unique"
	r.Index = nil
	p, err = e.PlanIndexes(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", "nullable_unique", "fixture ", "FIXTURE"} {
		r.ExpectedPlanSHA256 = p.SHA256
		r.Confirm = &dbplan.Confirmation{Action: r.Action, TargetName: target}
		_, err = e.ExecuteIndexes(context.Background(), r)
		requireMutationCode(t, err, "confirmation_mismatch")
	}
}
