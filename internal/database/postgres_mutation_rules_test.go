// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestLiveMutationsPostgresRules(t *testing.T) {
	raw := os.Getenv("DBM_TEST_POSTGRES_PORT")
	if raw == "" {
		t.Skip("Disposable PostgreSQL TLS fixture required")
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
	if err != nil {
		t.Fatal(err)
	}
	c := core.Connection{Driver: "postgres", Host: "localhost", Port: port, Database: "probe", Username: "postgres", Password: "public-b1-fixture"}
	ctx := context.Background()
	for _, operation := range []string{"update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			source := "rule_source_" + operation
			target := "rule_target_" + operation
			execute(t, e, c, "CREATE TABLE "+source+"(id INTEGER PRIMARY KEY,value TEXT)")
			execute(t, e, c, "CREATE TABLE "+target+"(id INTEGER PRIMARY KEY,value TEXT)")
			execute(t, e, c, "INSERT INTO "+source+" VALUES(1,'source')")
			execute(t, e, c, "INSERT INTO "+target+" VALUES(1,'target')")
			action := "UPDATE " + target + " SET value=NEW.value WHERE id=NEW.id"
			if operation == "delete" {
				action = "DELETE FROM " + target + " WHERE id=OLD.id"
			}
			execute(t, e, c, "CREATE RULE redirect_"+operation+" AS ON "+operation+" TO "+source+" DO INSTEAD "+action)
			ref := core.TableRef{Name: source}
			before := mutationSnapshot(t, e, c, ref)
			request := core.MutationRequest{Connection: c, Table: ref, Expected: before, Confirm: true}
			var result core.MutationResult
			var err error
			if operation == "update" {
				request.Changes = []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "redirected"}}}
				result, err = e.Update(ctx, request)
			} else {
				result, err = e.Delete(ctx, request)
			}
			victim, readErr := e.Browse(ctx, core.BrowseRequest{Connection: c, Table: core.TableRef{Name: target}})
			var p *core.Error
			if !errors.As(err, &p) || p.Code != "mutation_not_supported" {
				t.Fatalf("rule mutation unexpectedly accepted: result=%#v err=%v target=%#v readErr=%v", result, err, victim.Result.Rows, readErr)
			}
			if readErr != nil || len(victim.Result.Rows) != 1 || victim.Result.Rows[0][1].Value != "target" {
				t.Fatalf("rule changed unrelated row %#v %v", victim, readErr)
			}
			sourceAfter := mutationSnapshot(t, e, c, ref)
			if sourceAfter.Values[1].Value != "source" {
				t.Fatal("source changed")
			}
			eligible, err := e.Eligibility(ctx, core.DescribeRequest{Connection: c, Table: ref})
			if err != nil || eligible.Eligible || eligible.Reason != "unsupported_table" {
				t.Fatalf("rule eligibility %#v %v", eligible, err)
			}
		})
	}
}
