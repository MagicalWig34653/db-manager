// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func fuJoin(p core.ColumnsPlan) string {
	return strings.Join(p.Warnings, " | ") + " || " + ""
}

func fuWarn(p core.ColumnsPlan) string { return strings.Join(p.Warnings, " | ") }
func fuAff(p core.ColumnsPlan) string  { return strings.Join(p.Affected, " | ") }

func fuMyComment(r rvRig, d *core.ColumnDefinition) {
	if r.c.Driver == "mysql" {
		c := ""
		d.Comment = &c
		if d.Type.Kind == "text" {
			col := "utf8mb4_bin"
			d.Collation = &col
		}
	}
}

func fuColumns(t *testing.T, r rvRig) {
	driver := r.c.Driver
	r.exec("CREATE TABLE fu_t (id BIGINT PRIMARY KEY, a VARCHAR(20), b VARCHAR(20), c VARCHAR(20))")
	r.exec("CREATE TABLE fu_other (id BIGINT PRIMARY KEY, a VARCHAR(20))")
	r.exec("INSERT INTO fu_t VALUES (1,'x','y','z')")
	r.exec("CREATE INDEX fu_t_a ON fu_t (a)")
	r.exec("CREATE VIEW fu_v_a AS SELECT id, a FROM fu_t")
	r.exec("CREATE VIEW fu_v_other AS SELECT id, a FROM fu_other")
	r.exec("CREATE TABLE fu_child (id BIGINT PRIMARY KEY, p BIGINT, FOREIGN KEY (p) REFERENCES fu_t(id))")
	drop := func(col string) core.ColumnsPlan {
		p, err := r.plan("fu_t", core.ColumnChange{Action: "drop", Column: col})
		if err != nil {
			t.Fatalf("drop %s: %v", col, err)
		}
		return p
	}
	// (a) drop with a view and an index
	pa := drop("a")
	w := fuWarn(pa)
	switch driver {
	case "mysql":
		if !strings.Contains(w, "Sicht probe.fu_v_a") || !strings.Contains(w, "Index fu_t_a") || strings.Contains(w, "fu_v_other") || !strings.Contains(w, "lehnt die Änderung nicht ab") {
			t.Fatalf("mysql drop a warnings: %s", w)
		}
		if !strings.Contains(fuAff(pa), "view:probe.fu_v_a") || !strings.Contains(fuAff(pa), "index:fu_t_a") {
			t.Fatalf("mysql affected: %s", fuAff(pa))
		}
	case "postgres":
		if !strings.Contains(w, "fu_v_a") || !strings.Contains(w, "verhindert das Entfernen") || !strings.Contains(w, "fu_t_a") || strings.Contains(w, "fu_v_other") {
			t.Fatalf("postgres drop a warnings: %s", w)
		}
	case "sqlite":
		if !strings.Contains(w, "SQLite lehnt das Entfernen ab") {
			t.Fatalf("sqlite drop a warnings: %s", w)
		}
	}
	// No false alarm for columns without dependencies (b, c) and for a same-named column of another table.
	for _, col := range []string{"b", "c"} {
		pc := drop(col)
		ww := fuWarn(pc)
		if driver == "sqlite" && strings.Contains(ww, "SQLite lehnt das Entfernen ab") {
			t.Logf("FINDING L2 soft false alarm (sqlite, conditional wording) for column %s", col)
			ww = strings.ReplaceAll(ww, "SQLite lehnt das Entfernen ab, wenn Index, Sicht, Trigger oder Einschränkung die Spalte verwenden.", "")
		}
		for _, bad := range []string{"Sicht", "Index", "Constraint", "SQLite lehnt", "fu_v_"} {
			if strings.Contains(ww, bad) {
				if driver == "sqlite" && bad == "SQLite lehnt" {
					t.Logf("FINDING L2 soft false alarm (sqlite, conditional wording) for column %s", col)
					continue
				}
				t.Fatalf("false alarm for %s (%s): %s", col, driver, ww)
			}
		}
	}
	if driver == "mysql" {
		// view over fu_t that does not use b: no alarm even though the table is referenced
		if w := fuWarn(drop("b")); strings.Contains(w, "fu_v_a") {
			t.Fatalf("view not using the column flagged: %s", w)
		}
	}
	// PostgreSQL drop of a column with a view is still rejected, nothing changes.
	if driver == "postgres" {
		p := drop("a")
		res, _ := r.run("fu_t", p, dbplan.Confirmation{Action: "columns_alter", TargetName: "fu_t"}, p.SHA256, core.ColumnChange{Action: "drop", Column: "a"})
		if res.Outcome == "completed" || !strings.Contains(r.cols("fu_t"), "a") {
			t.Fatalf("pg dropped under view: %#v", res)
		}
	}
	// (c) rename warns with dependencies; Check still requires only action+hash (not destructive).
	prn, err := r.plan("fu_t", core.ColumnChange{Action: "rename", Column: "a", NewName: "a2"})
	if err != nil {
		t.Fatal(err)
	}
	rw := fuWarn(prn)
	if !strings.Contains(rw, "funktionieren danach nicht mehr") {
		t.Fatalf("rename generic warning missing: %s", rw)
	}
	if driver == "mysql" && !strings.Contains(rw, "fu_v_a") {
		t.Fatalf("mysql rename view warning missing: %s", rw)
	}
	if driver == "postgres" && strings.Contains(rw, "fu_v_a") {
		t.Logf("NOTE postgres rename warning mentions view: %s", rw)
	}
	t.Logf("RENAME warnings (%s): %s", driver, rw)
	prb, _ := r.plan("fu_t", core.ColumnChange{Action: "rename", Column: "b", NewName: "b2"})
	if strings.Contains(fuWarn(prb), "fu_v_") {
		t.Fatalf("rename b falsely warns about views: %s", fuWarn(prb))
	}
	// FK column rename
	if driver != "sqlite" {
		pfk, _ := r.plan("fu_t", core.ColumnChange{Action: "rename", Column: "id", NewName: "id2"})
		t.Logf("RENAME id warnings (%s): %s", driver, fuWarn(pfk))
		// InnoDB adjusts foreign keys on RENAME COLUMN (see columnDependencyWarnings), so a
		// rename must not claim that a foreign key blocks it; drops are covered by the dependency tests.
		if driver == "mysql" && strings.Contains(fuWarn(pfk), "Fremdschlüssel verhindert") {
			t.Fatalf("mysql FK rename falsely warns that a foreign key blocks it: %s", fuWarn(pfk))
		}
	}
	// (L2) NOT NULL planning with EXISTS.
	r.exec("CREATE TABLE fu_nn (id BIGINT PRIMARY KEY, e VARCHAR(10), n VARCHAR(10), z VARCHAR(10))")
	r.exec("INSERT INTO fu_nn VALUES (1,'','v',NULL),(2,'','w','q')")
	nn := func(col string) string {
		d := rvDef(col, rvText(10), false, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "d"}})
		fuMyComment(r, d)
		p, err := r.plan("fu_nn", core.ColumnChange{Action: "change", Column: col, Definition: d})
		if driver == "sqlite" {
			if err == nil {
				t.Fatal("sqlite change must be unsupported")
			}
			return ""
		}
		if err != nil {
			t.Fatalf("nn %s: %v", col, err)
		}
		return fuWarn(p)
	}
	if driver != "sqlite" {
		if w := nn("e"); strings.Contains(w, "NULL-Werte") {
			t.Fatalf("empty strings reported as NULL: %s", w)
		}
		if w := nn("n"); strings.Contains(w, "NULL-Werte") {
			t.Fatalf("no NULLs but warned: %s", w)
		}
		if w := nn("z"); !strings.Contains(w, "NULL-Werte") {
			t.Fatalf("NULL rows not reported: %s", w)
		}
		r.exec("CREATE TABLE fu_empty (id BIGINT PRIMARY KEY, x VARCHAR(10))")
		d := rvDef("x", rvText(10), false, &core.CreateDefault{Mode: "value", Value: &core.Cell{Kind: "text", Value: "d"}})
		fuMyComment(r, d)
		if p, err := r.plan("fu_empty", core.ColumnChange{Action: "change", Column: "x", Definition: d}); err != nil || strings.Contains(fuWarn(p), "NULL-Werte") {
			t.Fatalf("empty table: %v %v", err, fuWarn(p))
		}
	}
	// (b) default replacement warning, including MariaDB's textual 'NULL' default quirk.
	r.exec("CREATE TABLE fu_def (id BIGINT PRIMARY KEY, nodef VARCHAR(10), five INTEGER DEFAULT 5)")
	if driver != "sqlite" {
		d := rvDef("nodef", rvText(10), true, nil)
		fuMyComment(r, d)
		p, err := r.plan("fu_def", core.ColumnChange{Action: "change", Column: "nodef", Definition: d})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(fuWarn(p), "bisherige Standardwert") {
			t.Logf("FINDING L1 false alarm default warning for a column without default (%s): %s", driver, fuWarn(p))
		}
		d5 := rvDef("five", dbplan.ColumnType{Kind: "int64"}, true, nil)
		fuMyComment(r, d5)
		p5, err := r.plan("fu_def", core.ColumnChange{Action: "change", Column: "five", Definition: d5})
		if err != nil || !strings.Contains(fuWarn(p5), "bisherige Standardwert 5") || !strings.Contains(fuWarn(p5), "entfällt") {
			t.Fatalf("default 5 warning: %v %s", err, fuWarn(p5))
		}
	}
	if driver == "postgres" {
		r.exec("CREATE TABLE fu_ser (id BIGINT PRIMARY KEY, s SERIAL)")
		cc := core.ColumnChange{Action: "change", Column: "s", Definition: rvDef("s", dbplan.ColumnType{Kind: "int64"}, false, nil)}
		p, err := r.plan("fu_ser", cc)
		if err != nil {
			t.Fatal(err)
		}
		w := fuWarn(p)
		if !strings.Contains(w, "nextval(") || !strings.Contains(w, "Sequenz") || !strings.Contains(w, "entfällt") {
			t.Fatalf("serial warning: %s", w)
		}
	}
	// (L3) names: unicode and inner spaces are valid; edge whitespace/control rejected; legacy edge-space column.
	for _, ok := range []string{"ünï名前", "with inner space", "a-b", "x" + strings.Repeat("y", 40)} {
		add := core.ColumnChange{Action: "add", Definition: rvDef(ok, rvText(5), true, nil)}
		if _, err := r.plan("fu_t", add); err != nil {
			t.Errorf("valid name %q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{" lead", "trail ", "tab\t", "nl\n", "\x07bell", "\u0085nel"} {
		add := core.ColumnChange{Action: "add", Definition: rvDef(bad, rvText(5), true, nil)}
		if _, err := r.plan("fu_t", add); err == nil {
			t.Errorf("invalid name %q accepted", bad)
		}
		if _, err := r.plan("fu_t", core.ColumnChange{Action: "rename", Column: "b", NewName: bad}); err == nil {
			t.Errorf("invalid rename target %q accepted", bad)
		}
	}
	if driver != "mysql" { // MySQL itself forbids trailing spaces in names
		r.exec("CREATE TABLE fu_legacy (id BIGINT PRIMARY KEY, " + r.q("edge ") + " VARCHAR(10))")
		_, err := r.plan("fu_legacy", core.ColumnChange{Action: "drop", Column: "edge "})
		t.Logf("legacy edge-space column drop plan err=%v", err)
		d := rvDef("edge ", rvText(5), true, nil)
		_, err = r.plan("fu_legacy", core.ColumnChange{Action: "change", Column: "edge ", Definition: d})
		t.Logf("legacy edge-space column change plan err=%v (new validation applies to existing names)", err)
	}
}

func TestReviewFollowUpSQLite(t *testing.T) {
	fuColumns(t, rvRig{t, testEngine(t), core.Connection{Driver: "sqlite", Database: "fucols"}})
}

func TestReviewFollowUpLive(t *testing.T) { rvLive(t, fuColumns) }
