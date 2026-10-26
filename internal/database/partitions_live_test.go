// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func dumpPartition(t *testing.T, component string, v any) {
	t.Helper()
	dir := os.Getenv("DBM_PARTITIONS_DUMP")
	if dir == "" {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+"."+component+".json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestLivePartitions(t *testing.T) {
	for _, system := range []string{"mysql", "maria", "postgres"} {
		t.Run(system, func(t *testing.T) {
			port := os.Getenv("DBM_TEST_" + strings.ToUpper(system) + "_PORT")
			if port == "" {
				t.Skip("live fixture not configured")
			}
			n, err := strconv.Atoi(port)
			if err != nil {
				t.Fatal(err)
			}
			driver := system
			if driver == "maria" {
				driver = "mysql"
			}
			c := core.Connection{Driver: driver, Host: "localhost", Port: n, Username: "root", Password: os.Getenv("DBM_TEST_PASSWORD"), Database: "probe", TLSMode: "verify-full"}
			if driver == "postgres" {
				c.Username = "postgres"
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			schema := "probe"
			if driver == "postgres" {
				schema = "public"
			}
			prefixCases(t, e, c, schema)
			h := &opHarness{t: t, e: e, c: c, schema: schema}
			ctx := context.Background()
			table := "p29_partitioned"
			ddl := "CREATE TABLE " + h.q(table) + " (id INTEGER NOT NULL)"
			if driver == "postgres" {
				ddl += " PARTITION BY RANGE (id)"
			}
			h.exec(ddl)
			req := core.PartitionRequest{Connection: c, Table: *h.ref(table)}
			run := func(r core.PartitionRequest) core.ObjectOpResult {
				t.Helper()
				p, err := e.PlanPartitions(ctx, r)
				if err != nil {
					t.Fatal(err)
				}
				dumpPartition(t, "StructurePlan", p)
				if p.Confirmation.Destructive {
					_, err = e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: "wrong"}, ExpectedPlanSHA256: p.SHA256})
					requireMutationCode(t, err, "confirmation_mismatch")
				}
				_, err = e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: strings.Repeat("0", 64)})
				requireMutationCode(t, err, "plan_mismatch")
				result, err := e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: r, Confirm: dbplan.Confirmation{Action: p.Action, TargetName: p.Confirmation.TargetName}, ExpectedPlanSHA256: p.SHA256})
				if err != nil || result.Outcome != "completed" {
					t.Fatalf("%+v %v", result, err)
				}
				dumpPartition(t, "StructureResult", result)
				return result
			}
			if driver == "mysql" {
				r := req
				r.Action = "define"
				r.Method = "RANGE"
				r.Column = "id"
				r.Definitions = []core.PartitionDefinition{{Name: "p29_p0", LessThan: sp("10")}, {Name: "p29_p1", LessThan: sp("20")}}
				run(r)
			} else {
				for _, d := range []core.PartitionDefinition{{Name: "p29_p0", From: sp("0"), To: sp("10")}, {Name: "p29_p1", From: sp("10"), To: sp("20")}} {
					r := req
					r.Action = "add"
					r.Definitions = []core.PartitionDefinition{d}
					run(r)
				}
			}
			h.exec("INSERT INTO " + h.q(table) + " VALUES (1),(11)")
			info, err := e.Partitions(ctx, req)
			if err != nil || info.Method != "RANGE" || len(info.Partitions) != 2 {
				t.Fatalf("%+v %v", info, err)
			}
			dumpPartition(t, "PartitionsResult", info)
			r := req
			r.Action = "truncate"
			r.Partitions = []string{"p29_p0"}
			run(r)
			if h.count(table) != "1" {
				t.Fatal("truncate did not isolate partition")
			}
			r.Action = "drop"
			r.Partitions = []string{"p29_p1"}
			run(r)
			if h.count(table) != "0" || h.count("p29_keep") != "1" {
				t.Fatal("partition drop changed neighbor or retained data")
			}
			// Missing catalog target must never be interpreted as SQL.
			r.Partitions = []string{"p29_p0; DROP TABLE p29_keep"}
			_, err = e.PlanPartitions(ctx, r)
			requireMutationCode(t, err, "object_not_found")
			if driver == "mysql" {
				for _, action := range []string{"analyze", "check", "optimize", "rebuild"} {
					r = req
					r.Action = action
					r.Partitions = []string{"p29_p0"}
					run(r)
				}
				r = req
				r.Action = "repair"
				r.Partitions = []string{"p29_p0"}
				_, err = e.PlanPartitions(ctx, r)
				requireMutationCode(t, err, "unsupported_action")
				r = req
				r.Action = "reorganize"
				r.Partitions = []string{"p29_p0"}
				r.Definitions = []core.PartitionDefinition{{Name: "p29_pa", LessThan: sp("5")}, {Name: "p29_pb", LessThan: sp("10")}}
				run(r)
				r.Partitions = []string{"p29_pa", "p29_pb"}
				r.Definitions = []core.PartitionDefinition{{Name: "p29_p0", LessThan: sp("10")}}
				run(r)
				r = req
				r.Action = "add"
				r.Definitions = []core.PartitionDefinition{{Name: "odd`\"partition", LessThan: sp("20")}}
				run(r)
				r = req
				r.Action = "remove"
				run(r)
				for _, method := range []string{"HASH", "KEY"} {
					r = req
					r.Action = "define"
					r.Method = method
					r.Column = "id"
					r.Count = 4
					run(r)
					r = req
					r.Action = "coalesce"
					r.Count = 1
					run(r)
					r.Action = "add"
					r.Count = 1
					run(r)
					r = req
					r.Action = "remove"
					run(r)
				}
				r = req
				r.Action = "define"
				r.Method = "LIST"
				r.Column = "id"
				r.Definitions = []core.PartitionDefinition{{Name: "p29_p0", Values: []string{"1", "2"}}}
				run(r)
				r = req
				r.Action = "add"
				r.Definitions = []core.PartitionDefinition{{Name: "p29_p1", Values: []string{"3"}}}
				run(r)
			} else {
				r = req
				r.Action = "analyze"
				r.Partitions = []string{"p29_p0"}
				run(r)
				r = req
				r.Action = "detach"
				r.Partitions = []string{"p29_p0"}
				run(r)
				if !h.has("p29_p0") {
					t.Fatal("detach deleted child")
				}
				r = req
				r.Action = "attach"
				r.Definitions = []core.PartitionDefinition{{Name: "p29_p0", From: sp("0"), To: sp("10")}}
				run(r)
			}
			// A visible partition does not grant mutation rights.
			ro := c
			ro.Username = "p29_ro"
			ro.Password = "public-p29-fixture"
			if driver == "mysql" {
				h.exec("CREATE USER 'p29_ro'@'%' IDENTIFIED BY 'public-p29-fixture'")
				h.exec("GRANT SELECT ON " + quoteIdentifier(driver, schema) + ".* TO 'p29_ro'@'%'")
			} else {
				h.exec("CREATE ROLE p29_ro LOGIN PASSWORD 'public-p29-fixture'")
				h.exec("GRANT USAGE ON SCHEMA public TO p29_ro")
				h.exec("GRANT SELECT ON ALL TABLES IN SCHEMA public TO p29_ro")
			}
			r = req
			r.Connection = ro
			r.Action = "truncate"
			r.Partitions = []string{"p29_p0"}
			plan, err := e.PlanPartitions(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			result, err := e.ExecutePartitions(ctx, core.StructureExecuteRequest{PartitionRequest: r, Confirm: dbplan.Confirmation{Action: plan.Action, TargetName: plan.Confirmation.TargetName}, ExpectedPlanSHA256: plan.SHA256})
			if err != nil || result.Outcome != "rejected" || result.Reason != "permission_denied" {
				t.Fatalf("rights: %+v %v", result, err)
			}

		})
	}
}
