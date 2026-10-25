// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite "github.com/ncruces/go-sqlite3"
)

func invalidColumns() error {
	return public("invalid_columns", "Die strukturierte Spaltenänderung ist ungültig.")
}
func unsupportedColumns() error {
	return public("unsupported_action", "Diese Spaltenänderung wird für diese Tabelle oder diesen Treiber nicht unterstützt.")
}

func validateColumns(r core.ColumnsRequest) error {
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > 1<<20 {
		return invalidColumns()
	}
	if !validTableRef(r.Table) || len(r.Changes) < 1 || len(r.Changes) > 16 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return invalidColumns()
	}
	for _, ch := range r.Changes {
		if !core.ColumnsCapabilities().Allows(r.Connection.Driver, ch.Action) {
			return unsupportedColumns()
		}
		switch ch.Action {
		case "add":
			if ch.Column != "" || ch.Definition == nil || ch.NewName != "" {
				return invalidColumns()
			}
		case "change", "reorder":
			if ch.Column == "" || ch.Definition == nil || ch.NewName != "" {
				return invalidColumns()
			}
			if ch.Action == "reorder" && ch.Position == nil {
				return invalidColumns()
			}
		case "rename":
			if ch.Column == "" || ch.NewName == "" || ch.Definition != nil || ch.Position != nil {
				return invalidColumns()
			}
		case "drop":
			if ch.Column == "" || ch.Definition != nil || ch.NewName != "" || ch.Position != nil {
				return invalidColumns()
			}
		default:
			return invalidColumns()
		}
		if ch.NewName != "" && !columnName(ch.NewName) {
			return invalidColumns()
		}
		if ch.Position != nil {
			if r.Connection.Driver != "mysql" {
				return unsupportedColumns()
			}
			p := ch.Position
			if p.Mode == "first" {
				if p.Column != "" {
					return invalidColumns()
				}
			} else if (p.Mode != "before" && p.Mode != "after") || p.Column == "" {
				return invalidColumns()
			}
		}
		if d := ch.Definition; d != nil {
			if !createName(d.Name) || (ch.Action == "add" || d.Name != ch.Column) && !columnName(d.Name) || d.Nullable == nil || d.Default == nil {
				return invalidColumns()
			}
			if _, err := dbplan.TypeSQL(r.Connection.Driver, d.Type); err != nil {
				return invalidColumns()
			}
			if _, err := columnDefault(*d); err != nil {
				return err
			}
			if d.Comment != nil && (len(*d.Comment) > 2048 || strings.ContainsRune(*d.Comment, 0)) {
				return invalidColumns()
			}
			if d.Collation != nil && (*d.Collation == "" || len(*d.Collation) > 1024 || strings.ContainsRune(*d.Collation, 0)) {
				return invalidColumns()
			}
			if r.Connection.Driver == "sqlite" && (d.Comment != nil || d.Collation != nil) {
				return unsupportedColumns()
			}
		}
	}
	return nil
}

// columnsSession pins catalog lookup, confirmation and execution to one connection.
func (e *Engine) columnsSession(ctx context.Context, r core.ColumnsRequest) (*mutationSession, func(), error) {
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return nil, nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		cleanup()
		return nil, nil, safe(ctx, err)
	}
	closeAll := func() { conn.Close(); cleanup() }
	s := &mutationSession{conn: conn, driver: r.Connection.Driver}
	switch s.driver {
	case "mysql":
		var modes string
		err = conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&modes)
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET SESSION sql_mode = ?", modes+",NO_BACKSLASH_ESCAPES,STRICT_ALL_TABLES")
		}
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET SESSION lock_wait_timeout = 10")
		}
	case "postgres":
		_, err = conn.ExecContext(ctx, "SET standard_conforming_strings = on")
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET lock_timeout = '10s'")
		}
	}
	if err != nil {
		closeAll()
		return nil, nil, safe(ctx, err)
	}
	return s, closeAll, nil
}
func columnsContext(ctx context.Context, r core.ColumnsRequest) (context.Context, context.CancelFunc) {
	ms := r.TimeoutMS
	if ms == 0 {
		ms = 30000
	}
	return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
}
func (e *Engine) PlanColumns(ctx context.Context, r core.ColumnsRequest) (core.ColumnsPlan, error) {
	if err := validateColumns(r); err != nil {
		return core.ColumnsPlan{}, err
	}
	ctx, cancel := columnsContext(ctx, r)
	defer cancel()
	s, cleanup, err := e.columnsSession(ctx, r)
	if err != nil {
		return core.ColumnsPlan{}, err
	}
	defer cleanup()
	p, err := buildColumns(ctx, s, r)
	if err == nil {
		encoded, marshalErr := json.Marshal(p)
		if marshalErr != nil || len(encoded)+1 > MaxResponseBytes {
			return core.ColumnsPlan{}, public("result_too_large", "Spaltenplan ist zu groß.")
		}
	}
	return p, safeColumns(ctx, err)
}
func safeColumns(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	return safe(ctx, err)
}

