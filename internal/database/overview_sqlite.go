// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func sqliteSchemaOK(schema string) bool { return schema == "" || schema == "main" }

func sqliteDatabases(ctx context.Context, db *sql.DB, c core.Connection, limit, offset int) (core.DatabasesResult, error) {
	result := core.DatabasesResult{Databases: []core.DatabaseInfo{}, Total: 1}
	// The sandbox authorizer denies PRAGMA page_count/page_size, so no size is reported.
	var count int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM main.sqlite_schema WHERE type IN ('table','view') AND name NOT GLOB 'sqlite_*'").Scan(&count); err != nil {
		return result, err
	}
	if offset == 0 {
		binary := "BINARY"
		charset := "UTF-8"
		result.Databases = append(result.Databases, core.DatabaseInfo{Name: c.Database, Charset: &charset, Collation: &binary, Current: true, ObjectCount: &count})
	}
	result.HasMore = offset+len(result.Databases) < 1
	return result, nil
}

func sqliteObjects(ctx context.Context, db *sql.DB, schema string, limit, offset int) (core.ObjectsResult, error) {
	result := core.ObjectsResult{Schema: "main", Objects: []core.ObjectInfo{}}
	if !sqliteSchemaOK(schema) {
		return result, invalidOverview("Schema ist nicht verfügbar.")
	}
	const where = "type IN ('table','view') AND name NOT GLOB 'sqlite_*'"
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM main.sqlite_schema WHERE "+where).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := db.QueryContext(ctx, "SELECT name, type, COALESCE(sql,'') FROM main.sqlite_schema WHERE "+where+" ORDER BY name LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, kind, definition string
		if err := rows.Scan(&name, &kind, &definition); err != nil {
			return result, err
		}
		if len(name) > MaxCellBytes {
			return result, public("result_too_large", "Objektname ist zu groß.")
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(definition)), "CREATE VIRTUAL TABLE") {
			kind = "virtual"
		}
		engine := "sqlite"
		result.Objects = append(result.Objects, core.ObjectInfo{Schema: "main", Name: name, Kind: kind, Rows: core.RowCount{State: "unknown"}, Engine: &engine})
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.HasMore = int64(offset+len(result.Objects)) < result.Total
	return result, nil
}

func sqliteDefinition(ctx context.Context, db *sql.DB, ref core.TableRef) (core.DefinitionResult, error) {
	result := core.DefinitionResult{Warnings: []string{}}
	if !sqliteSchemaOK(ref.Schema) || strings.HasPrefix(strings.ToLower(ref.Name), "sqlite_") {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	var name, kind, definition string
	err := db.QueryRowContext(ctx, "SELECT name, type, COALESCE(sql,'') FROM main.sqlite_schema WHERE name=? AND type IN ('table','view')", ref.Name).Scan(&name, &kind, &definition)
	if err == sql.ErrNoRows {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	if err != nil {
		return result, err
	}
	result.Object = core.Table{Schema: "main", Name: name, Kind: kind}
	result.Definition, result.Source = definition, "sqlite_schema"
	return result, nil
}

func sqliteServerInfo(ctx context.Context, db *sql.DB, result *core.ServerInfoResult) error {
	result.Variant = "sqlite"
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&result.Version); err != nil {
		return err
	}
	name := result.Connection.Database
	result.CurrentDatabase = &name
	utf8 := "UTF-8"
	result.CharacterSets = append(result.CharacterSets, core.CharacterSetInfo{Name: "UTF-8", Description: &utf8})
	for _, n := range []string{"BINARY", "NOCASE", "RTRIM"} {
		result.Collations = append(result.Collations, core.CollationInfo{Name: n, Default: n == "BINARY"})
	}
	return nil
}
