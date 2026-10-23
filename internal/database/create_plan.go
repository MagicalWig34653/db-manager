// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

const MaxCreateColumns = 32

var createDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func createInvalid() error {
	return public("invalid_create", "Tabellendefinition oder ausdrückliche Bestätigung ist ungültig.")
}
func createName(name string) bool {
	return name != "" && len(name) <= 63 && utf8.ValidString(name) && !strings.ContainsRune(name, 0)
}
func createRef(c core.Connection, ref core.TableRef) (core.TableRef, error) {
	if !createName(ref.Name) {
		return ref, createInvalid()
	}
	if ref.Schema == "" {
		switch c.Driver {
		case "sqlite":
			ref.Schema = "main"
		case "mysql":
			ref.Schema = c.Database
		case "postgres":
			ref.Schema = "public"
		default:
			return ref, createInvalid()
		}
	}
	if !createName(ref.Schema) {
		return ref, createInvalid()
	}
	schema := strings.ToLower(ref.Schema)
	switch c.Driver {
	case "sqlite":
		if ref.Schema != "main" || strings.HasPrefix(strings.ToLower(ref.Name), "sqlite_") {
			return ref, createInvalid()
		}
	case "mysql":
		switch schema {
		case "mysql", "sys", "information_schema", "performance_schema":
			return ref, createInvalid()
		}
	case "postgres":
		if strings.HasPrefix(schema, "pg_") || schema == "information_schema" {
			return ref, createInvalid()
		}
	default:
		return ref, createInvalid()
	}
	return ref, nil
}
func createTypeSQL(driver string, t core.CreateType) (string, error) {
	switch t.Kind {
	case "int64":
		if t.Length == 0 && t.Precision == 0 && t.Scale == 0 {
			return "BIGINT", nil
		}
	case "text":
		if t.Length >= 1 && t.Length <= 1024 && t.Precision == 0 && t.Scale == 0 {
			return fmt.Sprintf("VARCHAR(%d)", t.Length), nil
		}
	case "binary":
		if t.Length == 0 && t.Precision == 0 && t.Scale == 0 {
			if driver == "postgres" {
				return "BYTEA", nil
			}
			return "BLOB", nil
		}
	case "decimal":
		if t.Length == 0 && t.Precision >= 1 && t.Precision <= 38 && t.Scale >= 0 && t.Scale <= t.Precision {
			return fmt.Sprintf("DECIMAL(%d,%d)", t.Precision, t.Scale), nil
		}
	}
	return "", createInvalid()
}
func createDefaultSQL(column core.CreateColumn) (string, error) {
	d := column.Default
	switch d.Mode {
	case "none":
		if d.Value == nil {
			return "", nil
		}
	case "null":
		if d.Value == nil && column.Nullable {
			return "NULL", nil
		}
	case "value":
		if d.Value == nil {
			return "", createInvalid()
		}
		value := d.Value.Value
		switch column.Type.Kind {
		case "int64":
			n, err := strconv.ParseInt(value, 10, 64)
			if err == nil && d.Value.Kind == "integer" && strconv.FormatInt(n, 10) == value {
				return value, nil
			}
		case "text":
			if d.Value.Kind == "text" && utf8.ValidString(value) && !strings.ContainsRune(value, 0) && len(value) <= 4096 && utf8.RuneCountInString(value) <= column.Type.Length {
				return "'" + strings.ReplaceAll(value, "'", "''") + "'", nil
			}
		case "decimal":
			if d.Value.Kind != "decimal" || len(value) > 80 || !createDecimal.MatchString(value) {
				return "", createInvalid()
			}
			parts := strings.Split(strings.TrimPrefix(value, "-"), ".")
			whole := len(parts[0])
			if parts[0] == "0" {
				whole = 0
			}
			fraction := 0
			if len(parts) == 2 {
				fraction = len(parts[1])
			}
			if whole <= column.Type.Precision-column.Type.Scale && fraction <= column.Type.Scale {
				return value, nil
			}
		}
	}
	return "", createInvalid()
}
func createSQL(r core.CreateRequest) (string, error) {
	if !r.Confirm || len(r.Columns) < 1 || len(r.Columns) > MaxCreateColumns || len(r.PrimaryKey) > 8 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return "", createInvalid()
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > 1<<20 {
		return "", createInvalid()
	}
	ref, err := createRef(r.Connection, r.Table)
	if err != nil {
		return "", err
	}
	names := map[string]int{}
	parts := []string{}
	textBudget := 0
	for i, column := range r.Columns {
		if !createName(column.Name) {
			return "", createInvalid()
		}
		name := strings.ToLower(column.Name)
		if _, exists := names[name]; exists {
			return "", createInvalid()
		}
		names[name] = i
		typ, err := createTypeSQL(r.Connection.Driver, column.Type)
		if err != nil {
			return "", err
		}
		literal, err := createDefaultSQL(column)
		if err != nil {
			return "", err
		}
		textBudget += column.Type.Length
		if textBudget > 4096 {
			return "", createInvalid()
		}
		part := quoteIdentifier(r.Connection.Driver, column.Name) + " " + typ
		if !column.Nullable {
			part += " NOT NULL"
		}
		if column.Default.Mode != "none" {
			part += " DEFAULT " + literal
		}
		parts = append(parts, part)
	}
	seen := map[string]bool{}
	keys := []string{}
	keyBytes := 0
	for _, name := range r.PrimaryKey {
		i, ok := names[strings.ToLower(name)]
		if !ok || seen[name] || r.Columns[i].Name != name || r.Columns[i].Nullable {
			return "", createInvalid()
		}
		column := r.Columns[i]
		if column.Type.Kind != "int64" && column.Type.Kind != "text" {
			return "", createInvalid()
		}
		if column.Type.Kind == "text" {
			keyBytes += 4 * column.Type.Length
		} else {
			keyBytes += 8
		}
		if keyBytes > 2048 {
			return "", createInvalid()
		}
		seen[name] = true
		keys = append(keys, quoteIdentifier(r.Connection.Driver, name))
	}
	if len(keys) > 0 {
		parts = append(parts, "PRIMARY KEY ("+strings.Join(keys, ",")+")")
	}
	statement := "CREATE TABLE " + quoteIdentifier(r.Connection.Driver, ref.Schema) + "." + quoteIdentifier(r.Connection.Driver, ref.Name) + " (" + strings.Join(parts, ",") + ")"
	if r.Connection.Driver == "mysql" {
		statement += " ENGINE=InnoDB DEFAULT CHARACTER SET=utf8mb4"
	}
	return statement, nil
}
func createDefaultMatches(driver string, column core.CreateColumn, actual *string, maria bool) bool {
	// Compare only recognized constant renderings; never execute catalog default SQL.
	if column.Default.Mode == "none" || column.Default.Mode == "null" {
		if actual == nil {
			return true
		}
		value := strings.ToUpper(*actual)
		return column.Nullable && (value == "NULL" || value == "NULL::BIGINT" || value == "NULL::CHARACTER VARYING" || value == "NULL::NUMERIC" || value == "NULL::BYTEA")
	}
	if actual == nil || column.Default.Value == nil {
		return false
	}
	value := *actual
	if driver == "postgres" {
		for _, suffix := range []string{"::character varying", "::bigint", "::numeric"} {
			value = strings.TrimSuffix(value, suffix)
		}
	}
	literal, _ := createDefaultSQL(column)
	if column.Type.Kind == "text" {
		if driver == "mysql" {
			if !maria {
				return value == column.Default.Value.Value
			}
			// MariaDB COLUMNS renders a quoted literal with doubled backslashes.
			return value == "'"+strings.ReplaceAll(strings.ReplaceAll(column.Default.Value.Value, "\\", "\\\\"), "'", "''")+"'"
		}
		return value == literal
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = value[1 : len(value)-1]
	}
	if len(value) > 128 || !createDecimal.MatchString(value) {
		return false
	}
	a, ok := new(big.Rat).SetString(value)
	if !ok {
		return false
	}
	b, ok := new(big.Rat).SetString(column.Default.Value.Value)
	return ok && a.Cmp(b) == 0
}
func createMatches(r core.CreateRequest, d core.TableDescription, maria bool) bool {
	if d.Kind != "table" || d.Table != r.Table || len(d.Columns) != len(r.Columns) || len(d.PrimaryKey) != len(r.PrimaryKey) {
		return false
	}
	if r.Connection.Driver == "mysql" && !strings.EqualFold(d.StorageEngine, "InnoDB") {
		return false
	}
	for i, key := range r.PrimaryKey {
		if d.PrimaryKey[i] != key {
			return false
		}
	}
	for i, column := range r.Columns {
		actual := d.Columns[i]
		expected, _ := createTypeSQL(r.Connection.Driver, column.Type)
		expected = strings.ToLower(expected)
		typ := strings.ToLower(actual.DatabaseType)
		if r.Connection.Driver == "postgres" {
			expected = strings.ReplaceAll(expected, "varchar(", "character varying(")
			expected = strings.ReplaceAll(expected, "decimal(", "numeric(")
		}
		if r.Connection.Driver == "mysql" && column.Type.Kind == "int64" {
			typ = strings.TrimSuffix(typ, "(20)")
		}
		if actual.Name != column.Name || actual.Nullable != column.Nullable || typ != expected || actual.Generated != "none" || !createDefaultMatches(r.Connection.Driver, column, actual.DefaultValue, maria) {
			return false
		}
	}
	return true
}
