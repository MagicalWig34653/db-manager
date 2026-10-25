// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func rvTrue() *bool  { b := true; return &b }
func rvFalse() *bool { b := false; return &b }
func rvText(n int) dbplan.ColumnType {
	return dbplan.ColumnType{Kind: "text", Length: n}
}
func rvDef(name string, typ dbplan.ColumnType, nullable bool, def *core.CreateDefault) *core.ColumnDefinition {
	n := nullable
	if def == nil {
		def = &core.CreateDefault{Mode: "none"}
	}
	return &core.ColumnDefinition{Name: name, Type: typ, Nullable: &n, Default: def}
}

type rvRig struct {
	t *testing.T
	e *Engine
	c core.Connection
}

func (r rvRig) q(n string) string { return columnQuote(r.c.Driver, n) }
func (r rvRig) exec(s string)     { r.t.Helper(); execute(r.t, r.e, r.c, s) }
func (r rvRig) text(s string) string {
	r.t.Helper()
	return queryText(r.t, r.e, r.c, s)
}
func (r rvRig) plan(table string, ch ...core.ColumnChange) (core.ColumnsPlan, error) {
	return r.e.PlanColumns(context.Background(), core.ColumnsRequest{Connection: r.c, Table: core.TableRef{Name: table}, Changes: ch})
}
func (r rvRig) run(table string, p core.ColumnsPlan, confirm dbplan.Confirmation, sha string, ch ...core.ColumnChange) (core.ColumnsResult, error) {
	return r.e.ExecuteColumns(context.Background(), core.ColumnsRequest{Connection: r.c, Table: core.TableRef{Name: table}, Changes: ch, Confirm: &confirm, ExpectedPlanSHA256: sha})
}
func (r rvRig) cols(table string) string {
	q := "SELECT group_concat(name) FROM pragma_table_info(" + sqlText(table) + ")"
	switch r.c.Driver {
	case "mysql":
		q = "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY ORDINAL_POSITION) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=" + sqlText(table)
	case "postgres":
		q = "SELECT string_agg(column_name, ',' ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=" + sqlText(table)
	}
	return r.text(q)
}