func (e *Engine) ExecuteColumns(ctx context.Context, r core.ColumnsRequest) (core.ColumnsResult, error) {
	empty := core.ColumnsResult{}
	if err := validateColumns(r); err != nil {
		return empty, err
	}
	if r.Confirm == nil || r.ExpectedPlanSHA256 == "" {
		return empty, invalidColumns()
	}
	ctx, cancel := columnsContext(ctx, r)
	defer cancel()
	s, cleanup, err := e.columnsSession(ctx, r)
	if err != nil {
		return empty, err
	}
	defer cleanup()
	transactional := s.driver != "mysql"
	if transactional {
		if err = s.begin(ctx); err != nil {
			return empty, safe(ctx, err)
		}
		defer s.rollback()
	}
	p, err := buildColumns(ctx, s, r)
	if err != nil {
		return empty, safe(ctx, err)
	}
	// The target name is the table name returned by describe, never request text.
	if err = dbplan.Check(p.Plan, r.ExpectedPlanSHA256, *r.Confirm, "columns_alter", p.Table.Name, p.Destructive); err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) || errors.Is(err, dbplan.ErrInvalidDigest) {
			return empty, public("plan_mismatch", "Der Plan stimmt nicht mehr mit der bestätigten Vorschau überein.")
		}
		return empty, public("confirmation_mismatch", "Aktion oder getippter Tabellenname stimmt nicht überein.")
	}
	return runColumns(ctx, r, p, columnsRunner{s}), nil
}

type columnsExecution interface {
	exec(context.Context, string, ...any) (sql.Result, error)
	commit(context.Context) error
	observeColumns(context.Context, core.Connection, core.TableRef) (core.TableDescription, error)
}
type columnsRunner struct{ *mutationSession }

func (s columnsRunner) observeColumns(ctx context.Context, c core.Connection, table core.TableRef) (core.TableDescription, error) {
	return describe(ctx, s, c, table)
}

func runColumns(ctx context.Context, r core.ColumnsRequest, p core.ColumnsPlan, s columnsExecution) core.ColumnsResult {
	var err error
	transactional := r.Connection.Driver != "mysql"
	result := core.ColumnsResult{Outcome: dbplan.StatusRejected, Reason: "database_error", Message: "Die Änderung wurde abgewiesen.", Observation: "unavailable"}
	for i, statement := range p.Statements {
		if ctx.Err() != nil && i == 0 {
			result.Reason = "canceled"
			return result
		}
		if _, err = s.exec(ctx, statement); err != nil {
			reason, definite := columnsFailure(err)
			result.Reason = reason
			// A lost acknowledgement can never be advertised as rejection. MySQL DDL
			// is one statement on InnoDB; transactional drivers roll back known errors.
			if !definite || !transactional && i > 0 {
				result.Outcome = dbplan.StatusOutcomeUnknown
				result.Message = "Ausgang unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			}
			if transactional && definite {
				result.StatementsExecuted = 0
			}
			return result
		}
		result.StatementsExecuted++
	}
	d, observeErr := s.observeColumns(ctx, r.Connection, p.Table)
	if observeErr == nil {
		result.Description = &d
		result.Observation = "observed"
		if transactional {
			result.Observation = "transactional"
		}
	}
	if result.Description != nil {
		encoded, marshalErr := json.Marshal(result)
		if marshalErr != nil || len(encoded)+1024 > MaxResponseBytes {
			observeErr = errors.New("columns: observation budget")
			result.Description = nil
			result.Observation = "unavailable"
		}
	}
	if transactional {
		if observeErr != nil {
			result.StatementsExecuted = 0
			result.Reason = "observation_unavailable"
			return result
		}
		if err = s.commit(ctx); err != nil {
			result.Outcome = dbplan.StatusOutcomeUnknown
			result.Reason = "commit_unknown"
			result.Message = "Commit-Ausgang unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			result.Description = nil
			result.Observation = "unavailable"
			return result
		}
		result.Atomic = true
	}
	result.Outcome = dbplan.StatusCompleted
	result.Reason = "completed"
	result.Message = "Spaltenänderung ausgeführt; Strukturbeobachtung ist separat ausgewiesen."
	return result
}

func columnsFailure(err error) (string, bool) {
	var my *mysql.MySQLError
	var pg *pgconn.PgError
	var sq *sqlite.Error
	if errors.As(err, &my) {
		switch my.Number {
		case 1044, 1045, 1142, 1143, 1227:
			return "permission_denied", true
		case 1205, 1213:
			return "lock_timeout", true
		case 1451, 1553, 1828, 1829, 3730:
			return "dependency_blocks", true
		}
		return "database_error", true
	}
	if errors.As(err, &pg) {
		switch pg.Code {
		case "42501":
			return "permission_denied", true
		case "55P03", "57014", "40P01":
			return "lock_timeout", true
		case "2BP01":
			return "dependency_blocks", true
		}
		return "database_error", true
	}
	if errors.As(err, &sq) {
		return "database_error", true
	}
	return "connection_lost", false
}

// columnName additionally rejects edge whitespace and control characters in new
// column names so unusable names fail in the plan, not as a generic server error.
func columnName(name string) bool {
	if !createName(name) || name != strings.TrimSpace(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
