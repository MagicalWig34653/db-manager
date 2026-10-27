// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func adminDump(t *testing.T, component, label string, value any) {
	t.Helper()
	dir := os.Getenv("DBM_SERVER_ADMIN_DUMP")
	if dir == "" {
		return
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, component+"."+strings.ReplaceAll(t.Name(), "/", "-")+"-"+label+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestLiveServerAdmin(t *testing.T) {
	for _, system := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.env)
			if raw == "" {
				t.Skip("disposable fixture required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: system.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			db, cleanup, err := e.open(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			control, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer control.Close()
			read := func(kind, label string, r core.ServerAdminReadRequest) core.ServerAdminReadResult {
				t.Helper()
				r.Connection = c
				result, err := e.ServerAdminRead(ctx, kind, r)
				if err != nil {
					t.Fatal(err)
				}
				adminDump(t, "ServerAdminReadResult", label, result)
				return result
			}
			settingQuery := "SELECT @@SESSION.tmp_table_size"
			if c.Driver == "postgres" {
				settingQuery = "SHOW statement_timeout"
			}
			var originalSetting string
			if err = control.QueryRowContext(ctx, settingQuery).Scan(&originalSetting); err != nil {
				t.Fatal(err)
			}
			// Filter the metric explicitly: Com_select need not be on the first status page.
			metric := "Com_select"
			if c.Driver == "postgres" {
				metric = "xact_commit"
			}
			before := read("status", "before", core.ServerAdminReadRequest{Filter: metric})
			for i := 0; i < 5; i++ {
				var n int
				if err = control.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil || n != 1 {
					t.Fatal("synthetic load failed")
				}
			}
			after := read("status", "after", core.ServerAdminReadRequest{Filter: metric})
			if len(after.Values) != 1 || after.SampledAt == before.SampledAt || after.Values[0].Value == nil {
				t.Fatal("missing metrics/timestamp")
			}
			if c.Driver == "mysql" {
				b, err := strconv.ParseUint(*before.Values[0].Value, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				a, err := strconv.ParseUint(*after.Values[0].Value, 10, 64)
				if err != nil || a <= b {
					t.Fatal("synthetic load did not increase Com_select")
				}
			}
			vars := read("variables", "variables", core.ServerAdminReadRequest{Filter: "timeout", Limit: 3})
			if len(vars.Values) == 0 || len(vars.Values) > 3 {
				t.Fatal("missing variable page")
			}
			for _, v := range vars.Values {
				if !strings.Contains(strings.ToLower(v.Name), "timeout") {
					t.Fatal("filter ignored")
				}
			}
			page1 := read("variables", "page1", core.ServerAdminReadRequest{Limit: 2})
			page2 := read("variables", "page2", core.ServerAdminReadRequest{Limit: 2, Offset: 2})
			if !page1.HasMore || len(page1.Values) != 2 || len(page2.Values) != 2 || page1.Values[1].Name >= page2.Values[0].Name {
				t.Fatal("variable paging/sorting failed")
			}
			secretFilter := "ssl"
			if c.Driver == "postgres" {
				secretFilter = "directory"
			}
			masked := read("variables", "masked", core.ServerAdminReadRequest{Filter: secretFilter})
			if len(masked.Values) == 0 {
				t.Fatal("mask fixture missing")
			}
			for _, v := range masked.Values {
				if !v.Masked || v.Value != nil {
					t.Fatal("sensitive value exposed")
				}
			}
			if c.Driver == "mysql" {
				read("variables", "global", core.ServerAdminReadRequest{Scope: "GLOBAL", Filter: "max_connections"})
			} else {
				_, err = e.ServerAdminRead(ctx, "variables", core.ServerAdminReadRequest{Connection: c, Scope: "GLOBAL"})
				adminWantCode(t, err, "unsupported_action")
			}
			idQuery := "SELECT CAST(CONNECTION_ID() AS CHAR)"
			if c.Driver == "postgres" {
				idQuery = "SELECT pg_backend_pid()::text"
			}
			var id string
			if err = control.QueryRowContext(ctx, idQuery).Scan(&id); err != nil {
				t.Fatal(err)
			}
			processes := read("processes", "processes", core.ServerAdminReadRequest{Filter: c.Username, Limit: 200})
			found := false
			for _, p := range processes.Processes {
				if p.ID == id {
					found = true
					if p.Self {
						t.Fatal("control session marked self")
					}
				}
			}
			if !found {
				t.Fatal("control session missing")
			}
			first := read("processes", "process-page1", core.ServerAdminReadRequest{Filter: c.Username, Limit: 1})
			second := read("processes", "process-page2", core.ServerAdminReadRequest{Filter: c.Username, Limit: 1, Offset: 1})
			if !first.HasMore || len(first.Processes) != 1 || len(second.Processes) != 1 {
				t.Fatalf("process paging failed: hasMore=%v first=%d second=%d", first.HasMore, len(first.Processes), len(second.Processes))
			}
			// Every read runs on its own session, so the listed set differs between the two
			// calls (the reader itself is part of it, ordered by a recycled pid or id). Two
			// snapshots may therefore legitimately both rank the long-lived control session
			// at their requested offset; equal ids only prove a defect when neither is it.
			if a, b := first.Processes[0].ID, second.Processes[0].ID; a == b && a != id {
				t.Fatalf("process paging returned the same foreign session twice: %s", a)
			}
			empty := read("processes", "no-match", core.ServerAdminReadRequest{Filter: "no-such-fixture-user"})
			if len(empty.Processes) != 0 || empty.HasMore {
				t.Fatal("process filter ignored")
			}
			// Reads must leave this independent session usable and its setting unchanged.
			var one int
			if err = control.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
				t.Fatal("control session affected")
			}
			var observedSetting string
			if err = control.QueryRowContext(ctx, settingQuery).Scan(&observedSetting); err != nil || observedSetting != originalSetting {
				t.Fatal("read changed session setting")
			}
			adminLiveVisibility(t, e, c, control)
			wrong := c
			wrong.Password = "never-echo-placeholder"
			_, err = e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: wrong})
			if err == nil || strings.Contains(err.Error(), wrong.Password) {
				t.Fatal("unsafe authentication failure")
			}
		})
	}
}

