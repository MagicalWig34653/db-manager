// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"strings"
	"testing"
)

func TestAccountsValidation(t *testing.T) {
	account := &core.AccountRef{Name: "test'` ; DROP USER innocent", Host: "%"}
	base := core.AccountRequest{Connection: core.Connection{Driver: "mysql"}, Account: account}
	if err := validateAccounts(base, "inspect"); err != nil {
		t.Fatal(err)
	}
	create := base
	create.Account = nil
	create.NewAccount = account
	if err := validateAccounts(create, "create"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		action string
		r      core.AccountRequest
	}{
		{"list", base}, {"create", base}, {"rename", base}, {"grant", base}, {"role_grant", base}, {"inspect", create}, {"unknown", base},
		{"inspect", core.AccountRequest{Connection: core.Connection{Driver: "postgres"}, Account: account}},
		{"inspect", core.AccountRequest{Connection: core.Connection{Driver: "mysql"}, Account: &core.AccountRef{Name: "a", Host: "\x00"}}},
	} {
		if validateAccounts(tc.r, tc.action) == nil {
			t.Errorf("accepted %s %#v", tc.action, tc.r)
		}
	}
	role := core.AccountRequest{Connection: core.Connection{Driver: "mysql"}, Account: &core.AccountRef{Name: "fixture_role"}}
	if err := validateAccounts(role, "inspect"); err != nil {
		t.Fatal("role inspection rejected before metadata lookup", err)
	}
	if err := validateAccounts(role, "drop"); err == nil {
		t.Fatal("empty-host user mutation accepted")
	}
	if err := validateAccounts(core.AccountRequest{Connection: role.Connection, NewAccount: role.Account}, "create"); err == nil {
		t.Fatal("empty-host user creation accepted")
	}
	for _, action := range append([]string{"list", "inspect"}, core.AccountActions...) {
		c := core.AccountRequest{Connection: core.Connection{Driver: "sqlite"}}
		if action == "create" {
			c.NewAccount = &core.AccountRef{Name: "x"}
		}
		if err := validateAccounts(c, action); err == nil {
			t.Fatal("sqlite accepted", action)
		}
	}
}

func TestAccountsQuoteAndDigest(t *testing.T) {
	a := core.AccountRef{Name: "a'\\`\"; DROP USER other", Host: "%'; DROP USER other"}
	quoted := accountQuote("mysql", a)
	if quoted != "'a''\\`\"; DROP USER other'@'%''; DROP USER other'" {
		t.Fatal(quoted)
	}
	statements := []string{"ALTER USER " + quoted + " IDENTIFIED BY <protected-secret>"}
	p, err := dbplan.NewPlan("mysql", statements, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The secret never appears in the input to the digest or response serialization.
	secret := "fixture-only-sensitive-secret"
	wire, _ := json.Marshal(core.AccountPlan{Plan: p})
	if strings.Contains(string(wire), secret) {
		t.Fatal("secret leaked")
	}
	changed := strings.TrimSuffix(statements[0], "<protected-secret>") + accountLiteral(secret)
	if strings.Contains(changed, "<protected-secret>") || !strings.HasSuffix(changed, "'"+secret+"'") {
		t.Fatal("substitution")
	}
	for _, confirm := range []dbplan.Confirmation{{Action: "drop"}, {Action: "password", TargetName: "other"}} {
		if dbplan.Check(p, p.SHA256, confirm, "password", accountLabel(a), true) == nil {
			t.Fatal("confirmation accepted")
		}
	}
	if dbplan.Check(p, strings.Repeat("0", 64), dbplan.Confirmation{Action: "password", TargetName: accountLabel(a)}, "password", accountLabel(a), true) == nil {
		t.Fatal("stale plan accepted")
	}
}

func TestAccountsFailBeforeConnection(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	r := core.AccountExecuteRequest{AccountRequest: core.AccountRequest{Connection: core.Connection{Driver: "mysql"}, Account: &core.AccountRef{Name: "a", Host: "%"}}}
	for _, action := range []string{"create", "password", "copy", "grant", "revoke", "role_grant"} {
		if _, err := e.AccountExecute(ctx, action, r); err == nil {
			t.Fatal("accepted", action)
		}
	}
	for _, g := range []core.AccountGrant{{Scope: "global", Privilege: "SELECT; DROP USER other"}, {Scope: "table", Privilege: "ALL"}, {Scope: "global", Privilege: "SELECT", Object: "other"}} {
		env := &opEnv{c: core.Connection{Driver: "mysql"}}
		if _, err := accountGrantSQL(ctx, env, "'a'@'%'", g, false); err == nil {
			t.Fatal("unsafe grant accepted")
		}
	}
}

func TestAccountsSQLiteUnsupported(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	r := core.AccountRequest{Connection: core.Connection{Driver: "sqlite", Database: "fixture"}, Account: &core.AccountRef{Name: "fixture"}}
	_, err := e.AccountDetails(ctx, r)
	var p *core.Error
	if !errors.As(err, &p) || p.Code != "unsupported_action" {
		t.Fatal(err)
	}
	for _, action := range core.AccountActions {
		rr := r
		if action == "create" {
			rr.Account = nil
			rr.NewAccount = &core.AccountRef{Name: "fixture"}
		}
		if _, err := e.AccountPlan(ctx, action, rr); !errors.As(err, &p) || p.Code != "unsupported_action" {
			t.Fatal(action, err)
		}
	}
}

func TestAccountsGrantOptionDialect(t *testing.T) {
	ctx := context.Background()
	env := &opEnv{c: core.Connection{Driver: "mysql"}}
	g := core.AccountGrant{Scope: "global", Privilege: "SELECT", GrantOption: true}
	statement, err := accountGrantSQL(ctx, env, "'fixture'@'%'", g, true)
	if err != nil || statement != "REVOKE GRANT OPTION ON *.* FROM 'fixture'@'%'" {
		t.Fatal(statement, err)
	}
}

func TestAccountsServerFeatures(t *testing.T) {
	for _, tc := range []struct {
		version            string
		maria, roles, lock bool
	}{
		{"5.7.5", false, false, false}, {"5.7.6", false, false, true}, {"8.4.0", false, true, true},
		{"10.0.4-MariaDB", true, false, false}, {"10.0.5-MariaDB", true, true, false}, {"10.4.1-MariaDB", true, true, false}, {"10.4.2-MariaDB", true, true, true}, {"5.5.5-12.3.0-MariaDB", true, true, true},
	} {
		roles, lock, err := accountServerFeatures(tc.version, tc.maria)
		if err != nil || roles != tc.roles || lock != tc.lock {
			t.Fatal(tc, roles, lock, err)
		}
	}
	if _, _, err := accountServerFeatures("unknown", false); err == nil {
		t.Fatal("unknown version accepted")
	}
}
