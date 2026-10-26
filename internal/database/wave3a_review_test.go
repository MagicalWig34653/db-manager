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

const rvSecretMarker = "rvSECRETmarker"

type w3 struct {
	t     *testing.T
	e     *Engine
	c     core.Connection
	maria bool
	name  string
}

func (r w3) exec(sql string) { r.t.Helper(); execute(r.t, r.e, r.c, sql) }
func (r w3) text(sql string) string {
	r.t.Helper()
	return queryText(r.t, r.e, r.c, sql)
}
func (r w3) as(user, pass string) core.Connection {
	c := r.c
	c.Username, c.Password = user, pass
	return c
}

func w3Live(t *testing.T, fn func(*testing.T, w3)) {
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
			fn(t, w3{t: t, e: e, c: c, maria: test.name == "maria", name: test.name})
		})
	}
}

func acctHost(r w3) string {
	if r.c.Driver == "postgres" {
		return ""
	}
	return "%"
}

func (r w3) aplan(action string, req core.AccountRequest) (core.AccountPlan, error) {
	req.Connection = r.c
	return r.e.AccountPlan(context.Background(), action, req)
}
func (r w3) arun(action string, req core.AccountRequest, secret *string, confirm core.AccountConfirm, sha string) (core.AccountResult, error) {
	req.Connection = r.c
	return r.e.AccountExecute(context.Background(), action, core.AccountExecuteRequest{AccountRequest: req, Secret: secret, Confirm: confirm, ExpectedPlanSHA256: sha})
}

func mustNotContain(t *testing.T, what string, v any, needles ...string) {
	t.Helper()
	b, _ := json.Marshal(v)
	s := string(b)
	if err, ok := v.(error); ok && err != nil {
		s = err.Error()
	}
	for _, n := range needles {
		if n != "" && strings.Contains(s, n) {
			t.Fatalf("%s leaks %q: %s", what, n, s)
		}
	}
}

