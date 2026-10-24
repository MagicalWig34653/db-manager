// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func sqlText(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func objectOpDatabaseCases(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	h := &opHarness{t: t, e: e, c: c}
	driver := c.Driver
	exists := func(name string) bool {
		t.Helper()
		query := "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = " + sqlText(name)
		if driver == "postgres" {
			query = "SELECT COUNT(*) FROM pg_database WHERE datname = " + sqlText(name)
		}
		return h.text(query) == "1"
	}
	collationOf := func(name string) string {
		query := "SELECT DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = " + sqlText(name)
		if driver == "postgres" {
			query = "SELECT datcollate FROM pg_database WHERE datname = " + sqlText(name)
		}
		return h.text(query)
	}
	dbReq := func(name string) core.ObjectOpRequest { return core.ObjectOpRequest{Database: sp(name)} }
	keepDB := "p07_keepdb"
	h.exec("CREATE DATABASE " + quoteIdentifier(driver, keepDB))
	keepCollation := collationOf(keepDB)
	name := "p07 odd`db\"x"
	collation := map[string]string{"mysql": "utf8mb4_bin", "postgres": "C"}[driver]

	// Creation: validation, collation from the catalog, independent read-back.
	for _, reserved := range map[string][]string{"mysql": {"mysql", "information_schema"}, "postgres": {"postgres", "template1"}}[driver] {
		_, err := h.plan(core.OpDatabaseCreate, dbReq(reserved))
		h.wantCode(err, "invalid_object_op")
	}
	_, err := h.plan(core.OpDatabaseCreate, core.ObjectOpRequest{Database: sp(name), Collation: sp("no_such_collation")})
	h.wantCode(err, "invalid_object_op")
	_, err = h.plan(core.OpDatabaseCreate, core.ObjectOpRequest{Database: sp(name), Table: &core.TableRef{Name: "x"}})
	h.wantCode(err, "invalid_object_op")
	_, err = h.plan(core.OpDatabaseCreate, dbReq(keepDB))
	h.wantCode(err, "object_exists")
	req := core.ObjectOpRequest{Database: sp(name), Collation: sp(collation)}
	p, res := h.run(core.OpDatabaseCreate, req)
	h.completed(res)
	dumpObjectOp(t, "plan-database", p)
	dumpObjectOp(t, "result-database", res)
	if !exists(name) || collationOf(name) != collation || p.Confirmation.Destructive || res.Observation == nil || res.Observation.TargetExists == nil || !*res.Observation.TargetExists {
		t.Fatalf("create %#v %#v", p, res)
	}
	if res.Observation.Collation == nil || *res.Observation.Collation != collation {
		t.Fatalf("observed collation %#v", res.Observation)
	}

	// Collation change (MySQL/MariaDB only).
	if driver == "mysql" {
		_, res = h.run(core.OpDatabaseCollation, core.ObjectOpRequest{Database: sp(name), Collation: sp("utf8mb4_general_ci")})
		h.completed(res)
		if collationOf(name) != "utf8mb4_general_ci" || collationOf(keepDB) != keepCollation {
			t.Fatal("collation change")
		}
	} else {
		_, err = h.plan(core.OpDatabaseCollation, core.ObjectOpRequest{Database: sp(name), Collation: sp("C")})
		h.wantCode(err, "unsupported_action")
	}

	// Drop: typed name, absent and system databases, only the target disappears.
	dp := h.mustPlan(core.OpDatabaseDrop, dbReq(name))
	if !dp.Confirmation.Destructive || dp.Confirmation.TargetName != name || dp.Confirmation.CurrentDatabase {
		t.Fatalf("drop plan %#v", dp)
	}
	_, err = h.execute(core.OpDatabaseDrop, dbReq(name), dp, core.ObjectOpConfirm{Action: core.OpDatabaseDrop, TargetName: keepDB})
	h.wantCode(err, "confirmation_mismatch")
	_, err = h.execute(core.OpDatabaseDrop, dbReq(name), dp, core.ObjectOpConfirm{Action: core.OpDatabaseDrop})
	h.wantCode(err, "confirmation_mismatch")
	if !exists(name) || !exists(keepDB) {
		t.Fatal("refused drop changed databases")
	}
	_, err = h.plan(core.OpDatabaseDrop, dbReq("p07_absent"))
	h.wantCode(err, "object_not_found")
	_, err = h.plan(core.OpDatabaseDrop, dbReq(name+"`; DROP DATABASE "+keepDB))
	h.wantCode(err, "object_not_found")
	for _, system := range map[string][]string{"mysql": {"mysql", "sys", "information_schema"}, "postgres": {"postgres", "template1"}}[driver] {
		_, err = h.plan(core.OpDatabaseDrop, dbReq(system))
		h.wantCode(err, "invalid_object_op")
	}
	_, res = h.run(core.OpDatabaseDrop, dbReq(name))
	h.completed(res)
	dumpObjectOp(t, "result-database-drop", res)
	if exists(name) || !exists(keepDB) || res.Observation == nil || res.Observation.TargetExists == nil || *res.Observation.TargetExists {
		t.Fatalf("drop %#v", res)
	}

	if driver == "postgres" {
		// The connection database cannot be dropped by PostgreSQL itself.
		_, err = h.plan(core.OpDatabaseDrop, dbReq(c.Database))
		h.wantCode(err, "invalid_object_op")
	} else {
		// The connection database needs the second confirmation.
		h.exec("CREATE DATABASE p07_cur")
		current := c
		current.Database = "p07_cur"
		ch := &opHarness{t: t, e: e, c: current}
		cp := ch.mustPlan(core.OpDatabaseDrop, dbReq("p07_cur"))
		if !cp.Confirmation.CurrentDatabase {
			t.Fatalf("current database not flagged: %#v", cp)
		}
		_, err = ch.execute(core.OpDatabaseDrop, dbReq("p07_cur"), cp, core.ObjectOpConfirm{Action: core.OpDatabaseDrop, TargetName: "p07_cur"})
		ch.wantCode(err, "confirmation_mismatch")
		if !exists("p07_cur") {
			t.Fatal("dropped without second confirmation")
		}
		res, err = ch.execute(core.OpDatabaseDrop, dbReq("p07_cur"), cp, core.ObjectOpConfirm{Action: core.OpDatabaseDrop, TargetName: "p07_cur", AcknowledgeCurrentDatabase: true})
		if err != nil {
			t.Fatal(err)
		}
		h.completed(res)
		if exists("p07_cur") {
			t.Fatal("current database still exists")
		}
		// A foreign key from another database is reported as a dependency.
		h.exec("CREATE DATABASE p07_fka")
		h.exec("CREATE DATABASE p07_fkb")
		h.exec("CREATE TABLE p07_fka.parent (id INT PRIMARY KEY) ENGINE=InnoDB")
		h.exec("CREATE TABLE p07_fkb.child (id INT PRIMARY KEY, pid INT, FOREIGN KEY (pid) REFERENCES p07_fka.parent (id)) ENGINE=InnoDB")
		fp := h.mustPlan(core.OpDatabaseDrop, dbReq("p07_fka"))
		if len(fp.Dependencies) != 1 || fp.Dependencies[0].Schema != "p07_fkb" || fp.Dependencies[0].Effect != "blocks" || len(fp.AffectedObjects) < 2 {
			t.Fatalf("cross database dependency %#v", fp)
		}
		_, res = h.run(core.OpDatabaseDrop, dbReq("p07_fka"))
		h.rejected(res, "dependency_blocks")
		if !exists("p07_fka") {
			t.Fatal("blocked database drop removed the database")
		}
	}
}