// rvColumnsConfirmation tries to bypass the confirmation of a destructive column drop.
func rvColumnsConfirmation(t *testing.T, r rvRig) {
	r.exec("CREATE TABLE rv_cols (id BIGINT PRIMARY KEY, victim VARCHAR(20), keep VARCHAR(20))")
	r.exec("INSERT INTO rv_cols VALUES (1,'a','b')")
	r.exec("CREATE TABLE rv_cols_other (id BIGINT PRIMARY KEY, victim VARCHAR(20))")
	drop := core.ColumnChange{Action: "drop", Column: "victim"}
	p, err := r.plan("rv_cols", drop)
	if err != nil || !p.Destructive || p.TargetName != "rv_cols" {
		t.Fatalf("plan %v %#v", err, p)
	}
	other, _ := r.plan("rv_cols_other", core.ColumnChange{Action: "drop", Column: "victim"})
	good := dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_cols"}
	for name, try := range map[string]struct {
		c    dbplan.Confirmation
		sha  string
		code string
	}{
		"no name":     {dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, "confirmation_mismatch"},
		"upper":       {dbplan.Confirmation{Action: "columns_alter", TargetName: "RV_COLS"}, p.SHA256, "confirmation_mismatch"},
		"trailing":    {dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_cols "}, p.SHA256, "confirmation_mismatch"},
		"other table": {dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_cols_other"}, p.SHA256, "confirmation_mismatch"},
		"column name": {dbplan.Confirmation{Action: "columns_alter", TargetName: "victim"}, p.SHA256, "confirmation_mismatch"},
		"wrong act":   {dbplan.Confirmation{Action: "table_drop", TargetName: "rv_cols"}, p.SHA256, "confirmation_mismatch"},
		"other plan":  {good, other.SHA256, "plan_mismatch"},
		"zero hash":   {good, strings.Repeat("0", 64), "plan_mismatch"},
		"upper hash":  {good, strings.ToUpper(p.SHA256), "plan_mismatch"},
		"short hash":  {good, p.SHA256[:10], "plan_mismatch"},
	} {
		res, err := r.run("rv_cols", p, try.c, try.sha, drop)
		if err == nil || res.Outcome == "completed" {
			t.Fatalf("%s: executed", name)
		}
		requireMutationCode(t, err, try.code)
		if !strings.Contains(r.cols("rv_cols"), "victim") {
			t.Fatalf("%s: column dropped", name)
		}
	}
	// A request naming the table with another spelling does not resolve.
	for _, spelling := range []string{"RV_COLS", "rv_cols ", "rv_cols​"} {
		if _, err := r.plan(spelling, drop); err == nil {
			t.Fatalf("spelling %q resolved", spelling)
		}
	}
	res, err := r.run("rv_cols", p, good, p.SHA256, drop)
	if err != nil || res.Outcome != "completed" || strings.Contains(r.cols("rv_cols"), "victim") || !strings.Contains(r.cols("rv_other"[:0]+"rv_cols_other"), "victim") {
		t.Fatalf("good run %v %#v", err, res)
	}
	// Replay of the drop and of an add: harmless failures, nothing else changes.
	_, err = r.run("rv_cols", p, good, p.SHA256, drop)
	requireMutationCode(t, err, "column_not_found")
	add := core.ColumnChange{Action: "add", Definition: rvDef("added", rvText(5), true, nil)}
	ap, _ := r.plan("rv_cols", add)
	res, err = r.run("rv_cols", ap, dbplan.Confirmation{Action: "columns_alter"}, ap.SHA256, add)
	if err != nil || res.Outcome != "completed" {
		t.Fatalf("add %v %#v", err, res)
	}
	_, err = r.run("rv_cols", ap, dbplan.Confirmation{Action: "columns_alter"}, ap.SHA256, add)
	if err == nil {
		t.Fatal("replayed add succeeded")
	}
}

// rvColumnsInjection feeds hostile values to every free-text field.
func rvColumnsInjection(t *testing.T, r rvRig) {
	r.exec("CREATE TABLE rv_inj (id BIGINT PRIMARY KEY, c VARCHAR(30) NOT NULL DEFAULT 'x')")
	r.exec("INSERT INTO rv_inj (id) VALUES (1)")
	r.exec("CREATE TABLE rv_neighbor (id BIGINT)")
	r.exec("INSERT INTO rv_neighbor VALUES (1)")
	max := map[string]int{"mysql": 64, "postgres": 63, "sqlite": 63}[r.c.Driver]
	names := []string{"a`b", `a"b`, "a'b", "x;DROP TABLE rv_neighbor;--", "ünï名前", "a b", "%_", strings.Repeat("n", max)}
	if r.c.Driver != "mysql" {
		names = append(names, "a\\b")
	}
	for i, n := range names {
		if len(n) > 63 {
			continue
		}
		add := core.ColumnChange{Action: "add", Definition: rvDef(n, rvText(10), true, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "d'e\"f`;--"}})}
		p, err := r.plan("rv_inj", add)
		if err != nil {
			t.Fatalf("name %d %q: %v", i, n, err)
		}
		res, err := r.run("rv_inj", p, dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, add)
		if err != nil || res.Outcome != "completed" {
			t.Fatalf("add %q: %v %#v", n, err, res)
		}
		renamed := "r" + strconv.Itoa(i)
		rn := core.ColumnChange{Action: "rename", Column: n, NewName: renamed}
		p, err = r.plan("rv_inj", rn)
		if err != nil {
			t.Fatalf("rename plan %q: %v", n, err)
		}
		if res, err = r.run("rv_inj", p, dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, rn); err != nil || res.Outcome != "completed" {
			t.Fatalf("rename %q: %v %#v", n, err, res)
		}
		if got := r.text("SELECT " + r.q(renamed) + " FROM rv_inj LIMIT 1"); got != "" && got != "d'e\"f`;--" {
			t.Fatalf("default round trip %q", got)
		}
	}
	// Hostile values in comment/collation/position/enum must be inert or rejected.
	hostile := "it's \\ `x` \"y\"; DROP TABLE rv_neighbor; --"
	if r.c.Driver != "sqlite" {
		d := rvDef("cm", rvText(10), true, nil)
		d.Comment = &hostile
		ch := core.ColumnChange{Action: "add", Definition: d}
		p, err := r.plan("rv_inj", ch)
		if err != nil {
			t.Fatalf("comment plan: %v", err)
		}
		if res, err := r.run("rv_inj", p, dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, ch); err != nil || res.Outcome != "completed" {
			t.Fatalf("comment run %v %#v", err, res)
		}
		q := "SELECT COLUMN_COMMENT FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='rv_inj' AND COLUMN_NAME='cm'"
		if r.c.Driver == "postgres" {
			q = "SELECT col_description('rv_inj'::regclass, (SELECT attnum FROM pg_attribute WHERE attrelid='rv_inj'::regclass AND attname='cm'))"
		}
		if got := r.text(q); got != hostile {
			t.Fatalf("comment round trip %q", got)
		}
		for _, bad := range []string{"utf8mb4_bin; DROP TABLE rv_neighbor", "no`such", "x' OR '1'='1", "\"; DROP TABLE rv_neighbor;--"} {
			d := rvDef("cl", rvText(10), true, nil)
			d.Collation = &bad
			if _, err := r.plan("rv_inj", core.ColumnChange{Action: "add", Definition: d}); err == nil {
				t.Fatalf("collation %q accepted", bad)
			}
		}
	}
	if r.c.Driver == "mysql" {
		ev := []string{"a'b", "x`y", `q"r`, "s;DROP TABLE rv_neighbor;--", "t\\u"}
		d := rvDef("en", dbplan.ColumnType{Kind: "enum", Values: ev}, true, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "a'b"}})
		ch := core.ColumnChange{Action: "add", Definition: d}
		p, err := r.plan("rv_inj", ch)
		if err != nil {
			t.Fatalf("enum plan: %v", err)
		}
		if res, err := r.run("rv_inj", p, dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, ch); err != nil || res.Outcome != "completed" {
			t.Fatalf("enum run %v %#v", err, res)
		}
		if got := r.text("SELECT en FROM rv_inj LIMIT 1"); got != "" && got != "a'b" {
			t.Fatalf("enum default %q", got)
		}
		bad := rvDef("en2", dbplan.ColumnType{Kind: "enum", Values: []string{"a"}}, true, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "b"}})
		if _, err := r.plan("rv_inj", core.ColumnChange{Action: "add", Definition: bad}); err == nil {
			t.Fatal("enum default outside list accepted")
		}
	}
	for _, bad := range []string{"2020-01-01'; DROP TABLE rv_neighbor;--", "2020-01-01\\", "20200101 25:61:61.000000000000000000"} {
		d := rvDef("dt", dbplan.ColumnType{Kind: "date"}, true, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: bad}})
		p, err := r.plan("rv_inj", core.ColumnChange{Action: "add", Definition: d})
		if err == nil {
			ch := core.ColumnChange{Action: "add", Definition: d}
			res, rerr := r.run("rv_inj", p, dbplan.Confirmation{Action: "columns_alter"}, p.SHA256, ch)
			if rerr == nil && res.Outcome == "completed" {
				t.Fatalf("hostile date default %q executed", bad)
			}
		}
	}
	if r.text("SELECT COUNT(*) FROM rv_neighbor") != "1" {
		t.Fatal("neighbor damaged")
	}
	// Hostile names for position and column lookup.
	for _, pos := range []core.ColumnPosition{{Mode: "after", Column: "id`; DROP TABLE rv_neighbor;--"}, {Mode: "sideways", Column: "id"}} {
		d := rvDef("pp", rvText(5), true, nil)
		if _, err := r.plan("rv_inj", core.ColumnChange{Action: "add", Definition: d, Position: &pos}); err == nil {
			t.Fatalf("position %#v accepted", pos)
		}
	}
}

