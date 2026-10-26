// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func invalidIndexes() error {
	return public("invalid_indexes", "Die strukturierte Schlüsseländerung ist ungültig.")
}
func unsupportedIndexes() error {
	return public("unsupported_action", "Diese Schlüsseländerung wird für diese Tabelle oder diesen Treiber nicht unterstützt.")
}

func validateIndexes(r core.IndexesRequest) error {
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > 1<<20 || !validTableRef(r.Table) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return invalidIndexes()
	}
	switch r.Action {
	case "index_create":
		if r.Name != "" || r.Index == nil || r.ForeignKey != nil || r.NewName != "" {
			return invalidIndexes()
		}
	case "index_alter":
		if r.Name == "" || r.Index == nil || r.ForeignKey != nil || r.NewName != "" {
			return invalidIndexes()
		}
	case "index_drop", "foreign_key_drop":
		if r.Name == "" || r.Index != nil || r.ForeignKey != nil || r.NewName != "" {
			return invalidIndexes()
		}
	case "index_rename":
		if r.Name == "" || !columnName(r.NewName) || r.Index != nil || r.ForeignKey != nil {
			return invalidIndexes()
		}
	case "primary_set":
		if r.Name != "" || r.Index == nil || r.Index.Kind != "primary" || r.ForeignKey != nil || r.NewName != "" {
			return invalidIndexes()
		}
	case "primary_drop":
		if r.Name != "" || r.Index != nil || r.ForeignKey != nil || r.NewName != "" {
			return invalidIndexes()
		}
	case "foreign_key_create", "foreign_key_alter":
		if r.ForeignKey == nil || r.Index != nil || r.NewName != "" || (r.Action == "foreign_key_create" && r.Name != "") || (r.Action == "foreign_key_alter" && r.Name == "") {
			return invalidIndexes()
		}
	default:
		return invalidIndexes()
	}
	if !core.IndexesCapabilities().Allows(r.Connection.Driver, r.Action) {
		return unsupportedIndexes()
	}
	if d := r.Index; d != nil {
		if !columnName(d.Name) || len(d.Terms) < 1 || len(d.Terms) > 16 {
			return invalidIndexes()
		}
		switch d.Kind {
		case "primary":
			if r.Action != "primary_set" {
				return invalidIndexes()
			}
		case "index", "unique":
		case "fulltext", "spatial":
			if r.Connection.Driver != "mysql" {
				return unsupportedIndexes()
			}
		default:
			return invalidIndexes()
		}
		for _, term := range d.Terms {
			if term.Column == "" || term.Direction != "" && term.Direction != "asc" && term.Direction != "desc" || term.PrefixLength < 0 || term.PrefixLength > 3072 {
				return invalidIndexes()
			}
			if term.PrefixLength > 0 && (r.Connection.Driver != "mysql" || d.Kind == "primary" || d.Kind == "fulltext" || d.Kind == "spatial") {
				return unsupportedIndexes()
			}
			if term.Expression != "" && (r.Connection.Driver != "postgres" || term.Expression != "lower" && term.Expression != "upper" || d.Kind == "primary") {
				return unsupportedIndexes()
			}
			if (d.Kind == "fulltext" || d.Kind == "spatial" || d.Kind == "primary") && term.Direction != "" {
				return unsupportedIndexes()
			}
		}
		if d.Predicate != nil && (r.Connection.Driver != "postgres" || d.Kind == "primary" || d.Predicate.Column == "" || d.Predicate.Operator != "is_null" && d.Predicate.Operator != "is_not_null") {
			return unsupportedIndexes()
		}
	}
	if f := r.ForeignKey; f != nil {
		if !columnName(f.Name) || !validTableRef(f.ReferencedTable) || len(f.Columns) < 1 || len(f.Columns) > 16 || len(f.Columns) != len(f.ReferencedColumns) {
			return invalidIndexes()
		}
		for _, a := range []string{f.OnDelete, f.OnUpdate} {
			switch a {
			case "no_action", "restrict", "cascade", "set_null":
			case "set_default":
				if r.Connection.Driver != "postgres" {
					return unsupportedIndexes()
				}
			default:
				return invalidIndexes()
			}
		}
	}
	return nil
}
func (e *Engine) PlanIndexes(ctx context.Context, r core.IndexesRequest) (core.IndexesPlan, error) {
	if err := validateIndexes(r); err != nil {
		return core.IndexesPlan{}, err
	}
	ctx, cancel := columnsContext(ctx, core.ColumnsRequest{TimeoutMS: r.TimeoutMS})
	defer cancel()
	s, closeAll, err := e.columnsSession(ctx, core.ColumnsRequest{Connection: r.Connection})
	if err != nil {
		return core.IndexesPlan{}, err
	}
	defer closeAll()
	p, err := buildIndexes(ctx, s, r)
	return p, safe(ctx, err)
}
func (e *Engine) ExecuteIndexes(ctx context.Context, r core.IndexesRequest) (core.IndexesResult, error) {
	empty := core.IndexesResult{}
	if err := validateIndexes(r); err != nil {
		return empty, err
	}
	if r.Confirm == nil || r.ExpectedPlanSHA256 == "" {
		return empty, invalidIndexes()
	}
	ctx, cancel := columnsContext(ctx, core.ColumnsRequest{TimeoutMS: r.TimeoutMS})
	defer cancel()
	s, closeAll, err := e.columnsSession(ctx, core.ColumnsRequest{Connection: r.Connection})
	if err != nil {
		return empty, err
	}
	defer closeAll()
	if s.driver != "mysql" {
		if err = s.begin(ctx); err != nil {
			return empty, safe(ctx, err)
		}
		defer s.rollback()
	}
	p, err := buildIndexes(ctx, s, r)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if err = dbplan.Check(p.Plan, r.ExpectedPlanSHA256, *r.Confirm, r.Action, p.Table.Name, p.Destructive); err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) || errors.Is(err, dbplan.ErrInvalidDigest) {
			return empty, public("plan_mismatch", "Der Plan stimmt nicht mit der bestätigten Vorschau überein.")
		}
		return empty, public("confirmation_mismatch", "Aktion oder getippter Tabellenname stimmt nicht überein.")
	}
	if p.DuplicateGroups != nil && *p.DuplicateGroups > 0 || p.ContainsNull {
		return core.IndexesResult{Outcome: dbplan.StatusRejected, Reason: "invalid_value", Message: "Vorhandene Duplikate oder NULL-Werte verhindern den Schlüssel.", Observation: "unavailable"}, nil
	}
	// Reuse the reviewed DDL runner: MySQL is one combined ALTER, other drivers
	// use a transaction with catalog observation before commit.
	result := runColumns(ctx, core.ColumnsRequest{Connection: r.Connection}, core.ColumnsPlan{Plan: p.Plan, Table: p.Table}, columnsRunner{s})
	if result.Outcome == dbplan.StatusCompleted {
		result.Message = "Schlüsseländerung ausgeführt; Strukturbeobachtung ist separat ausgewiesen."
	}
	return result, nil
}
