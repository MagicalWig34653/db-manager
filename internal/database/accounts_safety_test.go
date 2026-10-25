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
	executions int
}
type accountsTestRows struct{ values [][]driver.Value }

var registerAccountsTest sync.Once

func (accountsTestDriver) Open(name string) (driver.Conn, error) {
	return &accountsTestConn{denied: name == "denied"}, nil
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
			result := executeAccounts(ctx, env, statements)
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
		result := executeAccounts(ctx, accountTestEnv(t, "catalog"), tc.statements)
		if result.Outcome != tc.outcome || result.StatementsExecuted != tc.count || strings.Contains(result.Message, "PRIVATE") || strings.Contains(result.Message, "secret") {
			t.Fatal(result)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if result := executeAccounts(canceled, accountTestEnv(t, "catalog"), []string{"ALTER USER fixture"}); result.Outcome != "rejected" || result.Reason != "canceled" {
		t.Fatal(result)
	}
}