// rvColumnsData: data-loss and dependency behaviour.
func rvColumnsData(t *testing.T, r rvRig) {
	r.exec("CREATE TABLE rv_dat (id BIGINT PRIMARY KEY, t VARCHAR(20), n VARCHAR(20), u VARCHAR(20))")
	r.exec("INSERT INTO rv_dat VALUES (1,'abcdef','12',NULL),(2,'xy','not a number',NULL)")
	r.exec("CREATE INDEX rv_dat_t ON rv_dat (t)")
	r.exec("CREATE VIEW rv_dat_v AS SELECT id, t FROM rv_dat")
	r.exec("CREATE TABLE rv_dat_child (id BIGINT PRIMARY KEY, p BIGINT, FOREIGN KEY (p) REFERENCES rv_dat(id))")
	// dependencies are reported for drop
	p, err := r.plan("rv_dat", core.ColumnChange{Action: "drop", Column: "t"})
	if err != nil {
		t.Fatalf("drop plan: %v", err)
	}
	deps := strings.Join(p.Dependencies, "|")
	if !strings.Contains(deps, "rv_dat_t") {
		t.Fatalf("index dependency missing: %v", p.Dependencies)
	}
	t.Logf("DEPS drop t (%s): %v; view listed=%v", r.c.Driver, p.Dependencies, strings.Contains(deps, "rv_dat_v"))
	warn := strings.Join(p.Warnings, "\n")
	switch r.c.Driver {
	case "mysql":
		if !strings.Contains(warn, "rv_dat_v") || !strings.Contains(warn, "ungültig") || !strings.Contains(warn, "Index rv_dat_t") {
			t.Fatalf("mysql drop warnings miss view/index effect: %v", p.Warnings)
		}
		if !strings.Contains(strings.Join(p.Affected, "|"), "view:") {
			t.Fatalf("view not in affected_objects: %v", p.Affected)
		}
	case "postgres":
		if !strings.Contains(warn, "verhindert das Entfernen") {
			t.Fatalf("postgres drop warnings miss view block: %v", p.Warnings)
		}
	default:
		if !strings.Contains(warn, "SQLite lehnt") {
			t.Fatalf("sqlite drop warnings: %v", p.Warnings)
		}
	}
	pk, _ := r.plan("rv_dat", core.ColumnChange{Action: "drop", Column: "id"})
	t.Logf("DEPS drop id: %v", pk.Dependencies)
	rn, err := r.plan("rv_dat", core.ColumnChange{Action: "rename", Column: "t", NewName: "t2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(rn.Warnings, "\n"), "alten Namen") {
		t.Fatalf("rename plan misses the dependency warning: %v", rn.Warnings)
	}
	if r.c.Driver == "mysql" && !strings.Contains(strings.Join(rn.Warnings, "\n"), "rv_dat_v") {
		t.Fatalf("rename plan misses the view: %v", rn.Warnings)
	}
	conf := dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_dat"}
	// A nullable column without default (MariaDB reports the text 'NULL') raises no default warning.
	if r.c.Driver != "sqlite" {
		nodef := core.ColumnChange{Action: "change", Column: "u", Definition: rvDef("u", rvText(20), true, nil)}
		if r.c.Driver == "mysql" {
			cm, col := "", "utf8mb4_bin"
			nodef.Definition.Comment, nodef.Definition.Collation = &cm, &col
		}
		np, err := r.plan("rv_dat", nodef)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(np.Warnings, "\n"), "Standardwert") {
			t.Fatalf("false default warning: %v", np.Warnings)
		}
	}
	// Shrinking text and impossible casts must fail without changing data.
	if r.c.Driver != "sqlite" {
		shrink := core.ColumnChange{Action: "change", Column: "t", Definition: rvDef("t", rvText(3), true, nil)}
		if r.c.Driver == "mysql" {
			cm := ""
			col := "utf8mb4_bin"
			shrink.Definition.Comment, shrink.Definition.Collation = &cm, &col
		}
		sp, err := r.plan("rv_dat", shrink)
		if err != nil || !sp.Destructive {
			t.Fatalf("shrink plan %v %#v", err, sp.Destructive)
		}
		res, err := r.run("rv_dat", sp, conf, sp.SHA256, shrink)
		if err != nil || res.Outcome == "completed" || r.text("SELECT t FROM rv_dat WHERE id=1") != "abcdef" {
			t.Fatalf("shrink %v %#v", err, res)
		}
		if strings.Contains(res.Message, "rv_dat") || strings.Contains(res.Message, "ALTER") {
			t.Fatalf("message leaks %q", res.Message)
		}
		toInt := core.ColumnChange{Action: "change", Column: "n", Definition: rvDef("n", dbplan.ColumnType{Kind: "int64"}, true, nil)}
		if r.c.Driver == "mysql" {
			cm := ""
			toInt.Definition.Comment = &cm
		}
		ip, err := r.plan("rv_dat", toInt)
		if err != nil {
			t.Fatal(err)
		}
		joinedStmts := strings.Join(ip.Statements, " ; ")
		t.Logf("TYPE CHANGE statements (%s): %s", r.c.Driver, joinedStmts)
		res, err = r.run("rv_dat", ip, conf, ip.SHA256, toInt)
		if err != nil || res.Outcome == "completed" || r.text("SELECT n FROM rv_dat WHERE id=2") != "not a number" {
			t.Fatalf("cast %v %#v", err, res)
		}
		// NOT NULL over NULL data: warned and rejected, unchanged.
		nn := core.ColumnChange{Action: "change", Column: "u", Definition: rvDef("u", rvText(20), false, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "z"}})}
		if r.c.Driver == "mysql" {
			cm := ""
			col := "utf8mb4_bin"
			nn.Definition.Comment, nn.Definition.Collation = &cm, &col
		}
		np, err := r.plan("rv_dat", nn)
		if err != nil || !strings.Contains(strings.Join(np.Warnings, " "), "NULL-Werte") {
			t.Fatalf("null warning missing %v %v", err, np.Warnings)
		}
		res, _ = r.run("rv_dat", np, conf, np.SHA256, nn)
		if res.Outcome == "completed" && r.text("SELECT COUNT(*) FROM rv_dat WHERE u IS NULL") != "0" {
			t.Fatal("NOT NULL accepted with NULL rows")
		}
	}
	// Drop blocked by a view (PostgreSQL) or FK (all): no CASCADE, nothing changes.
	dp, _ := r.plan("rv_dat", core.ColumnChange{Action: "drop", Column: "t"})
	res, err := r.run("rv_dat", dp, conf, dp.SHA256, core.ColumnChange{Action: "drop", Column: "t"})
	t.Logf("DROP t with index+view (%s): err=%v outcome=%s reason=%s; view still valid=%s cols=%s", r.c.Driver, err, res.Outcome, res.Reason, safeCount(r, "rv_dat_v"), r.cols("rv_dat"))
	fk := core.ColumnChange{Action: "drop", Column: "id"}
	fp, _ := r.plan("rv_dat", fk)
	res, _ = r.run("rv_dat", fp, conf, fp.SHA256, fk)
	if res.Outcome == "completed" {
		t.Fatalf("PK drop under FK succeeded: %#v", res)
	}
	if r.text("SELECT COUNT(*) FROM rv_dat_child") != "0" || r.text("SELECT COUNT(*) FROM rv_dat") != "2" {
		t.Fatal("data changed")
	}
}

