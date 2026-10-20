// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqlite "github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

// sqliteQuery reads storage classes before invoking any coercing column getter.
// Declared column types remain metadata and never determine a cell's runtime kind.
func sqliteQuery(ctx context.Context, db *sql.DB, request core.QueryRequest, start time.Time, args ...any) (core.QueryResult, error) {
	result := core.QueryResult{Results: []core.ResultSet{}}
	connection, err := db.Conn(ctx)
	if err != nil {
		return result, safe(ctx, err)
	}
	defer connection.Close()
	set, err := sqliteRead(ctx, connection, request.SQL, request.MaxRows, args...)
	result.Results = append(result.Results, set)
	if err != nil {
		return core.QueryResult{}, safe(ctx, err)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	encoded, err := json.Marshal(result)
	if err != nil {
		return core.QueryResult{}, public("result_error", "Ergebnis konnte nicht verarbeitet werden.")
	}
	if len(encoded)+1 > MaxResponseBytes {
		return core.QueryResult{}, public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
	}
	return result, nil
}

// sqliteRead preserves native storage classes on the caller's pinned connection.
func sqliteRead(ctx context.Context, connection *sql.Conn, query string, maxRows int, args ...any) (core.ResultSet, error) {
	return sqliteReadMode(ctx, connection, query, maxRows, false, args...)
}
func sqliteReadMode(ctx context.Context, connection *sql.Conn, query string, maxRows int, strictStorage bool, args ...any) (core.ResultSet, error) {
	var result core.ResultSet
	err := connection.Raw(func(raw any) error {
		driver, ok := raw.(sqliteDriver.Conn)
		if !ok {
			return public("result_error", "SQLite-Verbindung konnte nicht verarbeitet werden.")
		}
		native := driver.Raw()
		oldContext := native.SetInterrupt(ctx)
		defer native.SetInterrupt(oldContext)
		statement, tail, err := native.Prepare(query)
		if err != nil {
			return err
		}
		if statement == nil {
			return public("invalid_query", "Genau eine SQL-Anweisung ist erforderlich.")
		}
		defer statement.Close()
		if strings.TrimSpace(tail) != "" {
			return public("invalid_query", "Genau eine SQL-Anweisung ist erforderlich.")
		}
		if statement.BindCount() != len(args) {
			return public("invalid_query", "Anzahl der Parameter ist ungültig.")
		}
		for index, value := range args {
			switch typed := value.(type) {
			case int64:
				err = statement.BindInt64(index+1, typed)
			case float64:
				err = statement.BindFloat(index+1, typed)
			case string:
				err = statement.BindText(index+1, typed)
			case []byte:
				err = statement.BindBlob(index+1, typed)
			case bool:
				err = statement.BindBool(index+1, typed)
			case nil:
				err = statement.BindNull(index + 1)
			default:
				return public("invalid_filter", "Filterwert ist nicht unterstützt.")
			}
			if err != nil {
				return err
			}
		}
		set := core.ResultSet{Columns: []core.Column{}, Rows: [][]core.Cell{}}
		count := statement.ColumnCount()
		if count > 1024 {
			return public("result_too_large", "Zu viele Ergebnisspalten.")
		}
		for index := 0; index < count; index++ {
			name, declared := statement.ColumnName(index), statement.ColumnDeclType(index)
			if len(name) > MaxCellBytes || len(declared) > MaxCellBytes {
				return public("result_too_large", "Spaltenmetadaten sind zu groß.")
			}
			set.Columns = append(set.Columns, core.Column{Name: name, DatabaseType: declared})
		}
		header, _ := json.Marshal(set)
		used := len(header) + 128
		for statement.Step() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if len(set.Rows) == maxRows {
				set.Truncated = true
				break
			}
			row := make([]core.Cell, count)
			for index := range row {
				var value any
				switch statement.ColumnType(index) {
				case sqlite.NULL:
					value = nil
				case sqlite.INTEGER:
					value = statement.ColumnInt64(index)
				case sqlite.FLOAT:
					value = statement.ColumnFloat(index)
				case sqlite.TEXT:
					value = statement.ColumnText(index)
					if strictStorage && !utf8.ValidString(value.(string)) {
						return public("mutation_not_supported", "Ungültiges UTF-8 in SQLite-TEXT ist für Änderungen nicht freigegeben.")
					}
				case sqlite.BLOB:
					value = statement.ColumnRawBlob(index)
				default:
					return public("result_error", "Unbekannte SQLite-Speicherklasse.")
				}
				row[index], err = cell(value, "")
				if err != nil {
					return err
				}
			}
			encoded, _ := json.Marshal(row)
			used += len(encoded) + 1
			if used > MaxResponseBytes+512 {
				return public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
			}
			set.Rows = append(set.Rows, row)
		}
		if err := statement.Err(); err != nil {
			return err
		}
		result = set
		return nil
	})
	return result, err
}
