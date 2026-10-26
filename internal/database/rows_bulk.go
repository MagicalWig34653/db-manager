// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func bulkInvalid() error { return public("invalid_bulk", "Auswahl, Aktion oder Werte sind ungültig.") }
func bulkDigest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(append([]byte("dbmanager/rows-bulk/v19\x00"), b...))
	return hex.EncodeToString(h[:])
}
func bulkValidate(r core.RowsBulkRequest) error { return bulkValidateLimit(r, 1<<20) }

// bulkValidateLimit bounds the canonical request size. Single file replacements carry the
// old snapshot plus the new value (both base64), so they get a budget that covers a full
// MaxFileValueBytes value on top of the standard 1 MiB.
func bulkValidateLimit(r core.RowsBulkRequest, limit int) error {
	if !validTableRef(r.Table) || len(r.Rows) < 1 || len(r.Rows) > core.MaxBulkRows || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return bulkInvalid()
	}
	if r.Action != "update" && r.Action != "delete" && r.Action != "copy" {
		return bulkInvalid()
	}
	if (r.Action == "update") != (len(r.Changes) > 0) || len(r.Changes) > 1024 {
		return bulkInvalid()
	}
	for _, row := range r.Rows {
		if len(row.Expected.Columns) == 0 || len(row.Expected.Columns) > 1024 || len(row.Expected.Columns) != len(row.Expected.Values) || (r.Action != "copy" && len(row.CopyValues) > 0) {
			return bulkInvalid()
		}
		for _, v := range row.Expected.Values {
			if _, err := mutationValue(v); err != nil {
				return err
			}
		}
		for _, v := range row.CopyValues {
			if _, err := mutationValue(v.Value); err != nil {
				return err
			}
		}
	}
	for _, v := range r.Changes {
		if _, err := mutationValue(v.Value); err != nil {
			return err
		}
	}
	b, err := json.Marshal(r)
	if err != nil || len(b) > limit {
		return bulkInvalid()
	}
	return nil
}
func (e *Engine) bulkSession(ctx context.Context, r core.RowsBulkRequest, execute bool) (core.RowsBulkPlan, core.RowsBulkResult, error) {
	var plan core.RowsBulkPlan
	result := core.RowsBulkResult{Outcome: "rejected", Rows: []core.BulkRowResult{}}
	if err := bulkValidate(r); err != nil {
		return plan, result, err
	}
	if execute && (!r.Confirm || len(r.ExpectedPlanSHA256) != 64) {
		return plan, result, bulkInvalid()
	}
	timeout := r.TimeoutMS
	if timeout == 0 {
		timeout = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return plan, result, err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return plan, result, safe(ctx, err)
	}
	defer conn.Close()
	s := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = s.begin(ctx); err != nil {
		return plan, result, safe(ctx, err)
	}
	defer s.rollback()
	d, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return plan, result, safe(ctx, err)
	}
	ip, err := insertPlan(ctx, s, r.Connection, d)
	if err != nil {
		return plan, result, safe(ctx, err)
	}
	if !ip.Eligible {
		return plan, result, public("mutation_not_supported", "Tabelle ist für diese Auswahloperation nicht freigegeben.")
	}
	plan = core.RowsBulkPlan{Table: d.Table, Action: r.Action, Rows: []core.BulkPlannedRow{}, RowCount: len(r.Rows), Warnings: []string{"Trigger und Kaskaden können weitere Wirkungen haben. ABA wird nicht erkannt; nach unbekanntem Commit nicht automatisch wiederholen."}}
	seen := map[string]bool{}
	for _, row := range r.Rows {
		current, key, err := bulkRead(ctx, s, d, row.Expected)
		if err != nil {
			return plan, result, err
		}
		if !reflect.DeepEqual(current, row.Expected) {
			return plan, result, rowConflict()
		}
		fingerprint := bulkDigest(struct {
			Description core.TableDescription
			Row         core.RowSnapshot
		}{d, current})
		if execute && row.StabilitySHA256 != fingerprint {
			return plan, result, rowConflict()
		}
		identity := bulkDigest(key)
		if seen[identity] {
			return plan, result, bulkInvalid()
		}
		seen[identity] = true
		plan.Rows = append(plan.Rows, core.BulkPlannedRow{Key: key, StabilitySHA256: fingerprint})
		if r.Action == "copy" {
			if _, err := bulkCopyValues(d, ip, row); err != nil {
				return plan, result, err
			}
		}
	}
	// Re-read metadata after all locks, including the PostgreSQL rewrite-rule guard.
	fresh, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return plan, result, safe(ctx, err)
	}
	if !reflect.DeepEqual(d, fresh) {
		return plan, result, rowConflict()
	}
	ip, err = insertPlan(ctx, s, r.Connection, fresh)
	if err != nil {
		return plan, result, safe(ctx, err)
	}
	if !ip.Eligible {
		return plan, result, rowConflict()
	}
	allowed := map[string]bool{}
	for _, c := range eligibility(d).WritableColumns {
		allowed[c] = true
	}
	used := map[string]bool{}
	for _, c := range r.Changes {
		if !allowed[c.Column] || used[c.Column] {
			return plan, result, bulkInvalid()
		}
		used[c.Column] = true
	}
	// Credentials never appear in the plan. The digest binds the destination and every value.
	canonical := r
	canonical.Confirm = false
	canonical.ExpectedPlanSHA256 = ""
	canonical.Connection.Password = ""
	canonical.Rows = append([]core.BulkRow{}, r.Rows...)
	for i := range canonical.Rows {
		canonical.Rows[i].StabilitySHA256 = ""
	}
	plan.PlanSHA256 = bulkDigest(struct {
		Request     core.RowsBulkRequest
		Description core.TableDescription
		Rows        []core.BulkPlannedRow
	}{canonical, d, plan.Rows})
	if !execute {
		e.plans.arm("bulk:" + plan.PlanSHA256)
		return plan, result, nil
	}
	if r.ExpectedPlanSHA256 != plan.PlanSHA256 {
		return plan, result, public("plan_conflict", "Plan wurde geändert. Neue Vorschau erforderlich.")
	}
	// Single use: consumed before the first write, also when the outcome stays unknown.
	if !e.plans.consume("bulk:" + plan.PlanSHA256) {
		return plan, result, public("plan_conflict", "Plan wurde bereits ausgeführt. Neue Vorschau erforderlich.")
	}
	for i, row := range r.Rows {
		entry := core.BulkRowResult{Index: i, Outcome: "rejected"}
		if r.Action == "copy" {
			values, err := bulkCopyValues(d, ip, row)
			if err != nil {
				return plan, result, err
			}
			inserted, err := rowsBulkInsert(ctx, s, core.InsertRequest{Connection: r.Connection, Table: d.Table, Values: values, Confirm: true})
			if err != nil {
				return plan, result, err
			}
			entry.Row = &inserted.Row
		} else {
			changed, err := rowsBulkMutate(ctx, s, core.MutationRequest{Connection: r.Connection, Table: d.Table, Expected: row.Expected, Changes: r.Changes, Confirm: true}, r.Action == "delete")
			if err != nil {
				return plan, result, err
			}
			entry.Row = changed.Row
		}
		result.Rows = append(result.Rows, entry)
	}
	// Later triggers must not silently invalidate an earlier returned row.
	for i := range result.Rows {
		if result.Rows[i].Row != nil {
			after, _, err := bulkRead(ctx, s, d, *result.Rows[i].Row)
			if err != nil {
				return plan, result, err
			}
			if !reflect.DeepEqual(after, *result.Rows[i].Row) {
				return plan, result, rowConflict()
			}
		}
		result.Rows[i].Outcome = "completed"
	}
	// Bound the final response before attempting COMMIT.
	result.Outcome = "completed"
	b, err := json.Marshal(result)
	if err != nil || len(b)+1 > MaxResponseBytes {
		return plan, result, public("result_too_large", "Auswahlergebnis ist zu groß.")
	}
	if err = s.commit(ctx); err != nil {
		result.Outcome = "outcome_unknown"
		result.Reason = "outcome_unknown"
		for i := range result.Rows {
			result.Rows[i].Outcome = "outcome_unknown"
			result.Rows[i].Row = nil
		}
		return plan, result, nil
	}
	result.Outcome = "completed"
	for i := range result.Rows {
		result.Rows[i].Outcome = "completed"
	}
	return plan, result, nil
}
func (e *Engine) RowsBulkPlan(ctx context.Context, r core.RowsBulkRequest) (core.RowsBulkPlan, error) {
	p, _, err := e.bulkSession(ctx, r, false)
	return p, err
}
func (e *Engine) RowsBulkExecute(ctx context.Context, r core.RowsBulkRequest) (core.RowsBulkResult, error) {
	_, out, err := e.bulkSession(ctx, r, true)
	if err != nil {
		out = core.RowsBulkResult{Outcome: "rejected", Reason: "operation_rejected", Rows: []core.BulkRowResult{}}
		var p *core.Error
		if errors.As(err, &p) {
			out.Reason = p.Code
		}
		for i := range r.Rows {
			if len(r.Rows) > core.MaxBulkRows {
				break
			}
			out.Rows = append(out.Rows, core.BulkRowResult{Index: i, Outcome: "rejected"})
		}
		return out, err
	}
	return out, nil
}
func bulkRead(ctx context.Context, s *mutationSession, d core.TableDescription, expected core.RowSnapshot) (core.RowSnapshot, []core.RowChange, error) {
	if len(expected.Columns) != len(d.Columns) || len(expected.Values) != len(d.Columns) {
		return core.RowSnapshot{}, nil, rowConflict()
	}
	names := []string{}
	positions := map[string]int{}
	for i, c := range d.Columns {
		if c.Name != expected.Columns[i] {
			return core.RowSnapshot{}, nil, rowConflict()
		}
		q, err := dbplan.QuoteIdentifier(s.driver, c.Name)
		if err != nil {
			return core.RowSnapshot{}, nil, bulkInvalid()
		}
		names = append(names, q)
		positions[c.Name] = i
	}
	table, err := dbplan.QualifiedName(s.driver, d.Table.Schema, d.Table.Name)
	if err != nil {
		return core.RowSnapshot{}, nil, bulkInvalid()
	}
	if s.driver == "postgres" {
		table = "ONLY " + table
	}
	args := []any{}
	where := []string{}
	key := []core.RowChange{}
	for _, name := range d.PrimaryKey {
		cell := expected.Values[positions[name]]
		if cell.Kind == "null" {
			return core.RowSnapshot{}, nil, bulkInvalid()
		}
		v, err := mutationValue(cell)
		if err != nil {
			return core.RowSnapshot{}, nil, err
		}
		args = append(args, v)
		q, err := dbplan.QuoteIdentifier(s.driver, name)
		if err != nil {
			return core.RowSnapshot{}, nil, bulkInvalid()
		}
		if s.driver == "sqlite" {
			q += " COLLATE BINARY"
		}
		marker := "?"
		if s.driver == "postgres" {
			marker = "$" + strconv.Itoa(len(args))
		}
		where = append(where, q+" = "+marker)
		key = append(key, core.RowChange{Column: name, Value: cell})
	}
	if len(where) == 0 {
		return core.RowSnapshot{}, nil, bulkInvalid()
	}
	query := "SELECT " + strings.Join(names, ",") + " FROM " + table + " WHERE " + strings.Join(where, " AND ")
	if s.driver != "sqlite" {
		query += " FOR UPDATE"
	}
	rows, err := s.read(ctx, query, args)
	if err != nil {
		return core.RowSnapshot{}, nil, safe(ctx, err)
	}
	if rows.Truncated || len(rows.Rows) != 1 {
		return core.RowSnapshot{}, nil, rowConflict()
	}
	return core.RowSnapshot{Columns: expected.Columns, Values: rows.Rows[0]}, key, nil
}
func bulkCopyValues(d core.TableDescription, ip core.InsertEligibility, row core.BulkRow) ([]core.RowChange, error) {
	overrides := map[string]core.Cell{}
	allowed := map[string]bool{}
	for _, c := range ip.InsertableColumns {
		allowed[c] = true
	}
	for _, v := range row.CopyValues {
		if _, ok := overrides[v.Column]; ok || !allowed[v.Column] {
			return nil, bulkInvalid()
		}
		overrides[v.Column] = v.Value
	}
	for _, pk := range d.PrimaryKey {
		if pk != ip.AutoPK {
			v, ok := overrides[pk]
			if !ok || v.Kind == "null" {
				return nil, public("invalid_bulk", "Kopieren benötigt für jede Zeile einen neuen expliziten Schlüssel oder einen unterstützten Auto-Schlüssel.")
			}
		}
	}
	values := []core.RowChange{}
	for i, c := range d.Columns {
		if v, ok := overrides[c.Name]; ok {
			values = append(values, core.RowChange{Column: c.Name, Value: v})
			continue
		}
		if allowed[c.Name] && c.Name != ip.AutoPK {
			values = append(values, core.RowChange{Column: c.Name, Value: row.Expected.Values[i]})
		}
	}
	return values, nil
}