func safeCount(r rvRig, view string) string {
	res, err := r.e.Query(context.Background(), core.QueryRequest{Connection: r.c, SQL: "SELECT COUNT(*) FROM " + view})
	if err != nil || len(res.Results) != 1 || len(res.Results[0].Rows) != 1 {
		return "invalid"
	}
	return res.Results[0].Rows[0][0].Value
}

// rvColumnsPG probes PostgreSQL specific type-change behaviour.
func rvColumnsPG(t *testing.T, r rvRig) {
	r.exec("CREATE TABLE rv_pg (id SERIAL PRIMARY KEY, idn BIGINT GENERATED ALWAYS AS IDENTITY, g INTEGER GENERATED ALWAYS AS (id*2) STORED, v VARCHAR(10))")
	r.exec("INSERT INTO rv_pg (v) VALUES ('1')")
	conf := dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_pg"}
	for _, col := range []string{"g"} {
		if _, err := r.plan("rv_pg", core.ColumnChange{Action: "drop", Column: col}); err == nil {
			t.Fatalf("generated column %s accepted", col)
		}
	}
	ch := core.ColumnChange{Action: "change", Column: "id", Definition: rvDef("id", dbplan.ColumnType{Kind: "int64"}, false, nil)}
	p, err := r.plan("rv_pg", ch)
	if err != nil {
		t.Logf("SERIAL change plan refused: %v", err)
		return
	}
	res, err := r.run("rv_pg", p, conf, p.SHA256, ch)
	def := r.text("SELECT COALESCE(column_default,'<none>') FROM information_schema.columns WHERE table_name='rv_pg' AND column_name='id' AND table_schema=current_schema()")
	if !strings.Contains(strings.Join(p.Warnings, "\n"), "nextval(") {
		t.Fatalf("serial default not named: %v", p.Warnings)
	}
	t.Logf("SERIAL change reason=%s", res.Reason)
	t.Logf("SERIAL change (default none): outcome=%s err=%v defaultAfter=%s warnings=%v", res.Outcome, err, def, p.Warnings)
	r.exec("CREATE TABLE rv_pg2 (id BIGINT PRIMARY KEY, s SERIAL, i2 INTEGER GENERATED BY DEFAULT AS IDENTITY)")
	for _, col := range []string{"s", "i2"} {
		cc := core.ColumnChange{Action: "change", Column: col, Definition: rvDef(col, dbplan.ColumnType{Kind: "int64"}, false, nil)}
		pp, err := r.plan("rv_pg2", cc)
		if err != nil {
			t.Logf("NONPK %s plan refused: %v", col, err)
			continue
		}
		if col == "s" && !strings.Contains(strings.Join(pp.Warnings, "\n"), "nextval(") {
			t.Fatalf("serial default not named in the plan: %v", pp.Warnings)
		}
		rs, err := r.run("rv_pg2", pp, dbplan.Confirmation{Action: "columns_alter", TargetName: "rv_pg2"}, pp.SHA256, cc)
		t.Logf("NONPK %s: outcome=%s reason=%s err=%v defaultAfter=%s identityAfter=%s", col, rs.Outcome, rs.Reason, err,
			r.text("SELECT COALESCE(column_default,'<none>') FROM information_schema.columns WHERE table_name='rv_pg2' AND column_name='"+col+"' AND table_schema=current_schema()"),
			r.text("SELECT is_identity FROM information_schema.columns WHERE table_name='rv_pg2' AND column_name='"+col+"' AND table_schema=current_schema()"))
	}
	ch2 := core.ColumnChange{Action: "change", Column: "idn", Definition: rvDef("idn", dbplan.ColumnType{Kind: "int64"}, false, nil)}
	p2, err := r.plan("rv_pg", ch2)
	if err == nil {
		res, err = r.run("rv_pg", p2, conf, p2.SHA256, ch2)
		t.Logf("IDENTITY change: outcome=%s reason=%s err=%v identityAfter=%s", res.Outcome, res.Reason, err, r.text("SELECT is_identity FROM information_schema.columns WHERE table_name='rv_pg' AND column_name='idn' AND table_schema=current_schema()"))
	}
}