func w3Accounts(t *testing.T, r w3) {
	host := acctHost(r)
	secrets := []string{`p'w"\x;--` + rvSecretMarker, "back\\slash\\'" + rvSecretMarker, "pässwörd😀" + rvSecretMarker, strings.Repeat("a", 240) + rvSecretMarker}
	for i, secret := range secrets {
		acc := core.AccountRef{Name: "rv_u" + strconv.Itoa(i), Host: host}
		req := core.AccountRequest{NewAccount: &acc}
		p, err := r.aplan("create", req)
		if err != nil {
			t.Fatalf("plan %d: %v", i, err)
		}
		mustNotContain(t, "plan", p, rvSecretMarker)
		if !strings.Contains(strings.Join(p.Statements, ";"), "<protected-secret>") {
			t.Fatalf("plan lacks placeholder: %v", p.Statements)
		}
		s := secret
		res, err := r.arun("create", req, &s, core.AccountConfirm{Action: "create"}, p.SHA256)
		mustNotContain(t, "result", res, rvSecretMarker)
		mustNotContain(t, "error", err, rvSecretMarker)
		if err != nil || res.Outcome != "completed" {
			t.Fatalf("create %d: %v %#v", i, err, res)
		}
		// The stored password equals the typed one: login works (proves quoting).
		login := r.as(acc.Name, secret)
		if r.c.Driver == "mysql" {
			login.Database = ""
		}
		if got, err := r.e.Query(context.Background(), core.QueryRequest{Connection: login, SQL: "SELECT 1"}); err != nil || len(got.Results) != 1 {
			t.Fatalf("login with secret %d failed: %v", i, err)
		}
		bad := r.as(acc.Name, secret+"x")
		if _, err := r.e.Query(context.Background(), core.QueryRequest{Connection: bad, SQL: "SELECT 1"}); err == nil {
			t.Fatalf("wrong password accepted %d", i)
		} else {
			mustNotContain(t, "login error", err, secret, rvSecretMarker)
		}
	}
	// Invalid secrets and field mismatches.
	acc := core.AccountRef{Name: "rv_bad", Host: host}
	req := core.AccountRequest{NewAccount: &acc}
	p, _ := r.aplan("create", req)
	for name, s := range map[string]string{"empty": "", "nul": "a\x00" + rvSecretMarker, "long": strings.Repeat("b", 4097) + rvSecretMarker, "badutf8": "\xff" + rvSecretMarker} {
		s := s
		res, err := r.arun("create", req, &s, core.AccountConfirm{Action: "create"}, p.SHA256)
		if err == nil || res.Outcome == "completed" {
			t.Fatalf("%s secret accepted", name)
		}
		mustNotContain(t, name, err, rvSecretMarker)
	}
	if res, err := r.arun("create", req, nil, core.AccountConfirm{Action: "create"}, p.SHA256); err == nil || res.Outcome == "completed" {
		t.Fatal("create without secret accepted")
	}
	// Server-side failure (no CREATE USER right): password must not appear anywhere in the result.
	r.exec(map[bool]string{true: "CREATE ROLE rv_lowpriv LOGIN PASSWORD 'rvlowpw'", false: "CREATE USER 'rv_lowpriv'@'%' IDENTIFIED BY 'rvlowpw'"}[r.c.Driver == "postgres"])
	low := r
	low.c = r.as("rv_lowpriv", "rvlowpw")
	lowPlan, err := low.aplan("create", core.AccountRequest{NewAccount: &core.AccountRef{Name: "rv_denied", Host: host}})
	if err != nil {
		t.Logf("low-priv plan refused: %v", err)
	} else {
		sec := "denied" + rvSecretMarker
		res, err := low.arun("create", core.AccountRequest{NewAccount: &core.AccountRef{Name: "rv_denied", Host: host}}, &sec, core.AccountConfirm{Action: "create"}, lowPlan.SHA256)
		mustNotContain(t, "denied result", res, rvSecretMarker)
		mustNotContain(t, "denied error", err, rvSecretMarker)
		if res.Outcome == "completed" || r.text(map[bool]string{true: "SELECT COUNT(*) FROM pg_roles WHERE rolname='rv_denied'", false: "SELECT COUNT(*) FROM mysql.user WHERE User='rv_denied'"}[r.c.Driver == "postgres"]) != "0" {
			t.Fatalf("low-priv create did something: %#v", res)
		}
		t.Logf("LOWPRIV create: outcome=%s reason=%s", res.Outcome, res.Reason)
		acc := low.accessActions()
		t.Logf("LOWPRIV access actions: %v", acc)
		if len(acc) != 0 {
			t.Fatalf("low-priv user is offered actions %v", acc)
		}
	}
	// Injection through account and grant identifiers.
	r.exec(map[bool]string{true: `CREATE ROLE rv_victim`, false: `CREATE USER 'rv_victim'@'%'`}[r.c.Driver == "postgres"])
	odd := core.AccountRef{Name: "x';DROP USER 'rv_victim'@'%';--", Host: host}
	if r.c.Driver == "postgres" {
		odd.Name = `x"; DROP ROLE rv_victim; --`
	}
	oreq := core.AccountRequest{NewAccount: &odd}
	op, err := r.aplan("create", oreq)
	if err != nil {
		t.Fatal(err)
	}
	s := "oddpw" + rvSecretMarker
	if res, err := r.arun("create", oreq, &s, core.AccountConfirm{Action: "create"}, op.SHA256); err != nil || res.Outcome != "completed" {
		t.Fatalf("odd create %v %#v", err, res)
	}
	vq := map[bool]string{true: "SELECT COUNT(*) FROM pg_roles WHERE rolname='rv_victim'", false: "SELECT COUNT(*) FROM mysql.user WHERE User='rv_victim'"}[r.c.Driver == "postgres"]
	if r.text(vq) != "1" {
		t.Fatal("victim account dropped via injection")
	}
	for _, g := range []core.AccountGrant{
		{Scope: "database", Database: "probe`; DROP USER 'rv_victim'@'%'; --", Privilege: "SELECT"},
		{Scope: "database", Database: "probe", Privilege: "SELECT; DROP USER rv_victim"},
		{Scope: "table", Database: "probe", Schema: map[bool]string{true: "public", false: ""}[r.c.Driver == "postgres"], Object: "t' OR '1'='1", Privilege: "SELECT"},
		{Scope: "global", Privilege: "ALL PRIVILEGES"},
		{Scope: "global", Privilege: "SELECT ON *.* TO 'rv_victim'@'%' WITH GRANT OPTION; --"},
	} {
		g := g
		if _, err := r.aplan("grant", core.AccountRequest{Account: &core.AccountRef{Name: "rv_u0", Host: host}, Grant: &g}); err == nil {
			t.Fatalf("grant %#v planned", g)
		}
	}
	if r.text(vq) != "1" {
		t.Fatal("victim changed")
	}
	// Self protection: every self-affecting action needs the second confirmation.
	self := core.AccountRef{Name: r.c.Username, Host: host}
	if r.c.Driver == "mysql" {
		self.Host = "%"
	}
	for _, action := range []string{"drop", "password", "lock", "rename", "revoke", "role_revoke"} {
		req := core.AccountRequest{Account: &self}
		switch action {
		case "rename":
			req.NewAccount = &core.AccountRef{Name: "rv_renamed_root", Host: self.Host}
		case "revoke":
			req.Grant = &core.AccountGrant{Scope: "database", Database: "probe", Privilege: "SELECT"}
		case "role_revoke":
			req.Role = &core.AccountRef{Name: "rv_role", Host: map[bool]string{true: "", false: "%"}[r.c.Driver == "postgres" || r.maria]}
			if r.c.Driver == "postgres" {
				r.exec("CREATE ROLE rv_role")
			}
		}
		pl, err := r.aplan(action, req)
		if err != nil {
			t.Logf("SELF %s plan refused: %v", action, err)
			continue
		}
		if !pl.Self || !pl.Confirmation.AcknowledgeSelf {
			t.Fatalf("SELF %s not flagged: self=%v", action, pl.Self)
		}
		var sec *string
		if action == "password" {
			x := "newpw" + rvSecretMarker
			sec = &x
		}
		res, err := r.arun(action, req, sec, core.AccountConfirm{Action: action, TargetName: pl.Confirmation.TargetName}, pl.SHA256)
		if err == nil || res.Outcome == "completed" {
			t.Fatalf("SELF %s executed without second confirmation: %#v", action, res)
		}
		mustNotContain(t, "self error", err, rvSecretMarker)
	}
	// Same user name, other host counts as self (name-based): user@localhost vs user@%.
	if r.c.Driver == "mysql" {
		other := core.AccountRef{Name: "root", Host: "localhost"}
		if pl, err := r.aplan("lock", core.AccountRequest{Account: &other}); err == nil {
			t.Logf("root@localhost lock plan self=%v", pl.Self)
		}
	}
	// Confirmation: wrong typed name, wrong hash, wrong action for a regular drop.
	dacc := core.AccountRef{Name: "rv_u1", Host: host}
	dp, err := r.aplan("drop", core.AccountRequest{Account: &dacc})
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		conf core.AccountConfirm
		sha  string
	}{
		"no name":   {core.AccountConfirm{Action: "drop", AcknowledgeSelf: true}, dp.SHA256},
		"upper":     {core.AccountConfirm{Action: "drop", TargetName: strings.ToUpper(dp.Confirmation.TargetName), AcknowledgeSelf: true}, dp.SHA256},
		"action":    {core.AccountConfirm{Action: "lock", TargetName: dp.Confirmation.TargetName, AcknowledgeSelf: true}, dp.SHA256},
		"zero hash": {core.AccountConfirm{Action: "drop", TargetName: dp.Confirmation.TargetName, AcknowledgeSelf: true}, strings.Repeat("0", 64)},
		"no ack":    {core.AccountConfirm{Action: "drop", TargetName: dp.Confirmation.TargetName}, dp.SHA256},
	} {
		res, err := r.arun("drop", core.AccountRequest{Account: &dacc}, nil, c.conf, c.sha)
		if err == nil || res.Outcome == "completed" {
			t.Fatalf("%s: drop executed", name)
		}
	}
	if r.text(map[bool]string{true: "SELECT COUNT(*) FROM pg_roles WHERE rolname='rv_u1'", false: "SELECT COUNT(*) FROM mysql.user WHERE User='rv_u1'"}[r.c.Driver == "postgres"]) != "1" {
		t.Fatal("account dropped by invalid confirmation")
	}
	// Host case: only exact host matches.
	if r.c.Driver == "mysql" {
		r.exec("CREATE USER 'rv_case'@'Host.Example'")
		for _, h := range []string{"host.example", "HOST.EXAMPLE"} {
			if _, err := r.aplan("lock", core.AccountRequest{Account: &core.AccountRef{Name: "rv_case", Host: h}}); err == nil {
				t.Logf("host spelling %q resolved (MySQL lowercases hosts at CREATE USER)", h)
			}
		}
	}
	// Grant escalation: privilege and option grants need no typed confirmation.
	tgt := core.AccountRef{Name: "rv_u2", Host: host}
	gl := core.AccountGrant{Scope: "database", Database: "probe", Privilege: "SELECT", GrantOption: true}
	if r.c.Driver == "postgres" {
		gl = core.AccountGrant{Scope: "database", Database: "probe", Privilege: "CONNECT", GrantOption: true}
	}
	gp, err := r.aplan("grant", core.AccountRequest{Account: &tgt, Grant: &gl})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ESCALATION grant WITH GRANT OPTION: destructive=%v typedTarget=%q warnings=%d statements=%v", gp.Destructive, gp.Confirmation.TargetName, len(gp.Warnings), gp.Statements)
	hasGrantOptionWarning := false
	for _, w := range gp.Warnings {
		hasGrantOptionWarning = hasGrantOptionWarning || strings.Contains(strings.ToUpper(w), "GRANT OPTION")
	}
	t.Logf("ESCALATION grant option warning present=%v", hasGrantOptionWarning)
	if r.c.Driver == "mysql" {
		for _, priv := range []string{"SUPER", "CREATE USER", "SET_USER_ID", "SYSTEM_USER", "SET_ANY_DEFINER", "ROLE_ADMIN"} {
			g := core.AccountGrant{Scope: "global", Privilege: priv}
			pp, err := r.aplan("grant", core.AccountRequest{Account: &tgt, Grant: &g})
			t.Logf("ESCALATION global %s: planned=%v err=%v destructive=%v", priv, err == nil, err, err == nil && pp.Destructive)
		}
	}
	// Executing as a user without GRANT OPTION must be refused by the server and change nothing.
	if lowPlan.SHA256 != "" || true {
		g := core.AccountGrant{Scope: "database", Database: "probe", Privilege: map[bool]string{true: "CONNECT", false: "SELECT"}[r.c.Driver == "postgres"]}
		lreq := core.AccountRequest{Account: &core.AccountRef{Name: "rv_u3", Host: host}, Grant: &g}
		if lp, err := low.aplan("grant", lreq); err == nil {
			res, err := low.arun("grant", lreq, nil, core.AccountConfirm{Action: "grant"}, lp.SHA256)
			if err == nil && res.Outcome == "completed" && r.c.Driver != "postgres" {
				t.Fatalf("low-priv grant executed: %#v", res)
			}
			t.Logf("LOWPRIV grant: err=%v outcome=%s reason=%s", err, res.Outcome, res.Reason)
		} else {
			t.Logf("LOWPRIV grant plan refused: %v", err)
		}
	}
	if r.c.Driver == "postgres" {
		r.exec("CREATE TABLE rv_gt (id int)")
		r.exec("GRANT SELECT ON rv_gt TO rv_lowpriv")
		tg := core.AccountGrant{Scope: "table", Database: "probe", Schema: "public", Object: "rv_gt", Privilege: "SELECT"}
		treq := core.AccountRequest{Account: &core.AccountRef{Name: "rv_u3"}, Grant: &tg}
		if lp, err := low.aplan("grant", treq); err == nil {
			res, err := low.arun("grant", treq, nil, core.AccountConfirm{Action: "grant"}, lp.SHA256)
			got := r.text("SELECT has_table_privilege('rv_u3','rv_gt','SELECT')")
			t.Logf("FINDING PG silent no-op grant: outcome=%s err=%v actually_granted=%s", res.Outcome, err, got)
		}
	}
	// System accounts.
	sys := map[string][]core.AccountRef{"mysql": {{Name: "mysql.sys", Host: "localhost"}, {Name: "mysql.session", Host: "localhost"}, {Name: "mysql.infoschema", Host: "localhost"}}, "postgres": {{Name: "pg_monitor"}, {Name: "pg_read_all_data"}}}[r.c.Driver]
	if r.maria {
		sys = []core.AccountRef{{Name: "mariadb.sys", Host: "localhost"}}
	}
	for _, a := range sys {
		a := a
		pl, err := r.aplan("drop", core.AccountRequest{Account: &a})
		if err != nil {
			t.Logf("SYSTEM %s drop plan refused: %v", a.Name, err)
			continue
		}
		res, err := r.arun("drop", core.AccountRequest{Account: &a}, nil, core.AccountConfirm{Action: "drop", TargetName: pl.Confirmation.TargetName, AcknowledgeSelf: true}, pl.SHA256)
		t.Logf("SYSTEM %s drop: planned self=%v typed=%q executed outcome=%s reason=%s err=%v", a.Name, pl.Self, pl.Confirmation.TargetName, res.Outcome, res.Reason, err)
	}
}

