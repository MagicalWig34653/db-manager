// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestSQLiteRowsBulkEdges(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE p13_auto(id INTEGER PRIMARY KEY AUTOINCREMENT,value TEXT,raw BLOB)")
	execute(t, e, c, "INSERT INTO p13_auto(value,raw) VALUES('one',X''),('two',X'00')")
	ref := core.TableRef{Name: "p13_auto"}
	request := core.RowsBulkRequest{Connection: c, Table: ref, Action: "copy", Rows: bulkSnapshots(t, e, c, ref)}
	request = confirmBulk(t, e, request)
	result, err := e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "completed" || result.Rows[0].Row.Values[0].Value != "3" || result.Rows[1].Row.Values[0].Value != "4" {
		t.Fatal(result, err)
	}
	all := bulkSnapshots(t, e, c, ref)
	request = core.RowsBulkRequest{Connection: c, Table: ref, Action: "delete", Rows: []core.BulkRow{all[0], all[0]}}
	_, err = e.RowsBulkPlan(ctx, request)
	requireMutationCode(t, err, "invalid_bulk")
	request.Rows = make([]core.BulkRow, 101)
	_, err = e.RowsBulkPlan(ctx, request)
	requireMutationCode(t, err, "invalid_bulk")
	request.Rows = all[:1]
	request.Action = "update"
	request.Changes = []core.RowChange{{Column: "id", Value: core.Cell{Kind: "integer", Value: "30"}}}
	_, err = e.RowsBulkPlan(ctx, request)
	requireMutationCode(t, err, "invalid_bulk")
	request.Changes = []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "one"}}}
	request = confirmBulk(t, e, request)
	execute(t, e, c, "CREATE TABLE p13_audit(value INTEGER)")
	execute(t, e, c, "CREATE TRIGGER p13_noop AFTER UPDATE ON p13_auto BEGIN INSERT INTO p13_audit VALUES(1); END")
	// Replan after DDL because table metadata describes columns rather than trigger definitions.
	result, err = e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "completed" {
		t.Fatal(result, err)
	}
	if count := execute(t, e, c, "SELECT count(*) FROM p13_audit").Results[0].Rows[0][0].Value; count != "0" {
		t.Fatal("noop fired trigger", count)
	}
	execute(t, e, c, "DROP TRIGGER p13_noop")
	// A later trigger changing a prior result makes the whole batch reject.
	execute(t, e, c, "CREATE TRIGGER p13_later AFTER UPDATE ON p13_auto WHEN NEW.id=2 BEGIN UPDATE p13_auto SET value='trigger' WHERE id=1; END")
	request = core.RowsBulkRequest{Connection: c, Table: ref, Action: "update", Rows: bulkSnapshots(t, e, c, ref)[:2], Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "new"}}}}
	request = confirmBulk(t, e, request)
	result, err = e.RowsBulkExecute(ctx, request)
	if err == nil || result.Outcome != "rejected" {
		t.Fatal(result, err)
	}
	if got := bulkSnapshots(t, e, c, ref); got[0].Expected.Values[1].Value != "one" || got[1].Expected.Values[1].Value != "two" {
		t.Fatal("trigger rollback", got)
	}
	execute(t, e, c, "DROP TRIGGER p13_later")
	// Upper file bound is actually accepted; limit+1 is rejected before writes.
	fr := core.FileValueRequest{Connection: c, Table: ref, Expected: bulkSnapshots(t, e, c, ref)[0].Expected, Column: "raw", Confirm: true}
	upload, err := e.FileValueUpload(ctx, fr, strings.NewReader(strings.Repeat("a", core.MaxFileValueBytes)))
	if err != nil {
		t.Fatal("file upper bound", err)
	}
	fr.Expected = *upload.Row
	var downloaded bytes.Buffer
	err = e.FileValueDownload(ctx, fr, func(_ string, size int64) (io.Writer, error) {
		if size != core.MaxFileValueBytes {
			t.Fatal(size)
		}
		return &downloaded, nil
	})
	if err != nil || downloaded.Len() != core.MaxFileValueBytes {
		t.Fatal(downloaded.Len(), err)
	}
	// Composite keys and binary collation preserve exact SQLite identity.
	execute(t, e, c, "CREATE TABLE p13_composite(a TEXT COLLATE NOCASE NOT NULL,b INTEGER NOT NULL,value TEXT,PRIMARY KEY(a COLLATE BINARY,b))")
	execute(t, e, c, "INSERT INTO p13_composite VALUES('a',1,'lower'),('A',1,'upper')")
	composite := core.TableRef{Name: "p13_composite"}
	request = core.RowsBulkRequest{Connection: c, Table: composite, Action: "delete", Rows: bulkSnapshots(t, e, c, composite)[:1]}
	request = confirmBulk(t, e, request)
	result, err = e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "completed" || len(bulkSnapshots(t, e, c, composite)) != 1 {
		t.Fatal(result, err)
	}
	// Genuine deferred-FK failure at COMMIT has an unknown outcome, never rejected/completed.
	execute(t, e, c, "CREATE TABLE p13_fkparent(id INTEGER PRIMARY KEY)")
	execute(t, e, c, "INSERT INTO p13_fkparent VALUES(1)")
	execute(t, e, c, "CREATE TABLE p13_deferred(id INTEGER PRIMARY KEY,parent INTEGER REFERENCES p13_fkparent(id) DEFERRABLE INITIALLY DEFERRED)")
	execute(t, e, c, "INSERT INTO p13_deferred VALUES(1,1),(2,1)")
	deferred := core.TableRef{Name: "p13_deferred"}
	request = core.RowsBulkRequest{Connection: c, Table: deferred, Action: "update", Rows: bulkSnapshots(t, e, c, deferred), Changes: []core.RowChange{{Column: "parent", Value: core.Cell{Kind: "integer", Value: "999"}}}}
	request = confirmBulk(t, e, request)
	result, err = e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "outcome_unknown" || result.Rows[0].Outcome != "outcome_unknown" || result.Rows[0].Row != nil {
		t.Fatal(result, err)
	}
	for _, row := range bulkSnapshots(t, e, c, deferred) {
		if row.Expected.Values[1].Value != "1" {
			t.Fatal("concrete failed commit changed data")
		}
	}
}