func rvSystemAndKinds(t *testing.T, r rvRig, schema string) {
	r.exec("CREATE VIEW rv_view AS SELECT 1 AS x")
	if _, err := r.plan("rv_view", core.ColumnChange{Action: "add", Definition: rvDef("y", rvText(3), true, nil)}); err == nil {
		t.Fatal("view accepted")
	}
	sys := map[string]string{"mysql": "mysql", "postgres": "pg_catalog"}[r.c.Driver]
	name := map[string]string{"mysql": "user", "postgres": "pg_class"}[r.c.Driver]
	if sys != "" {
		_, err := r.e.PlanColumns(context.Background(), core.ColumnsRequest{Connection: r.c, Table: core.TableRef{Schema: sys, Name: name}, Changes: []core.ColumnChange{{Action: "add", Definition: rvDef("y", rvText(3), true, nil)}}})
		if err == nil {
			t.Fatalf("system table %s.%s accepted", sys, name)
		}
	}
	if r.c.Driver == "mysql" {
		r.exec("CREATE TABLE rv_myisam (id INT) ENGINE=MyISAM")
		if _, err := r.plan("rv_myisam", core.ColumnChange{Action: "add", Definition: rvDef("y", rvText(3), true, nil)}); err == nil {
			t.Fatal("MyISAM accepted")
		}
	}
	if r.c.Driver == "postgres" {
		r.exec("CREATE TABLE rv_inh_p (id INT)")
		r.exec("CREATE TABLE rv_inh_c () INHERITS (rv_inh_p)")
		for _, n := range []string{"rv_inh_p", "rv_inh_c"} {
			if _, err := r.plan(n, core.ColumnChange{Action: "add", Definition: rvDef("y", rvText(3), true, nil)}); err == nil {
				t.Fatalf("inheritance table %s accepted", n)
			}
		}
	}
}

