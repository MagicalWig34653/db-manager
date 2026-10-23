// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

const MaxStructureObjects = 1024
const MaxStructureTerms = 8192

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
func emptyConstraint(kind string) core.StructureConstraint {
	return core.StructureConstraint{Kind: kind, Columns: []string{}, ReferencedColumns: []string{}, OnUpdate: "not_applicable", OnDelete: "not_applicable", Deferrable: "unknown", InitiallyDeferred: "unknown", Enforced: "unknown", Validated: "unknown"}
}
func emptyTerm(position int) core.IndexTerm {
	return core.IndexTerm{Position: position, Kind: "unknown", Role: "unknown", Direction: "unknown", CollationState: "unknown", PrefixState: "unknown"}
}
func (e *Engine) Structure(ctx context.Context, r core.DescribeRequest) (core.TableStructure, error) {
	empty := core.TableStructure{}
	if !validTableRef(r.Table) {
		return empty, public("invalid_table", "Tabellenbezug ist ungültig.")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer cleanup()
	d, err := describe(ctx, db, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if d.Kind != "table" {
		return empty, public("unsupported_table", "Strukturmetadaten sind nur für Basistabellen verfügbar.")
	}
	result := core.TableStructure{Table: d.Table, Indexes: []core.StructureIndex{}, Constraints: []core.StructureConstraint{}, Coverage: core.StructureCoverage{Indexes: "catalog", Constraints: "catalog", Checks: "catalog"}}
	switch r.Connection.Driver {
	case "sqlite":
		err = sqliteStructure(ctx, db, d, &result)
	case "mysql":
		err = mysqlStructure(ctx, db, d, &result)
	case "postgres":
		err = postgresStructure(ctx, db, d, &result)
	}
	if err != nil {
		return empty, safe(ctx, err)
	}
	sort.SliceStable(result.Indexes, func(i, j int) bool { return result.Indexes[i].Name < result.Indexes[j].Name })
	if err = structureBudget(result); err != nil {
		return empty, err
	}
	return result, nil
}
func structureBudget(result core.TableStructure) error {
	if len(result.Indexes) > MaxStructureObjects || len(result.Constraints) > MaxStructureObjects {
		return public("result_too_large", "Zu viele Strukturmetadaten.")
	}
	terms := 0
	checkString := func(value string) bool { return len(value) <= MaxCellBytes }
	for _, index := range result.Indexes {
		terms += len(index.Terms)
		if terms > MaxStructureTerms || len(index.Terms) > MaxStructureObjects || !checkString(index.Name) || !checkString(index.Method) {
			return public("result_too_large", "Indexmetadaten sind zu groß.")
		}
		for _, term := range index.Terms {
			if term.Column != nil && !checkString(*term.Column) {
				return public("result_too_large", "Indexmetadaten sind zu groß.")
			}
			if term.Collation != nil && (!checkString(term.Collation.Name) || !checkString(term.Collation.Schema)) {
				return public("result_too_large", "Indexmetadaten sind zu groß.")
			}
		}
	}
	for _, constraint := range result.Constraints {
		terms += len(constraint.Columns) + len(constraint.ReferencedColumns)
		if terms > MaxStructureTerms {
			return public("result_too_large", "Zu viele Strukturterme.")
		}
		if !checkString(constraint.Name) || len(constraint.Columns) > MaxStructureObjects || len(constraint.ReferencedColumns) > MaxStructureObjects {
			return public("result_too_large", "Constraintmetadaten sind zu groß.")
		}
		for _, column := range append(append([]string{}, constraint.Columns...), constraint.ReferencedColumns...) {
			if !checkString(column) {
				return public("result_too_large", "Constraintmetadaten sind zu groß.")
			}
		}
		if constraint.ReferencedTable != nil && (!checkString(constraint.ReferencedTable.Name) || !checkString(constraint.ReferencedTable.Schema)) {
			return public("result_too_large", "Constraintmetadaten sind zu groß.")
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return public("result_too_large", "Strukturantwort ist zu groß.")
	}
	return nil
}
func fkAction(value string) string {
	switch value {
	case "a", "NO ACTION":
		return "no_action"
	case "r", "RESTRICT":
		return "restrict"
	case "c", "CASCADE":
		return "cascade"
	case "n", "SET NULL":
		return "set_null"
	case "d", "SET DEFAULT":
		return "set_default"
	}
	return "unknown"
}
