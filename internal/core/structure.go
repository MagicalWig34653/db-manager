// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const StructureContractVersion = "0.5.0"

type IndexTerm struct {
	Position       int       `json:"position"`
	Kind           string    `json:"kind"`
	Column         *string   `json:"column,omitempty"`
	Role           string    `json:"role"`
	Direction      string    `json:"direction"`
	CollationState string    `json:"collation_state"`
	Collation      *TableRef `json:"collation,omitempty"`
	PrefixState    string    `json:"prefix_state"`
	PrefixUnit     string    `json:"prefix_unit"`
	PrefixLength   *int      `json:"prefix_length,omitempty"`
}
type StructureIndex struct {
	Name          string      `json:"name"`
	Unique        string      `json:"unique"`
	Primary       string      `json:"primary"`
	Method        string      `json:"method"`
	MethodKnown   bool        `json:"method_known"`
	Partial       string      `json:"partial"`
	Valid         string      `json:"valid"`
	NullsDistinct string      `json:"nulls_distinct"`
	Terms         []IndexTerm `json:"terms"`
}
type StructureConstraint struct {
	Name                   string    `json:"name"`
	NameKnown              bool      `json:"name_known"`
	Kind                   string    `json:"kind"`
	Columns                []string  `json:"columns"`
	ColumnsKnown           bool      `json:"columns_known"`
	ReferencedTable        *TableRef `json:"referenced_table,omitempty"`
	ReferencedColumns      []string  `json:"referenced_columns"`
	ReferencedColumnsKnown bool      `json:"referenced_columns_known"`
	OnUpdate               string    `json:"on_update"`
	OnDelete               string    `json:"on_delete"`
	Deferrable             string    `json:"deferrable"`
	InitiallyDeferred      string    `json:"initially_deferred"`
	Enforced               string    `json:"enforced"`
	Validated              string    `json:"validated"`
}
type StructureCoverage struct {
	Indexes     string `json:"indexes"`
	Constraints string `json:"constraints"`
	Checks      string `json:"checks"`
}
type TableStructure struct {
	Table       TableRef              `json:"table"`
	Indexes     []StructureIndex      `json:"indexes"`
	Constraints []StructureConstraint `json:"constraints"`
	Coverage    StructureCoverage     `json:"coverage"`
}
type StructureEngine interface {
	Structure(context.Context, DescribeRequest) (TableStructure, error)
}