func rvAll(t *testing.T, r rvRig) {
	rvColumnsConfirmation(t, r)
	rvColumnsInjection(t, r)
	rvColumnsData(t, r)
	rvSystemAndKinds(t, r, "")
	if r.c.Driver == "postgres" {
		rvColumnsPG(t, r)
	}
}

func TestReviewW2ColumnsSQLite(t *testing.T) {
	r := rvRig{t, testEngine(t), core.Connection{Driver: "sqlite", Database: "rvcols"}}
	rvColumnsConfirmation(t, r)
	rvColumnsInjection(t, r)
	rvColumnsData(t, r)
}

func rvLive(t *testing.T, fn func(*testing.T, rvRig)) {
	for _, test := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.env)
			if raw == "" {
				t.Skip("disposable TLS fixture required")
			}
			port, _ := strconv.Atoi(raw)
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD")}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			fn(t, rvRig{t, e, c})
		})
	}
}

func TestReviewW2ColumnsLive(t *testing.T) { rvLive(t, rvAll) }

// ---- maintenance ----

func rvMaintenance(t *testing.T, r rvRig) {
	ctx := context.Background()
	schema := ""
	if r.c.Driver == "postgres" {
		schema = "public"
	}
	r.exec("CREATE TABLE rv_m (id INTEGER PRIMARY KEY, v VARCHAR(10))")
	r.exec("INSERT INTO rv_m VALUES (1,'a')")
	typed := map[string][]string{"mysql": {"optimize"}, "postgres": {"reindex"}}[r.c.Driver]
	plan := func(item core.MaintenanceItem) core.MaintenancePreview {
		res, err := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: []core.MaintenanceItem{item}})
		if err != nil || len(res.Items) != 1 {
			t.Fatalf("plan %v %#v", err, res)
		}
		return res.Items[0]
	}
	exec := func(item core.MaintenanceItem, sha string, c dbplan.Confirmation) core.MaintenanceOutcome {
		res, err := r.e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: r.c, Items: []core.MaintenanceExecuteItem{{MaintenanceItem: item, ExpectedPlanSHA256: sha, Confirm: c}}})
		if err != nil || len(res.Items) != 1 {
			t.Fatalf("exec %v %#v", err, res)
		}
		return res.Items[0]
	}
	for _, action := range typed {
		item := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "rv_m"}, Action: action}
		p := plan(item)
		if p.Plan == nil || !p.TypedConfirmation || p.TargetName != "rv_m" {
			t.Fatalf("%s plan %#v", action, p)
		}
		for name, try := range map[string]struct {
			sha string
			c   dbplan.Confirmation
			rsn string
		}{
			"no name":    {p.Plan.SHA256, dbplan.Confirmation{Action: action}, "confirmation_mismatch"},
			"upper":      {p.Plan.SHA256, dbplan.Confirmation{Action: action, TargetName: "RV_M"}, "confirmation_mismatch"},
			"space":      {p.Plan.SHA256, dbplan.Confirmation{Action: action, TargetName: "rv_m "}, "confirmation_mismatch"},
			"wrong act":  {p.Plan.SHA256, dbplan.Confirmation{Action: "check", TargetName: "rv_m"}, "confirmation_mismatch"},
			"zero hash":  {strings.Repeat("0", 64), dbplan.Confirmation{Action: action, TargetName: "rv_m"}, "plan_mismatch"},
			"upper hash": {strings.ToUpper(p.Plan.SHA256), dbplan.Confirmation{Action: action, TargetName: "rv_m"}, "plan_mismatch"},
			"empty hash": {"", dbplan.Confirmation{Action: action, TargetName: "rv_m"}, "plan_mismatch"},
		} {
			o := exec(item, try.sha, try.c)
			if o.Outcome != dbplan.StatusRejected || o.Reason != try.rsn {
				t.Fatalf("%s/%s: %#v", action, name, o)
			}
		}
		if o := exec(item, p.Plan.SHA256, dbplan.Confirmation{Action: action, TargetName: "rv_m"}); o.Outcome != dbplan.StatusCompleted {
			t.Fatalf("%s good run %#v", action, o)
		}
	}
	// Untyped actions still need the exact hash and action.
	item := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "rv_m"}, Action: "analyze"}
	p := plan(item)
	if o := exec(item, p.Plan.SHA256, dbplan.Confirmation{Action: "check"}); o.Outcome != dbplan.StatusRejected {
		t.Fatalf("analyze with wrong action %#v", o)
	}
	if o := exec(item, strings.Repeat("a", 64), dbplan.Confirmation{Action: "analyze"}); o.Outcome != dbplan.StatusRejected {
		t.Fatalf("analyze with wrong hash %#v", o)
	}
	// System tables and kinds in every spelling.
	systems := map[string][]core.TableRef{
		"mysql":    {{Schema: "mysql", Name: "user"}, {Schema: "MYSQL", Name: "user"}, {Schema: "information_schema", Name: "TABLES"}, {Schema: "sys", Name: "x"}, {Schema: "performance_schema", Name: "threads"}},
		"postgres": {{Schema: "pg_catalog", Name: "pg_class"}, {Schema: "PG_CATALOG", Name: "pg_class"}, {Schema: "information_schema", Name: "tables"}, {Schema: "pg_toast", Name: "x"}, {Name: "pg_class"}, {Name: "pg_database"}, {Name: "pg_authid"}},
	}[r.c.Driver]
	for _, ref := range systems {
		for _, action := range []string{"analyze", "check", "vacuum", "reindex", "optimize"} {
			if !core.MaintenanceCapabilities().Allows(r.c.Driver, action) {
				continue
			}
			it := core.MaintenanceItem{Table: ref, Action: action}
			pr := plan(it)
			if pr.Plan != nil {
				t.Errorf("SYSTEM TABLE PLANNED %s %#v: state=%s", action, ref, pr.State)
			}
		}
	}
	r.exec("CREATE VIEW rv_mv AS SELECT * FROM rv_m")
	if pr := plan(core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "rv_mv"}, Action: "analyze"}); pr.Plan != nil {
		t.Fatal("view planned")
	}
	if r.c.Driver == "mysql" {
		r.exec("CREATE TABLE rv_inno (id INT) ENGINE=InnoDB")
		if pr := plan(core.MaintenanceItem{Table: core.TableRef{Name: "rv_inno"}, Action: "repair"}); pr.Plan != nil {
			t.Fatal("REPAIR planned on InnoDB")
		}
	}
	if r.c.Driver == "postgres" {
		r.exec("CREATE TABLE rv_pp (id INT, k INT) PARTITION BY LIST (k)")
		r.exec("CREATE TABLE rv_pp1 PARTITION OF rv_pp FOR VALUES IN (1)")
		r.exec("CREATE TABLE rv_ip (id INT)")
		r.exec("CREATE TABLE rv_ic () INHERITS (rv_ip)")
		for _, n := range []string{"rv_pp", "rv_pp1", "rv_ip", "rv_ic"} {
			if pr := plan(core.MaintenanceItem{Table: core.TableRef{Schema: "public", Name: n}, Action: "vacuum"}); pr.Plan != nil {
				t.Fatalf("%s planned: %#v", n, pr)
			}
		}
	}
	// Hostile names stay inert; result carries no raw diagnostics.
	odd := "m`\"';DROP TABLE rv_m;--"
	oq, _ := dbplan.QualifiedName(r.c.Driver, schema, odd)
	r.exec("CREATE TABLE " + oq + " (id INTEGER)")
	for _, action := range []string{"analyze", "check"} {
		if !core.MaintenanceCapabilities().Allows(r.c.Driver, action) {
			continue
		}
		it := core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: odd}, Action: action}
		pr := plan(it)
		if pr.Plan == nil {
			t.Logf("odd name %s: %s", action, pr.Reason)
			continue
		}
		o := exec(it, pr.Plan.SHA256, dbplan.Confirmation{Action: action})
		raw, _ := json.Marshal(o)
		if o.Outcome != dbplan.StatusCompleted || strings.Contains(string(raw), "ANALYZE") || strings.Contains(string(raw), "integrity_check") {
			t.Fatalf("odd %s: %s", action, raw)
		}
	}
	if r.text("SELECT COUNT(*) FROM rv_m") != "1" {
		t.Fatal("neighbor changed")
	}
	// Batch size and timeout bounds.
	tooMany := make([]core.MaintenanceItem, core.MaintenanceMaxItems+1)
	for i := range tooMany {
		tooMany[i] = item
	}
	if _, err := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: tooMany}); err == nil {
		t.Fatal("51 items accepted")
	}
	if _, err := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: []core.MaintenanceItem{item}, TimeoutMS: core.MaintenanceMaxTimeoutMS + 1}); err == nil {
		t.Fatal("timeout above max accepted")
	}
	// Vacuum flags only for vacuum.
	if pr := plan(core.MaintenanceItem{Table: core.TableRef{Schema: schema, Name: "rv_m"}, Action: "analyze", Full: true}); pr.Plan != nil {
		t.Fatal("full flag on analyze accepted")
	}
}

