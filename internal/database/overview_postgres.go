// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// pgVisible mirrors the visibility rule of information_schema.tables: objects
// the role owns or holds some privilege on. System schemas are excluded.
const pgVisible = `n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp\_%'
 AND (pg_has_role(c.relowner,'USAGE') OR has_table_privilege(c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER') OR has_any_column_privilege(c.oid,'SELECT,INSERT,UPDATE,REFERENCES'))`

func postgresDatabases(ctx context.Context, db *sql.DB, limit, offset int) (core.DatabasesResult, error) {
	result := core.DatabasesResult{Databases: []core.DatabaseInfo{}}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_database WHERE datallowconn").Scan(&result.Total); err != nil {
		return result, err
	}
	var objects sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','f') AND "+pgVisible).Scan(&objects); err != nil {
		return result, err
	}
	rows, err := db.QueryContext(ctx, `SELECT datname, pg_encoding_to_char(encoding), COALESCE(datcollate, datlocale), datname = current_database(), datistemplate,
 CASE WHEN has_database_privilege(datname,'CONNECT') THEN pg_database_size(datname) END
FROM pg_database WHERE datallowconn ORDER BY datname LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var charset, collation sql.NullString
		var current, template bool
		var size sql.NullInt64
		if err := rows.Scan(&name, &charset, &collation, &current, &template, &size); err != nil {
			return result, err
		}
		info := core.DatabaseInfo{Name: name, Charset: nullString(charset), Collation: nullString(collation), Current: current, System: template || name == "postgres", SizeBytes: nullInt(size)}
		if current {
			info.ObjectCount = nullInt(objects)
		}
		result.Databases = append(result.Databases, info)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.HasMore = int64(offset+len(result.Databases)) < result.Total
	return result, nil
}

func pgKind(relkind string) string {
	switch relkind {
	case "r", "p":
		return "table"
	case "v":
		return "view"
	case "m":
		return "materialized_view"
	case "f":
		return "foreign_table"
	}
	return "other"
}

func postgresObjects(ctx context.Context, db *sql.DB, schema string, limit, offset int) (core.ObjectsResult, error) {
	result := core.ObjectsResult{Schema: schema, Objects: []core.ObjectInfo{}}
	where := "c.relkind IN ('r','p','v','m','f') AND ($1::text = '' OR n.nspname = $1::text) AND " + pgVisible
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE "+where, schema).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := db.QueryContext(ctx, `SELECT n.nspname, c.relname, c.relkind::text, c.reltuples::bigint,
 CASE WHEN c.relkind IN ('r','p','m') THEN pg_total_relation_size(c.oid) END, obj_description(c.oid,'pg_class')
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE `+where+` ORDER BY n.nspname, c.relname LIMIT $2 OFFSET $3`, schema, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var ns, name, relkind string
		var tuples, size sql.NullInt64
		var comment sql.NullString
		if err := rows.Scan(&ns, &name, &relkind, &tuples, &size, &comment); err != nil {
			return result, err
		}
		if len(name) > MaxCellBytes || len(ns) > MaxCellBytes || len(comment.String) > MaxCellBytes {
			return result, public("result_too_large", "Objektmetadaten sind zu groß.")
		}
		object := core.ObjectInfo{Schema: ns, Name: name, Kind: pgKind(relkind), Rows: core.RowCount{State: "unknown"}, SizeBytes: nullInt(size), Comment: nullStringNonEmpty(comment)}
		// reltuples is -1 until the first VACUUM/ANALYZE: not an estimate yet.
		if relkind != "v" && tuples.Valid && tuples.Int64 >= 0 {
			object.Rows = core.RowCount{Value: nullInt(tuples), State: "estimated"}
		}
		result.Objects = append(result.Objects, object)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.HasMore = int64(offset+len(result.Objects)) < result.Total
	return result, nil
}

func postgresDefinition(ctx context.Context, db *sql.DB, ref core.TableRef) (core.DefinitionResult, error) {
	result := core.DefinitionResult{Warnings: []string{}}
	var oid, ns, name, relkind string
	err := db.QueryRowContext(ctx, `SELECT c.oid::text, n.nspname, c.relname, c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE c.relkind IN ('r','p','v','m') AND n.nspname = COALESCE(NULLIF($1::text,''), current_schema()) AND c.relname = $2 AND `+pgVisible, ref.Schema, ref.Name).Scan(&oid, &ns, &name, &relkind)
	if err == sql.ErrNoRows {
		return result, public("object_not_found", "Objekt wurde nicht gefunden.")
	}
	if err != nil {
		return result, err
	}
	qualified, err := dbplan.QualifiedName("postgres", ns, name)
	if err != nil {
		return result, err
	}
	result.Object = core.Table{Schema: ns, Name: name, Kind: pgKind(relkind)}
	result.Derived, result.Source = true, "catalog_reconstruction"
	notice := "Aus dem Katalog rekonstruiert; PostgreSQL besitzt kein SHOW CREATE. Die Anweisung kann von der ursprünglichen abweichen."
	result.Notice = &notice
	if relkind == "v" || relkind == "m" {
		var def sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT pg_get_viewdef($1::oid, true)", oid).Scan(&def); err != nil {
			return result, err
		}
		head := "CREATE VIEW "
		if relkind == "m" {
			head = "CREATE MATERIALIZED VIEW "
		}
		result.Definition = head + qualified + " AS\n" + strings.TrimSpace(def.String)
		result.Warnings = append(result.Warnings, "Rechte, Kommentare, Trigger und Storage-Parameter sind nicht enthalten.")
		return result, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT a.attname, format_type(a.atttypid,a.atttypmod), a.attnotnull, pg_get_expr(d.adbin,d.adrelid), a.attgenerated::text, a.attidentity::text
FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
WHERE a.attrelid=$1::oid AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, oid)
	if err != nil {
		return result, err
	}
	var parts []string
	for rows.Next() {
		var column, typ, generated, identity string
		var notNull bool
		var def sql.NullString
		if err := rows.Scan(&column, &typ, &notNull, &def, &generated, &identity); err != nil {
			rows.Close()
			return result, err
		}
		quoted, qerr := dbplan.QuoteIdentifier("postgres", column)
		if qerr != nil {
			rows.Close()
			return result, qerr
		}
		line := "  " + quoted + " " + typ
		switch {
		case identity == "a":
			line += " GENERATED ALWAYS AS IDENTITY"
		case identity == "d":
			line += " GENERATED BY DEFAULT AS IDENTITY"
		case generated == "s" && def.Valid:
			line += " GENERATED ALWAYS AS (" + def.String + ") STORED"
		case def.Valid:
			line += " DEFAULT " + def.String
		}
		if notNull {
			line += " NOT NULL"
		}
		parts = append(parts, line)
		if len(parts) > 4096 {
			rows.Close()
			return result, public("result_too_large", "Definition überschreitet die erlaubte Größe.")
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, "SELECT conname, pg_get_constraintdef(oid, true) FROM pg_constraint WHERE conrelid=$1::oid AND contype IN ('p','u','c','f','x') ORDER BY contype DESC, conname", oid)
	if err != nil {
		return result, err
	}
	if _, err = scanStrings(rows, 2, 4096, func(v []sql.NullString) error {
		quoted, qerr := dbplan.QuoteIdentifier("postgres", v[0].String)
		if qerr != nil {
			return qerr
		}
		parts = append(parts, "  CONSTRAINT "+quoted+" "+v[1].String)
		return nil
	}); err != nil {
		return result, err
	}
	text := "CREATE TABLE " + qualified + " (\n" + strings.Join(parts, ",\n") + "\n);"
	rows, err = db.QueryContext(ctx, `SELECT pg_get_indexdef(i.indexrelid) FROM pg_index i WHERE i.indrelid=$1::oid
AND NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conindid=i.indexrelid AND k.conrelid=i.indrelid) ORDER BY 1`, oid)
	if err != nil {
		return result, err
	}
	if _, err = scanStrings(rows, 1, 4096, func(v []sql.NullString) error {
		text += "\n" + v[0].String + ";"
		return nil
	}); err != nil {
		return result, err
	}
	result.Definition = text
	if relkind == "p" {
		result.Warnings = append(result.Warnings, "Partitionsschlüssel und Partitionen sind nicht enthalten.")
	}
	result.Warnings = append(result.Warnings, "Rechte, Kommentare, Trigger, Regeln und Storage-Parameter sind nicht enthalten.")
	return result, nil
}

func postgresServerInfo(ctx context.Context, db *sql.DB, result *core.ServerInfoResult) error {
	var full string
	var uptime sql.NullInt64
	var current sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version'), version(), current_database(), EXTRACT(EPOCH FROM now()-pg_postmaster_start_time())::bigint").Scan(&result.Version, &full, &current, &uptime); err != nil {
		return err
	}
	result.Variant, result.VersionComment, result.CurrentDatabase, result.UptimeSeconds = "postgres", &full, nullString(current), nullInt(uptime)
	rows, err := db.QueryContext(ctx, "SELECT n, current_setting(n, true) FROM unnest(ARRAY['server_encoding','client_encoding','lc_collate','lc_ctype','DateStyle','TimeZone','standard_conforming_strings']) n")
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
	if rows, err = db.QueryContext(ctx, "SELECT DISTINCT pg_encoding_to_char(i) e FROM generate_series(0,60) i WHERE pg_encoding_to_char(i) <> '' ORDER BY e"); err != nil {
		return err
	}
	if result.CharacterSetsCapped, err = scanStrings(rows, 1, max, func(v []sql.NullString) error {
		result.CharacterSets = append(result.CharacterSets, core.CharacterSetInfo{Name: v[0].String})
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT collname, pg_encoding_to_char(collencoding) FROM pg_collation ORDER BY collname, collencoding LIMIT $1", max+1); err != nil {
		return err
	}
	if result.CollationsCapped, err = scanStrings(rows, 2, max, func(v []sql.NullString) error {
		var charset *string
		if v[1].Valid && v[1].String != "" {
			charset = nullString(v[1])
		}
		result.Collations = append(result.Collations, core.CollationInfo{Name: v[0].String, Charset: charset})
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT amname FROM pg_am WHERE amtype='t' ORDER BY amname"); err != nil {
		return err
	}
	if _, err = scanStrings(rows, 1, max, func(v []sql.NullString) error {
		yes := "YES"
		result.Engines = append(result.Engines, core.EngineInfo{Name: v[0].String, Support: "YES", Comment: stringPointer("table access method"), Transactions: &yes})
		return nil
	}); err != nil {
		return err
	}
	if rows, err = db.QueryContext(ctx, "SELECT extname, extversion FROM pg_extension ORDER BY extname LIMIT $1", max+1); err != nil {
		return err
	}
	result.PluginsCapped, err = scanStrings(rows, 2, max, func(v []sql.NullString) error {
		result.Plugins = append(result.Plugins, core.PluginInfo{Name: v[0].String, Type: "extension", Status: "INSTALLED", Version: nullStringNonEmpty(v[1])})
		return nil
	})
	return err
}
