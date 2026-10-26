// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"compress/gzip"
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

func importRequest(c core.Connection) core.ImportRequest {
	return core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "p12_rows"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
}
func importConfirm(r core.ImportRequest, p core.ImportPlan) core.ImportRequest {
	r.Confirm = &dbplan.Confirmation{Action: "import_" + r.Format}
	r.ExpectedPlanSHA256 = p.SHA256
	return r
}
func dumpImport(t *testing.T, name string, v any) {
	t.Helper()
	if dir := os.Getenv("DBM_IMPORT_DUMP"); dir != "" {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name+".json"), append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func checkImportRoundTrip(t *testing.T, e *Engine, c core.Connection) {
	t.Helper()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE p12_rows (id INTEGER PRIMARY KEY, value VARCHAR(100))`)
	execute(t, e, c, `CREATE TABLE p12_neighbor (id INTEGER PRIMARY KEY)`)
	r := importRequest(c)
	r.CSV.Header = true
	r.CSV.Null = new(string)
	*r.CSV.Null = "\\N"
	file := []byte("id,value\r\n1,\"Grüße, \"\"Welt\"\"\nzweite Zeile\"\r\n2,\\N\r\n3,\"\"\r\n")
	p, err := e.PlanImport(ctx, r, file)
	if err != nil || p.Rows != 3 || p.ValidRows != 3 || len(p.Preview) != 3 {
		t.Fatalf("plan %#v %v", p, err)
	}
	dumpImport(t, "ImportPlan.real-"+strings.ReplaceAll(t.Name(), "/", "_"), p)
	confirmed := importConfirm(r, p)
	got, err := e.ExecuteImport(ctx, confirmed, file, nil)
	if err != nil || got.Outcome != "completed" || got.Inserted != 3 {
		t.Fatalf("import %#v %v", got, err)
	}
	dumpImport(t, "ImportResult.real-"+strings.ReplaceAll(t.Name(), "/", "_"), got)
	rows := execute(t, e, c, "SELECT value FROM p12_rows ORDER BY id").Results[0].Rows
	if len(rows) != 3 || rows[0][0].Value != "Grüße, \"Welt\"\nzweite Zeile" || rows[1][0].Kind != "null" || rows[2][0].Value != "" {
		t.Fatalf("readback %#v", rows)
	}
	// All bound inputs, including contents and schema, invalidate the confirmed plan.
	confirmed.CSV.Header = false
	if _, err = e.ExecuteImport(ctx, confirmed, []byte("4,changed"), nil); err == nil {
		t.Fatal("changed content accepted")
	}
	execute(t, e, c, "ALTER TABLE p12_rows ADD COLUMN extra INTEGER")
	if _, err = e.ExecuteImport(ctx, importConfirm(r, p), file, nil); err == nil {
		t.Fatal("changed schema accepted")
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_neighbor").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("neighbor changed")
	}
}
func TestSQLiteImportRoundTrip(t *testing.T) {
	checkImportRoundTrip(t, testEngine(t), sqliteConnection())
}
func TestSQLiteImportStrategies(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE p12_rows (id INTEGER PRIMARY KEY ON CONFLICT REPLACE, value TEXT CHECK(value<>'bad'))`)
	execute(t, e, c, `INSERT INTO p12_rows VALUES (1,'original')`)
	for _, mode := range []string{"stop", "skip"} {
		r := importRequest(c)
		r.Mode = mode
		file := []byte("2,new\n1,replacement\n3,bad\n4,last")
		p, err := e.PlanImport(ctx, r, file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := e.ExecuteImport(ctx, importConfirm(r, p), file, nil)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "stop" && (got.Outcome != "rejected" || got.Inserted != 0) {
			t.Fatalf("stop %#v", got)
		}
		if mode == "skip" && (got.Outcome != "completed" || got.Inserted != 2 || got.Skipped != 2 || len(got.Errors) != 2) {
			t.Fatalf("skip %#v", got)
		}
	}
	if v := execute(t, e, c, "SELECT value FROM p12_rows WHERE id=1").Results[0].Rows[0][0].Value; v != "original" {
		t.Fatal("REPLACE deleted existing row", v)
	}
	r := importRequest(c)
	file := []byte("5,valid\nnot-an-int,x\n6,valid")
	p, err := e.PlanImport(ctx, r, file)
	if err != nil || p.ValidRows != 2 || len(p.Errors) != 1 {
		t.Fatalf("validation %#v %v", p, err)
	}
	got, err := e.ExecuteImport(ctx, importConfirm(r, p), file, nil)
	if err != nil || got.Outcome != "rejected" || got.Processed != 0 {
		t.Fatalf("validation writes %#v %v", got, err)
	}
	r.Mode = "skip"
	p, err = e.PlanImport(ctx, r, file)
	if err != nil {
		t.Fatal(err)
	}
	got, err = e.ExecuteImport(ctx, importConfirm(r, p), file, nil)
	if err != nil || got.Inserted != 2 || got.Skipped != 1 {
		t.Fatalf("skip validation %#v %v", got, err)
	}
	dumpImport(t, "ImportResult.real-sqlite-skip", got)
}
func TestSQLiteImportSQL(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	r := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	file := []byte("CREATE TABLE p12_sql(id INTEGER PRIMARY KEY); INSERT INTO p12_sql VALUES(1); INSERT INTO missing_secret VALUES('secret'); INSERT INTO p12_sql VALUES(2)")
	got, err := e.ExecuteImport(ctx, r, file, nil)
	if err != nil || got.Outcome != "outcome_unknown" || len(got.Statements) != 4 || got.Statements[1].Outcome != "completed" || got.Statements[2].Outcome != "outcome_unknown" || got.Statements[3].Outcome != "not_executed" {
		t.Fatalf("sql %#v %v", got, err)
	}
	dumpImport(t, "ImportResult.real-sqlite-sql", got)
	encoded, _ := json.Marshal(got)
	if bytes.Contains(encoded, []byte("secret")) {
		t.Fatal("SQL leaked")
	}
	r.Mode = "continue"
	got, err = e.ExecuteImport(ctx, r, []byte("INSERT INTO missing VALUES(1); INSERT INTO p12_sql VALUES(2)"), nil)
	if err != nil || got.Statements[1].Outcome != "completed" {
		t.Fatalf("continue %#v %v", got, err)
	}
	for _, file := range []string{"INSERT INTO p12_sql VALUES(3); LOAD DATA LOCAL INFILE 'secret' INTO TABLE p12_sql", "INSERT INTO p12_sql VALUES(3); BEGIN", "INSERT INTO p12_sql VALUES(3); SET autocommit=0", "INSERT INTO p12_sql VALUES(3); ATTACH 'secret' AS x", "-- DB Manager SQL export 1;\nINSERT INTO p12_sql VALUES(3);"} {
		if _, err = e.ExecuteImport(ctx, r, []byte(file), nil); err == nil {
			t.Fatal("unsafe/incomplete SQL accepted", file)
		}
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_sql WHERE id=3").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("prevalidation wrote")
	}
	// Explicit transactions and the same-dialect exporter work with the shared lexer.
	var dump bytes.Buffer
	er := core.ExportRequest{Connection: c, Format: "sql", Tables: []core.TableRef{{Name: "p12_sql"}}, Transaction: true}
	if err = e.Export(ctx, er, &dump); err != nil {
		t.Fatal(err)
	}
	execute(t, e, c, "DROP TABLE p12_sql")
	got, err = e.ExecuteImport(ctx, r, dump.Bytes(), nil)
	if err != nil || got.Outcome != "completed" {
		t.Fatalf("export restore %#v %v\n%s", got, err, dump.String())
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_sql").Results[0].Rows[0][0].Value; n != "2" {
		t.Fatal("restore mismatch", n)
	}
}
func TestSQLiteImportCancellationAndLimits(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT)")
	r := importRequest(c)
	ctx := context.Background()
	file := []byte("1,one\n2,two")
	p, err := e.PlanImport(ctx, r, file)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	got, err := e.ExecuteImport(canceled, importConfirm(r, p), file, func(p core.ImportProgress) {
		if p.Processed == 1 {
			cancel()
		}
	})
	if err != nil || got.Outcome != "rejected" || got.Inserted != 0 {
		t.Fatalf("cancel %#v %v", got, err)
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_rows").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("cancel persisted", n)
	}
	r.MaxRows = 1
	if _, err = e.PlanImport(ctx, r, file); err == nil {
		t.Fatal("row limit accepted")
	}
	r.MaxRows = 0
	r.MaxBytes = 4
	if _, err = e.PlanImport(ctx, r, file); err == nil {
		t.Fatal("byte limit accepted")
	}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	_, _ = z.Write(bytes.Repeat([]byte("a"), 1<<20))
	_ = z.Close()
	r.Compression = "gzip"
	r.MaxBytes = 1024
	if _, err = e.PlanImport(ctx, r, compressed.Bytes()); err == nil {
		t.Fatal("gzip bomb accepted")
	}
	r.MaxBytes = 0
	r.Compression = "zip"
	if _, err = e.PlanImport(ctx, r, file); err == nil {
		t.Fatal("zip accepted")
	}
	r.Compression = "none"
	for _, file := range []string{"1,\"unterminated", "1,\"x\"junk", "1,x\x00"} {
		if _, err = e.PlanImport(ctx, r, []byte(file)); err == nil {
			t.Fatal("malformed accepted")
		}
	}
}
func TestSQLiteImportCSVOptions(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT)")
	r := importRequest(c)
	r.CSV.Separator = ";"
	r.CSV.Quote = "'"
	r.CSV.Escape = "backslash"
	r.CSV.Charset = "iso-8859-1"
	file := []byte("1;'Gr\xfc\xdfe; \\'Text\\''\r\n2;plain")
	p, err := e.PlanImport(context.Background(), r, file)
	if err != nil || p.Preview[0][1].Value != "Grüße; 'Text'" {
		t.Fatalf("custom options %#v %v", p, err)
	}
	got, err := e.ExecuteImport(context.Background(), importConfirm(r, p), file, nil)
	if err != nil || got.Inserted != 2 {
		t.Fatalf("options %#v %v", got, err)
	}
}
func TestSQLiteImportSQLCancellation(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE p12_cancel(id INTEGER)")
	r := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}, TimeoutMS: 30}
	got, err := e.ExecuteImport(ctx, r, []byte("WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT sum(x) FROM n; INSERT INTO p12_cancel VALUES(1)"), nil)
	if err != nil || got.Outcome != "outcome_unknown" || got.Statements[0].ErrorCode != "query_canceled" || got.Statements[1].Outcome != "not_executed" {
		t.Fatalf("deadline %#v %v", got, err)
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_cancel").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("canceled remainder executed")
	}
}
func TestSQLiteImportErrorsAndBounds(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT)")
	r := importRequest(c)
	file := []byte(strings.Repeat("wrong,x\n", 150))
	p, err := e.PlanImport(ctx, r, file)
	if err != nil || len(p.Errors) != 100 || !p.ErrorsTruncated || p.ValidRows != 0 {
		t.Fatalf("errors %#v %v", p, err)
	}
	for _, change := range []func(*core.ImportRequest){func(r *core.ImportRequest) { r.CSV.Columns = []string{"id", "id"} }, func(r *core.ImportRequest) { r.CSV.Columns = []string{"id", "value);DROP TABLE p12_rows;--"} }, func(r *core.ImportRequest) { r.CSV.Separator = "::" }, func(r *core.ImportRequest) { r.CSV.Separator = "\"" }, func(r *core.ImportRequest) { r.CSV.Charset = "unknown" }, func(r *core.ImportRequest) { r.MaxRows = core.MaxImportRows + 1 }, func(r *core.ImportRequest) { r.TimeoutMS = 300001 }} {
		bad := r
		change(&bad)
		if _, err = e.PlanImport(ctx, bad, []byte("1,ok")); err == nil {
			t.Fatal("bad options accepted")
		}
	}
	// A file can exceed the old SQL-batch limit without relaxing that API.
	sqlFile := []byte(strings.Repeat("INSERT INTO p12_rows VALUES(1,'x');\n", 101))
	sqlRequest := core.ImportRequest{Connection: c, Format: "sql", Mode: "continue", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	got, err := e.ExecuteImport(ctx, sqlRequest, sqlFile, nil)
	if err != nil || len(got.Statements) != 101 {
		t.Fatalf("larger batch %#v %v", got, err)
	}
}
func TestSQLiteImportCommitUnknownAndIgnoredInsert(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, "CREATE TABLE p12_parent(id INTEGER PRIMARY KEY)")
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value INTEGER REFERENCES p12_parent(id) DEFERRABLE INITIALLY DEFERRED)")
	r := importRequest(c)
	file := []byte("1,99")
	p, err := e.PlanImport(ctx, r, file)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.ExecuteImport(ctx, importConfirm(r, p), file, nil)
	if err != nil || got.Outcome != "outcome_unknown" || got.Reason != "commit_unknown" {
		t.Fatalf("commit %#v %v", got, err)
	}
	dumpImport(t, "ImportResult.real-sqlite-commit-unknown", got)
	if n := execute(t, e, c, "SELECT count(*) FROM p12_rows").Results[0].Rows[0][0].Value; n != "0" {
		t.Fatal("deferred failure persisted", n)
	}
	execute(t, e, c, "DROP TABLE p12_rows")
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT)")
	execute(t, e, c, "CREATE TRIGGER p12_ignore BEFORE INSERT ON p12_rows BEGIN SELECT RAISE(IGNORE); END")
	file = []byte("1,ignored")
	p, err = e.PlanImport(ctx, r, file)
	if err != nil {
		t.Fatal(err)
	}
	got, err = e.ExecuteImport(ctx, importConfirm(r, p), file, nil)
	if err != nil || got.Outcome != "rejected" || got.Inserted != 0 {
		t.Fatalf("ignored %#v %v", got, err)
	}
}
func TestSQLiteImportMaxFile(t *testing.T) {
	if os.Getenv("DBM_IMPORT_STRESS") != "1" {
		t.Skip("set DBM_IMPORT_STRESS=1 for 64 MiB engine check")
	}
	e := testEngine(t)
	c := sqliteConnection()
	var file bytes.Buffer
	file.Grow(core.MaxImportBytes)
	file.WriteString("CREATE TABLE p12_max(id INTEGER);\n")
	padding := strings.Repeat("x", (core.MaxImportBytes-16384)/128)
	for i := range 128 {
		file.WriteString("-- ")
		file.WriteString(padding)
		file.WriteString("\nINSERT INTO p12_max VALUES(")
		file.WriteString(strconv.Itoa(i))
		file.WriteString(");\n")
	}
	file.WriteString("-- ")
	for file.Len() < core.MaxImportBytes {
		file.WriteByte('x')
	}
	if file.Len() != core.MaxImportBytes {
		t.Fatal("fixture size", file.Len())
	}
	r := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}, TimeoutMS: 300000}
	result, err := e.ExecuteImport(context.Background(), r, file.Bytes(), nil)
	if err != nil || result.Outcome != "completed" || len(result.Statements) != 129 {
		t.Fatalf("max file %#v %v", result, err)
	}
	if count := execute(t, e, c, "SELECT count(*) FROM p12_max").Results[0].Rows[0][0].Value; count != "128" {
		t.Fatal("max file count", count)
	}
}
func TestSQLiteImportTypePreview(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	execute(t, e, c, "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY, amount DECIMAL(5,2), flag BOOLEAN, value VARCHAR(3), ratio REAL)")
	r := importRequest(c)
	r.CSV.Columns = []string{"id", "amount", "flag", "value", "ratio"}
	file := []byte("1,123.45,true,abc,1.5\n2,1234.56,false,abc,1.5\n3,1.234,true,abc,1.5\n4,1.00,yes,abc,1.5\n5,1.00,true,abcd,1.5\n6,1.00,true,abc,NaN")
	p, err := e.PlanImport(context.Background(), r, file)
	if err != nil || p.ValidRows != 1 || len(p.Errors) != 5 {
		t.Fatalf("types %#v %v", p, err)
	}
	r.Mode = "skip"
	p, err = e.PlanImport(context.Background(), r, file)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.ExecuteImport(context.Background(), importConfirm(r, p), file, nil)
	if err != nil || out.Outcome != "completed" || out.Inserted != 1 || out.Skipped != 5 {
		t.Fatalf("type execution %#v %v", out, err)
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_rows WHERE id=1 AND flag=1").Results[0].Rows[0][0].Value; n != "1" {
		t.Fatal("typed readback", n)
	}
}
func TestSQLiteImportTriggerDelimiterAndWindows1252(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	r := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	sqlFile := []byte("CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT); CREATE TABLE p12_log(id INTEGER);\nDELIMITER //\nCREATE TRIGGER p12_trigger AFTER INSERT ON p12_rows BEGIN INSERT INTO p12_log VALUES(new.id); INSERT INTO p12_log VALUES(new.id+100); END//\nDELIMITER ;\nINSERT INTO p12_rows VALUES(1,'trigger');")
	out, err := e.ExecuteImport(ctx, r, sqlFile, nil)
	if err != nil || out.Outcome != "completed" {
		t.Fatalf("trigger %#v %v", out, err)
	}
	if n := execute(t, e, c, "SELECT count(*) FROM p12_log").Results[0].Rows[0][0].Value; n != "2" {
		t.Fatal("trigger readback", n)
	}
	csv := importRequest(c)
	csv.CSV.Charset = "windows-1252"
	file := []byte("2,\x80")
	plan, err := e.PlanImport(ctx, csv, file)
	if err != nil || plan.Preview[0][1].Value != "€" {
		t.Fatalf("windows preview %#v %v", plan, err)
	}
	out, err = e.ExecuteImport(ctx, importConfirm(csv, plan), file, nil)
	if err != nil || out.Inserted != 1 {
		t.Fatalf("windows insert %#v %v", out, err)
	}
	if v := execute(t, e, c, "SELECT value FROM p12_rows WHERE id=2").Results[0].Rows[0][0].Value; v != "€" {
		t.Fatal("charset readback", v)
	}
}
func TestSQLiteImportQuotedTargetsAndBoundValues(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	ctx := context.Background()
	execute(t, e, c, `CREATE TABLE "p12 odd "" ; DROP --" ("i""d" INTEGER PRIMARY KEY,"value; DROP p12_neighbor --" TEXT)`)
	execute(t, e, c, "CREATE TABLE p12_neighbor(id INTEGER)")
	r := importRequest(c)
	r.Table.Name = `p12 odd " ; DROP --`
	r.CSV.Columns = []string{`i"d`, `value; DROP p12_neighbor --`}
	file := []byte("1,'); DROP TABLE p12_neighbor;--")
	plan, err := e.PlanImport(ctx, r, file)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.ExecuteImport(ctx, importConfirm(r, plan), file, nil)
	if err != nil || out.Outcome != "completed" || out.Inserted != 1 {
		t.Fatalf("quoted target %#v %v", out, err)
	}
	rows := execute(t, e, c, `SELECT "value; DROP p12_neighbor --" FROM "p12 odd "" ; DROP --"`).Results[0].Rows
	if len(rows) != 1 || rows[0][0].Value != "'); DROP TABLE p12_neighbor;--" {
		t.Fatalf("bound value %#v", rows)
	}
	if count := execute(t, e, c, "SELECT count(*) FROM p12_neighbor").Results[0].Rows[0][0].Value; count != "0" {
		t.Fatal("neighbor changed", count)
	}
}
