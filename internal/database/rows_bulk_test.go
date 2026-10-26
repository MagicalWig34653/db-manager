// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func dumpRowsBulk(t *testing.T, component string, value any) {
	t.Helper()
	if dir := os.Getenv("DBM_ROWS_BULK_DUMP"); dir != "" {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, component+".real-"+strings.ReplaceAll(t.Name(), "/", "-")+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func bulkSnapshots(t *testing.T, e *Engine, c core.Connection, ref core.TableRef) []core.BulkRow {
	t.Helper()
	page, err := e.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: ref, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	columns := []string{}
	for _, c := range page.Description.Columns {
		columns = append(columns, c.Name)
	}
	rows := []core.BulkRow{}
	for _, r := range page.Result.Rows {
		rows = append(rows, core.BulkRow{Expected: core.RowSnapshot{Columns: columns, Values: r}})
	}
	return rows
}
func confirmBulk(t *testing.T, e *Engine, r core.RowsBulkRequest) core.RowsBulkRequest {
	t.Helper()
	p, err := e.RowsBulkPlan(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.RowCount != len(r.Rows) || len(p.PlanSHA256) != 64 {
		t.Fatal(p)
	}
	dumpRowsBulk(t, "RowsBulkPlan", p)
	for i := range r.Rows {
		r.Rows[i].StabilitySHA256 = p.Rows[i].StabilitySHA256
	}
	r.Confirm = true
	r.ExpectedPlanSHA256 = p.PlanSHA256
	return r
}
func TestSQLiteRowsBulk(t *testing.T) { checkRowsBulk(t, testEngine(t), sqliteConnection()) }
func checkRowsBulk(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	name := "p13 rows\"'`;--"
	q := func(s string) string { return quoteIdentifier(c.Driver, s) }
	ref := core.TableRef{Name: name}
	blob := "BLOB"
	if c.Driver == "postgres" {
		blob = "BYTEA"
	}
	execute(t, e, c, "CREATE TABLE "+q(name)+"(id INTEGER NOT NULL PRIMARY KEY, value VARCHAR(100) UNIQUE, raw "+blob+")")
	execute(t, e, c, "INSERT INTO "+q(name)+" VALUES(1,'one',"+binaryLiteral(c.Driver)+"),(2,'two',"+binaryLiteral(c.Driver)+")")
	request := core.RowsBulkRequest{Connection: c, Table: ref, Action: "update", Rows: bulkSnapshots(t, e, c, ref), Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "same"}}}}
	request = confirmBulk(t, e, request)
	result, err := e.RowsBulkExecute(ctx, request)
	if err == nil || result.Outcome != "rejected" || len(result.Rows) != 2 {
		t.Fatalf("constraint rollback: %#v %v", result, err)
	}
	if got := bulkSnapshots(t, e, c, ref); got[0].Expected.Values[1].Value != "one" || got[1].Expected.Values[1].Value != "two" {
		t.Fatal("partial commit", got)
	}
	request.Confirm = false
	request.ExpectedPlanSHA256 = ""
	request.Changes = []core.RowChange{{Column: "raw", Value: core.Cell{Kind: "binary", Value: "AAEC/w=="}}}
	request = confirmBulk(t, e, request)
	wrong := request
	wrong.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	_, err = e.RowsBulkExecute(ctx, wrong)
	requireMutationCode(t, err, "plan_conflict")
	wrong = request
	wrong.Rows = append([]core.BulkRow{}, request.Rows...)
	wrong.Rows[1].StabilitySHA256 = strings.Repeat("0", 64)
	_, err = e.RowsBulkExecute(ctx, wrong)
	requireMutationCode(t, err, "row_conflict")
	result, err = e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "completed" {
		t.Fatalf("update %#v %v", result, err)
	}
	dumpRowsBulk(t, "RowsBulkResult", result)
	for _, row := range bulkSnapshots(t, e, c, ref) {
		if row.Expected.Values[2].Value != "AAEC/w==" {
			t.Fatal(row)
		}
	}
	_, err = e.RowsBulkExecute(ctx, request)
	requireMutationCode(t, err, "row_conflict")
	// A conflict in the second row prevents changing the first.
	request = core.RowsBulkRequest{Connection: c, Table: ref, Action: "delete", Rows: bulkSnapshots(t, e, c, ref)}
	request = confirmBulk(t, e, request)
	execute(t, e, c, "UPDATE "+q(name)+" SET value='changed' WHERE id=2")
	result, err = e.RowsBulkExecute(ctx, request)
	if err == nil || result.Outcome != "rejected" || len(bulkSnapshots(t, e, c, ref)) != 2 {
		t.Fatal("stale delete was not atomic", result, err)
	}
	copyRequest := core.RowsBulkRequest{Connection: c, Table: ref, Action: "copy", Rows: bulkSnapshots(t, e, c, ref)}
	for i := range copyRequest.Rows {
		copyRequest.Rows[i].CopyValues = []core.RowChange{{Column: "id", Value: core.Cell{Kind: "integer", Value: strconv.Itoa(i + 3)}}, {Column: "value", Value: core.Cell{Kind: "text", Value: "copy" + strconv.Itoa(i)}}}
	}
	copyRequest = confirmBulk(t, e, copyRequest)
	result, err = e.RowsBulkExecute(ctx, copyRequest)
	if err != nil || result.Outcome != "completed" || len(bulkSnapshots(t, e, c, ref)) != 4 {
		t.Fatal("copy", result, err)
	}
	request = core.RowsBulkRequest{Connection: c, Table: ref, Action: "delete", Rows: bulkSnapshots(t, e, c, ref)}
	request = confirmBulk(t, e, request)
	result, err = e.RowsBulkExecute(ctx, request)
	if err != nil || result.Outcome != "completed" || len(bulkSnapshots(t, e, c, ref)) != 0 {
		t.Fatal("delete", result, err)
	}
	// Real file values, empty blob, bounds, stale snapshots, no text-column uploads.
	execute(t, e, c, "INSERT INTO "+q(name)+" VALUES(1,'file',"+binaryLiteral(c.Driver)+")")
	fr := core.FileValueRequest{Connection: c, Table: ref, Column: "raw", Expected: bulkSnapshots(t, e, c, ref)[0].Expected, Confirm: true}
	data := []byte{0, 1, 2, 255, 0, 128}
	upload, err := e.FileValueUpload(ctx, fr, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.FileValueUpload(ctx, fr, bytes.NewReader(nil))
	requireMutationCode(t, err, "row_conflict")
	fr.Expected = *upload.Row
	var got bytes.Buffer
	err = e.FileValueDownload(ctx, fr, func(media string, size int64) (io.Writer, error) {
		if media != "application/octet-stream" || size != int64(len(data)) {
			t.Fatal(media, size)
		}
		return &got, nil
	})
	if err != nil || !bytes.Equal(data, got.Bytes()) {
		t.Fatal(got.Bytes(), err)
	}
	_, err = e.FileValueUpload(ctx, fr, strings.NewReader(strings.Repeat("x", core.MaxFileValueBytes+1)))
	requireMutationCode(t, err, "file_too_large")
	bad := fr
	bad.Column = "value"
	_, err = e.FileValueUpload(ctx, bad, strings.NewReader("x"))
	requireMutationCode(t, err, "unsupported_file_column")
	fr.Preview = true
	err = e.FileValueDownload(ctx, fr, func(string, int64) (io.Writer, error) { t.Fatal("unsafe preview started"); return io.Discard, nil })
	requireMutationCode(t, err, "unsupported_preview")
	fr.Preview = false
	upload, err = e.FileValueUpload(ctx, fr, bytes.NewReader(nil))
	if err != nil || upload.Row.Values[2].Kind != "binary" || upload.Row.Values[2].Value != "" {
		t.Fatal(upload, err)
	}
	// FK target comes only from metadata; special-character search is a literal.
	execute(t, e, c, "CREATE TABLE p13_parent(id INTEGER NOT NULL PRIMARY KEY,label VARCHAR(40))")
	execute(t, e, c, "INSERT INTO p13_parent VALUES(1,'100% literal'),(2,'other'),(3,'100_ literal')")
	execute(t, e, c, "CREATE TABLE p13_child(id INTEGER NOT NULL PRIMARY KEY,parent INTEGER,FOREIGN KEY(parent) REFERENCES p13_parent(id))")
	lookup := core.ForeignKeyLookupRequest{Connection: c, Table: core.TableRef{Name: "p13_child"}, Column: "parent", DisplayColumn: "label", PageSize: 1}
	choices, err := e.ForeignKeyLookup(ctx, lookup)
	if err != nil || len(choices.Choices) != 1 || !choices.HasMore || choices.Choices[0].Value.Value != "1" {
		t.Fatal(choices, err)
	}
	dumpRowsBulk(t, "ForeignKeyLookupResult", choices)
	lookup.Offset = 1
	choices, err = e.ForeignKeyLookup(ctx, lookup)
	if err != nil || choices.Choices[0].Value.Value != "2" {
		t.Fatal(choices, err)
	}
	lookup.Offset = 0
	lookup.Search = "%"
	choices, err = e.ForeignKeyLookup(ctx, lookup)
	if err != nil || len(choices.Choices) != 1 || choices.HasMore || choices.Choices[0].Value.Value != "1" {
		t.Fatal(choices, err)
	}
	lookup.Search = "' OR 1=1 --"
	choices, err = e.ForeignKeyLookup(ctx, lookup)
	if err != nil || len(choices.Choices) != 0 {
		t.Fatal(choices, err)
	}
	lookup.Column = "id"
	_, err = e.ForeignKeyLookup(ctx, lookup)
	requireMutationCode(t, err, "unsupported_lookup")
	// Auto-key copy and ineligible tables are exercised for every real driver too.
	identity := "INTEGER PRIMARY KEY AUTOINCREMENT"
	if c.Driver == "mysql" {
		identity = "BIGINT PRIMARY KEY AUTO_INCREMENT"
	}
	if c.Driver == "postgres" {
		identity = "BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY"
	}
	execute(t, e, c, "CREATE TABLE p13_identity(id "+identity+",value VARCHAR(20))")
	execute(t, e, c, "INSERT INTO p13_identity(value) VALUES('original')")
	autoRef := core.TableRef{Name: "p13_identity"}
	autoRequest := core.RowsBulkRequest{Connection: c, Table: autoRef, Action: "copy", Rows: bulkSnapshots(t, e, c, autoRef)}
	autoRequest = confirmBulk(t, e, autoRequest)
	autoResult, err := e.RowsBulkExecute(ctx, autoRequest)
	if err != nil || autoResult.Outcome != "completed" || autoResult.Rows[0].Row.Values[0].Value != "2" || len(bulkSnapshots(t, e, c, autoRef)) != 2 {
		t.Fatal(autoResult, err)
	}
	execute(t, e, c, "CREATE TABLE p13_no_pk(value VARCHAR(20))")
	execute(t, e, c, "INSERT INTO p13_no_pk VALUES('original')")
	noPK := core.TableRef{Name: "p13_no_pk"}
	_, err = e.RowsBulkPlan(ctx, core.RowsBulkRequest{Connection: c, Table: noPK, Action: "delete", Rows: bulkSnapshots(t, e, c, noPK)})
	requireMutationCode(t, err, "mutation_not_supported")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = e.RowsBulkPlan(cancelled, core.RowsBulkRequest{Connection: c, Table: autoRef, Action: "delete", Rows: bulkSnapshots(t, e, c, autoRef)})
	if err == nil {
		t.Fatal("canceled plan succeeded")
	}

}
func TestRowsBulkPreview(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	raw := append(encoded.Bytes(), []byte("<script>bad()</script>")...)
	safe, err := safeFilePreview(raw)
	if err != nil || bytes.Contains(safe, []byte("script")) {
		t.Fatal(err)
	}
	if _, err = safeFilePreview([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)); err == nil {
		t.Fatal("SVG accepted")
	}
}
func TestRowsBulkValidation(t *testing.T) {
	e := testEngine(t)
	ctx := context.Background()
	for _, action := range []string{"update", "delete", "copy", "anything"} {
		_, err := e.RowsBulkPlan(ctx, core.RowsBulkRequest{Connection: sqliteConnection(), Table: core.TableRef{Name: "x"}, Action: action})
		requireMutationCode(t, err, "invalid_bulk")
	}
	execute(t, e, sqliteConnection(), "CREATE TABLE p13_nokey(value TEXT)")
	_, err := e.RowsBulkPlan(ctx, core.RowsBulkRequest{Connection: sqliteConnection(), Table: core.TableRef{Name: "p13_nokey"}, Action: "delete", Rows: []core.BulkRow{{Expected: core.RowSnapshot{Columns: []string{"value"}, Values: []core.Cell{{Kind: "text", Value: "x"}}}}}})
	requireMutationCode(t, err, "mutation_not_supported")
}
func TestLiveRowsBulk(t *testing.T) {
	for _, system := range []struct{ name, driver, env string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(system.name, func(t *testing.T) {
			raw := os.Getenv(system.env)
			if raw == "" {
				t.Skip("disposable fixture port required")
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
			checkRowsBulk(t, e, c)
		})
	}
}
