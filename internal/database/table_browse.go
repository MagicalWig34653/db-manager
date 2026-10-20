// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

var decimalValue = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)

func (e *Engine) Browse(ctx context.Context, request core.BrowseRequest) (core.BrowseResult, error) {
	start := time.Now()
	empty := core.BrowseResult{}
	if !validTableRef(request.Table) || request.PageSize < 0 || request.PageSize > 1000 || request.Offset < 0 || request.Offset > 1000000 || len(request.Sort) > 16 || len(request.Filters) > 16 || request.TimeoutMS < 0 || request.TimeoutMS > 60000 {
		return empty, public("invalid_browse", "Tabellenabfrage oder Grenzen sind ungültig.")
	}
	if request.PageSize == 0 {
		request.PageSize = 100
	}
	if request.TimeoutMS == 0 {
		request.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(request.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, request.Connection)
	if err != nil {
		return empty, err
	}
	defer closeDB()
	description, err := describe(ctx, db, request.Connection, request.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	query, args, err := browseSQL(request, description)
	if err != nil {
		return empty, err
	}
	set, err := boundTableRows(ctx, db, request.Connection, query, args, request.PageSize)
	if err != nil {
		return empty, safe(ctx, err)
	}
	result := core.BrowseResult{Description: description, Result: set, Offset: request.Offset, PageSize: request.PageSize, StableOrder: description.StableOrder, HasMore: set.Truncated, DurationMS: time.Since(start).Milliseconds()}
	if len(set.Rows) > request.PageSize {
		result.HasMore = true
		result.Result.Rows = set.Rows[:request.PageSize]
	}
	// has_more is pagination, never an unmarked truncated query result.
	result.Result.Truncated = false
	encoded, err := json.Marshal(result)
	if err != nil {
		return empty, public("result_error", "Ergebnis konnte nicht verarbeitet werden.")
	}
	if len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Tabellenseite überschreitet die erlaubte Größe.")
	}
	return result, nil
}
func browseSQL(request core.BrowseRequest, description core.TableDescription) (string, []any, error) {
	driver := request.Connection.Driver
	quote := func(value string) string { return quoteIdentifier(driver, value) }
	known := map[string]bool{}
	names := []string{}
	for _, column := range description.Columns {
		known[column.Name] = true
		names = append(names, quote(column.Name))
	}
	args := []any{}
	parameter := func(value any) string {
		args = append(args, value)
		if driver == "postgres" {
			return "$" + strconv.Itoa(len(args))
		}
		return "?"
	}
	query := "SELECT " + strings.Join(names, ",") + " FROM " + quote(description.Table.Schema) + "." + quote(description.Table.Name)
	predicates := []string{}
	for _, filter := range request.Filters {
		if !known[filter.Column] {
			return "", nil, public("invalid_filter", "Filterspalte ist nicht verfügbar.")
		}
		name := quote(filter.Column)
		switch filter.Operator {
		case "is_null", "is_not_null":
			if filter.Value != nil {
				return "", nil, public("invalid_filter", "NULL-Filter dürfen keinen Wert enthalten.")
			}
			if filter.Operator == "is_null" {
				predicates = append(predicates, name+" IS NULL")
			} else {
				predicates = append(predicates, name+" IS NOT NULL")
			}
		case "eq":
			if filter.Value == nil {
				return "", nil, public("invalid_filter", "Gleichheitsfilter benötigt einen Wert.")
			}
			value, err := filterValue(*filter.Value)
			if err != nil {
				return "", nil, err
			}
			predicates = append(predicates, name+" = "+parameter(value))
		default:
			return "", nil, public("invalid_filter", "Filteroperator ist nicht verfügbar.")
		}
	}
	if len(predicates) > 0 {
		query += " WHERE " + strings.Join(predicates, " AND ")
	}
	order := []string{}
	sorted := map[string]bool{}
	for _, item := range request.Sort {
		if !known[item.Column] || sorted[item.Column] || (item.Direction != "asc" && item.Direction != "desc") {
			return "", nil, public("invalid_sort", "Sortierung ist ungültig.")
		}
		sorted[item.Column] = true
		order = append(order, quote(item.Column)+" "+strings.ToUpper(item.Direction))
	}
	for _, key := range description.PrimaryKey {
		if driver == "sqlite" {
			// Column and PK-index collations may differ. A binary final key also
			// resolves ties when an explicit user sort already names the PK.
			order = append(order, quote(key)+" COLLATE BINARY ASC")
		} else if !sorted[key] {
			order = append(order, quote(key)+" ASC")
		}
	}
	if len(order) > 0 {
		query += " ORDER BY " + strings.Join(order, ",")
	}
	query += " LIMIT " + parameter(int64(request.PageSize+1)) + " OFFSET " + parameter(int64(request.Offset))
	return query, args, nil
}
func filterValue(value core.Cell) (any, error) {
	invalid := func() (any, error) { return nil, public("invalid_filter", "Filterwert ist ungültig.") }
	if len(value.Value) > MaxCellBytes*2 {
		return invalid()
	}
	switch value.Kind {
	case "text", "timestamp", "decimal":
		if len(value.Value) > MaxCellBytes {
			return invalid()
		}
		if value.Kind == "decimal" && !decimalValue.MatchString(value.Value) {
			return invalid()
		}
		return value.Value, nil
	case "integer":
		if parsed, err := strconv.ParseInt(value.Value, 10, 64); err == nil {
			return parsed, nil
		}
		if _, err := strconv.ParseUint(value.Value, 10, 64); err == nil {
			return value.Value, nil
		}
		return invalid()
	case "float":
		parsed, err := strconv.ParseFloat(value.Value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return invalid()
		}
		return parsed, nil
	case "boolean":
		if value.Value == "true" {
			return true, nil
		}
		if value.Value == "false" {
			return false, nil
		}
		return invalid()
	case "binary":
		data, err := base64.StdEncoding.Strict().DecodeString(value.Value)
		if err != nil || len(data) > MaxCellBytes {
			return invalid()
		}
		return data, nil
	default:
		return invalid()
	}
}
func boundTableRows(ctx context.Context, db *sql.DB, connection core.Connection, query string, args []any, limit int) (core.ResultSet, error) {
	if connection.Driver == "sqlite" {
		result, err := sqliteQuery(ctx, db, core.QueryRequest{Connection: connection, SQL: query, MaxRows: limit}, time.Now(), args...)
		if err != nil {
			return core.ResultSet{}, err
		}
		return result.Results[0], nil
	}
	statement, err := db.PrepareContext(ctx, query)
	if err != nil {
		return core.ResultSet{}, err
	}
	defer statement.Close()
	rows, err := statement.QueryContext(ctx, args...)
	if err != nil {
		return core.ResultSet{}, err
	}
	defer rows.Close()
	result, _, err := readSet(ctx, rows, limit, MaxResponseBytes+512)
	return result, err
}