func adminLiveVisibility(t *testing.T, e *Engine, c core.Connection, control *sql.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	low := c
	low.Username = "w_p17_limited"
	low.Password = "public-disposable-placeholder"
	// Account DDL is fixture setup only; no server variables or connections are changed.
	create := "CREATE USER 'w_p17_limited'@'%' IDENTIFIED BY 'public-disposable-placeholder'"
	drop := "DROP USER 'w_p17_limited'@'%'"
	if c.Driver == "postgres" {
		create = "CREATE ROLE w_p17_limited LOGIN PASSWORD 'public-disposable-placeholder'"
		drop = "DROP ROLE w_p17_limited"
	}
	if _, err := control.ExecContext(ctx, create); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := control.ExecContext(ctx, drop); err != nil {
			t.Error("fixture account cleanup failed")
		}
	}()
	if c.Driver == "mysql" {
		if _, err := control.ExecContext(ctx, "GRANT SELECT ON probe.* TO 'w_p17_limited'@'%'"); err != nil {
			t.Fatal(err)
		}
	}

	result, err := e.ServerAdminRead(ctx, "processes", core.ServerAdminReadRequest{Connection: low, Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	adminDump(t, "ServerAdminReadResult", "limited-visibility", result)
	for _, p := range result.Processes {
		if c.Driver == "mysql" && p.User != low.Username {
			t.Fatal("unprivileged account sees foreign process")
		}
	}
	// PostgreSQL exposes foreign IDs/users, but restricted query metadata is never selected.
}
