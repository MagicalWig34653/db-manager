// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ColumnType is a structured type selection, never a SQL fragment.
type ColumnType struct {
	Kind      string   `json:"kind"`
	Length    int      `json:"length,omitempty"`
	Precision int      `json:"precision,omitempty"`
	Scale     int      `json:"scale,omitempty"`
	Values    []string `json:"values,omitempty"`
}

// TypeCatalog returns a fresh dialect-specific list suitable for API clients.
func TypeCatalog(driver string) []string {
	switch driver {
	case "sqlite", "postgres":
		return []string{"int64", "text", "binary", "decimal", "boolean", "date", "timestamp"}
	case "mysql":
		return []string{"int64", "text", "binary", "decimal", "boolean", "date", "timestamp", "enum", "set"}
	}
	return []string{}
}

// TypeSQL renders only catalog types and bounded parameters. String constants
// require NO_BACKSLASH_ESCAPES on MySQL and standard_conforming_strings on PG.
func TypeSQL(driver string, t ColumnType) (string, error) {
	invalid := errors.New("dbplan: invalid column type")
	found := false
	for _, kind := range TypeCatalog(driver) {
		if t.Kind == kind {
			found = true
		}
	}
	if !found {
		return "", invalid
	}
	if t.Kind != "text" && t.Length != 0 || t.Kind != "decimal" && (t.Precision != 0 || t.Scale != 0) || t.Kind != "enum" && t.Kind != "set" && len(t.Values) != 0 {
		return "", invalid
	}
	switch t.Kind {
	case "int64":
		return "BIGINT", nil
	case "text":
		if t.Length >= 1 && t.Length <= 1024 {
			return fmt.Sprintf("VARCHAR(%d)", t.Length), nil
		}
	case "binary":
		if driver == "postgres" {
			return "BYTEA", nil
		}
		return "BLOB", nil
	case "decimal":
		if t.Precision >= 1 && t.Precision <= 38 && t.Scale >= 0 && t.Scale <= t.Precision {
			return fmt.Sprintf("DECIMAL(%d,%d)", t.Precision, t.Scale), nil
		}
	case "boolean":
		return "BOOLEAN", nil
	case "date":
		return "DATE", nil
	case "timestamp":
		return "TIMESTAMP", nil
	case "enum", "set":
		max := 256
		if t.Kind == "set" {
			max = 64
		}
		if len(t.Values) == 0 || len(t.Values) > max {
			return "", invalid
		}
		seen := map[string]bool{}
		values := []string{}
		for _, value := range t.Values {
			if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || len(value) > 255 || seen[value] || strings.HasSuffix(value, " ") || t.Kind == "set" && strings.Contains(value, ",") {
				return "", invalid
			}
			seen[value] = true
			values = append(values, "'"+strings.ReplaceAll(value, "'", "''")+"'")
		}
		return strings.ToUpper(t.Kind) + "(" + strings.Join(values, ",") + ")", nil
	}
	return "", invalid
}

// UnmarshalJSON rejects irrelevant parameters even when their numeric value is zero.
func (t *ColumnType) UnmarshalJSON(data []byte) error {
	type plain ColumnType
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{"kind": true}
	required := []string{"kind"}
	switch value.Kind {
	case "text":
		allowed["length"] = true
		required = append(required, "length")
	case "decimal":
		allowed["precision"] = true
		allowed["scale"] = true
		required = append(required, "precision")
	case "enum", "set":
		allowed["values"] = true
		required = append(required, "values")
	}
	for key := range fields {
		if !allowed[key] {
			return errors.New("dbplan: irrelevant type parameter")
		}
	}
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			return errors.New("dbplan: missing type parameter")
		}
	}
	*t = ColumnType(value)
	return nil
}
