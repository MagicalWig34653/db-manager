// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func joined(v []string) string { return strings.Join(v, " | ") }

// rv2Postgres re-checks M1 on PostgreSQL: ONLY semantics, affected objects, warnings.
func rv2Postgres(t *testing.T, h *opHarness) {
	h.exec("CREATE TABLE " + h.q("p_parent") + "(id INTEGER)")
	h.exec("CREATE TABLE " + h.q("p_child") + "() INHERITS (" + h.q("p_parent") + ")")
	h.exec("CREATE TABLE " + h.q("p_grand") + "() INHERITS (" + h.q("p_child") + ")")
	h.exec("INSERT INTO " + h.q("p_parent") + " VALUES(1)")
	h.exec("INSERT INTO " + h.q("p_child") + " VALUES(2),(3)")
	h.exec("INSERT INTO " + h.q("p_grand") + " VALUES(4),(5),(6)")
	plan := h.mustPlan(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("p_parent")})
	if !strings.Contains(plan.Statements[0], "TRUNCATE TABLE ONLY") || !strings.Contains(joined(plan.AffectedObjects), "p_child") || !strings.Contains(joined(plan.AffectedObjects), "p_grand") || !strings.Contains(joined(plan.Warnings), "Kindtabellen bleiben unver") {
		t.Fatalf("truncate plan %#v", plan)
	}
	res, err := h.execute(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("p_parent")}, plan, core.ObjectOpConfirm{Action: core.OpTableTruncate, TargetName: "p_parent"})
	if err != nil || res.Outcome != "completed" {
		t.Fatalf("truncate %v %#v", err, res)
	}
	if got := h.text("SELECT COUNT(*) FROM ONLY " + h.q("p_parent")); got != "0" {
		t.Fatalf("parent rows %s", got)
	}
	if h.count("p_child") != "5" { // child + grandchild rows through inheritance
		t.Fatalf("children changed: %s", h.count("p_child"))
	}
	if res.Observation == nil || res.Observation.Rows == nil || *res.Observation.Rows != 0 {
		t.Fatalf("observation must count ONLY the parent: %#v", res.Observation)
	}
	// Drop of an inheritance parent is refused by the server (no CASCADE) and warns.
	dp := h.mustPlan(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("p_parent")})
	if !strings.Contains(joined(dp.Warnings), "Kindtabellen verhindern") || !strings.Contains(joined(dp.AffectedObjects), "p_child") {
		t.Fatalf("drop plan %#v", dp)
	}
	dres, err := h.execute(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("p_parent")}, dp, core.ObjectOpConfirm{Action: core.OpTableDrop, TargetName: "p_parent"})
	if err != nil || dres.Outcome != "rejected" || dres.Reason != "dependency_blocks" || !h.has("p_parent") || !h.has("p_child") || h.count("p_child") != "5" {
		t.Fatalf("drop parent %v %#v", err, dres)
	}
	// Partitioned table.
	h.exec("CREATE TABLE " + h.q("p_part") + "(id INTEGER, k INTEGER) PARTITION BY LIST (k)")
	h.exec("CREATE TABLE " + h.q("p_part_1") + " PARTITION OF " + h.q("p_part") + " FOR VALUES IN (1)")
	h.exec("CREATE TABLE " + h.q("p_part_2") + " PARTITION OF " + h.q("p_part") + " FOR VALUES IN (2)")
	h.exec("INSERT INTO " + h.q("p_part") + " VALUES(1,1),(2,2)")
	tp := h.mustPlan(core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("p_part")})
	if strings.Contains(tp.Statements[0], "ONLY") || !strings.Contains(joined(tp.Warnings), "samt ihren Partitionen") || !strings.Contains(joined(tp.AffectedObjects), "p_part_2") {
		t.Fatalf("partition truncate plan %#v", tp)
	}
	dpp := h.mustPlan(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("p_part")})
	if !strings.Contains(joined(dpp.Warnings), "Partitionen entfernt") || !strings.Contains(joined(dpp.AffectedObjects), "p_part_1") {
		t.Fatalf("partition drop plan %#v", dpp)
	}
	r, err := h.execute(core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("p_part")}, dpp, core.ObjectOpConfirm{Action: core.OpTableDrop, TargetName: "p_part"})
	if err != nil || r.Outcome != "completed" || h.has("p_part_1") || h.has("p_part_2") || !h.has("p_parent") {
		t.Fatalf("partition drop %v %#v", err, r)
	}
	// Fingerprint probe for finding F1: identity changes that a stateless digest could include.
	h.exec("CREATE TABLE " + h.q("p_fp") + "(id INTEGER)")
	q := "SELECT c.oid::text||'/'||c.relfilenode::text FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=" + sqlText(h.schema) + " AND c.relname='p_fp'"
	before := h.text(q)
	h.exec("INSERT INTO " + h.q("p_fp") + " VALUES(1)")
	mid := h.text(q)
	h.exec("TRUNCATE " + h.q("p_fp"))
	after := h.text(q)
	t.Logf("FINGERPRINT postgres oid/relfilenode before=%s afterInsert=%s afterTruncate=%s", before, mid, after)
	if before != mid || mid == after {
		t.Logf("relfilenode not a clean signal")
	}
}