func TestReviewW2MaintenanceSQLite(t *testing.T) {
	r := rvRig{t, testEngine(t), core.Connection{Driver: "sqlite", Database: "rvmaint"}}
	ctx := context.Background()
	r.exec("CREATE TABLE rv_m (id INTEGER PRIMARY KEY)")
	for _, ref := range []core.TableRef{{Name: "sqlite_master"}, {Name: "SQLITE_schema"}, {Schema: "temp", Name: "rv_m"}, {Schema: "main", Name: "sqlite_sequence"}} {
		res, err := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: []core.MaintenanceItem{{Table: ref, Action: "check"}}})
		if err != nil || res.Items[0].Plan != nil {
			t.Fatalf("%#v planned: %v %#v", ref, err, res)
		}
	}
	for _, action := range []string{"vacuum", "reindex", "optimize", "repair"} {
		res, _ := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: []core.MaintenanceItem{{Table: core.TableRef{Name: "rv_m"}, Action: action}}})
		if res.Items[0].Plan != nil {
			t.Fatalf("sqlite %s planned", action)
		}
	}
	odd := `m"x'y;--`
	r.exec(`CREATE TABLE "m""x'y;--" (id INTEGER)`)
	item := core.MaintenanceItem{Table: core.TableRef{Name: odd}, Action: "check"}
	res, _ := r.e.PlanMaintenance(ctx, core.MaintenancePlanRequest{Connection: r.c, Items: []core.MaintenanceItem{item}})
	if res.Items[0].Plan == nil {
		t.Fatalf("odd plan %#v", res.Items[0])
	}
	ex, err := r.e.ExecuteMaintenance(ctx, core.MaintenanceExecuteRequest{Connection: r.c, Items: []core.MaintenanceExecuteItem{{MaintenanceItem: item, ExpectedPlanSHA256: res.Items[0].Plan.SHA256, Confirm: dbplan.Confirmation{Action: "check"}}}})
	if err != nil || ex.Items[0].Outcome != dbplan.StatusCompleted {
		t.Fatalf("odd exec %v %#v", err, ex)
	}
	// The normal query connection still denies integrity_check.
	if _, err := r.e.Query(ctx, core.QueryRequest{Connection: r.c, SQL: "PRAGMA integrity_check('rv_m')"}); err == nil {
		t.Fatal("ordinary connection allows integrity_check")
	}
}

func TestReviewW2MaintenanceLive(t *testing.T) { rvLive(t, rvMaintenance) }
