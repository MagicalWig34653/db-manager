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
)

func TestSQLiteServerAdmin(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	r, err := e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: c})
	if err != nil || len(r.Values) != 1 || r.Values[0].Name != "sqlite_version" || r.Values[0].Value == nil || len(r.Processes) != 0 {
		t.Fatalf("basic info: %#v %v", r, err)
	}
	adminDump(t, "ServerAdminReadResult", "sqlite-real", r)
	for _, kind := range []string{"variables", "processes"} {
		_, err = e.ServerAdminRead(ctx, kind, core.ServerAdminReadRequest{Connection: c})
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

	for _, d := range []string{"sqlite", "mysql", "postgres"} {
		for _, a := range []string{"status", "variables", "processes"} {
			cap := core.ServerAdminCapabilities().Get(d, a)
			if cap.Support != "supported" && cap.Reason == "" {
				t.Fatalf("missing reason: %s/%s", d, a)
			}
		}
	}
	for _, value := range []string{"+", "1e", "--1", "NaN", "Inf", "1e999", "123/private"} {
		if item := adminSafeValue("tmp_table_size", value, true); !item.Masked || item.Value != nil {
			t.Fatalf("invalid number: %#v", item)
		}
	}
}

// A deterministic read-only metadata driver checks bounds and pagination offline.
type adminFixtureDriver struct{}
type adminFixtureConn struct{ mode string }
type adminFixtureRows struct {
	data     [][]driver.Value
	index    int
	columns  int
	rowError bool
}

