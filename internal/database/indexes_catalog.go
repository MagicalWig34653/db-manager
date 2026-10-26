// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"strings"
)

type indexObject struct{ name, kind string }

// indexObjects reads names and ownership only. No catalog SQL is replayed.
func indexObjects(ctx context.Context, s *mutationSession, d core.TableDescription) ([]indexObject, error) {
	var query string
	var args []any
	switch s.driver {
	case "sqlite":
		query = `SELECT name,CASE WHEN origin='c' THEN 'index' ELSE 'owned' END FROM pragma_index_list(?) ORDER BY name LIMIT 1025`
		args = []any{d.Table.Name}
	case "mysql":
		query = `SELECT INDEX_NAME,CASE WHEN INDEX_NAME='PRIMARY' THEN 'primary' ELSE 'index' END FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? GROUP BY INDEX_NAME UNION ALL SELECT CONSTRAINT_NAME,'foreign' FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=? AND TABLE_NAME=? ORDER BY 1 LIMIT 1025`
		args = []any{d.Table.Schema, d.Table.Name, d.Table.Schema, d.Table.Name}
	case "postgres":
		query = `SELECT ic.relname,CASE WHEN EXISTS(SELECT 1 FROM pg_catalog.pg_constraint co WHERE co.conindid=i.indexrelid AND co.conrelid=t.oid AND co.contype IN ('p','u','x')) THEN 'owned' ELSE 'index' END FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class t ON t.oid=i.indrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace JOIN pg_catalog.pg_class ic ON ic.oid=i.indexrelid WHERE n.nspname=$1 AND t.relname=$2 UNION ALL SELECT co.conname,CASE co.contype WHEN 'p' THEN 'primary' WHEN 'f' THEN 'foreign' ELSE 'constraint' END FROM pg_catalog.pg_constraint co JOIN pg_catalog.pg_class t ON t.oid=co.conrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname=$1 AND t.relname=$2 ORDER BY 1 LIMIT 1025`
		args = []any{d.Table.Schema, d.Table.Name}
	}
	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []indexObject{}
	for rows.Next() {
		var o indexObject
		if err = rows.Scan(&o.name, &o.kind); err != nil {
			return nil, err
		}
		out = append(out, o)
		if len(out) > 1024 {
			return nil, public("result_too_large", "Zu viele Schlüsselobjekte.")
		}
	}
	return out, rows.Err()
}
func indexTable(ctx context.Context, s *mutationSession, c core.Connection, ref core.TableRef) (core.TableDescription, error) {
	d, err := describe(ctx, s, c, ref)
	if err != nil {
		return d, err
	}
	if d.Kind != "table" || d.Table.Name != ref.Name || ref.Schema != "" && ref.Schema != d.Table.Schema {
		return d, invalidIndexes()
	}
	if c.Driver == "sqlite" && strings.HasPrefix(strings.ToLower(d.Table.Name), "sqlite_") {
		return d, invalidIndexes()
	}
	lower := strings.ToLower(d.Table.Schema)
	if c.Driver == "postgres" && (strings.HasPrefix(lower, "pg_") || lower == "information_schema") || c.Driver == "mysql" && mysqlSystemSchemas[lower] {
		return d, invalidIndexes()
	}
	if c.Driver == "mysql" && !strings.EqualFold(d.StorageEngine, "InnoDB") {
		return d, unsupportedIndexes()
	}
	if c.Driver == "postgres" {
		var special bool
		err = s.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&special)
		if err != nil {
			return d, err
		}
		if special {
			return d, unsupportedIndexes()
		}
	}
	return d, nil
}
func indexColumn(d core.TableDescription, name string) (core.TableColumn, error) {
	for _, c := range d.Columns {
		if c.Name == name {
			if c.Generated != "none" && c.Generated != "identity" {
				return c, unsupportedIndexes()
			}
			return c, nil
		}
	}
	return core.TableColumn{}, public("column_not_found", "Die Spalte ist nicht vorhanden.")
}
func indexTextType(c core.TableColumn) bool {
	typ := strings.ToLower(c.DatabaseType)
	return strings.Contains(typ, "char") || strings.Contains(typ, "text")
}
