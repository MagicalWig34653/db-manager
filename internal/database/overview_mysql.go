// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

var mysqlSystemSchemas = map[string]bool{"information_schema": true, "mysql": true, "performance_schema": true, "sys": true}

func mysqlDatabases(ctx context.Context, db *sql.DB, limit, offset int) (core.DatabasesResult, error) {
	result := core.DatabasesResult{Databases: []core.DatabaseInfo{}}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.SCHEMATA").Scan(&result.Total); err != nil {
		return result, err
	}
	var current sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&current); err != nil {
		return result, err
	}
	// The page of schemata is cut first so statistics are aggregated only for it.
	rows, err := db.QueryContext(ctx, `SELECT s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME, COUNT(t.TABLE_NAME), CAST(SUM(COALESCE(t.DATA_LENGTH,0)+COALESCE(t.INDEX_LENGTH,0)) AS SIGNED)
FROM (SELECT SCHEMA_NAME, DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME LIMIT ? OFFSET ?) s
LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
GROUP BY s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME ORDER BY s.SCHEMA_NAME`, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var charset, collation sql.NullString
		var count, size sql.NullInt64
		if err := rows.Scan(&name, &charset, &collation, &count, &size); err != nil {
			return result, err
		}
		if len(name) > MaxCellBytes {
			return result, public("result_too_large", "Datenbankname ist zu groß.")
		}
		result.Databases = append(result.Databases, core.DatabaseInfo{Name: name, Charset: nullString(charset), Collation: nullString(collation), Current: current.Valid && current.String == name, System: mysqlSystemSchemas[strings.ToLower(name)], ObjectCount: nullInt(count), SizeBytes: nullInt(size), Estimated: true})
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.HasMore = int64(offset+len(result.Databases)) < result.Total
	return result, nil
}

func mysqlSchema(c core.Connection, schema string) (string, error) {
	if schema == "" {
		schema = c.Database
	}
	if schema == "" {
		return "", invalidOverview("Datenbankschema ist erforderlich.")
	}
	return schema, nil
}

func mysqlKind(tableType string) string {
	switch tableType {
	case "BASE TABLE":
		return "table"
	case "VIEW", "SYSTEM VIEW":
		return "view"
	}
	return strings.ToLower(strings.ReplaceAll(tableType, " ", "_"))
}

func mysqlObjects(ctx context.Context, db *sql.DB, c core.Connection, schema string, limit, offset int) (core.ObjectsResult, error) {
	result := core.ObjectsResult{Objects: []core.ObjectInfo{}}
	schema, err := mysqlSchema(c, schema)
	if err != nil {
		return result, err
	}
	result.Schema = schema
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=?", schema).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := db.QueryContext(ctx, "SELECT TABLE_NAME, TABLE_TYPE, CAST(TABLE_ROWS AS SIGNED), ENGINE, TABLE_COLLATION, CAST(DATA_LENGTH+INDEX_LENGTH AS SIGNED), TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA=? ORDER BY TABLE_NAME LIMIT ? OFFSET ?", schema, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, tableType string
		var tableRows, size sql.NullInt64
		var engine, collation, comment sql.NullString
		if err := rows.Scan(&name, &tableType, &tableRows, &engine, &collation, &size, &comment); err != nil {
			return result, err
		}
		if len(name) > MaxCellBytes || len(comment.String) > MaxCellBytes {
			return result, public("result_too_large", "Objektmetadaten sind zu groß.")
		}
		object := core.ObjectInfo{Schema: schema, Name: name, Kind: mysqlKind(tableType), Rows: core.RowCount{State: "unknown"}, Engine: nullStringNonEmpty(engine), Collation: nullStringNonEmpty(collation), SizeBytes: nullInt(size), Comment: nullStringNonEmpty(comment)}
		if object.Kind == "table" {
			if value := nullInt(tableRows); value != nil {
				object.Rows = core.RowCount{Value: value, State: "estimated"}
			}
		} else {
			object.Comment, object.SizeBytes = nil, nil
		}
		result.Objects = append(result.Objects, object)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.HasMore = int64(offset+len(result.Objects)) < result.Total
	return result, nil
}

