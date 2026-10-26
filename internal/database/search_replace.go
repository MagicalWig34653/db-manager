// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

type replaceState struct {
	description core.TableDescription
	rows        [][]core.Cell
	after       []core.Cell
	position    int
	plan        core.ReplacePlan
}

func replaceRead(ctx context.Context, s *mutationSession, q string, args []any, limit int) (core.ResultSet, error) {
	if s.driver == "sqlite" {
		return sqliteReadMode(ctx, s.conn, q, limit, true, args...)
	}
	rows, err := s.QueryContext(ctx, q, args...)
	if err != nil {
		return core.ResultSet{}, err
	}
	defer rows.Close()
	result, _, err := readSet(ctx, rows, limit, MaxResponseBytes)
	return result, err
}
func replaceValidate(r *core.ReplaceRequest) error {
	preview := 1
	if err := searchLimits(&r.MaxRows, &preview, &r.TimeoutMS); err != nil {
		return err
	}
	if !validTableRef(r.Table) || r.Column == nil || r.Replacement == nil || !searchText(*r.Replacement) || !searchText(r.Term) || r.Term == "" || (r.Mode != "substring" && r.Mode != "exact" && r.Mode != "regex") {
		return invalidSearch()
	}
	if r.Mode == "regex" && r.Connection.Driver == "sqlite" {
		return public("unsupported_search", "SQLite bietet hier keine native Regex-Ersetzung.")
	}
	return nil
}
func replacePrepare(ctx context.Context, s *mutationSession, r core.ReplaceRequest, lock bool) (replaceState, error) {
	state := replaceState{}
	d, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return state, err
	}
	if !eligibility(d).Eligible {
		return state, public("unsupported_search", "Tabelle ist nicht für transaktionales Ersetzen freigegeben.")
	}
	if _, err := searchNames(d, []string{*r.Column}, s.driver); err != nil {
		return state, err
	}
	position := -1
	names := []string{}
	order := []string{}
	for i, col := range d.Columns {
		names = append(names, quoteIdentifier(s.driver, col.Name))
		if col.Name == *r.Column {
			position = i
			if col.Generated != "none" || col.PKPosition != 0 {
				return state, public("unsupported_search", "Zielspalte ist nicht beschreibbar.")
			}
		}
	}
	if s.driver == "mysql" {
		var onUpdate int
		if err = s.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND LOWER(EXTRA) LIKE '%on update%'", d.Table.Schema, d.Table.Name).Scan(&onUpdate); err != nil {
			return state, err
		}
		if onUpdate > 0 {
			return state, public("unsupported_search", "Die Tabelle hat eine ON UPDATE-Spalte (zum Beispiel ON UPDATE CURRENT_TIMESTAMP); das Update würde sie ändern und die Nachprüfung scheitern lassen. Ersetzen ist hier nicht verfügbar.")
		}
	}
	table := searchTable(d, s.driver)
	if s.driver == "postgres" {
		// This relation lock prevents concurrent DML and DDL during execution. Plans
		// use the same lock in SHARE mode and always roll back without any UPDATE.
		mode := "SHARE"
		if lock {
			mode = "SHARE ROW EXCLUSIVE"
		}
		if _, err = s.exec(ctx, "LOCK TABLE ONLY "+table+" IN "+mode+" MODE"); err != nil {
			return state, err
		}
		var unsupported bool
		err = s.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR c.relhasrules OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&unsupported)
		if err != nil {
			return state, err
		}
		if unsupported {
			return state, public("unsupported_search", "Vererbung, Partitionen und Rewrite-Regeln sind ausgeschlossen.")
		}
		table = "ONLY " + table
	}
	for _, key := range d.PrimaryKey {
		name := quoteIdentifier(s.driver, key)
		if s.driver == "sqlite" {
			name += " COLLATE BINARY"
		}
		order = append(order, name)
	}
	b := searchBuilder{driver: s.driver}
	name := names[position]
	pred, err := b.predicate([]string{name}, r.Term, r.Mode)
	if err != nil {
		return state, err
	}
	var replacement string
	switch r.Mode {
	case "exact":
		replacement = b.bind(*r.Replacement)
	case "substring":
		replacement = "REPLACE(" + name + "," + b.bind(r.Term) + "," + b.bind(*r.Replacement) + ")"
	case "regex":
		replacement = "REGEXP_REPLACE(" + name + "," + b.bind(r.Term) + "," + b.bind(*r.Replacement)
		if s.driver == "postgres" {
			replacement += ",'g'"
		}
		replacement += ")"
	}
	if s.driver == "sqlite" {
		pred = "typeof(" + name + ")='text' AND (" + pred + ")"
	}
	expr := "CASE WHEN " + pred + " THEN " + replacement + " ELSE " + name + " END"
	q := "SELECT " + strings.Join(names, ",") + "," + expr + " FROM " + table + " ORDER BY " + strings.Join(order, ",") + " LIMIT " + b.bind(int64(r.MaxRows+1))
	if s.driver != "sqlite" {
		q += " FOR UPDATE"
	}
	set, err := replaceRead(ctx, s, q, b.args, r.MaxRows+1)
	if err != nil {
		return state, err
	}
	if set.Truncated || len(set.Rows) > r.MaxRows {
		return state, public("replace_limit", "Der vollständige Tabellenbestand überschreitet das Ersetzungslimit.")
	}
	fresh, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return state, err
	}
	if !reflect.DeepEqual(d, fresh) {
		return state, rowConflict()
	}
	state = replaceState{description: d, rows: [][]core.Cell{}, after: []core.Cell{}, position: position}
	plan := core.ReplacePlan{Driver: s.driver, Table: d.Table, Column: *r.Column, Examples: []core.ReplaceExample{}, Statements: []string{}, Warnings: []string{"Vollständiger begrenzter Tabellenbestand und berechnete Werte werden geprüft. Kollation und native Ersetzungsfunktion bestimmen die Semantik.", "Trigger und Kaskaden können zusätzliche oder externe Wirkungen haben. Kein automatischer Retry; ABA bleibt unerkennbar."}}
	// AffectedRows counts values changed by the native expression. LIKE
	// collation can match a literal whose case-sensitive REPLACE is a no-op.
	for _, row := range set.Rows {
		after := row[len(d.Columns)]
		before := row[position]
		state.rows = append(state.rows, append([]core.Cell{}, row[:len(d.Columns)]...))
		state.after = append(state.after, after)
		if before == after {
			continue
		}
		if before.Kind != "text" || after.Kind != "text" || len(before.Value) > core.SearchMaxValue || len(after.Value) > core.SearchMaxValue {
			return state, public("replace_limit", "Ersetzungswerte sind zu groß oder nicht als Text darstellbar.")
		}
		plan.AffectedRows++
		if len(plan.Examples) < core.SearchMaxPreview {
			keys := []core.Cell{}
			for _, key := range d.PrimaryKey {
				for i, col := range d.Columns {
					if col.Name == key {
						keys = append(keys, row[i])
					}
				}
			}
			plan.Examples = append(plan.Examples, core.ReplaceExample{Key: keys, Before: before, After: after})
		}
	}
	prefix := "UPDATE "
	if s.driver == "sqlite" {
		prefix = "UPDATE OR ABORT "
	}
	// Template only: values and keys are always bound separately during execution.
	update := searchBuilder{driver: s.driver}
	assignment := quoteIdentifier(s.driver, *r.Column) + " = " + update.bind("")
	where := []string{}
	for _, key := range d.PrimaryKey {
		n := quoteIdentifier(s.driver, key)
		if s.driver == "sqlite" {
			n += " COLLATE BINARY"
		}
		where = append(where, n+" = "+update.bind(""))
	}
	plan.Statements = []string{prefix + table + " SET " + assignment + " WHERE " + strings.Join(where, " AND ")}
	// Bind the request semantics, canonical metadata and every complete row into
	// a domain-separated digest; never hash credentials or a private file path.
	payload, err := json.Marshal(struct {
		Domain                          string
		Table                           core.TableDescription
		Column, Term, Replacement, Mode string
		Rows                            [][]core.Cell
		After                           []core.Cell
	}{"dbmanager/replace/v1", d, *r.Column, r.Term, *r.Replacement, r.Mode, state.rows, state.after})
	if err != nil {
		return state, err
	}
	plan.SHA256, err = dbplan.Digest(s.driver, []string{plan.Statements[0], string(payload)})
	state.plan = plan
	encoded, encodeErr := json.Marshal(plan)
	if encodeErr != nil || len(encoded)+1 > MaxResponseBytes {
		return state, public("result_too_large", "Ersetzungsplan ist zu groß.")
	}
	return state, err
}
func (e *Engine) ReplacePlan(ctx context.Context, r core.ReplaceRequest) (core.ReplacePlan, error) {
	if err := replaceValidate(&r); err != nil {
		return core.ReplacePlan{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.ReplacePlan{}, err
	}
	defer closeDB()
	searchServerLimit(ctx, db, r.Connection.Driver)
	conn, err := db.Conn(ctx)
	if err != nil {
		return core.ReplacePlan{}, safe(ctx, err)
	}
	defer conn.Close()
	s := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = s.begin(ctx); err != nil {
		return core.ReplacePlan{}, safe(ctx, err)
	}
	defer s.rollback()
	state, err := replacePrepare(ctx, s, r, false)
	if err != nil {
		return core.ReplacePlan{}, safe(ctx, err)
	}
	return state.plan, nil
}
func (e *Engine) ReplaceExecute(ctx context.Context, r core.ReplaceExecuteRequest) (core.ReplaceResult, error) {
	empty := core.ReplaceResult{}
	if err := replaceValidate(&r.ReplaceRequest); err != nil {
		return empty, err
	}
	if r.Confirm.Action != "replace" || r.Confirm.TargetName != r.Table.Name || r.Confirm.AffectedRows < 0 || len(r.ExpectedPlanSHA256) != 64 {
		return empty, public("confirmation_mismatch", "Bestätigung mit Tabellenname, Trefferzahl und Plan-Hash ist erforderlich.")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeDB()
	searchServerLimit(ctx, db, r.Connection.Driver)
	conn, err := db.Conn(ctx)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer conn.Close()
	s := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = s.begin(ctx); err != nil {
		return empty, safe(ctx, err)
	}
	defer s.rollback()
	state, err := replacePrepare(ctx, s, r.ReplaceRequest, true)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if err = dbplan.Check(dbplan.Plan{SHA256: state.plan.SHA256}, r.ExpectedPlanSHA256, dbplan.Confirmation{Action: r.Confirm.Action, TargetName: r.Confirm.TargetName}, "replace", state.plan.Table.Name, true); err != nil {
		return empty, public("plan_mismatch", "Tabellenbestand oder Ersetzung wurde seit der Vorschau geändert.")
	}
	if r.Confirm.TargetName != state.plan.Table.Name || r.Confirm.AffectedRows != state.plan.AffectedRows {
		return empty, public("confirmation_mismatch", "Bestätigte Trefferzahl oder Tabelle stimmt nicht überein.")
	}

	reject := func(reason string) (core.ReplaceResult, error) {
		return core.ReplaceResult{Outcome: dbplan.StatusRejected, Reason: reason}, nil
	}
	d := state.description
	expected := make([][]core.Cell, len(state.rows))
	for n, row := range state.rows {
		expected[n] = append([]core.Cell{}, row...)
		if row[state.position] == state.after[n] {
			continue
		}
		// Re-read each complete row after earlier triggers, using its exact PK.
		b := searchBuilder{driver: s.driver}
		keys := []string{}
		names := []string{}
		for _, col := range d.Columns {
			names = append(names, quoteIdentifier(s.driver, col.Name))
		}
		for _, key := range d.PrimaryKey {
			for i, col := range d.Columns {
				if col.Name == key {
					value, err := mutationValue(row[i])
					if err != nil {
						return reject("invalid_value")
					}
					name := quoteIdentifier(s.driver, key)
					if s.driver == "sqlite" {
						name += " COLLATE BINARY"
					}
					keys = append(keys, name+" = "+b.bind(value))
				}
			}
		}
		table := searchTable(d, s.driver)
		if s.driver == "postgres" {
			table = "ONLY " + table
		}
		set, err := replaceRead(ctx, s, "SELECT "+strings.Join(names, ",")+" FROM "+table+" WHERE "+strings.Join(keys, " AND "), b.args, 2)
		if err != nil {
			return reject("database_error")
		}
		if len(set.Rows) != 1 || !reflect.DeepEqual(set.Rows[0], row) {
			return reject("row_conflict")
		}
		args := []any{state.after[n].Value}
		args = append(args, b.args...)
		result, err := s.exec(ctx, state.plan.Statements[0], args...)
		if err != nil {
			return reject("database_error")
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return reject("row_conflict")
		}
		expected[n][state.position] = state.after[n]
	}
	// Verify the whole bounded table after all updates, including rows not targeted
	// by replacement. Generated values may legitimately follow changed columns.
	b := searchBuilder{driver: s.driver}
	names := []string{}
	order := []string{}
	for _, col := range d.Columns {
		names = append(names, quoteIdentifier(s.driver, col.Name))
	}
	for _, key := range d.PrimaryKey {
		name := quoteIdentifier(s.driver, key)
		if s.driver == "sqlite" {
			name += " COLLATE BINARY"
		}
		order = append(order, name)
	}
	q := "SELECT " + strings.Join(names, ",") + " FROM " + searchTable(d, s.driver) + " ORDER BY " + strings.Join(order, ",") + " LIMIT " + b.bind(int64(r.MaxRows+1))
	after, err := replaceRead(ctx, s, q, b.args, r.MaxRows+1)
	if err != nil {
		return reject("database_error")
	}
	if after.Truncated || len(after.Rows) != len(expected) {
		return reject("row_conflict")
	}
	for i, row := range after.Rows {
		for j, cell := range row {
			if d.Columns[j].Generated == "none" && cell != expected[i][j] {
				return reject("row_conflict")
			}
		}
	}
	if err = s.commit(ctx); err != nil {
		return core.ReplaceResult{Outcome: dbplan.StatusOutcomeUnknown, Reason: "commit_unknown"}, nil
	}
	return core.ReplaceResult{Outcome: dbplan.StatusCompleted, AffectedRows: state.plan.AffectedRows}, nil
}
