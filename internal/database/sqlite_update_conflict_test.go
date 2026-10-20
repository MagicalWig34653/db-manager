// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestSQLiteUpdateOverridesTableReplacePolicy(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE update_replace(id INTEGER PRIMARY KEY,value TEXT UNIQUE ON CONFLICT REPLACE)`)
	execute(t, e, c, `INSERT INTO update_replace VALUES(1,'first'),(2,'second')`)
	ref := core.TableRef{Name: "update_replace"}
	before := mutationSnapshot(t, e, c, ref)
	_, err := e.Update(ctx, core.MutationRequest{Connection: c, Table: ref, Expected: before, Confirm: true, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "second"}}}})
	if err == nil {
		after, readErr := e.Browse(ctx, core.BrowseRequest{Connection: c, Table: ref})
		t.Fatalf("UNIQUE REPLACE returned success; remaining rows=%d readErr=%v", len(after.Result.Rows), readErr)
	}
	after, err := e.Browse(ctx, core.BrowseRequest{Connection: c, Table: ref})
	if err != nil || len(after.Result.Rows) != 2 || after.Result.Rows[0][1].Value != "first" || after.Result.Rows[1][1].Value != "second" {
		t.Fatalf("conflicting update changed rows: %#v %v", after, err)
	}
}
