// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/sqlsplit"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite "github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func normalizeSQLBatch(r *core.SQLBatchRequest) ([]string, error) {
	if !r.Confirm || r.MaxRows < 0 || r.MaxRows > 10000 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 || (r.Mode != "" && r.Mode != "stop" && r.Mode != "continue") {
		return nil, public("invalid_sqlbatch", "Bestätigung, Modus oder Grenzen sind ungültig.")
	}
	if r.MaxRows == 0 {
		r.MaxRows = 1000
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	if r.Mode == "" {
		r.Mode = "stop"
	}
	statements, err := sqlsplit.Split(r.SQL, r.Delimiter, r.Connection.Driver)
	if err != nil {
		return nil, public("invalid_sqlbatch", "SQL-Stapel kann nicht getrennt werden.")
	}
	return statements, nil
}

func (e *Engine) SQLBatch(ctx context.Context, r core.SQLBatchRequest) (core.SQLBatchResult, error) {
	return e.sqlBatch(ctx, r, false)
}
func (e *Engine) SQLExplain(ctx context.Context, r core.SQLBatchRequest) (core.SQLBatchResult, error) {
	return e.sqlBatch(ctx, r, true)
}

func (e *Engine) sqlBatch(ctx context.Context, r core.SQLBatchRequest, explain bool) (core.SQLBatchResult, error) {
	start := time.Now()
	out := core.SQLBatchResult{Statements: []core.SQLStatementResult{}}
	statements, err := normalizeSQLBatch(&r)
	if err != nil {
		return out, err
	}
	if explain {
		if len(statements) != 1 {
			return out, public("invalid_sqlbatch", "EXPLAIN verlangt genau eine Anweisung.")
		}
		switch sqlsplit.Keyword(statements[0], r.Connection.Driver) {
		case "SELECT", "WITH", "VALUES", "INSERT", "UPDATE", "DELETE":
		default:
			return out, public("sql_not_supported", "Diese Anweisung ist für EXPLAIN nicht freigegeben.")
		}
		prefix := "EXPLAIN "
		if r.Connection.Driver == "sqlite" {
			prefix = "EXPLAIN QUERY PLAN "
		}
		statements[0] = prefix + statements[0]
	}
	// Explicit transaction/session controls cannot form a transaction across items.
	// This is an ergonomic restriction, not a sandbox for trusted user SQL.
	for _, s := range statements {
		switch sqlsplit.Keyword(s, r.Connection.Driver) {
		case "", "BEGIN", "START", "COMMIT", "END", "ROLLBACK", "SAVEPOINT", "RELEASE", "SET":
			return out, public("sql_not_supported", "Transaktions- und Sitzungskontrolle ist im Stapel nicht freigegeben.")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
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
	stopped := false
	for i, s := range statements {
		item := core.SQLStatementResult{Index: i + 1, Outcome: "not_executed", Results: []core.ResultSet{}, Warnings: []core.SQLWarning{}, DurationMS: 0}
		if !stopped && ctx.Err() == nil {
			before := time.Now()
			item.Outcome = "completed"
			item.Results, item.RowsAffected, err = sqlBatchStatement(ctx, conn, r.Connection.Driver, s, r.MaxRows)
			item.DurationMS = time.Since(before).Milliseconds()
			if err != nil {
				item.Outcome = "outcome_unknown"
				item.Results = []core.ResultSet{}
				item.RowsAffected = nil
				item.ErrorCode = sqlBatchErrorCode(ctx, err)
				item.ServerCode = sqlServerCode(err)
				stopped = r.Mode == "stop" || ctx.Err() != nil || item.ErrorCode == "result_too_large"
			} else if r.Connection.Driver == "mysql" {
				item.Warnings = sqlBatchWarnings(ctx, conn)
			} else if r.Connection.Driver == "postgres" {
				item.Warnings = append(item.Warnings, core.SQLWarning{Code: "notices_unavailable", Message: "PostgreSQL-Hinweise werden von diesem Adapter nicht erfasst."})
			}
			for _, set := range item.Results {
				if set.Truncated {
					item.Outcome = "outcome_unknown"
					item.Warnings = append(item.Warnings, core.SQLWarning{Code: "result_truncated", Message: "Ergebnis begrenzt; Ausführung und weitere Ergebnisse können unvollständig sein."})
					stopped = true
					break
				}
			}
		} else {
			stopped = true
		}
		out.Statements = append(out.Statements, item)
		// Reserve framing and every remaining status before another statement starts.
		encoded, _ := json.Marshal(out)
		if len(encoded)+65536 > MaxResponseBytes {
			last := &out.Statements[len(out.Statements)-1]
			last.Results = []core.ResultSet{}
			last.RowsAffected = nil
			last.Outcome = "outcome_unknown"
			last.ErrorCode = "result_too_large"
			stopped = true
		}
	}
	out.DurationMS = time.Since(start).Milliseconds()
	return out, nil
}

func sqlBatchStatement(ctx context.Context, conn *sql.Conn, driver, query string, maxRows int) ([]core.ResultSet, *string, error) {
	sets := []core.ResultSet{}
	if driver == "sqlite" {
		set, err := sqliteRead(ctx, conn, query, maxRows)
		if err != nil {
			return sets, nil, err
		}
		sets = append(sets, set)
		var affected *string
		switch sqlsplit.Keyword(query, driver) {
		case "INSERT", "UPDATE", "DELETE", "REPLACE":
			err = conn.Raw(func(raw any) error {
				value := strconv.FormatInt(raw.(sqliteDriver.Conn).Raw().Changes(), 10)
				affected = &value
				return nil
			})
		}
		return sets, affected, err
	}
	// Prepared PostgreSQL statements reject a hidden second command. MySQL's
	// connection has MultiStatements=false; COM_QUERY also supports routine DDL.
	keyword := sqlsplit.Keyword(query, driver)
	returnsRows := keyword == "SELECT" || keyword == "WITH" || keyword == "VALUES" || keyword == "TABLE" || keyword == "SHOW" || keyword == "EXPLAIN" || keyword == "CALL" || sqlsplit.HasKeyword(query, "RETURNING", driver)
	var stmt *sql.Stmt
	if driver == "postgres" {
		var err error
		stmt, err = conn.PrepareContext(ctx, query)
		if err != nil {
			return sets, nil, err
		}
		defer stmt.Close()
	}
	if !returnsRows {
		var result sql.Result
		var err error
		if stmt != nil {
			result, err = stmt.ExecContext(ctx)
		} else {
			result, err = conn.ExecContext(ctx, query)
		}
		if err != nil {
			return sets, nil, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return sets, nil, nil
		}
		value := strconv.FormatInt(n, 10)
		return sets, &value, nil
	}
	var rows *sql.Rows
	var err error
	if stmt != nil {
		rows, err = stmt.QueryContext(ctx)
	} else {
		rows, err = conn.QueryContext(ctx, query)
	}
	if err != nil {
		return sets, nil, err
	}
	defer rows.Close()
	used := 128
	for {
		if len(sets) == 16 {
			return sets, nil, public("result_too_large", "Zu viele Ergebnissätze.")
		}
		set, n, err := readSet(ctx, rows, maxRows, MaxResponseBytes-used)
		if err != nil {
			return sets, nil, err
		}
		used += n
		sets = append(sets, set)
		if set.Truncated {
			return sets, nil, nil
		}
		if !rows.NextResultSet() {
			break
		}
	}
	return sets, nil, rows.Err()
}

func sqlBatchErrorCode(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return "query_canceled"
	}
	var p *core.Error
	if errors.As(safe(ctx, err), &p) {
		return p.Code
	}
	return "database_error"
}
func sqlServerCode(err error) string {
	var m *mysql.MySQLError
	if errors.As(err, &m) {
		return strconv.Itoa(int(m.Number))
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		if len(p.Code) == 5 {
			valid := true
			for _, c := range p.Code {
				if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
					valid = false
				}
			}
			if valid {
				return p.Code
			}
		}
		return ""
	}
	var s *sqlite.Error
	if errors.As(err, &s) {
		return strconv.Itoa(int(s.Code()))
	}
	return ""
}
func sqlBatchWarnings(ctx context.Context, conn *sql.Conn) []core.SQLWarning {
	out := []core.SQLWarning{}
	rows, err := conn.QueryContext(ctx, "SHOW WARNINGS LIMIT 100")
	if err != nil {
		return []core.SQLWarning{{Code: "warnings_unavailable", Message: "Warnungen konnten nicht gelesen werden."}}
	}
	defer rows.Close()
	for rows.Next() {
		var level, message string
		var code int
		if rows.Scan(&level, &code, &message) != nil {
			break
		}
		out = append(out, core.SQLWarning{Code: strconv.Itoa(code), Message: "Serverwarnung; Rohtext wird aus Datenschutzgründen nicht ausgegeben."})
	}
	if rows.Err() != nil {
		out = append(out, core.SQLWarning{Code: "warnings_unavailable", Message: "Warnungen konnten nicht vollständig gelesen werden."})
	}
	return out
}