func (r w3) accessActions() []string {
	a, err := r.e.AccountAccess(context.Background(), core.AccountRequest{Connection: r.c})
	if err != nil {
		r.t.Fatal(err)
	}
	return a.Actions
}

func TestReviewW3Accounts(t *testing.T) { w3Live(t, w3Accounts) }

// ---- programs ----

func (r w3) pplan(req core.ProgramChangeRequest) (core.ProgramPlan, error) {
	req.Connection = r.c
	return r.e.ProgramPlan(context.Background(), req)
}
func (r w3) prun(req core.ProgramChangeRequest, confirm dbplan.Confirmation, sha string) (core.ProgramResult, error) {
	req.Connection = r.c
	return r.e.ProgramExecute(context.Background(), core.ProgramExecuteRequest{ProgramChangeRequest: req, Confirm: confirm, ExpectedPlanSHA256: sha})
}

// pdo plans and executes with the correct confirmation.
func (r w3) pdo(req core.ProgramChangeRequest) (core.ProgramResult, core.ProgramPlan, error) {
	p, err := r.pplan(req)
	if err != nil {
		return core.ProgramResult{}, p, err
	}
	typed := ""
	if p.Destructive {
		typed = p.Object.Name
	}
	res, err := r.prun(req, dbplan.Confirmation{Action: req.Action, TargetName: typed}, p.SHA256)
	return res, p, err
}