// rv2MySQL re-checks partitions, L2 and L3 on MySQL/MariaDB.
func rv2MySQL(t *testing.T, h *opHarness) {
	h.exec("CREATE TABLE " + h.q("m_part") + "(id INTEGER NOT NULL, PRIMARY KEY(id)) PARTITION BY HASH(id) PARTITIONS 3")
	h.exec("INSERT INTO " + h.q("m_part") + " VALUES(1),(2),(3),(4)")
	for _, action := range []string{core.OpTableTruncate, core.OpTableDrop} {
		p := h.mustPlan(action, core.ObjectOpRequest{Table: h.ref("m_part")})
		if !strings.Contains(joined(p.AffectedObjects), "partition:p0") || !strings.Contains(joined(p.AffectedObjects), "partition:p2") || !strings.Contains(joined(p.Warnings), "p0, p1, p2") {
			t.Fatalf("%s plan %#v", action, p)
		}
	}
	// L2: engine change and convert are destructive; comment-only is not.
	for _, req := range []core.ObjectOpRequest{
		{Table: h.ref("m_part"), Collation: sp("utf8mb4_bin"), CollationMode: sp("convert")},
	} {
		p := h.mustPlan(core.OpTableOptions, req)
		if !p.Confirmation.Destructive || p.Confirmation.TargetName != "m_part" {
			t.Fatalf("convert not destructive %#v", p.Confirmation)
		}
		req.Connection = h.c
		_, err := h.e.ObjectOpExecute(context.Background(), core.OpTableOptions, execWith(req, core.ObjectOpConfirm{Action: core.OpTableOptions}, p.PlanSHA256))
		h.wantCode(err, "confirmation_mismatch")
	}
	h.exec("CREATE TABLE " + h.q("m_eng") + "(id INTEGER PRIMARY KEY) ENGINE=InnoDB")
	other := "MyISAM"
	if h.text("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_NAME='m_eng' AND TABLE_SCHEMA="+sqlText(h.schema)) == "MyISAM" {
		other = "InnoDB"
	}
	if p, err := h.plan(core.OpTableOptions, core.ObjectOpRequest{Table: h.ref("m_eng"), Engine: sp(other)}); err == nil && !p.Confirmation.Destructive {
		t.Fatalf("engine change not destructive %#v", p.Confirmation)
	}
	cp := h.mustPlan(core.OpTableOptions, core.ObjectOpRequest{Table: h.ref("m_eng"), Comment: sp("only a comment")})
	if cp.Confirmation.Destructive {
		t.Fatal("comment-only must stay non-destructive")
	}
	// Fingerprint probe: CREATE_TIME across TRUNCATE.
	h.exec("CREATE TABLE " + h.q("m_fp") + "(id INTEGER PRIMARY KEY)")
	q := "SELECT CAST(CREATE_TIME AS CHAR) FROM information_schema.TABLES WHERE TABLE_NAME='m_fp' AND TABLE_SCHEMA=" + sqlText(h.schema)
	before := h.text(q)
	h.exec("INSERT INTO " + h.q("m_fp") + " VALUES(1)")
	time.Sleep(1100 * time.Millisecond)
	mid := h.text(q)
	h.exec("TRUNCATE TABLE " + h.q("m_fp"))
	after := h.text(q)
	t.Logf("FINGERPRINT mysql CREATE_TIME before=%s afterInsert=%s afterTruncate=%s", before, mid, after)
	// L3: acknowledgement cannot be skipped for the connection database.
	reviewCurrentDatabase(t, h.e, h.c)
	// A connection without default database dropping a database is not "current".
	h.exec("CREATE DATABASE " + quoteIdentifier("mysql", "rv2_plain"))
	nodb := h.c
	nodb.Database = ""
	p, err := h.e.ObjectOpPlan(context.Background(), core.OpDatabaseDrop, core.ObjectOpRequest{Connection: nodb, Database: sp("rv2_plain")})
	if err != nil || p.Confirmation.CurrentDatabase {
		t.Fatalf("no default database: %v %#v", err, p.Confirmation)
	}
	h.exec("DROP DATABASE " + quoteIdentifier("mysql", "rv2_plain"))
}

func TestReview2ObjectOpsLive(t *testing.T) {
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
			create := "CREATE DATABASE "
			if c.Driver == "postgres" {
				c.Username, create = "postgres", "CREATE SCHEMA "
			}
			execute(t, e, c, create+quoteIdentifier(c.Driver, "rv2_main"))
			h := &opHarness{t: t, e: e, c: c, schema: "rv2_main"}
			if c.Driver == "postgres" {
				rv2Postgres(t, h)
			} else {
				rv2MySQL(t, h)
			}
		})
	}
}