func mysqlDefinition(ctx context.Context, db *sql.DB, c core.Connection, ref core.TableRef) (core.DefinitionResult, error) {
	result := core.DefinitionResult{Warnings: []string{}}
	schema, err := mysqlSchema(c, ref.Schema)
	if err != nil {
		return result, err
	}
	var name, tableType string
	err = db.QueryRowContext(ctx, "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", schema, ref.Name).Scan(&name, &tableType)
	if err == sql.ErrNoRows {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	if err != nil {
		return result, err
	}
	kind := mysqlKind(tableType)
	if kind != "table" && kind != "view" {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	// Both names come from the catalog row above and are quoted by dbplan.
	qualified, err := dbplan.QualifiedName("mysql", schema, name)
	if err != nil {
		return result, err
	}
	statement := "SHOW CREATE TABLE "
	if kind == "view" {
		statement = "SHOW CREATE VIEW "
	}
	rows, err := db.QueryContext(ctx, statement+qualified)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return result, err
	}
	if !rows.Next() {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	values := make([]sql.NullString, len(columns))
	dest := make([]any, len(columns))
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return result, err
	}
	if len(columns) < 2 || !values[1].Valid {
		return result, public("database_error", "Datenbankoperation fehlgeschlagen. Verbindung und SQL prüfen.")
	}
	result.Object = core.Table{Schema: schema, Name: name, Kind: kind}
	result.Definition, result.Source = values[1].String, "show_create"
	return result, rows.Err()
}

func mysqlServerInfo(ctx context.Context, db *sql.DB, result *core.ServerInfoResult) error {
	var comment, current sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment, DATABASE()").Scan(&result.Version, &comment, &current); err != nil {
		return err
	}
	result.VersionComment, result.CurrentDatabase = nullString(comment), nullString(current)
	result.Variant = "mysql"
	if strings.Contains(strings.ToLower(result.Version+" "+comment.String), "mariadb") {
		result.Variant = "mariadb"
	}
	var name string
	var uptime sql.NullString
	if err := db.QueryRowContext(ctx, "SHOW GLOBAL STATUS LIKE 'Uptime'").Scan(&name, &uptime); err == nil && uptime.Valid {
		if seconds, perr := strconv.ParseInt(uptime.String, 10, 64); perr == nil {
			result.UptimeSeconds = &seconds
		}
	} else if err != nil && err != sql.ErrNoRows {
		return err
	}
	rows, err := db.QueryContext(ctx, "SHOW SESSION VARIABLES WHERE Variable_name IN ('character_set_client','character_set_connection','character_set_results','character_set_database','character_set_server','collation_connection','collation_database','collation_server') ")
	if err != nil {
		return err
	}
	if _, err = scanStrings(rows, 2, 64, func(v []sql.NullString) error {
		result.Session = append(result.Session, core.NameValue{Name: v[0].String, Value: nullString(v[1])})
		return nil
	}); err != nil {
		return err
	}
	max := core.OverviewMaxListItems
	if rows, err = db.QueryContext(ctx, "SELECT CHARACTER_SET_NAME, DEFAULT_COLLATE_NAME, DESCRIPTION, MAXLEN FROM information_schema.CHARACTER_SETS ORDER BY CHARACTER_SET_NAME LIMIT ?", max+1); err != nil {
		return err
	}
	if result.CharacterSetsCapped, err = scanStrings(rows, 4, max, func(v []sql.NullString) error {
		item := core.CharacterSetInfo{Name: v[0].String, DefaultCollation: nullString(v[1]), Description: nullString(v[2])}
		if n, perr := strconv.Atoi(v[3].String); perr == nil {
			item.MaxBytes = &n
		}
		result.CharacterSets = append(result.CharacterSets, item)
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT COLLATION_NAME, CHARACTER_SET_NAME, IS_DEFAULT FROM information_schema.COLLATIONS ORDER BY COLLATION_NAME LIMIT ?", max+1); err != nil {
		return err
	}
	if result.CollationsCapped, err = scanStrings(rows, 3, max, func(v []sql.NullString) error {
		result.Collations = append(result.Collations, core.CollationInfo{Name: v[0].String, Charset: nullString(v[1]), Default: strings.EqualFold(v[2].String, "Yes")})
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT ENGINE, SUPPORT, COMMENT, TRANSACTIONS FROM information_schema.ENGINES ORDER BY ENGINE LIMIT ?", max); err != nil {
		return err
	}
	if _, err = scanStrings(rows, 4, max, func(v []sql.NullString) error {
		result.Engines = append(result.Engines, core.EngineInfo{Name: v[0].String, Support: v[1].String, Comment: nullStringNonEmpty(v[2]), Transactions: nullString(v[3])})
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT PLUGIN_NAME, PLUGIN_TYPE, PLUGIN_STATUS, PLUGIN_VERSION FROM information_schema.PLUGINS ORDER BY PLUGIN_NAME LIMIT ?", max+1); err != nil {
		return err
	}
	result.PluginsCapped, err = scanStrings(rows, 4, max, func(v []sql.NullString) error {
		result.Plugins = append(result.Plugins, core.PluginInfo{Name: v[0].String, Type: v[1].String, Status: v[2].String, Version: nullStringNonEmpty(v[3])})
		return nil
	})
	return err
}