func TestLiveObjectOps(t *testing.T) {
	for _, test := range []struct{ name, driver, variable string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.variable)
			if raw == "" {
				t.Skip("W-P07 disposable TLS fixture required")
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
			schema := "p07_main"
			create := "CREATE SCHEMA "
			if c.Driver == "mysql" {
				create = "CREATE DATABASE "
			}
			execute(t, e, c, create+quoteIdentifier(c.Driver, schema))
			objectOpCases(t, e, c, schema)
			objectOpDatabaseCases(t, e, c)
			objectOpRights(t, e, c, schema)
		})
	}
}

// objectOpRights proves that missing rights surface as a rejection and that
// nothing is changed; the interface never pretends to protect against them.
func objectOpRights(t *testing.T, e *Engine, c core.Connection, schema string) {
	t.Helper()
	admin := &opHarness{t: t, e: e, c: c, schema: schema}
	limited := c
	limited.Username, limited.Password = "p07_ro", "public-p07-fixture"
	if c.Driver == "mysql" {
		admin.exec("CREATE USER 'p07_ro'@'%' IDENTIFIED BY 'public-p07-fixture'")
		admin.exec("GRANT SELECT ON " + quoteIdentifier("mysql", schema) + ".* TO 'p07_ro'@'%'")
		limited.Database = schema // the user holds no right on the connection database of the fixture
	} else {
		admin.exec("CREATE ROLE p07_ro LOGIN PASSWORD 'public-p07-fixture'")
		admin.exec("GRANT USAGE ON SCHEMA " + quoteIdentifier("postgres", schema) + " TO p07_ro")
		admin.exec("GRANT SELECT ON ALL TABLES IN SCHEMA " + quoteIdentifier("postgres", schema) + " TO p07_ro")
	}
	h := &opHarness{t: t, e: e, c: limited, schema: schema}
	for _, test := range []struct {
		action string
		req    core.ObjectOpRequest
	}{
		{core.OpTableDrop, core.ObjectOpRequest{Table: h.ref("keep")}},
		{core.OpTableTruncate, core.ObjectOpRequest{Table: h.ref("keep")}},
		{core.OpTableRename, core.ObjectOpRequest{Table: h.ref("keep"), NewName: sp("keep_renamed")}},
		{core.OpTableCopy, core.ObjectOpRequest{Table: h.ref("keep"), NewName: sp("keep_copy"), Mode: sp("structure")}},
		{core.OpDatabaseCreate, core.ObjectOpRequest{Database: sp("p07_denied")}},
	} {
		_, res := h.run(test.action, test.req)
		h.rejected(res, "permission_denied")
		if strings.Contains(res.Message, "p07_ro") || strings.Contains(res.Message, "public-p07-fixture") {
			t.Fatalf("message leaks %q", res.Message)
		}
	}
	if !admin.has("keep") || admin.has("keep_renamed") || admin.has("keep_copy") || admin.count("keep") != "2" {
		t.Fatal("a refused action changed the fixture")
	}
}
