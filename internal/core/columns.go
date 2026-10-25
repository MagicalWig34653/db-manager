// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const ColumnsContractVersion = "0.9.0"

type ColumnDefinition struct {
	Name      string            `json:"name"`
	Type      dbplan.ColumnType `json:"type"`
	Nullable  *bool             `json:"nullable"`
	Default   *CreateDefault    `json:"default"`
	Comment   *string           `json:"comment,omitempty"`
	Collation *string           `json:"collation,omitempty"`
}
type ColumnPosition struct {
	Mode   string `json:"mode"`
	Column string `json:"column,omitempty"`
}
type ColumnChange struct {
	Action     string            `json:"action"`
	Column     string            `json:"column,omitempty"`
	Definition *ColumnDefinition `json:"definition,omitempty"`
	NewName    string            `json:"new_name,omitempty"`
	Position   *ColumnPosition   `json:"position,omitempty"`
}
type ColumnsRequest struct {
	Connection         Connection           `json:"connection"`
	Table              TableRef             `json:"table"`
	Changes            []ColumnChange       `json:"changes"`
	TimeoutMS          int                  `json:"timeout_ms,omitempty"`
	Confirm            *dbplan.Confirmation `json:"confirm,omitempty"`
	ExpectedPlanSHA256 string               `json:"expected_plan_sha256,omitempty"`
}
type ColumnsPlan struct {
	dbplan.Plan
	Table            TableRef `json:"table"`
	Destructive      bool     `json:"destructive"`
	TargetName       string   `json:"target_name"`
	Dependencies     []string `json:"dependencies"`
	DDLTransactional bool     `json:"ddl_transactional"`
}
type ColumnsResult struct {
	Outcome            dbplan.Status     `json:"outcome"`
	Reason             string            `json:"reason"`
	Message            string            `json:"message"`
	StatementsExecuted int               `json:"statements_executed"`
	Atomic             bool              `json:"atomic"`
	Observation        string            `json:"observation"`
	Description        *TableDescription `json:"description,omitempty"`
}
type ColumnsEngine interface {
	PlanColumns(context.Context, ColumnsRequest) (ColumnsPlan, error)
	ExecuteColumns(context.Context, ColumnsRequest) (ColumnsResult, error)
}

func ColumnsCapabilities() dbplan.Capabilities {
	result := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		actions := map[string]dbplan.Capability{}
		for _, action := range []string{"add", "change", "rename", "drop", "reorder", "generated"} {
			cap := dbplan.Capability{Support: "limited", Reason: "Feste Typen; Sperren und Datenverlust möglich. Tatsächliche DB-Rechte gelten."}
			if action == "generated" {
				cap = dbplan.Capability{Support: "unsupported", Reason: "Generierte Ausdrücke sind nach E5 nicht freigegeben."}
			}
			if driver == "sqlite" && (action == "change" || action == "reorder") || driver == "postgres" && action == "reorder" {
				cap = dbplan.Capability{Support: "unsupported", Reason: "Kein Tabellenneuaufbau; diese ALTER-Operation ist nicht verfügbar."}
			}
			actions[action] = cap
		}
		result[driver] = actions
	}
	return result
}

// UnmarshalJSON enforces action-specific fields, including explicit empty fields.
func (ch *ColumnChange) UnmarshalJSON(data []byte) error {
	type plain ColumnChange
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	required := []string{"action"}
	optional := []string{}
	switch value.Action {
	case "add":
		required = append(required, "definition")
		optional = append(optional, "position")
	case "change":
		required = append(required, "column", "definition")
		optional = append(optional, "position")
	case "reorder":
		required = append(required, "column", "definition", "position")
	case "rename":
		required = append(required, "column", "new_name")
	case "drop":
		required = append(required, "column")
	default:
		// The engine supplies the unsupported capability response for known future actions.
		if value.Action == "generated" {
			*ch = ColumnChange(value)
			return nil
		}
		return errors.New("core: unknown column action")
	}
	allowed := map[string]bool{}
	for _, key := range required {
		allowed[key] = true
		if _, ok := fields[key]; !ok {
			return errors.New("core: missing column action field")
		}
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range fields {
		if !allowed[key] {
			return errors.New("core: irrelevant column action field")
		}
	}
	*ch = ColumnChange(value)
	return nil
}

// UnmarshalJSON keeps empty text constants distinct from a missing value field.
func (d *ColumnDefinition) UnmarshalJSON(data []byte) error {
	type plain ColumnDefinition
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range []string{"name", "type", "nullable", "default"} {
		if _, ok := fields[key]; !ok {
			return errors.New("core: missing column definition field")
		}
	}
	var def map[string]json.RawMessage
	if err := json.Unmarshal(fields["default"], &def); err != nil {
		return err
	}
	if _, ok := def["mode"]; !ok {
		return errors.New("core: missing default mode")
	}
	if value.Default != nil && value.Default.Mode == "value" {
		var cell map[string]json.RawMessage
		if err := json.Unmarshal(def["value"], &cell); err != nil {
			return err
		}
		for _, key := range []string{"kind", "value"} {
			if _, ok := cell[key]; !ok {
				return errors.New("core: missing default constant field")
			}
		}
	}
	*d = ColumnDefinition(value)
	return nil
}

func (p *ColumnPosition) UnmarshalJSON(data []byte) error {
	type plain ColumnPosition
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key := range fields {
		if key != "mode" && (key != "column" || value.Mode == "first") {
			return errors.New("core: irrelevant position field")
		}
	}
	*p = ColumnPosition(value)
	return nil
}
