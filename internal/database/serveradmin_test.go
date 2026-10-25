// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLiteServerAdmin(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	r, err := e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: c})
	if err != nil || len(r.Values) != 1 || r.Values[0].Name != "sqlite_version" || r.Values[0].Value == nil || len(r.Processes) != 0 {
		t.Fatalf("basic info: %#v %v", r, err)
	}
	for _, kind := range []string{"variables", "processes"} {
		_, err = e.ServerAdminRead(ctx, kind, core.ServerAdminReadRequest{Connection: c})
		adminWantCode(t, err, "unsupported_action")
	}
	for _, action := range []string{"kill", "set_variable"} {
		_, err = e.ServerAdminPlan(ctx, action, core.ServerAdminRequest{Connection: c})
		adminWantCode(t, err, "unsupported_action")
		_, err = e.ServerAdminExecute(ctx, action, core.ServerAdminExecuteRequest{ServerAdminRequest: core.ServerAdminRequest{Connection: c}})
		adminWantCode(t, err, "unsupported_action")
	}
	for _, r := range []core.ServerAdminReadRequest{{Connection: c, Limit: 201}, {Connection: c, Offset: 10001}, {Connection: c, TimeoutMS: 10001}, {Connection: c, Limit: -1}, {Connection: c, Scope: "GLOBAL"}, {Connection: c, Filter: strings.Repeat("x", 129)}} {
		_, err = e.ServerAdminRead(ctx, "status", r)
		adminWantCode(t, err, "invalid_server_admin")
	}
	for _, r := range []core.ServerAdminReadRequest{{Connection: c, Offset: 1}, {Connection: c, Filter: "no_match"}} {
		result, err := e.ServerAdminRead(ctx, "status", r)
		if err != nil || len(result.Values) != 0 {
			t.Fatalf("paging/filter: %#v %v", result, err)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: c}); err == nil {
		t.Fatal("canceled context accepted")
	}
}
func adminWantCode(t *testing.T, err error, want string) {
	t.Helper()
	var p *core.Error
	if !errors.As(err, &p) || p.Code != want {
		t.Fatalf("want %s, got %v", want, err)
	}
}
func TestServerAdminMaskAndWhitelist(t *testing.T) {
	for _, name := range []string{"password", "secret_token", "ssl_key", "data_directory", "log_file", "primary_conninfo", "plugin_custom", "tmp_table_size_password"} {
		for _, value := range []string{"12345", "private-placeholder", "/placeholder/path"} {
			item := adminSafeValue(name, value, true)
			if !item.Masked || item.Value != nil {
				t.Fatalf("leak: %#v", item)
			}
		}
	}
	if item := adminSafeValue("tmp_table_size", "16384", true); item.Masked || item.Value == nil {
		t.Fatal("numeric allowlist masked")
	}
	for _, r := range []struct{ driver, name, scope, value string }{{"mysql", "password", "GLOBAL", "16384"}, {"mysql", "max_connections", "GLOBAL", "16384"}, {"mysql", "tmp_table_size;DROP", "GLOBAL", "16384"}, {"mysql", "tmp_table_size", "GLOBAL", "16384;DROP"}, {"mysql", "tmp_table_size", "GLOBAL", "016384"}, {"mysql", "tmp_table_size", "GLOBAL", "1073741825"}, {"postgres", "statement_timeout", "GLOBAL", "1000"}, {"postgres", "statement_timeout", "SESSION", "0"}, {"postgres", "lock_timeout", "SESSION", "600001"}} {
		if adminVariable(r.driver, r.name, r.scope, r.value) {
			t.Fatalf("unsafe allowlist: %#v", r)
		}
	}
	for _, d := range []string{"sqlite", "mysql", "postgres"} {
		for _, a := range []string{"status", "variables", "processes", "kill", "set_variable", "flush"} {
			cap := core.ServerAdminCapabilities().Get(d, a)
			if cap.Support != "supported" && cap.Reason == "" {
				t.Fatalf("missing reason: %s/%s", d, a)
			}
		}
	}
}

