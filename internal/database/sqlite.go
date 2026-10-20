// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqlite "github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

// sqliteQuery reads storage classes before invoking any coercing column getter.
// Declared column types remain metadata and never determine a cell's runtime kind.
func sqliteQuery(ctx context.Context, db *sql.DB, request core.QueryRequest, start time.Time) (core.QueryResult, error) {
	result := core.QueryResult{Results: []core.ResultSet{}}
	connection, err := db.Conn(ctx)
	if err != nil {
		return result, safe(ctx, err)
	}
	defer connection.Close()
	err = connection.Raw(func(raw any) error {
		driver, ok := raw.(sqliteDriver.Conn)
		if !ok {
			return public("result_error", "SQLite-Verbindung konnte nicht verarbeitet werden.")
		}
		native := driver.Raw()
		oldContext := native.SetInterrupt(ctx)
		defer native.SetInterrupt(oldContext)
		statement, tail, err := native.Prepare(request.SQL)
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
			if len(set.Rows) == request.MaxRows {
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
		result.Results = append(result.Results, set)
		return nil
	})
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
