// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/MagicalWig34653/db-manager/internal/sqlsplit"
)

// Fixed export session headers are allowed on this request's dedicated connection.
// Arbitrary session/transaction controls could hide an uncommitted transaction.
func importSessionHeader(s string) bool {
	switch importControlText(s) {
	case "SET standard_conforming_strings=on", "SET TIME ZONE 'UTC'", "SET NAMES utf8mb4", "SET time_zone='+00:00'", "SET @dbm_export_old_sql_mode=@@SQL_MODE", "SET SESSION sql_mode=TRIM(BOTH ',' FROM REPLACE(CONCAT(',',@@SQL_MODE,','),',NO_BACKSLASH_ESCAPES,',','))", "SET SESSION sql_mode=@dbm_export_old_sql_mode":
		return true
	}
	return false
}
func importControlText(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "--") {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return ""
		}
		s = strings.TrimSpace(s[i+1:])
	}
	return dbplan.Normalize(s)
}
func (e *Engine) importSQL(ctx context.Context, r core.ImportRequest, text string, progress func(core.ImportProgress)) (core.ImportResult, error) {
	out := emptyImportResult()
	if strings.HasPrefix(text, "-- DB Manager SQL export 1;") && !strings.HasSuffix(strings.TrimSpace(text), "-- DBMANAGER EXPORT COMPLETE") {
		return out, public("incomplete_export", "SQL-Export hat keine Schlussmarke.")
	}
	if r.ExpectedPlanSHA256 != "" {
		return out, invalidImport()
	}
	statements, err := sqlsplit.SplitLimitedContext(ctx, text, r.Delimiter, r.Connection.Driver, core.MaxImportBytes, core.MaxImportStatements)
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		return out, invalidImport()
	}
	// Validate the entire file before the first statement can change anything.
	depth := 0
	for _, s := range statements {
		if len(s) > sqlsplit.MaxBytes {
			return out, public("import_too_large", "Einzelanweisung überschreitet 1 MiB.")
		}
		keyword := sqlsplit.Keyword(s, r.Connection.Driver)
		switch keyword {
		case "BEGIN":
			if strings.ToUpper(importControlText(s)) != "BEGIN" || depth != 0 {
				return out, invalidImport()
			}
			depth = 1
		case "COMMIT":
			if strings.ToUpper(importControlText(s)) != "COMMIT" || depth != 1 {
				return out, invalidImport()
			}
			depth = 0
		case "SET":
			if !importSessionHeader(s) {
				return out, public("sql_not_supported", "Nur feste Export-Sitzungseinstellungen sind freigegeben.")
			}
		case "", "START", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE", "LOCK", "UNLOCK", "LOAD", "COPY", "USE", "ATTACH", "DETACH":
			return out, public("sql_not_supported", "Diese Importanweisung ist nicht freigegeben.")
		}
	}
	if depth != 0 {
		return out, invalidImport()
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
	out.Warnings = []string{importSQLClassWarning(statements, r.Connection.Driver), "Anweisungsabschluss bestätigt keine universelle Transaktion. Abbruch, Trigger und externe Effekte können Teiländerungen hinterlassen. Kein automatischer Retry."}
	if r.Connection.Driver == "mysql" {
		out.Warnings = append(out.Warnings, "MySQL/MariaDB: DDL kann implizit committen; auch BEGIN/COMMIT garantieren keine atomare Datei und nichttransaktionale Tabellen keinen Rollback.")
	}
	if progress != nil {
		progress(core.ImportProgress{Total: len(statements)})
	}
	stopped, failed := false, false
	// Track the explicit transaction: statements that were rolled back later must not
	// stay "completed" on engines with transactional DDL (SQLite, PostgreSQL).
	transactional := r.Connection.Driver == "sqlite" || r.Connection.Driver == "postgres"
	inTx, txFailed, txStart := false, false, 0
	markRolledBack := func(from, to int) {
		for j := from; j <= to && j < len(out.Statements); j++ {
			if out.Statements[j].Outcome == "completed" {
				out.Statements[j].Outcome = "rolled_back"
				out.Statements[j].RowsAffected = nil
			}
		}
	}
	for i, s := range statements {
		item := core.SQLStatementResult{Index: i + 1, Outcome: "not_executed", Results: []core.ResultSet{}, Warnings: []core.SQLWarning{}}
		if !stopped && ctx.Err() == nil {
			before := time.Now()
			item.Outcome = "completed"
			sets, affected, execErr := sqlBatchStatement(ctx, conn, r.Connection.Driver, s, 1)
			item.RowsAffected = affected
			item.DurationMS = time.Since(before).Milliseconds()
			// Import returns execution statuses, never arbitrary result data. Limit read
			// results and stop if truncation might have interrupted a RETURNING statement.
			for _, set := range sets {
				if set.Truncated {
					execErr = public("result_too_large", "Importanweisung lieferte zu viele Ergebniszeilen.")
				}
			}
			out.Processed++
			if execErr != nil {
				item.Outcome = "outcome_unknown"
				item.RowsAffected = nil
				item.ErrorCode = sqlBatchErrorCode(ctx, execErr)
				failed = true
				stopped = r.Mode == "stop" || ctx.Err() != nil || item.ErrorCode == "result_too_large"
				if inTx {
					txFailed = true
				}
			} else if r.Connection.Driver == "mysql" {
				warnings := sqlBatchWarnings(ctx, conn)
				if len(warnings) > 0 {
					item.Warnings = append(item.Warnings, core.SQLWarning{Code: "server_warning", Message: "Serverwarnung; Datenzustand prüfen."})
				}
			}
		} else {
			stopped = true
			failed = true
		}
		out.Statements = append(out.Statements, item)
		if item.Outcome == "completed" && transactional {
			switch sqlsplit.Keyword(s, r.Connection.Driver) {
			case "BEGIN":
				inTx, txFailed, txStart = true, false, len(out.Statements)
			case "COMMIT":
				if inTx && txFailed && r.Connection.Driver == "postgres" {
					// COMMIT of an aborted PostgreSQL transaction is a ROLLBACK.
					markRolledBack(txStart, len(out.Statements)-1)
				}
				inTx, txFailed = false, false
			}
		}
		if progress != nil {
			progress(core.ImportProgress{Processed: out.Processed, Total: len(statements)})
		}
	}
	// An interrupted explicit transaction is rolled back on the pinned connection;
	// earlier commits and MySQL implicit commits remain visible in the result.
	cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, rollbackErr := conn.ExecContext(cleanup, "ROLLBACK")
	if inTx && rollbackErr == nil {
		markRolledBack(txStart, len(out.Statements)-1)
	}
	out.Outcome = "completed"
	if failed {
		out.Outcome = "outcome_unknown"
		out.Reason = "partial_or_unknown"
	}
	return out, nil
}

// importSQLClassWarning states plainly that a SQL import runs arbitrary SQL without a
// plan and counts the statement classes that were executed (the file is fully
// validated and counted before the first statement runs).
func importSQLClassWarning(statements []string, driver string) string {
	var ddl, drop, del, upd, ins, other int
	for _, s := range statements {
		switch sqlsplit.Keyword(s, driver) {
		case "CREATE", "ALTER", "RENAME", "COMMENT", "GRANT", "REVOKE", "ANALYZE", "REINDEX", "VACUUM":
			ddl++
		case "DROP", "TRUNCATE":
			drop++
		case "DELETE":
			del++
		case "UPDATE":
			upd++
		case "INSERT", "REPLACE":
			ins++
		default:
			other++
		}
	}
	return fmt.Sprintf("SQL-Import führt beliebiges SQL ohne Plan mit den Rechten der Verbindung aus. Anweisungen: DDL %d, DROP/TRUNCATE %d, DELETE %d, UPDATE %d, INSERT %d, sonstige %d.", ddl, drop, del, upd, ins, other)
}