func (e *Engine) SQLSimulate(ctx context.Context, r core.SQLBatchRequest) (core.SQLSimulationResult, error) {
	start := time.Now()
	out := core.SQLSimulationResult{Outcome: "rejected", Warnings: []core.SQLWarning{}}
	statements, err := normalizeSQLBatch(&r)
	if err != nil {
		return out, err
	}
	if r.Connection.Driver != "sqlite" {
		return out, public("simulation_not_supported", "Freies DML ist hier wegen nicht rücknehmbarer Sequenz-, Trigger-, Engine- und externer Effekte nicht zur Simulation freigegeben.")
	}
	if len(statements) != 1 {
		return out, public("simulation_not_supported", "Simulation verlangt genau eine einfache DML-Anweisung.")
	}
	keyword := sqlsplit.Keyword(statements[0], r.Connection.Driver)
	if keyword != "INSERT" && keyword != "UPDATE" && keyword != "DELETE" {
		return out, public("simulation_not_supported", "Nur INSERT, UPDATE und DELETE sind zur Simulation freigegeben.")
	}
	// RETURNING is deliberately excluded: truncating returned rows could stop the
	// statement before all affected rows are counted. False positives are safe.
	if sqlsplit.HasKeyword(statements[0], "RETURNING", r.Connection.Driver) {
		return out, public("simulation_not_supported", "RETURNING ist für die Simulation nicht freigegeben.")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
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
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return out, safe(ctx, err)
	}
	defer tx.Rollback()
	// No external/virtual storage is admitted, even if present in an imported DB.
	var virtual int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type='table' AND lower(sql) LIKE '%virtual%'").Scan(&virtual); err != nil {
		return out, safe(ctx, err)
	}
	if virtual != 0 {
		return out, public("simulation_not_supported", "Virtuelle Tabellen verhindern die sichere Simulation.")
	}
	_, affected, executionErr := sqlBatchStatement(ctx, conn, "sqlite", statements[0], r.MaxRows)
	rollbackErr := tx.Rollback()
	out.DurationMS = time.Since(start).Milliseconds()
	out.Warnings = append(out.Warnings, core.SQLWarning{Code: "simulation_scope", Message: "DML-Versuch in Rollback-Transaktion. Anzahl zählt direkte Änderungen, keine Trigger-/Kaskadenzeilen. Keine allgemeine Garantie für externe Effekte oder andere Engines; Vorschau ist kein späterer Commitplan."})
	if rollbackErr != nil {
		out.Outcome = "outcome_unknown"
		out.Warnings = append(out.Warnings, core.SQLWarning{Code: "rollback_unknown", Message: "Rollback konnte nicht bestätigt werden; keinen automatischen Wiederholungsversuch ausführen."})
		return out, nil
	}
	out.Outcome = "rolled_back"
	if executionErr != nil {
		out.Warnings = append(out.Warnings, core.SQLWarning{Code: sqlBatchErrorCode(ctx, executionErr), Message: "DML-Ausführung fehlgeschlagen; Transaktion wurde zurückgerollt."})
		return out, nil
	}
	out.RowsAffected = affected
	return out, nil
}
