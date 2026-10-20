// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"encoding/json"
)

const ContractVersion = "0.1.0"

type Connection struct {
	Driver   string `json:"driver"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	TLSMode  string `json:"tls_mode,omitempty"`
}

type QueryRequest struct {
	Connection Connection `json:"connection"`
	SQL        string     `json:"sql"`
	MaxRows    int        `json:"max_rows,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

type Column struct {
	Name         string `json:"name"`
	DatabaseType string `json:"database_type"`
}

// Cell preserves numeric precision and distinguishes NULL, text and binary data.
type Cell struct {
	Kind  string `json:"kind"`
	Value string `json:"value,omitempty"`
}

// MarshalJSON preserves empty strings while keeping NULL distinct from a value.
func (c Cell) MarshalJSON() ([]byte, error) {
	if c.Kind == "null" {
		return []byte(`{"kind":"null"}`), nil
	}
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}{c.Kind, c.Value})
}

type ResultSet struct {
	Columns      []Column `json:"columns"`
	Rows         [][]Cell `json:"rows"`
	RowsAffected *string  `json:"rows_affected,omitempty"`
	Truncated    bool     `json:"truncated"`
}

type QueryResult struct {
	Results    []ResultSet `json:"results"`
	DurationMS int64       `json:"duration_ms"`
}

type Table struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

type CatalogRequest struct {
	Connection Connection `json:"connection"`
}

type Engine interface {
	Test(context.Context, Connection) error
	Query(context.Context, QueryRequest) (QueryResult, error)
	Catalog(context.Context, Connection) ([]Table, error)
	Drivers() []string
}

// Error carries a public code without exposing driver errors or credentials.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }
