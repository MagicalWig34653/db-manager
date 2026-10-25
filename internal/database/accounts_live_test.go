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

func dumpAccount(t *testing.T, kind string, v any) {
	t.Helper()
	if dir := os.Getenv("DBM_ACCOUNTS_DUMP"); dir != "" {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dir+"/"+strings.ReplaceAll(t.Name(), "/", "_")+"."+kind+".json", b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestLiveAccounts(t *testing.T) {
	for _, fixture := range []struct{ name, driver, port string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(fixture.name, func(t *testing.T) {
			raw := os.Getenv(fixture.port)
			if raw == "" {
				t.Skip("disposable dbfix fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: fixture.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			ctx := context.Background()
			secret := "fixture-only-account-secret-18"
			account := core.AccountRef{Name: "p18 odd'`\"; x", Host: "%"}
			if c.Driver == "postgres" {
				account.Host = ""
			}
			copyAccount := core.AccountRef{Name: "p18_copy", Host: account.Host}
			renamed := core.AccountRef{Name: "p18_renamed", Host: account.Host}
			role := core.AccountRef{Name: "p18_role", Host: "%"}
			if fixture.name == "maria" || c.Driver == "postgres" {
				role.Host = ""
			}
			direct := func(sql string) core.QueryResult {
				t.Helper()
				result, err := e.Query(ctx, core.QueryRequest{Connection: c, SQL: sql})
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			cleanup := func(a core.AccountRef) {
				keyword := "USER"
				if c.Driver == "postgres" {
					keyword = "ROLE"
				}
				_, _ = e.Query(ctx, core.QueryRequest{Connection: c, SQL: "DROP " + keyword + " IF EXISTS " + accountQuote(c.Driver, a)})
			}
			defer cleanup(account)
			defer cleanup(copyAccount)
			defer cleanup(renamed)
			defer cleanup(role)
			request := func(a *core.AccountRef) core.AccountRequest { return core.AccountRequest{Connection: c, Account: a} }
			plan := func(action string, r core.AccountRequest) core.AccountPlan {
				t.Helper()
				p, err := e.AccountPlan(ctx, action, r)
				if err != nil {
					t.Fatalf("plan %s: %v", action, err)
				}
				encoded, _ := json.Marshal(p)
				if strings.Contains(string(encoded), secret) {
					t.Fatal("secret in plan")
				}
				return p
			}
			executePlan := func(action string, r core.AccountRequest, p core.AccountPlan) core.AccountResult {
				t.Helper()
				x := core.AccountExecuteRequest{AccountRequest: r, Confirm: p.Confirmation, ExpectedPlanSHA256: p.SHA256}
				if p.RequiresSecret {
					x.Secret = &secret
				}
				result, err := e.AccountExecute(ctx, action, x)
				if err != nil {
					t.Fatalf("execute %s: %v", action, err)
				}
				encoded, _ := json.Marshal(result)
				if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "IDENTIFIED") {
					t.Fatal("secret or SQL in result")
				}
				return result
			}
			run := func(action string, r core.AccountRequest) core.AccountResult {
				t.Helper()
				result := executePlan(action, r, plan(action, r))
				if result.Outcome != "completed" {
					t.Fatalf("%s: %#v", action, result)
				}
				return result
			}
			r := request(nil)
			r.NewAccount = &account
			cp := plan("create", r)
			dumpAccount(t, "AccountPlan", cp)
			dumpAccount(t, "AccountResult", executePlan("create", r, cp))
			details, err := e.AccountDetails(ctx, request(&account))
			if err != nil {
				t.Fatal(err)
			}
			dumpAccount(t, "AccountDetails", details)
			accounts, err := e.Accounts(ctx, request(nil))
			if err != nil || len(accounts.Accounts) < 2 {
				t.Fatal(accounts, err)
			}
			dumpAccount(t, "AccountsResult", accounts)
			// Independent catalog read rather than trusting create's own response.
			query := "SELECT COUNT(*) FROM mysql.user WHERE User=" + accountLiteral(account.Name) + " AND Host=" + accountLiteral(account.Host)
			if c.Driver == "postgres" {
				query = "SELECT COUNT(*) FROM pg_roles WHERE rolname=" + accountLiteral(account.Name)
			}
			if direct(query).Results[0].Rows[0][0].Value != "1" {
				t.Fatal("created account absent")
			}
			passwordRequest := request(&account)
			pp := plan("password", passwordRequest)
			for _, mutate := range []func(*core.AccountExecuteRequest){func(x *core.AccountExecuteRequest) { x.ExpectedPlanSHA256 = strings.Repeat("0", 64) }, func(x *core.AccountExecuteRequest) { x.Confirm.TargetName = "innocent" }, func(x *core.AccountExecuteRequest) { x.Confirm.Action = "drop" }} {
				x := core.AccountExecuteRequest{AccountRequest: passwordRequest, Secret: &secret, Confirm: pp.Confirmation, ExpectedPlanSHA256: pp.SHA256}
				mutate(&x)
				if _, err := e.AccountExecute(ctx, "password", x); err == nil {
					t.Fatal("invalid confirmation accepted")
				}
			}
			oldSecret := secret
			secret += "-changed"
			run("password", passwordRequest)
			login := c
			login.Username = account.Name
			login.Password = secret
			if c.Driver == "mysql" {
				login.Database = ""
			}
			if err := e.Test(ctx, login); err != nil {
				t.Fatal("new password rejected", err)
			}
			oldLogin := login
			oldLogin.Password = oldSecret
			if err := e.Test(ctx, oldLogin); err == nil {
				t.Fatal("old password still accepted")
			}
			if fresh := plan("password", passwordRequest); fresh.SHA256 != pp.SHA256 {
				t.Fatal("password changed digest")
			}
			table := "p18_items"
			schema := "probe"
			if c.Driver == "postgres" {
				schema = "public"
			}
			qualified := quoteIdentifier(c.Driver, schema) + "." + quoteIdentifier(c.Driver, table)
			direct("CREATE TABLE " + qualified + " (value INTEGER)")
			defer direct("DROP TABLE " + qualified)
			direct("INSERT INTO " + qualified + " VALUES (18)")
			grants := []core.AccountGrant{{Scope: "database", Database: "probe", Privilege: "SELECT"}, {Scope: "table", Database: "probe", Object: table, Privilege: "SELECT"}, {Scope: "column", Database: "probe", Object: table, Column: "value", Privilege: "UPDATE"}}
			if c.Driver == "postgres" {
				grants[0].Privilege = "CONNECT"
				grants[1].Schema = "public"
				grants[2].Schema = "public"
			}
			for _, g := range grants {
				rr := request(&account)
				rr.Grant = &g
				run("grant", rr)
				run("revoke", rr)
			}
			if c.Driver == "mysql" {
				g := core.AccountGrant{Scope: "global", Privilege: "PROCESS"}
				rr := request(&account)
				rr.Grant = &g
				run("grant", rr)
				run("revoke", rr)
			}
			grant := grants[1]
			grant.GrantOption = true
			gr := request(&account)
			gr.Grant = &grant
			run("grant", gr)
			// Roles are catalog-resolved; MariaDB uses its role identifier syntax.
			roleSQL := accountQuote(c.Driver, role)
			if fixture.name == "maria" {
				roleSQL = quoteIdentifier(c.Driver, role.Name)
			}
			direct("CREATE ROLE " + roleSQL)
			roleRequest := request(&account)
			roleRequest.Role = &role
			run("role_grant", roleRequest)
			// Routines use catalog OIDs on PostgreSQL, quoted catalog names on MySQL.
			routine := core.AccountGrant{Scope: "routine", Database: "probe", Object: "p18_fn", RoutineKind: "function", Privilege: "EXECUTE"}
			if c.Driver == "postgres" {
				direct("CREATE FUNCTION public.p18_fn(integer) RETURNS integer LANGUAGE SQL AS 'SELECT $1'")
				defer direct("DROP FUNCTION public.p18_fn(integer)")
				routine.Schema = "public"
				routine.RoutineID = direct("SELECT 'public.p18_fn(integer)'::regprocedure::oid::text").Results[0].Rows[0][0].Value
			} else {
				direct("CREATE FUNCTION probe.p18_fn() RETURNS INTEGER DETERMINISTIC NO SQL RETURN 18")
				defer direct("DROP FUNCTION probe.p18_fn")
			}
			rr := request(&account)
			rr.Grant = &routine
			run("grant", rr)
			details, err = e.AccountDetails(ctx, request(&account))
			if err != nil || len(details.Grants) == 0 || len(details.Roles) != 1 {
				t.Fatal(details, err)
			}
			if c.Driver == "mysql" {
				observed := direct("SHOW GRANTS FOR " + accountQuote(c.Driver, account))
				found := false
				for _, row := range observed.Results[0].Rows {
					for _, cell := range row {
						if strings.Contains(cell.Value, table) && strings.Contains(cell.Value, "SELECT") && strings.Contains(cell.Value, "WITH GRANT OPTION") {
							found = true
						}
					}
				}
				if !found {
					t.Fatal("SHOW GRANTS did not confirm expected table SELECT WITH GRANT OPTION")
				}
			} else {
				observed := direct("SELECT COUNT(*) FROM information_schema.role_table_grants WHERE grantee=" + accountLiteral(account.Name) + " AND table_schema='public' AND table_name='p18_items' AND privilege_type='SELECT' AND is_grantable='YES'")
				if observed.Results[0].Rows[0][0].Value != "1" {
					t.Fatal("independent PostgreSQL ACL check failed")
				}
			}
			copyRequest := request(&account)
			copyRequest.NewAccount = &copyAccount
			run("copy", copyRequest)
			copied, err := e.AccountDetails(ctx, request(&copyAccount))
			if err != nil || len(copied.Grants) != len(details.Grants) || len(copied.Roles) != 1 {
				t.Fatal(copied, err)
			}
			run("role_revoke", roleRequest)
			run("revoke", rr)
			run("lock", request(&account))
			locked, err := e.AccountDetails(ctx, request(&account))
			if err != nil || locked.Info.Locked == nil || !*locked.Info.Locked {
				t.Fatal(locked, err)
			}
			if err := e.Test(ctx, login); err == nil {
				t.Fatal("locked account logged in")
			}
			run("unlock", request(&account))
			if err := e.Test(ctx, login); err != nil {
				t.Fatal("unlocked account login failed", err)
			}
			// Restricted login cannot create users; capabilities must hide mutations.
			restricted := c
			restricted.Username = account.Name
			restricted.Password = secret
			access, err := e.AccountAccess(ctx, core.AccountRequest{Connection: restricted})
			if err != nil || len(access.Actions) != 0 {
				t.Fatal(access, err)
			}
			dumpAccount(t, "AccountAccess", access)
			outsider := core.AccountRef{Name: "p18_forbidden", Host: account.Host}
			defer cleanup(outsider)
			restrictedCreate := core.AccountRequest{Connection: restricted, NewAccount: &outsider}
			if p, err := e.AccountPlan(ctx, "create", restrictedCreate); err == nil {
				result := executePlan("create", restrictedCreate, p)
				if result.Outcome != "rejected" || result.Reason != "permission_denied" {
					t.Fatal(result)
				}
			} else {
				var publicErr *core.Error
				if !errors.As(err, &publicErr) || publicErr.Code != "permission_denied" {
					t.Fatal(err)
				}
			}
			// Own lock requires a second confirmation; do not actually lock fixture admin.
			own := core.AccountRef{Name: c.Username, Host: "%"}
			if c.Driver == "postgres" {
				own.Host = ""
			}
			ownRequest := request(&own)
			ownPlan := plan("lock", ownRequest)
			_, err = e.AccountExecute(ctx, "lock", core.AccountExecuteRequest{AccountRequest: ownRequest, ExpectedPlanSHA256: ownPlan.SHA256, Confirm: core.AccountConfirm{Action: "lock", TargetName: ownPlan.Confirmation.TargetName}})
			if err == nil {
				t.Fatal("self lock accepted")
			}
			active, closeActive, err := e.openAccounts(ctx, restricted)
			if err != nil {
				t.Fatal(err)
			}
			defer closeActive()
			renameRequest := request(&account)
			renameRequest.NewAccount = &renamed
			run("rename", renameRequest)
			for _, a := range []core.AccountRef{renamed, copyAccount} {
				g := grant
				rr := request(&a)
				rr.Grant = &g
				run("revoke", rr)
				if a == copyAccount {
					rr.Grant = &routine
					run("revoke", rr)
				}
			}
			if c.Driver == "mysql" {
				run("flush", request(nil))
			}
			run("drop", request(&renamed))
			run("drop", request(&copyAccount))
			var alive int
			if err := active.sess.QueryRowContext(ctx, "SELECT 1").Scan(&alive); err != nil || alive != 1 {
				t.Fatal("existing session was unexpectedly terminated", err)
			}
			deniedLogin := restricted
			deniedLogin.Username = renamed.Name
			if err := e.Test(ctx, deniedLogin); err == nil {
				t.Fatal("deleted account can still log in")
			}
			if direct("SELECT COUNT(*) FROM " + qualified).Results[0].Rows[0][0].Value != "1" {
				t.Fatal("unrelated fixture changed")
			}
		})
	}
}