func (r w3) tableExists(name string) bool {
	q := "SELECT COUNT(*) FROM sqlite_schema WHERE name='" + name + "'"
	switch r.c.Driver {
	case "mysql":
		q = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='" + name + "'"
	case "postgres":
		q = "SELECT COUNT(*) FROM pg_tables WHERE tablename='" + name + "' AND schemaname=current_schema()"
	}
	return r.text(q) == "1"
}

func pv(name string) core.ProgramRef { return core.ProgramRef{Kind: "view", Name: name} }

func w3Programs(t *testing.T, r w3) {
	driver := r.c.Driver
	r.exec("CREATE TABLE rv_victim (id INTEGER PRIMARY KEY, v TEXT)")
	r.exec("INSERT INTO rv_victim VALUES (1,'keep')")
	smuggles := []string{
		"SELECT 1 AS x; DROP TABLE rv_victim",
		"SELECT 1 AS x;\nDROP TABLE rv_victim;",
		"SELECT 1 AS x /* c */; DROP TABLE rv_victim -- tail",
		"DELIMITER //\nSELECT 1 AS x//",
		"SELECT 1 AS x\x00; DROP TABLE rv_victim",
	}
	for i, body := range smuggles {
		for _, kind := range []string{"view"} {
			req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: kind, Name: "rv_s" + strconv.Itoa(i)}, Input: &core.ProgramInput{Body: body}}
			res, _, err := r.pdo(req)
			if err == nil && res.Outcome == "completed" {
				t.Fatalf("SMUGGLE view body %q completed", body)
			}
			if !r.tableExists("rv_victim") || r.text("SELECT COUNT(*) FROM rv_victim") != "1" {
				t.Fatalf("SMUGGLE view body %q damaged rv_victim", body)
			}
		}
	}
	// routines, triggers, events: second statement after compound body
	switch driver {
	case "mysql":
		for i, body := range []string{"BEGIN SELECT 1; END; DROP TABLE rv_victim", "SELECT 1; DROP TABLE rv_victim"} {
			for _, kind := range []string{"procedure"} {
				req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: kind, Name: "rv_p" + strconv.Itoa(i)}, Input: &core.ProgramInput{Body: body}}
				res, _, err := r.pdo(req)
				if err == nil && res.Outcome == "completed" {
					t.Fatalf("SMUGGLE %s %q completed", kind, body)
				}
			}
			req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "trigger", Name: "rv_t" + strconv.Itoa(i), Table: &core.TableRef{Name: "rv_victim"}}, Input: &core.ProgramInput{Timing: "after", Event: "insert", Body: body}}
			res, _, err := r.pdo(req)
			if err == nil && res.Outcome == "completed" {
				t.Fatalf("SMUGGLE trigger %q completed", body)
			}
			ev := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "event", Name: "rv_e" + strconv.Itoa(i)}, Input: &core.ProgramInput{Body: body, Status: "disabled", Schedule: &core.ProgramSchedule{At: "2099-01-01 00:00:00"}}}
			res, _, err = r.pdo(ev)
			if err == nil && res.Outcome == "completed" {
				t.Fatalf("SMUGGLE event %q completed", body)
			}
		}
		if !r.tableExists("rv_victim") {
			t.Fatal("rv_victim dropped through routine/trigger/event body")
		}
	case "postgres":
		// trigger function body is a string literal; a function body with $$ and quotes stays inert.
		body := "BEGIN RAISE NOTICE '$$ it''s ;DROP TABLE rv_victim;--'; RETURN NEW; END;"
		req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "function", Name: "rv_f"}, Input: &core.ProgramInput{Body: body, Language: "plpgsql", Returns: "trigger"}}
		res, _, err := r.pdo(req)
		t.Logf("PG $$ function body: outcome=%s err=%v", res.Outcome, err)
		if err != nil || res.Outcome != "completed" || !r.tableExists("rv_victim") {
			t.Fatalf("pg function body: %v %#v", err, res)
		}
		if got := r.text("SELECT prosrc FROM pg_proc WHERE proname='rv_f'"); got != body {
			t.Fatalf("pg body not stored verbatim: %q", got)
		}
		// SQL-language function: a second statement must be inert or rejected (SQL functions may legally contain several statements).
		req = core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "function", Name: "rv_f2"}, Input: &core.ProgramInput{Body: "SELECT 1; DROP TABLE rv_victim;", Language: "sql", Returns: "integer"}}
		res, _, err = r.pdo(req)
		t.Logf("PG sql-function with second statement: outcome=%s err=%v victim_exists=%v (statement runs only when the function is called)", res.Outcome, err, r.tableExists("rv_victim"))
		if !r.tableExists("rv_victim") {
			t.Fatal("rv_victim dropped at creation")
		}
		// view smuggle checked above; trigger with function in other schema is catalog-resolved
		bad := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "trigger", Name: "rv_tr", Table: &core.TableRef{Name: "nonexistent_table"}}, Input: &core.ProgramInput{Timing: "after", Event: "insert", Function: &core.ProgramRef{Kind: "function", Name: "rv_f"}}}
		if _, err := r.pplan(bad); err == nil {
			t.Fatal("trigger on nonexistent table planned")
		}
	}

	// Confirmation matrix on replace/drop of a view.
	r.exec("CREATE VIEW rv_v1 AS SELECT id FROM rv_victim")
	r.exec("CREATE VIEW rv_v2 AS SELECT id FROM rv_victim")
	rep := core.ProgramChangeRequest{Action: "replace", Object: pv("rv_v1"), Input: &core.ProgramInput{Body: "SELECT id, v FROM rv_victim"}}
	if driver == "sqlite" {
		rep.Action = "drop"
		rep.Input = nil
	}
	p, err := r.pplan(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Destructive {
		t.Fatalf("%s of existing view not destructive", rep.Action)
	}
	other := rep
	other.Object = pv("rv_v2")
	op, _ := r.pplan(other)
	for name, c := range map[string]struct {
		conf dbplan.Confirmation
		sha  string
	}{
		"no name":   {dbplan.Confirmation{Action: rep.Action}, p.SHA256},
		"upper":     {dbplan.Confirmation{Action: rep.Action, TargetName: "RV_V1"}, p.SHA256},
		"space":     {dbplan.Confirmation{Action: rep.Action, TargetName: "rv_v1 "}, p.SHA256},
		"other":     {dbplan.Confirmation{Action: rep.Action, TargetName: "rv_v2"}, p.SHA256},
		"action":    {dbplan.Confirmation{Action: "create", TargetName: "rv_v1"}, p.SHA256},
		"swap plan": {dbplan.Confirmation{Action: rep.Action, TargetName: "rv_v1"}, op.SHA256},
		"zero":      {dbplan.Confirmation{Action: rep.Action, TargetName: "rv_v1"}, strings.Repeat("0", 64)},
		"upper sha": {dbplan.Confirmation{Action: rep.Action, TargetName: "rv_v1"}, strings.ToUpper(p.SHA256)},
	} {
		res, err := r.prun(rep, c.conf, c.sha)
		if err == nil && res.Outcome == "completed" {
			t.Fatalf("%s: executed", name)
		}
	}
	if r.text("SELECT COUNT(*) FROM rv_v1") != "1" {
		t.Fatal("view altered")
	}
	// Create over an existing object must not replace.
	dup := core.ProgramChangeRequest{Action: "create", Object: pv("rv_v1"), Input: &core.ProgramInput{Body: "SELECT 1 AS id"}}
	if res, _, err := r.pdo(dup); err == nil && res.Outcome == "completed" {
		t.Fatalf("create replaced existing view")
	}
	// Dependencies with backtick/quote spellings.
	weird := "rv`w\"v"
	if driver == "postgres" || driver == "sqlite" {
		weird = `rv"w'v`
	}
	r.exec("CREATE VIEW " + quoteIdentifier(driver, weird) + " AS SELECT id FROM rv_victim")
	r.exec("CREATE VIEW rv_dep AS SELECT id FROM " + quoteIdentifier(driver, weird))
	dp, err := r.pplan(core.ProgramChangeRequest{Action: "drop", Object: pv(weird)})
	if err != nil {
		t.Fatal(err)
	}
	foundDep := false
	for _, d := range dp.Dependencies {
		foundDep = foundDep || d.Object.Name == "rv_dep" && d.Direction == "incoming"
	}
	if !foundDep {
		t.Fatalf("incoming dependency on %q missed: %#v", weird, dp.Dependencies)
	}

	// Definer / security warnings.
	if driver == "mysql" {
		in := &core.ProgramInput{Body: "SELECT id FROM rv_victim", Security: "definer", Definer: &core.ProgramDefiner{Name: "root", Host: "%"}}
		dpl, err := r.pplan(core.ProgramChangeRequest{Action: "create", Object: pv("rv_def"), Input: in})
		if err != nil || !strings.Contains(strings.Join(dpl.Warnings, " "), "Privilegierte Aktion") {
			t.Fatalf("definer plan: %v %v", err, dpl.Warnings)
		}
		// low-priv user must not be able to impersonate root as definer.
		r.exec("CREATE USER 'rv_dev'@'%' IDENTIFIED BY 'rvdevpw'")
		r.exec("GRANT CREATE VIEW, SELECT, SHOW VIEW ON probe.* TO 'rv_dev'@'%'")
		dev := r
		dev.c = r.as("rv_dev", "rvdevpw")
		req := core.ProgramChangeRequest{Action: "create", Object: pv("rv_def"), Input: in}
		res, _, err := dev.pdo(req)
		t.Logf("DEFINER escalation by low-priv: err=%v outcome=%s reason=%s", err, res.Outcome, res.Reason)
		if err == nil && res.Outcome == "completed" {
			t.Fatalf("low-priv user created a view with DEFINER=root")
		}
		if _, _, err := r.pdo(core.ProgramChangeRequest{Action: "create", Object: pv("rv_def2"), Input: &core.ProgramInput{Body: "SELECT 1", Definer: &core.ProgramDefiner{Name: "nobody", Host: "%"}}}); err == nil {
			t.Fatal("nonexistent definer accepted")
		}
		// Replace losing the object (DROP+CREATE): function with invalid new body.
		fn := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "function", Name: "rv_fn"}, Input: &core.ProgramInput{Body: "RETURN 1", Returns: "integer", Deterministic: true}}
		if res, _, err := r.pdo(fn); err != nil || res.Outcome != "completed" {
			t.Fatalf("create fn: %v %#v", err, res)
		}
		fn.Action = "replace"
		fn.Input = &core.ProgramInput{Body: "RETURN (((", Returns: "integer", Deterministic: true}
		res, rp, err := r.pdo(fn)
		exists := r.text("SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME='rv_fn'")
		t.Logf("FINDING replace with invalid body: err=%v outcome=%s executed=%d/%d routine_still_exists=%s statements=%v", err, res.Outcome, res.StatementsExecuted, res.StatementsTotal, exists, rp.Statements)
		// Events: scheduler is only read.
		before := r.text("SELECT @@GLOBAL.event_scheduler")
		evreq := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "event", Name: "rv_ev"}, Input: &core.ProgramInput{Body: "SELECT 1", Status: "disabled", Schedule: &core.ProgramSchedule{At: "2099-01-01 00:00:00"}}}
		if res, _, err := r.pdo(evreq); err != nil || res.Outcome != "completed" {
			t.Fatalf("event create: %v %#v", err, res)
		}
		if after := r.text("SELECT @@GLOBAL.event_scheduler"); after != before {
			t.Fatalf("scheduler changed %s -> %s", before, after)
		}
		if r.text("SELECT STATUS FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE() AND EVENT_NAME='rv_ev'") != "DISABLED" {
			t.Fatal("disabled event not stored disabled")
		}
	}
	if driver == "postgres" {
		fn := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "function", Name: "rv_sd"}, Input: &core.ProgramInput{Body: "SELECT 1", Language: "sql", Returns: "integer", Security: "definer"}}
		sp, err := r.pplan(fn)
		if err != nil {
			t.Fatal(err)
		}
		w := strings.Join(sp.Warnings, " | ")
		t.Logf("PG SECURITY DEFINER statements=%v", sp.Statements)
		t.Logf("PG SECURITY DEFINER warnings mention search_path=%v mention PUBLIC=%v", strings.Contains(strings.ToLower(w), "search_path"), strings.Contains(w, "PUBLIC"))
		// Overloads and identity: replace targets exactly one overload; a hostile identity is not interpolated.
		r.exec("CREATE FUNCTION rv_ov(a integer) RETURNS integer LANGUAGE sql AS 'SELECT 1'")
		r.exec("CREATE FUNCTION rv_ov(a text) RETURNS integer LANGUAGE sql AS 'SELECT 2'")
		for _, id := range []string{"a integer); DROP TABLE rv_victim; --", "a integer", "a text) ,", ""} {
			req := core.ProgramChangeRequest{Action: "drop", Object: core.ProgramRef{Kind: "function", Name: "rv_ov", Identity: id}}
			pl, err := r.pplan(req)
			t.Logf("identity %q -> err=%v statements=%v", id, err, pl.Statements)
			if err == nil && strings.Contains(strings.Join(pl.Statements, " "), "DROP TABLE") {
				t.Fatalf("identity injected: %v", pl.Statements)
			}
		}
		req := core.ProgramChangeRequest{Action: "drop", Object: core.ProgramRef{Kind: "function", Name: "rv_ov", Identity: "a integer"}}
		if res, _, err := r.pdo(req); err != nil || res.Outcome != "completed" {
			t.Fatalf("drop overload: %v %#v", err, res)
		}
		if r.text("SELECT COUNT(*) FROM pg_proc WHERE proname='rv_ov'") != "1" || !r.tableExists("rv_victim") {
			t.Fatal("drop touched another overload or table")
		}
	}
	// No routine is ever called by the API.
	if driver == "mysql" {
		r.exec("CREATE TABLE rv_calls (n INT)")
		if res, _, err := r.pdo(core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "procedure", Name: "rv_called"}, Input: &core.ProgramInput{Body: "INSERT INTO rv_calls VALUES (1)"}}); err != nil || res.Outcome != "completed" {
			t.Fatalf("create procedure: %v %#v", err, res)
		}
		r.e.Programs(context.Background(), core.ProgramsRequest{Connection: r.c, Kind: "procedure"})
		if _, err := r.pplan(core.ProgramChangeRequest{Action: "drop", Object: core.ProgramRef{Kind: "procedure", Name: "rv_called"}}); err != nil {
			t.Fatal(err)
		}
		if r.text("SELECT COUNT(*) FROM rv_calls") != "0" {
			t.Fatal("routine was executed")
		}
	}
}

