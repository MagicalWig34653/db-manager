// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const IndexesContractVersion = "0.13.0"

type IndexDefinitionTerm struct {
	Column       string `json:"column"`
	Direction    string `json:"direction,omitempty"`
	PrefixLength int    `json:"prefix_length,omitempty"`
	Expression   string `json:"expression,omitempty"`
}

// IndexPredicate is a structured NULL test, never a SQL fragment.
type IndexPredicate struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
}
type IndexDefinition struct {
	Name      string                `json:"name"`
	Kind      string                `json:"kind"`
	Terms     []IndexDefinitionTerm `json:"terms"`
	Predicate *IndexPredicate       `json:"predicate,omitempty"`
}
type ForeignKeyDefinition struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedTable   TableRef `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
	OnDelete          string   `json:"on_delete"`
	OnUpdate          string   `json:"on_update"`
}
type IndexesRequest struct {
	Connection         Connection            `json:"connection"`
	Table              TableRef              `json:"table"`
	Action             string                `json:"action"`
	Name               string                `json:"name,omitempty"`
	NewName            string                `json:"new_name,omitempty"`
	Index              *IndexDefinition      `json:"index,omitempty"`
	ForeignKey         *ForeignKeyDefinition `json:"foreign_key,omitempty"`
	TimeoutMS          int                   `json:"timeout_ms,omitempty"`
	Confirm            *dbplan.Confirmation  `json:"confirm,omitempty"`
	ExpectedPlanSHA256 string                `json:"expected_plan_sha256,omitempty"`
}
type IndexesPlan struct {
	dbplan.Plan
	Table            TableRef           `json:"table"`
	Destructive      bool               `json:"destructive"`
	TargetName       string             `json:"target_name"`
	Dependencies     []ObjectDependency `json:"dependencies"`
	DDLTransactional bool               `json:"ddl_transactional"`
	DuplicateGroups  *int64             `json:"duplicate_groups,omitempty"`
	ContainsNull     bool               `json:"contains_null"`
}

// IndexesResult shares the established execution/observation semantics.
type IndexesResult = ColumnsResult

type IndexesEngine interface {
	PlanIndexes(context.Context, IndexesRequest) (IndexesPlan, error)
	ExecuteIndexes(context.Context, IndexesRequest) (IndexesResult, error)
}

func IndexesCapabilities() dbplan.Capabilities {
	result := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		actions := map[string]dbplan.Capability{}
		for _, action := range []string{"index_create", "index_alter", "index_drop", "index_rename", "primary_set", "primary_drop", "foreign_key_create", "foreign_key_alter", "foreign_key_drop", "fulltext", "spatial", "prefix", "direction", "expression", "partial"} {
			cap := dbplan.Capability{Support: "limited", Reason: "DB-Rechte gelten; Sperren und lange Laufzeiten möglich. Kein CASCADE, kein automatischer Retry."}
			if driver == "sqlite" && action != "index_create" && action != "index_drop" && action != "direction" || driver != "mysql" && (action == "fulltext" || action == "spatial" || action == "prefix") || driver != "postgres" && (action == "expression" || action == "partial") {
				cap = dbplan.Capability{Support: "unsupported", Reason: "Für diesen Treiber nicht unterstützt; SQLite ohne Tabellenneuaufbau nur CREATE/DROP INDEX."}
			}
			if action == "expression" && driver == "postgres" {
				cap.Reason = "Nur pg_catalog.lower/upper für Textspalten; kein freies SQL."
			}
			if action == "partial" && driver == "postgres" {
				cap.Reason = "Nur strukturierte IS NULL/IS NOT NULL-Bedingungen."
			}
			actions[action] = cap
		}
		result[driver] = actions
	}
	return result
}