// Metadata driver permits deterministic stale-target/self-protection tests
// without a network or a third-party mocking dependency.
type adminFixtureDriver struct{}
type adminFixtureConn struct {
	self  bool
	mode  string
	calls int
}
type adminFixtureRows struct {
	data    [][]driver.Value
	index   int
	columns int
}

func (adminFixtureDriver) Open(name string) (driver.Conn, error) {
	return &adminFixtureConn{self: name == "self", mode: name}, nil
}
func (*adminFixtureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (*adminFixtureConn) Close() error              { return nil }
func (*adminFixtureConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c *adminFixtureConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.HasPrefix(query, "SELECT pg_cancel_backend") {
		return &adminFixtureRows{columns: 1, data: [][]driver.Value{{false}}}, nil
	}
	if strings.HasPrefix(query, "SELECT name,setting") {
		return &adminFixtureRows{columns: 2, data: [][]driver.Value{{"statement_timeout", "0"}}}, nil
	}
	if strings.HasPrefix(query, "SHOW SESSION VARIABLES") {
		return &adminFixtureRows{columns: 2, data: [][]driver.Value{{"tmp_table_size", "16384"}}}, nil
	}
	return &adminFixtureRows{columns: 7, data: [][]driver.Value{{"123", "fixture_user", "probe", int64(3), c.self, "stable-generation", "private-query-placeholder"}}}, nil
}
func (r *adminFixtureRows) Columns() []string { return make([]string, r.columns) }
func (*adminFixtureRows) Close() error        { return nil }
func (r *adminFixtureRows) Next(dest []driver.Value) error {
	if r.index == len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.index])
	r.index++
	return nil
}
func init() { sql.Register("w_p17_metadata", adminFixtureDriver{}) }
func TestServerAdminMetadataPlans(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("w_p17_metadata", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	processes, _, err := adminProcesses(ctx, conn, "postgres", 1, 0, "", "123")
	if err != nil {
		t.Fatal(err)
	}
	req := core.ServerAdminRequest{Connection: core.Connection{Driver: "postgres", Host: "placeholder.invalid"}, ID: "123", Identity: processes[0].QueryIdentity, Mode: "query"}
	plan, err := adminBuild(ctx, conn, "kill", req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetName != "123:fixture_user" || !strings.Contains(plan.Statements[0], "pg_cancel_backend(123)") || strings.Contains(plan.Statements[0], "private-query-placeholder") {
		t.Fatalf("unsafe plan: %#v", plan)
	}
	same, err := adminBuild(ctx, conn, "kill", req)
	if err != nil || plan.SHA256 != same.SHA256 {
		t.Fatal("unstable plan")
	}
	req.Connection.Host = "other.invalid"
	other, err := adminBuild(ctx, conn, "kill", req)
	if err != nil || plan.SHA256 == other.SHA256 {
		t.Fatal("plan does not bind connection target")
	}
	for _, confirm := range []dbplan.Confirmation{{Action: "kill", TargetName: "123:other_user"}, {Action: "set_variable", TargetName: plan.TargetName}, {Action: "kill"}} {
		if err := dbplan.Check(plan.Plan, plan.SHA256, confirm, "kill", plan.TargetName, true); err == nil {
			t.Fatal("confirmation bypass")
		}
	}
	if err := dbplan.Check(plan.Plan, strings.Repeat("0", 64), dbplan.Confirmation{Action: "kill", TargetName: plan.TargetName}, "kill", plan.TargetName, true); !errors.Is(err, dbplan.ErrPlanMismatch) {
		t.Fatal("stale hash accepted")
	}
	req.Identity = strings.Repeat("0", 64)
	_, err = adminBuild(ctx, conn, "kill", req)
	adminWantCode(t, err, "plan_mismatch")
	req.ID = "123; DROP"
	_, err = adminBuild(ctx, conn, "kill", req)
	adminWantCode(t, err, "invalid_server_admin")
	req = core.ServerAdminRequest{Connection: core.Connection{Driver: "postgres"}, Name: "statement_timeout", Scope: "SESSION", Value: "1000"}
	plan, err = adminBuild(ctx, conn, "set_variable", req)
	if err != nil || plan.TargetName != "SESSION:statement_timeout" || !strings.HasPrefix(plan.Statements[0], "SET SESSION statement_timeout = 1000") {
		t.Fatalf("variable plan %#v %v", plan, err)
	}
	req = core.ServerAdminRequest{Connection: core.Connection{Driver: "mysql"}, Name: "tmp_table_size", Scope: "SESSION", Value: "32768"}
	if _, err = adminBuild(ctx, conn, "set_variable", req); err != nil {
		t.Fatal(err)
	}
	selfDB, _ := sql.Open("w_p17_metadata", "self")
	defer selfDB.Close()
	selfConn, _ := selfDB.Conn(ctx)
	defer selfConn.Close()
	self, _, _ := adminProcesses(ctx, selfConn, "postgres", 1, 0, "", "123")
	req = core.ServerAdminRequest{Connection: core.Connection{Driver: "postgres"}, ID: "123", Identity: self[0].Identity, Mode: "connection"}
	_, err = adminBuild(ctx, selfConn, "kill", req)
	adminWantCode(t, err, "unsupported_action")
}

func (c *adminFixtureConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	c.calls++
	if c.mode == "lost" {
		return nil, errors.New("private-placeholder driver path and password")
	}
	if c.mode == "denied" {
		return nil, &mysql.MySQLError{Number: 1227, Message: "private-placeholder"}
	}
	return driver.RowsAffected(0), nil
}
func TestServerAdminExecutionOutcomes(t *testing.T) {
	for _, mode := range []string{"lost", "denied", "ok", "canceled", "signal-false"} {
		t.Run(mode, func(t *testing.T) {
			db, err := sql.Open("w_p17_metadata", mode)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx := context.Background()
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			req := core.ServerAdminRequest{Connection: core.Connection{Driver: "mysql"}}
			plan := core.ServerAdminPlan{Plan: dbplan.Plan{Statements: []string{"KILL CONNECTION 123"}}}
			if mode == "signal-false" {
				req.Connection.Driver = "postgres"
				plan.Statements[0] = "SELECT pg_cancel_backend(123)"
			}
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			result, err := adminRun(ctx, conn, "kill", req, plan)
			if err != nil || strings.Contains(result.Message, "private-placeholder") {
				t.Fatalf("unsafe error %#v %v", result, err)
			}
			want := dbplan.StatusRejected
			if mode == "ok" {
				want = dbplan.StatusCompleted
			}
			if mode == "lost" {
				want = dbplan.StatusOutcomeUnknown
			}
			if result.Outcome != want {
				t.Fatalf("outcome %#v", result)
			}
			if mode == "denied" && result.Reason != "permission_denied" {
				t.Fatalf("reason %#v", result)
			}
			if err = conn.Raw(func(raw any) error {
				calls := raw.(*adminFixtureConn).calls
				if calls > 1 || (mode == "canceled" && calls != 0) {
					t.Fatalf("retried/sent after cancel: %d", calls)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServerAdminDisconnectUncertainty(t *testing.T) {
	for _, err := range []error{&mysql.MySQLError{Number: 1927}, &mysql.MySQLError{Number: 2013}, &pgconn.PgError{Code: "57P01"}, &pgconn.PgError{Code: "08006"}, &pgconn.PgError{Code: "57014"}} {
		if _, definite := adminReason(err); definite {
			t.Fatalf("disconnect/cancel claimed definite: %T", err)
		}
	}
}
