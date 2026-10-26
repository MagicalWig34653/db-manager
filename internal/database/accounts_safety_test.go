// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/go-sql-driver/mysql"
)

// A local protocol fixture exercises real plan/execution code without sockets.
// It returns catalog metadata and server failures; it never implements SQL.
type accountsTestDriver struct{}
type accountsTestConn struct {
	denied     bool
	ambiguous  bool
	modernRole bool
	executions int
}
type accountsTestRows struct{ values [][]driver.Value }

var registerAccountsTest sync.Once

func (accountsTestDriver) Open(name string) (driver.Conn, error) {
	return &accountsTestConn{denied: name == "denied", ambiguous: name == "ambiguous", modernRole: name == "modern_role"}, nil
}
func (*accountsTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (*accountsTestConn) Close() error                        { return nil }
func (*accountsTestConn) Begin() (driver.Tx, error)           { return nil, errors.New("not used") }
func (r *accountsTestRows) Columns() []string {
	if len(r.values) == 0 {
		return []string{"empty"}
	}
	cols := make([]string, len(r.values[0]))
	for i := range cols {
		cols[i] = "column"
	}
	return cols
}
func (*accountsTestRows) Close() error { return nil }
func (r *accountsTestRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}
func (c *accountsTestConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch {
	case query == "SELECT VERSION()":
		return &accountsTestRows{[][]driver.Value{{"8.4.0"}}}, nil
	case query == "SELECT CURRENT_USER()":
		return &accountsTestRows{[][]driver.Value{{"fixture@%"}}}, nil
	case strings.Contains(query, "SELECT COUNT(*) FROM information_schema.TABLES"):
		return &accountsTestRows{[][]driver.Value{{int64(1)}}}, nil
	case strings.HasPrefix(query, "SELECT COALESCE(JSON_VALUE(Priv,'$.access')"):
		access := "0"
		if c.modernRole {
			access = "1073741824"
		}
		return &accountsTestRows{[][]driver.Value{{access}}}, nil
	case strings.Contains(query, "SELECT COLUMN_NAME FROM information_schema.COLUMNS"):
		return &accountsTestRows{[][]driver.Value{{"Process_priv"}, {"Grant_priv"}}}, nil
	case strings.HasPrefix(query, "SELECT `Process_priv`,`Grant_priv` FROM mysql.user"):
		return &accountsTestRows{[][]driver.Value{{"Y", "Y"}}}, nil
	case strings.Contains(query, "FROM mysql.user WHERE BINARY CONCAT"):
		count := int64(1)
		if c.ambiguous {
			count = 2
		}
		return &accountsTestRows{[][]driver.Value{{count}}}, nil
	case strings.Contains(query, "FROM information_schema.USER_PRIVILEGES"):
		if !strings.Contains(query, "WHERE BINARY GRANTEE=BINARY ?") || len(args) != 1 || args[0].Value != "'odd'name'@'%'" {
			return nil, errors.New("unexpected GRANTEE metadata value")
		}
		return &accountsTestRows{[][]driver.Value{{"", "", "", "", "SELECT", "NO"}, {"", "", "", "", "BACKUP_ADMIN", "YES"}}}, nil
	case strings.Contains(query, "FROM mysql.global_grants"):
		return &accountsTestRows{[][]driver.Value{{"BACKUP_ADMIN", "Y"}}}, nil
	case strings.Contains(query, "FROM information_schema.") || strings.Contains(query, "FROM mysql.procs_priv"):
		return &accountsTestRows{}, nil
	case strings.Contains(query, "FROM mysql.user u"):
		if c.denied {
			return nil, &mysql.MySQLError{Number: 1142, Message: "fixture server error that must remain private"}
		}
		return &accountsTestRows{[][]driver.Value{{"fixture", "%", "caching_sha2_password", "ANY", int64(0), int64(0), int64(0), int64(0), false, true}}}, nil
	default:
		return nil, errors.New("unexpected test query")
	}
}
func (c *accountsTestConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.executions++
	if strings.Contains(query, "FAIL_PRIVATE") {
		return nil, &mysql.MySQLError{Number: 1227, Message: query}
	}
	if strings.Contains(query, "DISCONNECT_PRIVATE") {
		return nil, errors.New(query)
	}
	return driver.RowsAffected(0), nil
}
func accountTestEnv(t *testing.T, name string) *opEnv {
	t.Helper()
	registerAccountsTest.Do(func() { sql.Register("accounts_test", accountsTestDriver{}) })
	db, err := sql.Open("accounts_test", name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return &opEnv{c: core.Connection{Driver: "mysql", Username: "fixture"}, sess: &mutationSession{conn: conn, driver: "mysql"}}
}
func TestAccountsRealPlanSecretProtection(t *testing.T) {
	ctx := context.Background()
	a := &core.AccountRef{Name: "fixture", Host: "%"}
	for _, name := range []string{"catalog", "denied"} {
		env := accountTestEnv(t, name)
		r := core.AccountRequest{Connection: env.c, Account: a}
		p, err := buildAccountPlan(ctx, env, "password", r)
		if err != nil {
			t.Fatal(name, err)
		}
		if !p.Self || !p.Destructive || !p.RequiresSecret || p.Confirmation.TargetName != accountLabel(*a) {
			t.Fatal(p)
		}
		first := p.SHA256
		for _, secret := range []string{"FIXTURE_SECRET_ONE", "FIXTURE_SECRET_TWO'\\"} {
			statements := append([]string{}, p.Statements...)
			statements[0] = strings.TrimSuffix(statements[0], "<protected-secret>") + accountLiteral(secret)
			result := executeAccounts(ctx, env, statements, nil)
			if result.Outcome != "completed" {
				t.Fatal(result)
			}
			fresh, err := buildAccountPlan(ctx, env, "password", r)
			if err != nil || first != fresh.SHA256 {
				t.Fatal("secret affected plan digest", err)
			}
		}
	}
}
func TestAccountsExecutionSanitizesAndClassifies(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		statements []string
		outcome    string
		count      int
	}{
		{[]string{"FAIL_PRIVATE 'secret'"}, "rejected", 0},
		{[]string{"CREATE USER fixture", "FAIL_PRIVATE 'secret'"}, "outcome_unknown", 1},
		{[]string{"DISCONNECT_PRIVATE 'secret'"}, "outcome_unknown", 0},
		{[]string{"ALTER USER fixture"}, "completed", 1},
	} {
		result := executeAccounts(ctx, accountTestEnv(t, "catalog"), tc.statements, nil)
		if result.Outcome != tc.outcome || result.StatementsExecuted != tc.count || strings.Contains(result.Message, "PRIVATE") || strings.Contains(result.Message, "secret") {
			t.Fatal(result)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if result := executeAccounts(canceled, accountTestEnv(t, "catalog"), []string{"ALTER USER fixture"}, nil); result.Outcome != "rejected" || result.Reason != "canceled" {
		t.Fatal(result)
	}
}

func TestAccountsQuotedGranteeAndDynamicDeduplication(t *testing.T) {
	out := core.AccountDetails{Grants: []core.AccountGrant{}}
	err := readMySQLAccountGrants(context.Background(), accountTestEnv(t, "catalog"), core.AccountRef{Name: "odd'name", Host: "%"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Grants) != 2 || out.Grants[0].Privilege != "SELECT" || out.Grants[1].Privilege != "BACKUP_ADMIN" || !out.Grants[1].GrantOption {
		t.Fatalf("missing or duplicated catalog grants: %#v", out.Grants)
	}
}

func TestAccountsAmbiguousCatalogGranteeRejected(t *testing.T) {
	out := core.AccountDetails{Grants: []core.AccountGrant{}}
	err := readMySQLAccountGrants(context.Background(), accountTestEnv(t, "ambiguous"), core.AccountRef{Name: "a'@'b", Host: "c"}, &out)
	var publicErr *core.Error
	if !errors.As(err, &publicErr) || publicErr.Code != "unsupported_action" || len(out.Grants) != 0 {
		t.Fatalf("ambiguous metadata identity accepted: %v", err)
	}
}

func TestAccountsMariaRoleGlobalPrivileges(t *testing.T) {
	out := core.AccountDetails{Grants: []core.AccountGrant{}}
	err := readMariaRoleGlobalGrants(context.Background(), accountTestEnv(t, "catalog"), core.AccountRef{Name: "fixture_role"}, &out)
	if err != nil || len(out.Grants) != 1 || out.Grants[0] != (core.AccountGrant{Scope: "global", Privilege: "PROCESS", GrantOption: true}) {
		t.Fatalf("global role privileges missing: %#v, %v", out.Grants, err)
	}
}

func TestAccountsMariaRoleModernPrivilegesFailClosed(t *testing.T) {
	out := core.AccountDetails{Grants: []core.AccountGrant{}}
	err := readMariaRoleGlobalGrants(context.Background(), accountTestEnv(t, "modern_role"), core.AccountRef{Name: "fixture_role"}, &out)
	var publicErr *core.Error
	if !errors.As(err, &publicErr) || publicErr.Code != "unsupported_action" {
		t.Fatalf("unrepresented role privileges silently omitted: %v", err)
	}
}
