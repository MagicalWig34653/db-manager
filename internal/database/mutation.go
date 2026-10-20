// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqlite "github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func eligibility(d core.TableDescription) core.MutationEligibility {
	result := core.MutationEligibility{Description: d, Reason: "unsupported_table", WritableColumns: []string{}}
	if d.Kind != "table" || d.Transactional != "yes" || !d.StableOrder || len(d.PrimaryKey) == 0 {
		return result
	}
	result.Eligible = true
	result.Reason = "eligible"
	for _, column := range d.Columns {
		if column.Generated == "none" && column.PKPosition == 0 {
			result.WritableColumns = append(result.WritableColumns, column.Name)
		}
	}
	return result
}
func (e *Engine) Eligibility(ctx context.Context, r core.DescribeRequest) (core.MutationEligibility, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !validTableRef(r.Table) {
		return core.MutationEligibility{}, public("invalid_table", "Tabellenbezug ist ungültig.")
	}
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.MutationEligibility{}, err
	}
	defer closeDB()
	d, err := describe(ctx, db, r.Connection, r.Table)
	if err != nil {
		return core.MutationEligibility{}, safe(ctx, err)
	}
	result := eligibility(d)
	if r.Connection.Driver == "postgres" && result.Eligible {
		var children bool
		err = db.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&children)
		if err != nil {
			return result, safe(ctx, err)
		}
		if children {
			result.Eligible = false
			result.Reason = "unsupported_inheritance"
			result.WritableColumns = []string{}
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return core.MutationEligibility{}, public("result_too_large", "Eignungsmetadaten sind zu groß.")
	}
	return result, nil
}
func (e *Engine) Update(ctx context.Context, r core.MutationRequest) (core.MutationResult, error) {
	return e.mutate(ctx, r, false)
}
func (e *Engine) Delete(ctx context.Context, r core.MutationRequest) (core.MutationResult, error) {
	return e.mutate(ctx, r, true)
}

type mutationSession struct {
	conn   *sql.Conn
	tx     *sql.Tx
	driver string
}

