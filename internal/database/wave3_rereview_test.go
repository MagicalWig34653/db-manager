// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// Second independent review of wave 3 (re-review). Live parts need the dbfix environment.

func TestRereviewW3Accounts(t *testing.T) {
	w3Live(t, func(t *testing.T, r w3) {
		host := acctHost(r)
		// privileged plans need the typed name
		grants := []core.AccountGrant{{Scope: "database", Database: "probe", Privilege: "CONNECT", GrantOption: true}}
		if r.c.Driver == "mysql" {
			grants[0].Privilege = "SELECT"
		}
		for _, g := range grants {
			acc := core.AccountRef{Name: "rr_u1", Host: host}
			p0, err := r.aplan("create", core.AccountRequest{NewAccount: &acc})
			if err != nil {
				t.Fatal(err)
			}
			s := "rrpw-1"
			if res, err := r.arun("create", core.AccountRequest{NewAccount: &acc}, &s, core.AccountConfirm{Action: "create"}, p0.SHA256); err != nil || res.Outcome != "completed" {
				t.Fatalf("create: %v %#v", err, res)
			}
			gp, err := r.aplan("grant", core.AccountRequest{Account: &acc, Grant: &g})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("grant-option plan destructive=%v warnings=%d", gp.Destructive, len(gp.Warnings))
			if !gp.Destructive {
				t.Fatalf("GRANT OPTION plan not privileged")
			}
		}
		if r.c.Driver == "mysql" {
			acc := core.AccountRef{Name: "rr_u1", Host: host}
			gp, err := r.aplan("grant", core.AccountRequest{Account: &acc, Grant: &core.AccountGrant{Scope: "global", Privilege: "SUPER"}})
			if err != nil || !gp.Destructive {
				t.Fatalf("SUPER grant not privileged: %v", err)
			}
			// mixed-case / other-host system accounts
			for _, a := range []core.AccountRef{{Name: "mysql.sys", Host: "localhost"}, {Name: "mysql.sys", Host: "%"}, {Name: "mysql.infoschema", Host: "localhost"}, {Name: "mariadb.sys", Host: "localhost"}} {
				for _, act := range []string{"drop", "lock", "password", "grant", "revoke"} {
					a := a
					req := core.AccountRequest{Account: &a}
					if act == "grant" || act == "revoke" {
						req.Grant = &core.AccountGrant{Scope: "database", Database: "probe", Privilege: "SELECT"}
					}
					if _, err := r.aplan(act, req); err == nil {
						t.Errorf("system account %v action %s planned", a, act)
					}
				}
			}
			// rename onto / from system name
			from := core.AccountRef{Name: "rr_u1", Host: host}
			to := core.AccountRef{Name: "mysql.sys", Host: "localhost"}
			if _, err := r.aplan("rename", core.AccountRequest{Account: &from, NewAccount: &to}); err == nil {
				t.Errorf("rename onto mysql.sys planned")
			}
			// create with system name
			if _, err := r.aplan("create", core.AccountRequest{NewAccount: &core.AccountRef{Name: "mariadb.sys", Host: "localhost"}}); err == nil {
				t.Logf("NOTE: create of existing system name planned (server rejects as exists)")
			}
			// multibyte name 20 chars (40 bytes) on MySQL
			mb := core.AccountRef{Name: strings.Repeat("ü", 20), Host: host}
			_, err = r.aplan("create", core.AccountRequest{NewAccount: &mb})
			t.Logf("20-char multibyte name plan err=%v", err)
		} else {
			for _, n := range []string{"pg_monitor", "pg_read_all_data", "PG_signal_backend"} {
				a := core.AccountRef{Name: n}
				for _, act := range []string{"drop", "password", "lock"} {
					if _, err := r.aplan(act, core.AccountRequest{Account: &a}); err == nil {
						t.Errorf("pg system role %s %s planned", n, act)
					}
				}
			}
			// role_grant of a pg_ role as the *role* operand
			acc := core.AccountRef{Name: "rr_u1"}
			role := core.AccountRef{Name: "pg_read_all_data"}
			rp, err := r.aplan("role_grant", core.AccountRequest{Account: &acc, Role: &role})
			t.Logf("role_grant pg_read_all_data err=%v destructive=%v", err, func() bool { return rp.Destructive }())
		}
	})
}

