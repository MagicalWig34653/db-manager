// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func dumpMonitor(t *testing.T, name string, value any) {
	t.Helper()
	if dir := os.Getenv("DBM_MONITOR_DUMP"); dir != "" {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name+".MonitorResult.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestSQLiteMonitor(t *testing.T) {
	e := testEngine(t)
	c := core.Connection{Driver: "sqlite", Database: "monitor"}
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE sentinel (id INTEGER PRIMARY KEY)")
	execute(t, e, c, "INSERT INTO sentinel VALUES(7)")
	for _, action := range []string{"sample", "engines"} {
		r, err := e.Monitor(ctx, action, core.MonitorRequest{Connection: c})
		if err != nil || r.State != "available" || r.Source != "sqlite_version" || len(r.Records) != 1 || r.Records[0]["version"] == "" {
			t.Fatalf("%s %#v %v", action, r, err)
		}
		raw, _ := json.Marshal(r)
		if strings.Contains(string(raw), e.dataDir) || strings.Contains(string(raw), "sentinel") {
			t.Fatal("private data leaked")
		}
		dumpMonitor(t, "sqlite-"+action, r)
	}
	for _, action := range []string{"analysis", "advisor", "replication", "binary-logs", "binary-events", "profiling"} {
		request := core.MonitorRequest{Connection: c, SQL: ""}
		if action == "profiling" {
			request.SQL = "SELECT 1"
		}
		if action == "binary-events" {
			request.Log = "binlog.000001"
		}
		_, err := e.Monitor(ctx, action, request)
		requireMutationCode(t, err, "unsupported_feature")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := e.Monitor(ctx, "sample", core.MonitorRequest{Connection: c})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	if queryText(t, e, c, "SELECT id FROM sentinel") != "7" {
		t.Fatal("read changed sentinel")
	}
}
func TestMonitorValidation(t *testing.T) {
	for _, query := range []string{"SELECT 1", "select id FROM sentinel WHERE id >= 1 LIMIT 10", "SELECT id,value FROM public.sentinel", "SELECT * FROM sentinel"} {
		r := core.MonitorRequest{SQL: query}
		if err := normalizeMonitor("profiling", &r); err != nil {
			t.Fatalf("valid %q: %v", query, err)
		}
	}
	for _, query := range []string{"SELECT 1; DELETE FROM x", "SELECT SLEEP(10)", "SELECT pg_sleep(1)", "EXPLAIN ANALYZE SELECT 1", "SELECT 1 INTO OUTFILE '/tmp/x'", "WITH x AS (DELETE FROM y) SELECT 1", "SET PROFILING=1", "START REPLICA", "STOP SLAVE", "PURGE BINARY LOGS", "SELECT /* hint */ 1", "SELECT /*!50000 SLEEP(1) */ 1", "SELECT @x:=1", "SELECT 1 FOR UPDATE", "SELECT 1;", "SELECT (SELECT 1)"} {
		r := core.MonitorRequest{SQL: query}
		requireMutationCode(t, normalizeMonitor("profiling", &r), "invalid_monitor")
	}
	for _, r := range []core.MonitorRequest{{Limit: 201}, {Offset: 10001}, {TimeoutMS: 30001}, {Limit: -1}, {Offset: -1}, {TimeoutMS: -1}, {SQL: "SELECT 1"}, {Log: "bin.000001"}, {Position: 1}} {
		requireMutationCode(t, normalizeMonitor("sample", &r), "invalid_monitor")
	}
	for _, log := range []string{"../bin.000001", "/var/log/bin.000001", "bin.000001' OR 1=1", "bin.000001\\", "bin.000001\n", ""} {
		r := core.MonitorRequest{Log: log}
		requireMutationCode(t, normalizeMonitor("binary-events", &r), "invalid_monitor")
	}
	for _, position := range []int64{-1, 9007199254740992} {
		r := core.MonitorRequest{Log: "bin.000001", Position: position}
		requireMutationCode(t, normalizeMonitor("binary-events", &r), "invalid_monitor")
	}
}
func TestMonitorRedactionAndPaging(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	db, cleanup, err := e.open(ctx, core.Connection{Driver: "sqlite", Database: "redaction"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	query := "SELECT 1 AS Pos, 'hidden-password' AS Master_Password, '/private/path' AS Relay_Log_File, 'SELECT secret' AS Info UNION ALL SELECT 2,'other','other','other' UNION ALL SELECT 3,'other','other','other'"
	out := core.MonitorResult{Records: []core.MonitorRecord{}}
	err = monitorRead(ctx, db, query, numericFields("Pos"), core.MonitorRequest{Limit: 1, Offset: 1}, &out)
	if err != nil || len(out.Records) != 1 || out.Records[0]["pos"] != "2" || len(out.Records[0]) != 1 || !out.HasMore {
		t.Fatalf("page/redaction %#v %v", out, err)
	}
	out = core.MonitorResult{Records: []core.MonitorRecord{}}
	err = monitorRead(ctx, db, query, numericFields("Pos"), core.MonitorRequest{Limit: 1, Offset: 2}, &out)
	if err != nil || out.HasMore || out.Records[0]["pos"] != "3" {
		t.Fatalf("last page %#v %v", out, err)
	}
	for _, value := range []string{"NaN", "Inf", "1;select secret", "/private/path", "1e999"} {
		if monitorValue(value, "number") {
			t.Fatalf("unsafe numeric %q", value)
		}
	}
	raw := "History list length 123\nSELECT 'secret'; /private/path\nFree buffers 8\n"
	matches := innodbMetric.FindAllStringSubmatch(raw, -1)
	if len(matches) != 2 || matches[0][2] != "123" {
		t.Fatalf("metric extraction %#v", matches)
	}
}
func TestMonitorAdvisor(t *testing.T) {
	records := []core.MonitorRecord{{"threads_running": "33", "slow_queries": "1"}}
	a := monitorAdvice(records, "global_status")
	b := monitorAdvice(records, "global_status")
	if !reflect.DeepEqual(a, b) || len(a) != 2 || a[0].Source != "global_status" || a[0].Threshold != "> 32" {
		t.Fatalf("rules %#v", a)
	}
	if got := monitorAdvice([]core.MonitorRecord{{"threads_running": "32", "slow_queries": "0"}}, "global_status"); len(got) != 0 {
		t.Fatalf("boundary %#v", got)
	}
}

func TestLiveMonitor(t *testing.T) {
	for _, system := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.env)
			if raw == "" {
				t.Skip("disposable fixture port required")
			}
			// The required sources (performance schema, slow log table, binary log,
			// pg_stat_statements preload) exist only on the specially started monitor
			// fixtures of scripts/test-monitor.py; plain engine fixtures do not provide them.
			if os.Getenv("DBM_TEST_MONITOR_FIXTURE") != "1" {
				t.Skip("monitor-configured fixture required (scripts/test-monitor.py)")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: system.driver, Host: "localhost", Port: port, Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), Database: "probe", TLSMode: "verify-full"}
			if c.Driver == "postgres" {
				c.Username = "postgres"
			}
			if c.Driver == "postgres" {
				absent, err := e.Monitor(context.Background(), "analysis", core.MonitorRequest{Connection: c})
				if err != nil || absent.State != "unavailable" {
					t.Fatalf("missing extension %#v %v", absent, err)
				}
				execute(t, e, c, "CREATE EXTENSION pg_stat_statements")
			}
			execute(t, e, c, "CREATE TABLE monitor_sentinel (id INTEGER PRIMARY KEY)")
			execute(t, e, c, "INSERT INTO monitor_sentinel VALUES(7)")
			for _, action := range []string{"sample", "analysis", "advisor", "replication", "binary-logs", "profiling", "engines"} {
				request := core.MonitorRequest{Connection: c, Limit: 1}
				if action == "profiling" {
					request.SQL = "SELECT id FROM monitor_sentinel WHERE id = 7"
				}
				out, err := e.Monitor(context.Background(), action, request)
				if system.driver == "postgres" && action == "binary-logs" {
					requireMutationCode(t, err, "unsupported_feature")
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", action, err)
				}
				if (action == "sample" || action == "profiling" || action == "engines" || action == "analysis") && (out.State != "available" || len(out.Records) == 0) {
					t.Fatalf("required source %s %#v", action, out)
				}
				if len(out.Records) > 1 {
					t.Fatalf("page bound %#v", out)
				}
				encoded, _ := json.Marshal(out)
				for _, secret := range []string{c.Password, "SELECT id", e.dataDir} {
					if secret != "" && strings.Contains(string(encoded), secret) {
						t.Fatalf("private data in %s", action)
					}
				}
				dumpMonitor(t, system.name+"-"+action, out)
				if action == "binary-logs" && len(out.Records) > 0 && out.Records[0]["log_name"] != "" {
					log := out.Records[0]["log_name"]
					events, err := e.Monitor(context.Background(), "binary-events", core.MonitorRequest{Connection: c, Log: log, Limit: 1})
					if err != nil {
						t.Fatal(err)
					}
					if len(events.Records) > 1 {
						t.Fatal("event bound")
					}
					dumpMonitor(t, system.name+"-binary-events", events)
				}
			}
			if queryText(t, e, c, "SELECT id FROM monitor_sentinel") != "7" {
				t.Fatal("monitor mutated data")
			}
			// A fresh unprivileged account must never receive privileged statistics.
			restricted := c
			restricted.Username = "monitor_limited"
			restricted.Password = "fixture-monitor-only"
			if c.Driver == "postgres" {
				execute(t, e, c, "CREATE ROLE monitor_limited LOGIN PASSWORD 'fixture-monitor-only'")
			} else {
				execute(t, e, c, "CREATE USER 'monitor_limited'@'%' IDENTIFIED BY 'fixture-monitor-only'")
				restricted.Database = ""
			}
			for _, action := range []string{"replication", "analysis"} {
				out, err := e.Monitor(context.Background(), action, core.MonitorRequest{Connection: restricted})
				if err != nil {
					t.Fatal(err)
				}
				if out.State != "unavailable" || len(out.Records) != 0 {
					t.Fatalf("rights %s %#v", action, out)
				}
			}
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			before := time.Now()
			_, err = e.Monitor(canceled, "sample", core.MonitorRequest{Connection: c})
			if !errors.Is(err, context.Canceled) || time.Since(before) > 2*time.Second {
				t.Fatalf("cancel %v", err)
			}
		})
	}
}