func (s *mutationSession) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if s.tx != nil {
		return s.tx.QueryContext(ctx, q, args...)
	}
	return s.conn.QueryContext(ctx, q, args...)
}
func (s *mutationSession) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, q, args...)
	}
	return s.conn.QueryRowContext(ctx, q, args...)
}
func (s *mutationSession) exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, q, args...)
	}
	return s.conn.ExecContext(ctx, q, args...)
}
func (s *mutationSession) read(ctx context.Context, q string, args []any) (core.ResultSet, error) {
	if s.driver == "sqlite" {
		return sqliteReadMode(ctx, s.conn, q, 2, true, args...)
	}
	rows, err := s.tx.QueryContext(ctx, q, args...)
	if err != nil {
		return core.ResultSet{}, err
	}
	defer rows.Close()
	result, _, err := readSet(ctx, rows, 2, MaxResponseBytes)
	return result, err
}
func (s *mutationSession) rollback() {
	if s.tx != nil {
		_ = s.tx.Rollback()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = s.conn.ExecContext(ctx, "ROLLBACK")
}
func (s *mutationSession) commit(ctx context.Context) error {
	if s.tx != nil {
		return s.tx.Commit()
	}
	_, err := s.conn.ExecContext(ctx, "COMMIT")
	return err
}

func (e *Engine) mutate(ctx context.Context, r core.MutationRequest, deleting bool) (core.MutationResult, error) {
	empty := core.MutationResult{}
	if !r.Confirm || !validTableRef(r.Table) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 || len(r.Expected.Columns) == 0 || len(r.Expected.Columns) > 1024 || len(r.Expected.Columns) != len(r.Expected.Values) || (deleting && len(r.Changes) > 0) || (!deleting && (len(r.Changes) == 0 || len(r.Changes) > 1024)) {
		return empty, public("invalid_mutation", "Schreibbestätigung, Zeilenabbild oder Änderung ist ungültig.")
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	rawBytes := len(r.Connection.Password) + len(r.Connection.Username) + len(r.Connection.Database) + len(r.Connection.Host) + len(r.Table.Name) + len(r.Table.Schema)
	for i, value := range r.Expected.Values {
		if _, err := mutationValue(value); err != nil {
			return empty, err
		}
		rawBytes += len(value.Value) + len(r.Expected.Columns[i]) + 32
		if rawBytes > 1<<20 {
			return empty, public("invalid_mutation", "Änderungsanfrage ist zu groß.")
		}
	}
	for _, change := range r.Changes {
		if _, err := mutationValue(change.Value); err != nil {
			return empty, err
		}
		rawBytes += len(change.Column) + len(change.Value.Value) + 32
		if rawBytes > 1<<20 {
			return empty, public("invalid_mutation", "Änderungsanfrage ist zu groß.")
		}
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > 1<<20 {
		return empty, public("invalid_mutation", "Änderungsanfrage ist zu groß.")
	}

	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeDB()
	conn, err := db.Conn(ctx)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer conn.Close()
	session := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if session.driver == "sqlite" {
		// Mutations enforce declared FK constraints on this request's connection.
		err = conn.Raw(func(raw any) error {
			driver, ok := raw.(sqliteDriver.Conn)
			if !ok {
				return public("mutation_failed", "SQLite-Verbindung ist nicht verfügbar.")
			}
			enabled, err := driver.Raw().Config(sqlite.DBCONFIG_ENABLE_FKEY, true)
			if err != nil {
				return err
			}
			if !enabled {
				return public("mutation_failed", "Fremdschlüsselprüfung konnte nicht aktiviert werden.")
			}
			return nil
		})
		if err == nil {
			_, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE")
		}
	} else {
		session.tx, err = conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	}
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer session.rollback()
	d, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	suitable := eligibility(d)
	if !suitable.Eligible {
		return empty, public("mutation_not_supported", "Tabelle ist für strukturierte Änderungen nicht freigegeben.")
	}
	if session.driver == "postgres" {
		var inherited bool
		err = session.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&inherited)
		if err != nil {
			return empty, safe(ctx, err)
		}
		if inherited {
			return empty, public("mutation_not_supported", "Vererbte Tabellen sind nicht freigegeben.")
		}
	}
	if len(d.Columns) != len(r.Expected.Columns) {
		return empty, rowConflict()
	}
	positions := map[string]int{}
	for i, column := range d.Columns {
		if r.Expected.Columns[i] != column.Name {
			return empty, rowConflict()
		}
		positions[column.Name] = i
	}
	quote := func(v string) string { return quoteIdentifier(session.driver, v) }
	args := []any{}
	parameter := func(v any) string {
		args = append(args, v)
		if session.driver == "postgres" {
			return "$" + strconv.Itoa(len(args))
		}
		return "?"
	}
	predicates := []string{}
	for _, key := range d.PrimaryKey {
		v := r.Expected.Values[positions[key]]
		if v.Kind == "null" {
			return empty, public("invalid_mutation", "Primärschlüssel darf nicht NULL sein.")
		}
		value, err := mutationValue(v)
		if err != nil {
			return empty, err
		}
		name := quote(key)
		if session.driver == "sqlite" {
			name += " COLLATE BINARY"
		}
		predicates = append(predicates, name+" = "+parameter(value))
	}
	where := strings.Join(predicates, " AND ")
	names := []string{}
	for _, c := range d.Columns {
		names = append(names, quote(c.Name))
	}
	table := quote(d.Table.Schema) + "." + quote(d.Table.Name)
	if session.driver == "postgres" {
		table = "ONLY " + table
	}
	selectSQL := "SELECT " + strings.Join(names, ",") + " FROM " + table + " WHERE " + where
	if session.driver != "sqlite" {
		selectSQL += " FOR UPDATE"
	}
	current, err := session.read(ctx, selectSQL, args)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if current.Truncated || len(current.Rows) != 1 || !reflect.DeepEqual(current.Rows[0], r.Expected.Values) {
		return empty, rowConflict()
	}
	fresh, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !reflect.DeepEqual(d, fresh) {
		return empty, rowConflict()
	}
	assignments := []string{}
	seen := map[string]bool{}
	same := true
	// WHERE parameters must follow SET parameters in positional drivers.
	keyArgs := args
	args = []any{}
	for _, change := range r.Changes {
		pos, ok := positions[change.Column]
		if !ok || seen[change.Column] || d.Columns[pos].Generated != "none" || d.Columns[pos].PKPosition != 0 {
			return empty, public("invalid_mutation", "Zielspalte ist nicht beschreibbar.")
		}
		seen[change.Column] = true
		value, err := mutationValue(change.Value)
		if err != nil {
			return empty, err
		}
		if change.Value != r.Expected.Values[pos] {
			same = false
		}
		assignments = append(assignments, quote(change.Column)+" = "+parameter(value))
	}
	changed := deleting || !same
	if changed {
		predicates = []string{}
		for i, key := range d.PrimaryKey {
			name := quote(key)
			if session.driver == "sqlite" {
				name += " COLLATE BINARY"
			}
			predicates = append(predicates, name+" = "+parameter(keyArgs[i]))
		}
		query := "DELETE FROM " + table + " WHERE " + strings.Join(predicates, " AND ")
		if !deleting {
			query = "UPDATE " + table + " SET " + strings.Join(assignments, ",") + " WHERE " + strings.Join(predicates, " AND ")
		}
		result, err := session.exec(ctx, query, args...)
		if err != nil {
			return empty, safe(ctx, err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return empty, safe(ctx, err)
		}
		if count > 1 || count < 0 || (deleting && count != 1) {
			return empty, public("mutation_failed", "Einzelzeilenänderung konnte nicht bestätigt werden.")
		}
		if !deleting && count == 0 {
			changed = false
		}
	}
	result := core.MutationResult{Outcome: "committed", Changed: changed}
	if !deleting {
		after, err := session.read(ctx, selectSQL, keyArgs)
		if err != nil {
			return empty, safe(ctx, err)
		}
		if after.Truncated || len(after.Rows) != 1 {
			return empty, public("mutation_failed", "Geänderte Zeile konnte nicht bestätigt werden.")
		}
		result.Row = &core.RowSnapshot{Columns: r.Expected.Columns, Values: after.Rows[0]}
		result.Changed = !reflect.DeepEqual(current.Rows[0], after.Rows[0])
	}
	encoded, err = json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Änderungsergebnis ist zu groß.")
	}
	// Any error after attempting COMMIT is indeterminate; never retry blindly.
	if err := session.commit(ctx); err != nil {
		return empty, public("outcome_unknown", "Commit-Ergebnis ist unbekannt. Zustand neu laden; nicht blind wiederholen.")
	}
	return result, nil
}
func rowConflict() error {
	return public("row_conflict", "Zeile fehlt oder wurde geändert. Zustand neu laden.")
}
func mutationValue(c core.Cell) (any, error) {
	if c.Kind == "null" {
		if c.Value != "" {
			return nil, public("invalid_mutation", "NULL-Zelle darf keinen Wert enthalten.")
		}
		return nil, nil
	}
	return filterValue(c)
}
