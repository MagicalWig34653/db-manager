// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const PartitionsContractVersion = "0.21.0"
const PrefixMaxTables = 100

// PartitionDefinition accepts typed bounds, never SQL expressions. Values are
// signed decimal integers, quoted text, or the explicit range sentinels.
type PartitionDefinition struct {
	Name      string   `json:"name"`
	LessThan  *string  `json:"less_than,omitempty"`
	From      *string  `json:"from,omitempty"`
	To        *string  `json:"to,omitempty"`
	Values    []string `json:"values,omitempty"`
	Modulus   int      `json:"modulus,omitempty"`
	Remainder int      `json:"remainder,omitempty"`
}
type PartitionRequest struct {
	Connection  Connection            `json:"connection"`
	Table       TableRef              `json:"table"`
	Action      string                `json:"action,omitempty"`
	Method      string                `json:"method,omitempty"`
	Column      string                `json:"column,omitempty"`
	Partitions  []string              `json:"partitions,omitempty"`
	Definitions []PartitionDefinition `json:"definitions,omitempty"`
	Count       int                   `json:"count,omitempty"`
	TimeoutMS   int                   `json:"timeout_ms,omitempty"`
}
type PartitionInfo struct {
	Table         TableRef `json:"table"`
	Bound         string   `json:"bound"`
	EstimatedRows *int64   `json:"estimated_rows"`
}
type PartitionsResult struct {
	Driver     string          `json:"driver"`
	Table      TableRef        `json:"table"`
	Method     string          `json:"method"`
	Expression string          `json:"expression"`
	Partitions []PartitionInfo `json:"partitions"`
	Notes      []string        `json:"notes"`
}
type StructurePlan struct {
	dbplan.Plan
	Action       string               `json:"action"`
	Confirmation ObjectOpConfirmation `json:"confirmation"`
}
type StructureExecuteRequest struct {
	PartitionRequest
	Confirm            dbplan.Confirmation `json:"confirm"`
	ExpectedPlanSHA256 string              `json:"expected_plan_sha256"`
}
type PrefixRequest struct {
	Connection Connection `json:"connection"`
	Schema     string     `json:"schema,omitempty"`
	Prefix     string     `json:"prefix"`
	Action     string     `json:"action,omitempty"`
	NewPrefix  string     `json:"new_prefix,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}
type PrefixResult struct {
	Tables []TableRef `json:"tables"`
}
type PrefixExecuteRequest struct {
	PrefixRequest
	Confirm            dbplan.Confirmation `json:"confirm"`
	ExpectedPlanSHA256 string              `json:"expected_plan_sha256"`
}
type PartitionsEngine interface {
	Partitions(context.Context, PartitionRequest) (PartitionsResult, error)
	PlanPartitions(context.Context, PartitionRequest) (StructurePlan, error)
	ExecutePartitions(context.Context, StructureExecuteRequest) (ObjectOpResult, error)
	PrefixTables(context.Context, PrefixRequest) (PrefixResult, error)
	PlanPrefix(context.Context, PrefixRequest) (StructurePlan, error)
	ExecutePrefix(context.Context, PrefixExecuteRequest) (ObjectOpResult, error)
}

func PartitionsCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	actions := []string{"inspect", "define", "add", "drop", "reorganize", "coalesce", "remove", "analyze", "check", "optimize", "rebuild", "repair", "truncate", "attach", "detach"}
	for _, a := range actions {
		c.Set("sqlite", a, dbplan.Capability{Support: dbplan.Unsupported, Reason: "SQLite unterstützt keine Partitionierung."})
		my := dbplan.Capability{Support: dbplan.Limited, Reason: "Server und Speicherengine entscheiden über Unterstützung; DDL kann sperren und implizit committen. Strukturierte Definitionen verwenden eine Katalogspalte, keine freien Ausdrücke; keine Subpartitionen."}
		if a == "attach" || a == "detach" {
			my = dbplan.Capability{Support: dbplan.Unsupported, Reason: "ATTACH/DETACH ist hier nur für deklarative PostgreSQL-Partitionen verfügbar."}
		}
		c.Set("mysql", a, my)
		pg := dbplan.Capability{Support: dbplan.Unsupported, Reason: "Kein direktes Gegenstück für bestehende deklarative PostgreSQL-Tabellen; kein automatischer Tabellenumbau."}
		switch a {
		case "inspect", "add", "attach", "detach", "drop", "truncate", "analyze":
			pg = dbplan.Capability{Support: dbplan.Limited, Reason: "Nur deklarative Partitionierung; ADD erzeugt CREATE TABLE PARTITION OF. ANALYZE pro Kind; kein freies SQL in Grenzen."}
		}
		c.Set("postgres", a, pg)
	}
	return c
}
