// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func (e *Engine) CreateTable(ctx context.Context, r core.CreateRequest) (core.CreateResult, error) {
	empty := core.CreateResult{}
	statement, err := createSQL(r)
	if err != nil {
		return empty, err
	}
	r.Table, err = createRef(r.Connection, r.Table)
	if err != nil {
		return empty, err
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer conn.Close()
	maria := false
	s := &createSession{session: &mutationSession{conn: conn, driver: r.Connection.Driver}}
	if r.Connection.Driver == "mysql" {
		// All generated string literals use quote doubling under this pinned session mode.
		var modes, version string
		if err = conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode,VERSION()").Scan(&modes, &version); err != nil {
			return empty, safe(ctx, err)
		}
		maria = strings.Contains(version, "MariaDB")
		modes += ",NO_BACKSLASH_ESCAPES,STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION"
		if _, err = conn.ExecContext(ctx, "SET SESSION sql_mode = ?", modes); err != nil {
			return empty, safe(ctx, err)
		}
		if _, err = conn.ExecContext(ctx, "SET NAMES utf8mb4"); err != nil {
			return empty, safe(ctx, err)
		}
	} else {
		if r.Connection.Driver == "postgres" {
			if _, err = conn.ExecContext(ctx, "SET standard_conforming_strings=on"); err != nil {
				return empty, safe(ctx, err)
			}
		}
		if err = s.session.begin(ctx); err != nil {
			return empty, safe(ctx, err)
		}
		defer s.session.rollback()
	}
	// This lookup is advisory for errors only. CREATE without IF NOT EXISTS arbitrates races.
	_, err = describe(ctx, s.session, r.Connection, r.Table)
	if err == nil {
		return empty, public("create_conflict", "Ein Objekt mit diesem Tabellennamen ist bereits vorhanden.")
	}
	var p *core.Error
	if !errors.As(err, &p) || p.Code != "table_not_found" {
		return empty, safe(ctx, err)
	}
	return executeCreate(ctx, r, statement, s, maria)
}

type createSession struct{ session *mutationSession }

func (s *createSession) execCreate(ctx context.Context, statement string) error {
	_, err := s.session.exec(ctx, statement)
	return err
}
func (s *createSession) observeCreate(ctx context.Context, r core.CreateRequest) (core.TableDescription, error) {
	if r.Connection.Driver != "mysql" {
		return describe(ctx, s.session, r.Connection, r.Table)
	}
	// The MDL protects only this separate observation, never continuity from CREATE.
	tx, err := s.session.conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return core.TableDescription{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT 1 FROM "+quoteIdentifier("mysql", r.Table.Schema)+"."+quoteIdentifier("mysql", r.Table.Name)+" LIMIT 0")
	if err != nil {
		return core.TableDescription{}, err
	}
	if err = rows.Close(); err != nil {
		return core.TableDescription{}, err
	}
	return describe(ctx, tx, r.Connection, r.Table)
}
func (s *createSession) commitCreate(ctx context.Context) error { return s.session.commit(ctx) }

type createExecution interface {
	execCreate(context.Context, string) error
	observeCreate(context.Context, core.CreateRequest) (core.TableDescription, error)
	commitCreate(context.Context) error
}

func createUnknown() error {
	return public("outcome_unknown", "Anlageergebnis ist unbekannt. Zustand prüfen; nicht automatisch wiederholen oder entfernen.")
}
func executeCreate(ctx context.Context, r core.CreateRequest, statement string, s createExecution, maria bool) (core.CreateResult, error) {
	empty := core.CreateResult{}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if err := s.execCreate(ctx, statement); err != nil {
		var my *mysql.MySQLError
		var pg *pgconn.PgError
		if errors.As(err, &my) && my.Number == 1050 || errors.As(err, &pg) && pg.Code == "42P07" {
			return empty, public("create_conflict", "Ein Objekt mit diesem Tabellennamen ist bereits vorhanden.")
		}
		if r.Connection.Driver == "mysql" && !errors.As(err, &my) {
			return empty, createUnknown()
		}
		return empty, safe(ctx, err)
	}
	d, err := s.observeCreate(ctx, r)
	result := core.CreateResult{Outcome: "created", StructureCheck: "unavailable", Message: "Anlage bestätigt, Strukturprüfung nicht verfügbar."}
	if r.Connection.Driver == "mysql" {
		if err != nil {
			return result, nil
		}
		result.StructureCheck = "observed"
		result.Message = "Anlage bestätigt; Struktur separat beobachtet, kein atomarer Anlage-Snapshot."
		if !createMatches(r, d, maria) {
			result.StructureCheck = "different"
			result.Message = "Anlage bestätigt; die separat beobachtete Struktur konnte nicht als übereinstimmend bestätigt werden."
		}
		result.ActualDescription = &d
		if !createResultFits(result) {
			result.ActualDescription = nil
			result.StructureCheck = "unavailable"
			result.Message = "Anlage bestätigt, Strukturprüfung nicht verfügbar."
		}
		return result, nil
	}
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !createMatches(r, d, maria) {
		return empty, public("structure_mismatch", "Anlage vor Commit abgebrochen: rückgelesene Struktur nicht als übereinstimmend bestätigt.")
	}
	result.StructureCheck = "transactional"
	result.SnapshotAtomic = true
	result.ActualDescription = &d
	result.Message = "Anlage und Strukturprüfung in derselben Transaktion bestätigt."
	if !createResultFits(result) {
		return empty, public("result_too_large", "Strukturantwort ist zu groß.")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if err := s.commitCreate(ctx); err != nil {
		return empty, createUnknown()
	}
	return result, nil
}
func createResultFits(result core.CreateResult) bool {
	encoded, err := json.Marshal(result)
	return err == nil && len(encoded)+1 <= MaxResponseBytes
}