func (adminFixtureDriver) Open(name string) (driver.Conn, error) {
	return &adminFixtureConn{mode: name}, nil
}
func (*adminFixtureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (*adminFixtureConn) Close() error              { return nil }
func (*adminFixtureConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c *adminFixtureConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if query == "values" {
		data := [][]driver.Value{{"tmp_table_size", "16384"}, {"password", "12345"}, {"max_connections", "100"}, {"plugin_custom", "private-placeholder"}}
		switch c.mode {
		case "many":
			data = make([][]driver.Value, 4097)
			for i := range data {
				data[i] = []driver.Value{"counter", "1"}
			}
		case "large":
			data = [][]driver.Value{{"counter", strings.Repeat("1", MaxCellBytes+1)}}
		case "bytes":
			data = make([][]driver.Value, MaxResponseBytes/MaxCellBytes+1)
			for i := range data {
				data[i] = []driver.Value{"counter", strings.Repeat("1", MaxCellBytes)}
			}
		case "name":
			data = [][]driver.Value{{strings.Repeat("n", 257), "1"}}
		}
		return &adminFixtureRows{columns: 2, data: data, rowError: c.mode == "rows-error"}, nil
	}
	// Raw query/host columns must never be requested, even for private hashing.
	if strings.Contains(query, "INFO") || strings.Contains(query, "HOST") || strings.Contains(query, "backend_start") || strings.Contains(query, "query_start::text") || len(args) != 3 || args[0].Value != "fixture" || args[1].Value != int64(2) || args[2].Value != int64(0) {
		return nil, errors.New("unsafe process query")
	}
	data := [][]driver.Value{{"123", "fixture_user", "probe", int64(3), false}, {"124", "fixture_user", "probe", int64(4), true}}
	if c.mode == "long-user" {
		data[0][1] = strings.Repeat("u", 257)
	}
	if c.mode == "scan-error" {
		data[0][3] = "invalid-seconds"
	}
	if c.mode == "rows-error" {
		data = nil
	}
	return &adminFixtureRows{columns: 5, data: data, rowError: c.mode == "rows-error"}, nil
}
func (r *adminFixtureRows) Columns() []string { return make([]string, r.columns) }
func (*adminFixtureRows) Close() error        { return nil }
func (r *adminFixtureRows) Next(dest []driver.Value) error {
	if r.index == len(r.data) {
		if r.rowError {
			return errors.New("private-placeholder driver SQL path password")
		}
		return io.EOF
	}
	copy(dest, r.data[r.index])
	r.index++
	return nil
}
func init() { sql.Register("w_p17_metadata", adminFixtureDriver{}) }
func adminTestConn(t *testing.T, mode string) *sql.Conn {
	t.Helper()
	db, err := sql.Open("w_p17_metadata", mode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}
func TestServerAdminMetadataReads(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		conn := adminTestConn(t, "")
		processes, more, err := adminProcesses(context.Background(), conn, dialect, 1, 0, "fixture")
		if err != nil || !more || len(processes) != 1 || processes[0].ID != "123" {
			t.Fatalf("process page: %#v %v %v", processes, more, err)
		}
	}
	for _, mode := range []string{"long-user", "scan-error", "rows-error"} {
		_, _, err := adminProcesses(context.Background(), adminTestConn(t, mode), "postgres", 1, 0, "fixture")
		if err == nil {
			t.Fatalf("accepted %s", mode)
		}
		if strings.Contains(safe(context.Background(), err).Error(), "private-placeholder") {
			t.Fatal("driver error leaked")
		}
	}
	for _, test := range []struct {
		filter        string
		offset, limit int
		name          string
		more          bool
	}{
		{"", 0, 2, "max_connections", true}, {"", 2, 2, "plugin_custom", false}, {"TABLE", 0, 1, "tmp_table_size", false}, {"", 4, 1, "", false},
	} {
		rows, err := adminTestConn(t, "").QueryContext(context.Background(), "values")
		if err != nil {
			t.Fatal(err)
		}
		values, more, err := adminValues(rows, core.ServerAdminReadRequest{Filter: test.filter, Offset: test.offset, Limit: test.limit}, true)
		if err != nil || more != test.more {
			t.Fatalf("values: %#v %v %v", values, more, err)
		}
		if test.name == "" {
			if len(values) != 0 {
				t.Fatal("offset ignored")
			}
		} else if len(values) == 0 || values[0].Name != test.name {
			t.Fatalf("filter/sort/page: %#v", values)
		}
		for _, v := range values {
			if (v.Name == "password" || v.Name == "plugin_custom") && (!v.Masked || v.Value != nil) {
				t.Fatal("secret exposed")
			}
		}
	}
	for _, mode := range []string{"many", "large", "bytes", "name", "rows-error"} {
		rows, err := adminTestConn(t, mode).QueryContext(context.Background(), "values")
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = adminValues(rows, core.ServerAdminReadRequest{Limit: 1}, false)
		if mode == "rows-error" {
			if err == nil {
				t.Fatal("row error ignored")
			}
		} else {
			adminWantCode(t, err, "result_too_large")
		}
	}
}

func TestServerAdminTimeoutAndValidation(t *testing.T) {
	e := testEngine(t)
	for _, r := range []core.ServerAdminReadRequest{
		{Connection: core.Connection{Driver: "unknown"}},
		{Connection: sqliteConnection(), Offset: -1},
		{Connection: sqliteConnection(), TimeoutMS: -1},
	} {
		_, err := e.ServerAdminRead(context.Background(), "status", r)
		adminWantCode(t, err, "invalid_server_admin")
	}
	_, err := e.ServerAdminRead(context.Background(), "kill", core.ServerAdminReadRequest{Connection: sqliteConnection()})
	adminWantCode(t, err, "invalid_server_admin")
	_, err = e.ServerAdminRead(context.Background(), "variables", core.ServerAdminReadRequest{Connection: core.Connection{Driver: "postgres"}, Scope: "GLOBAL"})
	adminWantCode(t, err, "unsupported_action")
	// Exhaust the admission slots so expiration must interrupt a real engine wait.
	for i := 0; i < cap(e.slots); i++ {
		e.slots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(e.slots); i++ {
			<-e.slots
		}
	}()
	_, err = e.ServerAdminRead(context.Background(), "status", core.ServerAdminReadRequest{Connection: sqliteConnection(), TimeoutMS: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout ignored: %v", err)
	}
}
