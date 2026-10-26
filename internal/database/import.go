// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"golang.org/x/text/encoding/charmap"
)

func invalidImport() error {
	return public("invalid_import", "Importdatei, Optionen oder Grenzen sind ungültig.")
}
func normalizeImport(ctx context.Context, r *core.ImportRequest, data []byte) (string, error) {
	if len(data) == 0 || len(data) > core.MaxImportBytes || r.MaxBytes < 0 || r.MaxBytes > core.MaxImportBytes || r.MaxRows < 0 || r.MaxRows > core.MaxImportRows || r.TimeoutMS < 0 || r.TimeoutMS > 300000 {
		return "", invalidImport()
	}
	if r.MaxBytes == 0 {
		r.MaxBytes = core.MaxImportBytes
	}
	if r.MaxRows == 0 {
		r.MaxRows = core.MaxImportRows
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 60000
	}
	if r.Format != "sql" && r.Format != "csv" {
		return "", invalidImport()
	}
	if r.Mode == "" {
		r.Mode = "stop"
	}
	if r.Mode != "stop" && !(r.Format == "csv" && r.Mode == "skip") && !(r.Format == "sql" && r.Mode == "continue") {
		return "", invalidImport()
	}
	switch r.Compression {
	case "", "none":
	case "gzip":
		z, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", invalidImport()
		}
		defer z.Close()
		data, err = io.ReadAll(io.LimitReader(importContextReader{ctx: ctx, reader: z}, int64(r.MaxBytes)+1))
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", invalidImport()
		}
	default:
		return "", invalidImport()
	}
	if len(data) > r.MaxBytes {
		return "", public("import_too_large", "Entpackte Datei überschreitet die Bytegrenze.")
	}
	charset := r.CSV.Charset
	if r.Format == "sql" {
		charset = "utf-8"
	}
	var decoded io.Reader
	switch charset {
	case "", "utf-8":
	case "iso-8859-1":
		decoded = charmap.ISO8859_1.NewDecoder().Reader(bytes.NewReader(data))
	case "windows-1252":
		decoded = charmap.Windows1252.NewDecoder().Reader(bytes.NewReader(data))
	default:
		return "", invalidImport()
	}
	if decoded != nil {
		var err error
		data, err = io.ReadAll(io.LimitReader(importContextReader{ctx: ctx, reader: decoded}, int64(r.MaxBytes)+1))
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", invalidImport()
		}
	}
	if len(data) > r.MaxBytes {
		return "", public("import_too_large", "Dekodierte Datei überschreitet die Bytegrenze.")
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", invalidImport()
	}
	// data is never modified after this point; share it instead of copying up to 64 MiB again.
	text := unsafe.String(unsafe.SliceData(data), len(data))
	return strings.TrimPrefix(text, "\ufeff"), nil
}
func emptyImportResult() core.ImportResult {
	return core.ImportResult{Outcome: "rejected", Statements: []core.SQLStatementResult{}, Errors: []core.ImportIssue{}, Warnings: []string{}}
}
func addImportIssue(list *[]core.ImportIssue, truncated *bool, row, col int, code string) {
	if len(*list) < 100 {
		*list = append(*list, core.ImportIssue{Row: row, Column: col, Code: code})
	} else {
		*truncated = true
	}
}
func importColumns(d core.TableDescription, r core.ImportRequest) ([]core.TableColumn, string, error) {
	if d.Kind != "table" || d.Transactional != "yes" || len(r.CSV.Columns) == 0 || len(r.CSV.Columns) > 1024 {
		return nil, "", public("import_not_supported", "Nur vorhandene transaktionale Basistabellen und explizite Spaltenzuordnung sind freigegeben.")
	}
	cols := []core.TableColumn{}
	names := []string{}
	markers := []string{}
	seen := map[string]bool{}
	for _, name := range r.CSV.Columns {
		found := false
		for _, col := range d.Columns {
			if col.Name == name && (col.Generated == "none" || col.Generated == "identity") && !seen[name] {
				cols = append(cols, col)
				names = append(names, quoteIdentifier(r.Connection.Driver, col.Name))
				seen[name] = true
				found = true
				break
			}
		}
		if !found {
			return nil, "", invalidImport()
		}
		m := "?"
		if r.Connection.Driver == "postgres" {
			m = "$" + strconv.Itoa(len(markers)+1)
		}
		markers = append(markers, m)
	}
	prefix := "INSERT INTO "
	if r.Connection.Driver == "sqlite" {
		prefix = "INSERT OR ABORT INTO "
	}
	return cols, prefix + qualifiedTable(r.Connection.Driver, d.Table) + " (" + strings.Join(names, ",") + ") VALUES (" + strings.Join(markers, ",") + ")", nil
}
func scanImportCSV(ctx context.Context, text string, r core.ImportRequest, cols []core.TableColumn, visit func(int, []core.Cell, bool) error) (core.ImportPlan, error) {
	p := core.ImportPlan{Preview: [][]core.Cell{}, Errors: []core.ImportIssue{}}
	reader, err := newCSVImportReader(text, r.CSV)
	if err != nil {
		return p, err
	}
	if r.CSV.Header {
		header, err := reader.read(ctx)
		if err != nil || len(header) != len(cols) {
			return p, invalidImport()
		}
	}
	previewBytes := 0
	for {
		if ctx.Err() != nil {
			return p, ctx.Err()
		}
		record, err := reader.read(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return p, err
		}
		p.Rows++
		if p.Rows > r.MaxRows {
			return p, public("import_too_large", "CSV überschreitet die Zeilengrenze.")
		}
		cells := []core.Cell{}
		valid := true
		if len(record) != len(cols) {
			addImportIssue(&p.Errors, &p.ErrorsTruncated, p.Rows, 0, "column_count")
			valid = false
		} else {
			for i, v := range record {
				cell, err := csvImportCell(v, cols[i], r.CSV, r.Connection.Driver)
				if err != nil {
					addImportIssue(&p.Errors, &p.ErrorsTruncated, p.Rows, i+1, err.Error())
					valid = false
				}
				cells = append(cells, cell)
			}
		}
		if valid {
			p.ValidRows++
			if len(p.Preview) < 20 {
				encoded, _ := json.Marshal(cells)
				if previewBytes+len(encoded) <= 256<<10 {
					p.Preview = append(p.Preview, cells)
					previewBytes += len(encoded)
				}
			}
		}
		if visit != nil {
			if err := visit(p.Rows, cells, valid); err != nil {
				return p, err
			}
		}
	}
	if p.Rows == 0 {
		return p, invalidImport()
	}
	return p, nil
}
func makeImportPlan(ctx context.Context, q metadataQuerier, r core.ImportRequest, text string) (core.ImportPlan, []core.TableColumn, error) {
	if !validTableRef(r.Table) {
		return core.ImportPlan{}, nil, invalidImport()
	}
	d, err := describe(ctx, q, r.Connection, r.Table)
	if err != nil {
		return core.ImportPlan{}, nil, safe(ctx, err)
	}
	if err = checkImportPostgres(ctx, q, r.Connection, d); err != nil {
		return core.ImportPlan{}, nil, err
	}
	cols, statement, err := importColumns(d, r)
	if err != nil {
		return core.ImportPlan{}, nil, err
	}
	p, err := scanImportCSV(ctx, text, r, cols, nil)
	if err != nil {
		return p, nil, err
	}
	// Bind file contents, all options, the destination and observed schema. Passwords
	// are excluded; the authenticated database identity remains part of the digest.
	bound := r
	bound.Confirm = nil
	bound.ExpectedPlanSHA256 = ""
	bound.Connection.Password = ""
	encoded, _ := json.Marshal(struct {
		Request     core.ImportRequest
		Description core.TableDescription
		File        string
	}{bound, d, importHash([]byte(text))})
	p.Plan, err = dbplan.NewPlan(r.Connection.Driver, []string{statement, "-- import-bindings-sha256: " + importHash(encoded)}, []string{"Nur direkte Zeilen; Trigger, Sequenzen und externe Effekte sind nicht allgemein rücknehmbar. Typprüfung ist konservativ; Constraints werden erst beim Schreiben geprüft."}, []string{d.Table.Schema + "." + d.Table.Name})
	return p, cols, err
}
func importHash(data []byte) string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }
func (e *Engine) PlanImport(ctx context.Context, r core.ImportRequest, data []byte) (core.ImportPlan, error) {
	if r.Format != "csv" || r.Confirm != nil || r.ExpectedPlanSHA256 != "" {
		return core.ImportPlan{}, invalidImport()
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > 300000 {
		return core.ImportPlan{}, invalidImport()
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 60000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	text, err := normalizeImport(ctx, &r, data)
	if err != nil {
		return core.ImportPlan{}, err
	}
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.ImportPlan{}, err
	}
	defer closeDB()
	p, _, err := makeImportPlan(ctx, db, r, text)
	if err == nil && core.IsPlanPreflight(ctx) && e.plans.isConsumed("import:"+p.SHA256) {
		// Execute pre-check: refuse a consumed plan synchronously, without releasing it.
		return core.ImportPlan{}, public("plan_mismatch", "Importplan ist verbraucht (ein Ausführungsversuch hat begonnen). Neue Vorschau erforderlich.")
	}
	if err == nil && !core.IsPlanPreflight(ctx) {
		e.plans.arm("import:" + p.SHA256)
	}
	return p, err
}
func (e *Engine) ExecuteImport(ctx context.Context, r core.ImportRequest, data []byte, progress func(core.ImportProgress)) (core.ImportResult, error) {
	out := emptyImportResult()
	if r.Confirm == nil || r.Confirm.Action != "import_"+r.Format {
		return out, public("confirmation_mismatch", "Ausdrückliche Schreibzustimmung ist erforderlich.")
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > 300000 {
		return out, invalidImport()
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 60000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	text, err := normalizeImport(ctx, &r, data)
	if err != nil {
		return out, err
	}
	if r.Format == "sql" {
		return e.importSQL(ctx, r, text, progress)
	}
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return out, err
	}
	defer closeDB()
	conn, err := db.Conn(ctx)
	if err != nil {
		return out, safe(ctx, err)
	}
	defer conn.Close()
	session := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = session.begin(ctx); err != nil {
		return out, safe(ctx, err)
	}
	defer session.rollback()
	// Acquire the relation/metadata lock before recomputing the confirmed plan.
	d, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return out, safe(ctx, err)
	}
	if _, _, err = importColumns(d, r); err != nil {
		return out, err
	}
	if err = checkImportPostgres(ctx, session, r.Connection, d); err != nil {
		return out, err
	}
	lockRows, err := session.QueryContext(ctx, "SELECT 1 FROM "+qualifiedTable(r.Connection.Driver, d.Table)+" WHERE 1=0")
	if err != nil {
		return out, safe(ctx, err)
	}
	lockRows.Close()

	if r.Connection.Driver == "mysql" {
		if _, err = session.exec(ctx, "SET SESSION sql_mode=CONCAT(@@sql_mode,',STRICT_ALL_TABLES')"); err != nil {
			return out, safe(ctx, err)
		}
	}
	p, cols, err := makeImportPlan(ctx, session, r, text)
	if err != nil {
		return out, err
	}
	if err = dbplan.Check(p.Plan, r.ExpectedPlanSHA256, *r.Confirm, "import_csv", p.Affected[0], false); err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) || errors.Is(err, dbplan.ErrInvalidDigest) {
			return out, public("plan_mismatch", "Importplan hat sich geändert.")
		}
		return out, public("confirmation_mismatch", "Bestätigung passt nicht zum Importplan.")
	}
	out.Warnings = p.Warnings
	if progress != nil {
		progress(core.ImportProgress{Total: p.Rows})
	}
	if r.Mode == "stop" && p.ValidRows != p.Rows {
		out.Errors = p.Errors
		out.ErrorsTruncated = p.ErrorsTruncated
		out.Reason = "validation_failed"
		return out, nil
	}
	// Single use: consumed before the first write, also when the outcome stays unknown.
	if !e.plans.consume("import:" + p.SHA256) {
		return out, public("plan_mismatch", "Importplan ist verbraucht (ein Ausführungsversuch hat begonnen; auch ein fehlgeschlagener verbraucht ihn). Neue Vorschau erforderlich.")
	}
	// Recheck metadata while holding the write lock. PostgreSQL holds its relation
	// lock from the first INSERT; savepoints preserve the transaction after conflicts.
	_, err = scanImportCSV(ctx, text, r, cols, func(row int, cells []core.Cell, valid bool) error {
		out.Processed = row
		defer func() {
			if progress != nil {
				progress(core.ImportProgress{Processed: row, Total: p.Rows})
			}
		}()
		if !valid {
			out.Skipped++
			return nil
		}
		args := make([]any, len(cells))
		for i, cell := range cells {
			v, err := mutationValue(cell)
			if err != nil {
				return err
			}
			args[i] = v
		}
		if _, err := session.exec(ctx, "SAVEPOINT dbm_import_row"); err != nil {
			return err
		}
		applied, err := session.exec(ctx, p.Statements[0], args...)
		if err == nil {
			n, countErr := applied.RowsAffected()
			if countErr != nil {
				return countErr
			}
			if n != 1 {
				err = public("insert_not_applied", "Datenbank hat nicht genau eine direkte Zeile eingefügt.")
			}
		}
		if err == nil && r.Connection.Driver == "mysql" {
			var count int
			warningRows, warnErr := session.QueryContext(ctx, "SHOW WARNINGS LIMIT 1")
			if warnErr != nil {
				return warnErr
			}
			if warningRows.Next() {
				count++
			}
			warnErr = warningRows.Err()
			warningRows.Close()
			if warnErr != nil {
				return warnErr
			}
			if count > 0 {
				err = public("lossy_conversion", "Serverwarnung verhindert einen möglicherweise verlustbehafteten Import.")
			}
		}
		if err != nil {
			if _, rb := session.exec(ctx, "ROLLBACK TO SAVEPOINT dbm_import_row"); rb != nil {
				return rb
			}
			if _, release := session.exec(ctx, "RELEASE SAVEPOINT dbm_import_row"); release != nil {
				return release
			}
			addImportIssue(&out.Errors, &out.ErrorsTruncated, row, 0, sqlBatchErrorCode(ctx, insertError(ctx, err)))
			if r.Mode == "stop" {
				return err
			}
			out.Skipped++
			return nil
		}
		if _, err = session.exec(ctx, "RELEASE SAVEPOINT dbm_import_row"); err != nil {
			return err
		}
		out.Inserted++
		return nil
	})
	for _, issue := range p.Errors {
		addImportIssue(&out.Errors, &out.ErrorsTruncated, issue.Row, issue.Column, issue.Code)
	}
	out.ErrorsTruncated = out.ErrorsTruncated || p.ErrorsTruncated
	if err != nil {
		// SQLite uses a manual BEGIN; verify rollback instead of assuming cancellation
		// proves that every already attempted write has been undone.
		rb := rollbackImport(session)
		out.Reason = sqlBatchErrorCode(ctx, err)
		if rb != nil {
			out.Outcome = "outcome_unknown"
		} else {
			out.Inserted = 0
		}
		return out, nil
	}
	if err = session.commit(ctx); err != nil {
		out.Outcome = "outcome_unknown"
		out.Reason = "commit_unknown"
		return out, nil
	}
	out.Outcome = "completed"
	return out, nil
}
func rollbackImport(s *mutationSession) error {
	if s.tx != nil {
		// ErrTxDone can race the automatic rollback after cancellation. Without
		// its driver acknowledgement we keep the outcome conservative.
		// After cancellation database/sql has already rolled the transaction back and
		// reports ErrTxDone (pgx: "context already done", the connection is closed server-side); no COMMIT was sent, so nothing can be persistent.
		if err := s.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context already done") {
			return err
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := s.conn.ExecContext(ctx, "ROLLBACK")
	return err
}

// importContextReader makes decompression responsive to cancellation without
// adding an unbounded allocation or background goroutine.
type importContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r importContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func checkImportPostgres(ctx context.Context, q metadataQuerier, c core.Connection, d core.TableDescription) error {
	if c.Driver != "postgres" {
		return nil
	}
	var unsuitable bool
	err := q.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR c.relhasrules OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&unsuitable)
	if err != nil {
		return safe(ctx, err)
	}
	if unsuitable {
		return public("import_not_supported", "Regeln, Partitionierung und Vererbung sind nicht freigegeben.")
	}
	return nil
}