func TestMonitorPostgresPlanRedaction(t *testing.T) {
	raw := `[{"Plan":{"Node Type":"Limit","Startup Cost":0.00,"Total Cost":1.25,"Plan Rows":1,"Plan Width":8,"Output":["sensitive-password"],"Plans":[{"Node Type":"Seq Scan","Startup Cost":0,"Total Cost":10,"Plan Rows":100,"Plan Width":8,"Relation Name":"secret_table","Filter":"password = 'hidden'","Path":"/private/hidden"}]}}]`
	out := core.MonitorResult{Records: []core.MonitorRecord{}}
	err := monitorPostgresPlan(raw, core.MonitorRequest{Limit: 1, Offset: 1}, &out)
	if err != nil || len(out.Records) != 1 || out.HasMore || out.Records[0]["node_type"] != "Seq Scan" || out.Records[0]["plan_rows"] != "100" {
		t.Fatalf("plan %#v %v", out, err)
	}
	encoded, _ := json.Marshal(out)
	for _, secret := range []string{"sensitive", "hidden", "secret_table", "/private", "Filter"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("plan leaked %s", secret)
		}
	}
	out = core.MonitorResult{Records: []core.MonitorRecord{}}
	err = monitorPostgresPlan(raw, core.MonitorRequest{Limit: 1}, &out)
	if err != nil || !out.HasMore {
		t.Fatalf("plan page %#v %v", out, err)
	}
	requireMutationCode(t, monitorPostgresPlan(strings.Repeat("x", MaxCellBytes+1), core.MonitorRequest{Limit: 1}, &out), "result_too_large")
	if monitorPostgresPlan("broken", core.MonitorRequest{Limit: 1}, &out) == nil {
		t.Fatal("invalid plan accepted")
	}
}
