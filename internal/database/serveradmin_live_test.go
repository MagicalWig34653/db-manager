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
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
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
			// Dedicated fixtures preserve a target session and an unaffected control session.
			db.SetMaxOpenConns(4)
			target, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer target.Close()
			control, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer control.Close()
			var targetID string
			idQuery := "SELECT CAST(CONNECTION_ID() AS CHAR)"
			if c.Driver == "postgres" {
				idQuery = "SELECT pg_backend_pid()::text"
			}
			if err = target.QueryRowContext(ctx, idQuery).Scan(&targetID); err != nil {
				t.Fatal(err)
			}
			status := func() core.ServerAdminReadResult {
				r, err := e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: c, Limit: 200})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}
			before := status()
			for i := 0; i < 5; i++ {
				var n int
				if err = control.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
					t.Fatal(err)
				}
			}
			if c.Driver == "postgres" {
				time.Sleep(1100 * time.Millisecond)
			}
			after := status()
			adminDump(t, "ServerAdminReadResult", "status", after)
			if len(after.Values) == 0 || after.SampledAt == before.SampledAt {
				t.Fatal("missing metrics/timestamp")
			}
			if c.Driver == "mysql" {
				find := func(r core.ServerAdminReadResult) int64 {
					for _, v := range r.Values {
						if v.Name == "Com_select" && v.Value != nil {
							n, _ := strconv.ParseInt(*v.Value, 10, 64)
							return n
						}
					}
					return -1
				}
				if find(after) <= find(before) {
					t.Fatal("synthetic SELECT load did not increase Com_select")
				}
			}
			vars, err := e.ServerAdminRead(ctx, "variables", core.ServerAdminReadRequest{Connection: c, Filter: "timeout", Limit: 3})
			if err != nil || len(vars.Values) > 3 {
				t.Fatalf("variable filter/page %#v %v", vars, err)
			}
			adminDump(t, "ServerAdminReadResult", "variables", vars)
			for _, v := range vars.Values {
				if !strings.Contains(strings.ToLower(v.Name), "timeout") {
					t.Fatal("filter ignored")
				}
			}
			bad := core.ServerAdminRequest{Connection: c, Name: "password", Scope: "GLOBAL", Value: "16384"}
			if _, err = e.ServerAdminPlan(ctx, "set_variable", bad); err == nil {
				t.Fatal("nonallowlisted setting accepted")
			}
			set := core.ServerAdminRequest{Connection: c, Name: "tmp_table_size", Scope: "SESSION", Value: "32768"}
			if c.Driver == "postgres" {
				set.Name = "statement_timeout"
				set.Value = "1000"
			}
			run := func(action string, r core.ServerAdminRequest) core.ServerAdminResult {
				t.Helper()
				p, err := e.ServerAdminPlan(ctx, action, r)
				if err != nil {
					t.Fatal(err)
				}
				adminDump(t, "ServerAdminPlan", action+"-"+r.Mode+"-"+r.Scope, p)
				x := core.ServerAdminExecuteRequest{ServerAdminRequest: r, Confirm: dbplan.Confirmation{Action: action, TargetName: p.TargetName}, ExpectedPlanSHA256: p.SHA256}
				x.Confirm.TargetName = "wrong"
				_, err = e.ServerAdminExecute(ctx, action, x)
				adminWantCode(t, err, "confirmation_mismatch")
				x.Confirm.TargetName = p.TargetName
				x.ExpectedPlanSHA256 = strings.Repeat("0", 64)
				_, err = e.ServerAdminExecute(ctx, action, x)
				adminWantCode(t, err, "plan_mismatch")
				x.ExpectedPlanSHA256 = p.SHA256
				res, err := e.ServerAdminExecute(ctx, action, x)
				if err != nil || res.Outcome != dbplan.StatusCompleted {
					t.Fatalf("execute %#v %v", res, err)
				}
				adminDump(t, "ServerAdminResult", action+"-"+r.Mode+"-"+r.Scope, res)
				return res
			}
			res := run("set_variable", set)
			if res.ObservedValue == nil {
				t.Fatal("no same-session readback")
			}
			// A new SESSION read must show the original setting, not the closed API session's value.
			var original string
			settingQuery := "SELECT @@SESSION.tmp_table_size"
			if c.Driver == "postgres" {
				settingQuery = "SHOW statement_timeout"
			}
			if err = control.QueryRowContext(ctx, settingQuery).Scan(&original); err != nil {
				t.Fatal(err)
			}
			freshDB, freshCleanup, openErr := e.open(ctx, c)
			if openErr != nil {
				t.Fatal(openErr)
			}
			var freshValue string
			readErr := freshDB.QueryRowContext(ctx, settingQuery).Scan(&freshValue)
			freshCleanup()
			if readErr != nil || freshValue != original {
				t.Fatal("SESSION setting escaped closed API connection")
			}
			if c.Driver == "mysql" {
				var old string
				if err = control.QueryRowContext(ctx, "SELECT @@GLOBAL.tmp_table_size").Scan(&old); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := control.ExecContext(context.Background(), "SET GLOBAL tmp_table_size = "+old); err != nil {
						t.Error("fixture restore failed")
					}
				}()
				set.Scope = "GLOBAL"
				set.Value = "33554432"
				run("set_variable", set)
				var observed string
				if err = control.QueryRowContext(ctx, "SELECT @@GLOBAL.tmp_table_size").Scan(&observed); err != nil || observed != set.Value {
					t.Fatal("independent GLOBAL readback failed")
				}
			}
			// Kill a long query, then the same dedicated connection; a control connection survives both.
			done := make(chan error, 1)
			go func() {
				sleep := "SELECT SLEEP(20)"
				if c.Driver == "postgres" {
					sleep = "SELECT pg_sleep(20)"
				}
				_, err := target.ExecContext(ctx, sleep)
				done <- err
			}()
			var p core.ServerAdminProcess
			for attempt := 0; attempt < 30; attempt++ {
				listed, err := e.ServerAdminRead(ctx, "processes", core.ServerAdminReadRequest{Connection: c, Limit: 200})
				if err != nil {
					t.Fatal(err)
				}
				for _, candidate := range listed.Processes {
					if candidate.ID == targetID {
						p = candidate
					}
				}
				if p.ID != "" && p.Seconds >= 1 {
					adminDump(t, "ServerAdminReadResult", "processes", listed)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if p.ID == "" || p.Seconds < 1 {
				t.Fatal("long query missing")
			}
			kill := core.ServerAdminRequest{Connection: c, ID: p.ID, Identity: p.QueryIdentity, Mode: "query"}
			run("kill", kill)
			select {
			case queryErr := <-done:
				if queryErr == nil {
					t.Fatal("long query completed without cancellation")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("target query still running")
			}
			var one int
			if err = control.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
				t.Fatal("unrelated control connection affected")
			}
			listed, err := e.ServerAdminRead(ctx, "processes", core.ServerAdminReadRequest{Connection: c, Limit: 200})
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range listed.Processes {
				if candidate.ID == targetID {
					p = candidate
				}
			}
			kill.Identity = p.Identity
			kill.Mode = "connection"
			run("kill", kill)
			terminated := false
			for attempt := 0; attempt < 20; attempt++ {
				if target.QueryRowContext(ctx, "SELECT 1").Scan(&one) != nil {
					terminated = true
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if !terminated {
				t.Fatal("target connection survived termination")
			}
			if err = control.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
				t.Fatal("control affected by termination")
			}
			adminLivePrivileges(t, e, c, control)
			wrong := c
			wrong.Password = "never-echo-placeholder"
			_, err = e.ServerAdminRead(ctx, "status", core.ServerAdminReadRequest{Connection: wrong})
			if err == nil || strings.Contains(err.Error(), wrong.Password) {
				t.Fatal("unsafe authentication failure")
			}
		})
	}
}
func adminLivePrivileges(t *testing.T, e *Engine, c core.Connection, control *sql.Conn) {
	t.Helper()
	ctx := context.Background()
	low := c
	low.Username = "w_p17_limited"
	low.Password = "public-disposable-placeholder"
	if c.Driver == "mysql" {
		if _, err := control.ExecContext(ctx, "CREATE USER 'w_p17_limited'@'%' IDENTIFIED BY 'public-disposable-placeholder'"); err != nil {
			t.Fatal(err)
		}
		defer control.ExecContext(ctx, "DROP USER 'w_p17_limited'@'%'")
		if _, err := control.ExecContext(ctx, "GRANT SELECT ON probe.* TO 'w_p17_limited'@'%'"); err != nil {
			t.Fatal(err)
		}
		r := core.ServerAdminRequest{Connection: low, Name: "tmp_table_size", Scope: "GLOBAL", Value: "33554432"}
		p, err := e.ServerAdminPlan(ctx, "set_variable", r)
		if err != nil {
			t.Fatal(err)
		}
		res, err := e.ServerAdminExecute(ctx, "set_variable", core.ServerAdminExecuteRequest{ServerAdminRequest: r, Confirm: dbplan.Confirmation{Action: "set_variable", TargetName: p.TargetName}, ExpectedPlanSHA256: p.SHA256})
		if err != nil || res.Outcome != dbplan.StatusRejected || res.Reason != "permission_denied" {
			t.Fatalf("missing privilege not rejected %#v %v", res, err)
		}
		adminDump(t, "ServerAdminResult", "permission", res)
	} else {
		if _, err := control.ExecContext(ctx, "CREATE ROLE w_p17_limited LOGIN PASSWORD 'public-disposable-placeholder'"); err != nil {
			t.Fatal(err)
		}
		defer control.ExecContext(ctx, "DROP ROLE w_p17_limited")
		if _, err := control.ExecContext(ctx, "GRANT pg_read_all_stats TO w_p17_limited"); err != nil {
			t.Fatal(err)
		}
		var id string
		if err := control.QueryRowContext(ctx, "SELECT pg_backend_pid()::text").Scan(&id); err != nil {
			t.Fatal(err)
		}
		listed, err := e.ServerAdminRead(ctx, "processes", core.ServerAdminReadRequest{Connection: low, Limit: 200})
		if err != nil {
			t.Fatal(err)
		}
		var identity string
		for _, p := range listed.Processes {
			if p.ID == id {
				identity = p.Identity
			}
		}
		if identity == "" {
			t.Fatal("privilege fixture not visible")
		}
		r := core.ServerAdminRequest{Connection: low, ID: id, Identity: identity, Mode: "connection"}
		p, err := e.ServerAdminPlan(ctx, "kill", r)
		if err != nil {
			t.Fatal(err)
		}
		res, err := e.ServerAdminExecute(ctx, "kill", core.ServerAdminExecuteRequest{ServerAdminRequest: r, Confirm: dbplan.Confirmation{Action: "kill", TargetName: p.TargetName}, ExpectedPlanSHA256: p.SHA256})
		if err != nil || res.Outcome != dbplan.StatusRejected || res.Reason != "permission_denied" {
			t.Fatalf("superuser termination allowed %#v %v", res, err)
		}
		adminDump(t, "ServerAdminResult", "permission", res)
	}
}