func TestReviewW3ProgramsLive(t *testing.T) { w3Live(t, w3Programs) }

func TestReviewW3ProgramsSQLite(t *testing.T) {
	w3Programs(t, w3{t: t, e: testEngine(t), c: core.Connection{Driver: "sqlite", Database: "rvprog"}})
	r := w3{t: t, e: testEngine(t), c: core.Connection{Driver: "sqlite", Database: "rvprog2"}}
	r.exec("CREATE TABLE rv_victim (id INTEGER PRIMARY KEY, v TEXT)")
	r.exec("INSERT INTO rv_victim VALUES (1,'keep')")
	for _, body := range []string{"BEGIN SELECT 1; END; DROP TABLE rv_victim", "BEGIN UPDATE rv_victim SET v='x'; END; DROP TABLE rv_victim;"} {
		req := core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: "trigger", Name: "rv_tg", Table: &core.TableRef{Name: "rv_victim"}}, Input: &core.ProgramInput{Timing: "after", Event: "insert", Body: body}}
		res, _, err := r.pdo(req)
		if err == nil && res.Outcome == "completed" {
			t.Fatalf("SMUGGLE sqlite trigger %q completed", body)
		}
		if r.text("SELECT COUNT(*) FROM sqlite_schema WHERE name='rv_victim'") != "1" {
			t.Fatalf("sqlite trigger body %q dropped rv_victim", body)
		}
	}
	for _, kind := range []string{"function", "procedure", "event"} {
		if _, err := r.pplan(core.ProgramChangeRequest{Action: "create", Object: core.ProgramRef{Kind: kind, Name: "rv_x"}, Input: &core.ProgramInput{Body: "SELECT 1", Returns: "integer"}}); err == nil {
			t.Fatalf("sqlite %s planned", kind)
		}
	}
	// sandbox: dangerous statements inside view/trigger bodies
	for _, body := range []string{"SELECT load_extension('x')", "SELECT readfile('/etc/passwd')"} {
		req := core.ProgramChangeRequest{Action: "create", Object: pv("rv_sb"), Input: &core.ProgramInput{Body: body}}
		res, _, err := r.pdo(req)
		if err == nil && res.Outcome == "completed" {
			_, qerr := r.e.Query(context.Background(), core.QueryRequest{Connection: r.c, SQL: "SELECT * FROM rv_sb"})
			t.Logf("SANDBOX view %q created; selecting: err=%v", body, qerr)
			r.exec("DROP VIEW rv_sb")
		}
	}
}