// PostgreSQL: roles created with a non-NFKC password; the python wrapper logs in with libpq.
func TestRereviewW3ScramSASLprep(t *testing.T) {
	w3Live(t, func(t *testing.T, r w3) {
		if r.c.Driver != "postgres" {
			t.Skip()
		}
		cases := map[string]string{"rr_sp1": "ﬁx①ｗ" + "pw", "rr_sp2": "plain-ascii-pw", "rr_sp3": "café-pw", "rr_sp4": "café-pw"}
		for name, secret := range cases {
			acc := core.AccountRef{Name: name}
			p, err := r.aplan("create", core.AccountRequest{NewAccount: &acc})
			if err != nil {
				t.Fatal(err)
			}
			s := secret
			if res, err := r.arun("create", core.AccountRequest{NewAccount: &acc}, &s, core.AccountConfirm{Action: "create"}, p.SHA256); err != nil || res.Outcome != "completed" {
				t.Fatalf("%s: %v %#v", name, err, res)
			}
			// pgx does not apply SASLprep itself, so compare with what the server derives: let PostgreSQL hash the
			// same clear text (it applies SASLprep) and compare StoredKey/ServerKey for the salt it chose.
			r.exec("CREATE ROLE " + name + "_ref LOGIN")
			r.exec("ALTER ROLE " + name + "_ref PASSWORD '" + strings.ReplaceAll(secret, "'", "''") + "'")
			serverVerifier := r.text("SELECT rolpassword FROM pg_authid WHERE rolname='" + name + "_ref'")
			ours := r.text("SELECT rolpassword FROM pg_authid WHERE rolname='" + name + "'")
			parts := strings.SplitN(strings.TrimPrefix(serverVerifier, "SCRAM-SHA-256$4096:"), "$", 2)
			salt, derr := base64.StdEncoding.DecodeString(strings.SplitN(parts[0], "$", 2)[0])
			if derr != nil {
				t.Fatalf("server verifier %q: %v", serverVerifier, derr)
			}
			want, _ := scramVerifierWithSalt(saslPrep(secret), salt, 4096)
			if want != serverVerifier {
				t.Errorf("%s: our SASLprep'd verifier differs from the server's: %q vs %q", name, want, serverVerifier)
			}
			if !strings.HasPrefix(ours, "SCRAM-SHA-256$4096:") {
				t.Errorf("%s: stored verifier %q", name, ours)
			}
		}
		if f := os.Getenv("RR_SECRETS_OUT"); f != "" {
			var b strings.Builder
			for n, s := range cases {
				b.WriteString(n + "\t" + s + "\n")
			}
			os.WriteFile(f, []byte(b.String()), 0o600)
		}
	})
}

// MySQL/MariaDB replace restore fidelity.
func TestRereviewW3RestoreFidelity(t *testing.T) {
	w3Live(t, func(t *testing.T, r w3) {
		if r.c.Driver != "mysql" {
			t.Skip()
		}
		db, closeDB, err := r.e.open(context.Background(), r.c)
		if err != nil {
			t.Fatal(err)
		}
		defer closeDB()
		ctx := context.Background()
		for _, s := range []string{
			"SET sql_mode='ANSI_QUOTES,STRICT_ALL_TABLES'",
			"CREATE FUNCTION rr_f(a INT) RETURNS INT COMMENT 'rr comment' DETERMINISTIC SQL SECURITY INVOKER RETURN a+1",
			"SET sql_mode=DEFAULT",
			"CREATE EVENT rr_e ON SCHEDULE EVERY 1 DAY STARTS '2098-01-01 00:00:00' ON COMPLETION PRESERVE ENABLE COMMENT 'evc' DO SELECT 1",
		} {
			if _, err := db.ExecContext(ctx, s); err != nil {
				t.Fatal(s, err)
			}
		}
		snap := func(q string) string {
			var out string
			rows, _ := db.QueryContext(ctx, q)
			defer rows.Close()
			cols, _ := rows.Columns()
			for rows.Next() {
				v := make([]*string, len(cols))
				d := make([]any, len(cols))
				for i := range v {
					d[i] = &v[i]
				}
				rows.Scan(d...)
				for _, p := range v {
					if p != nil {
						out += *p + "|"
					}
				}
			}
			return out
		}
		q1 := "SELECT ROUTINE_DEFINITION,SQL_MODE,ROUTINE_COMMENT,SECURITY_TYPE,IS_DETERMINISTIC,DEFINER,CHARACTER_SET_CLIENT,COLLATION_CONNECTION FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() AND ROUTINE_NAME='rr_f'"
		q2 := "SELECT EVENT_DEFINITION,SQL_MODE,STATUS,EVENT_TYPE,INTERVAL_VALUE,INTERVAL_FIELD,STARTS,ON_COMPLETION,EVENT_COMMENT,DEFINER,TIME_ZONE FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE() AND EVENT_NAME='rr_e'"
		b1, b2 := snap(q1), snap(q2)
		res, _, err := r.pdo(core.ProgramChangeRequest{Action: "replace", Object: core.ProgramRef{Kind: "function", Name: "rr_f"}, Input: &core.ProgramInput{Body: "RETURN (((", Returns: "integer"}})
		t.Logf("fn replace: %v %s %s", err, res.Outcome, res.Message)
		if a := snap(q1); a != b1 {
			t.Errorf("function drift:\nbefore %s\nafter  %s", b1, a)
		}
		res, _, err = r.pdo(core.ProgramChangeRequest{Action: "replace", Object: core.ProgramRef{Kind: "event", Name: "rr_e"}, Input: &core.ProgramInput{Body: "SELECT (((", Status: "enabled", Schedule: &core.ProgramSchedule{Every: 1, Unit: "day", Starts: "2098-01-01 00:00:00", Preserve: true}}})
		t.Logf("event replace: %v %s %s", err, res.Outcome, res.Message)
		if a := snap(q2); a != b2 {
			t.Errorf("event drift:\nbefore %s\nafter  %s", b2, a)
		}
		_ = dbplan.Confirmation{}
	})
}
