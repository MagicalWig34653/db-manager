// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"encoding/json"
)

const CreateContractVersion = "0.6.0"

type CreateType struct {
	Kind      string `json:"kind"`
	Length    int    `json:"length,omitempty"`
	Precision int    `json:"precision,omitempty"`
	Scale     int    `json:"scale,omitempty"`
}
type CreateDefault struct {
	Mode  string `json:"mode"`
	Value *Cell  `json:"value,omitempty"`
}
type CreateColumn struct {
	Name     string        `json:"name"`
	Type     CreateType    `json:"type"`
	Nullable bool          `json:"nullable"`
	Default  CreateDefault `json:"default"`
}
type CreateRequest struct {
	Connection Connection     `json:"connection"`
	Table      TableRef       `json:"table"`
	Columns    []CreateColumn `json:"columns"`
	PrimaryKey []string       `json:"primary_key"`
	Confirm    bool           `json:"confirm"`
	TimeoutMS  int            `json:"timeout_ms,omitempty"`
}
type CreateResult struct {
	Outcome           string            `json:"outcome"`
	StructureCheck    string            `json:"structure_check"`
	SnapshotAtomic    bool              `json:"snapshot_atomic"`
	Message           string            `json:"message"`
	ActualDescription *TableDescription `json:"actual_description,omitempty"`
}
type CreateEngine interface {
	CreateTable(context.Context, CreateRequest) (CreateResult, error)
}

// MarshalJSON preserves the required decimal scale even when it is zero.
func (t CreateType) MarshalJSON() ([]byte, error) {
	type plain CreateType
	if t.Kind == "decimal" {
		return json.Marshal(struct {
			Kind      string `json:"kind"`
			Precision int    `json:"precision"`
			Scale     int    `json:"scale"`
		}{t.Kind, t.Precision, t.Scale})
	}
	return json.Marshal(plain(t))
}
