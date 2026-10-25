// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	sqlite "github.com/ncruces/go-sqlite3"
	sqliteDriver "github.com/ncruces/go-sqlite3/driver"
)

func exportQueryColumns(ctx context.Context, conn *sql.Conn, driver, query string) ([]core.TableColumn, error) {
	cols := []core.TableColumn{}
	if driver == "sqlite" {
		err := withExportSQLite(ctx, conn, query, func(stmt *sqlite.Stmt) error {
			for i := 0; i < stmt.ColumnCount(); i++ {
				cols = append(cols, core.TableColumn{Name: stmt.ColumnName(i), DatabaseType: stmt.ColumnDeclType(i), Generated: "none"})
			}
			return nil
		})
		return cols, err
	}
	// LIMIT 0 discovers metadata without reading the complete query result.
	stmt, err := conn.PrepareContext(ctx, "SELECT * FROM ("+strings.TrimSuffix(strings.TrimSpace(query), ";")+") AS dbm_export_query LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	if len(types) > 1024 {
		return nil, invalidExport()
	}
	for _, typ := range types {
		cols = append(cols, core.TableColumn{Name: typ.Name(), DatabaseType: typ.DatabaseTypeName(), Generated: "none"})
	}
	if len(cols) == 0 {
		return nil, invalidExport()
	}
	return cols, rows.Err()
}

func withExportSQLite(ctx context.Context, conn *sql.Conn, query string, run func(*sqlite.Stmt) error) error {
	return conn.Raw(func(raw any) error {
		native := raw.(sqliteDriver.Conn).Raw()
		old := native.SetInterrupt(ctx)
		defer native.SetInterrupt(old)
		stmt, tail, err := native.Prepare(query)
		if err != nil {
			return err
		}
		if stmt == nil {
			return invalidExport()
		}
		defer stmt.Close()
		if strings.TrimSpace(tail) != "" || !stmt.ReadOnly() || stmt.BindCount() != 0 || stmt.ColumnCount() == 0 || stmt.ColumnCount() > 1024 {
			return invalidExport()
		}
		return run(stmt)
	})
}

func streamExportRows(ctx context.Context, conn *sql.Conn, driver, query string, emit func([]core.Cell) error) error {
	if driver == "sqlite" {
		return withExportSQLite(ctx, conn, query, func(stmt *sqlite.Stmt) error {
			for stmt.Step() {
				if err := ctx.Err(); err != nil {
					return err
				}
				row := make([]core.Cell, stmt.ColumnCount())
				rowBytes := 0
				for i := range row {
					var v any
					switch stmt.ColumnType(i) {
					case sqlite.NULL:
						v = nil
					case sqlite.INTEGER:
						v = stmt.ColumnInt64(i)
					case sqlite.FLOAT:
						v = stmt.ColumnFloat(i)
					case sqlite.TEXT:
						if len(stmt.ColumnRawText(i)) > MaxCellBytes {
							return public("result_too_large", "Ergebniszelle ist zu groß.")
						}
						v = stmt.ColumnText(i)
						if !utf8.ValidString(v.(string)) {
							return public("unsupported_export", "Ungültiges UTF-8 in SQLite-TEXT wird nicht stillschweigend umgewandelt.")
						}
					case sqlite.BLOB:
						if len(stmt.ColumnRawBlob(i)) > MaxCellBytes {
							return public("result_too_large", "Ergebniszelle ist zu groß.")
						}
						v = stmt.ColumnRawBlob(i)
					}
					var err error
					row[i], err = cell(v, "")
					rowBytes += len(row[i].Value)
					if rowBytes > 4<<20 {
						return public("result_too_large", "Exportzeile ist zu groß.")
					}
					if err != nil {
						return err
					}
				}
				if err := emit(row); err != nil {
					return err
				}
			}
			return stmt.Err()
		})
	}
	stmt, err := conn.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	if len(types) > 1024 {
		return invalidExport()
	}
	values := make([]any, len(types))
	pointers := make([]any, len(types))
	for i := range values {
		pointers[i] = &values[i]
	}
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = rows.Scan(pointers...); err != nil {
			return err
		}
		row := make([]core.Cell, len(types))
		rowBytes := 0
		for i, v := range values {
			if t, ok := v.(time.Time); ok {
				v = t.UTC()
			}
			row[i], err = cell(v, types[i].DatabaseTypeName())
			rowBytes += len(row[i].Value)
			if rowBytes > 4<<20 {
				return public("result_too_large", "Exportzeile ist zu groß.")
			}
			if err != nil {
				return err
			}
		}
		if err = emit(row); err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if rows.NextResultSet() {
		return invalidExport()
	}
	return rows.Err()
}
