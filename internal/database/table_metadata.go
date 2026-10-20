// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func validTableRef(ref core.TableRef) bool {
	return ref.Name != "" && len(ref.Name) <= 1024 && len(ref.Schema) <= 1024 && !strings.ContainsRune(ref.Name, 0) && !strings.ContainsRune(ref.Schema, 0)
}
func (e *Engine) Describe(ctx context.Context, request core.DescribeRequest) (core.TableDescription, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !validTableRef(request.Table) {
		return core.TableDescription{}, public("invalid_table", "Tabellenbezug ist ungültig.")
	}
	db, closeDB, err := e.open(ctx, request.Connection)
	if err != nil {
		return core.TableDescription{}, err
	}
	defer closeDB()
	result, err := describe(ctx, db, request.Connection, request.Table)
	if err != nil {
		return core.TableDescription{}, safe(ctx, err)
	}
	encoded, _ := json.Marshal(result)
	if len(encoded)+1 > MaxResponseBytes {
		return core.TableDescription{}, public("result_too_large", "Metadaten überschreiten die erlaubte Größe.")
	}
	return result, nil
}

type metadataQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func describe(ctx context.Context, db metadataQuerier, c core.Connection, ref core.TableRef) (core.TableDescription, error) {
	d := core.TableDescription{Table: ref, Columns: []core.TableColumn{}, PrimaryKey: []string{}, ReadOnly: true, Transactional: "unknown"}
	var rows *sql.Rows
	var err error
	switch c.Driver {
	case "sqlite":
		if ref.Schema == "" {
			ref.Schema = "main"
		}
		if ref.Schema != "main" || strings.HasPrefix(strings.ToLower(ref.Name), "sqlite_") {
			return d, public("invalid_table", "Tabellenbezug ist nicht verfügbar.")
		}
		var definition string
		err = db.QueryRowContext(ctx, "SELECT name,type,COALESCE(sql,'') FROM main.sqlite_schema WHERE name=? AND type IN ('table','view')", ref.Name).Scan(&ref.Name, &d.Kind, &definition)
		if err != nil {
			return d, tableLookupError(err)
		}
		d.StorageEngine = "sqlite"
		d.Transactional = "yes"
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(definition)), "CREATE VIRTUAL TABLE") {
			d.Kind = "virtual"
			d.Transactional = "unknown"
		}
		rows, err = db.QueryContext(ctx, `SELECT name,type,"notnull",dflt_value,pk,hidden FROM pragma_table_xinfo(?) ORDER BY cid`, ref.Name)
	case "mysql":
		if ref.Schema == "" {
			ref.Schema = c.Database
		}
		if ref.Schema == "" {
			return d, public("invalid_table", "Datenbankschema ist erforderlich.")
		}
		var tableType, engine sql.NullString
		err = db.QueryRowContext(ctx, "SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE,ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?", ref.Schema, ref.Name).Scan(&ref.Schema, &ref.Name, &tableType, &engine)
		if err != nil {
			return d, tableLookupError(err)
		}
		d.Kind = "table"
		if tableType.String == "VIEW" {
			d.Kind = "view"
		}
		d.StorageEngine = engine.String
		switch strings.ToUpper(engine.String) {
		case "INNODB":
			d.Transactional = "yes"
		case "MYISAM", "MEMORY", "CSV", "ARCHIVE":
			d.Transactional = "no"
		}
		rows, err = db.QueryContext(ctx, `SELECT c.COLUMN_NAME,c.COLUMN_TYPE,c.IS_NULLABLE,c.COLUMN_DEFAULT,COALESCE(k.ORDINAL_POSITION,0),c.EXTRA FROM information_schema.COLUMNS c LEFT JOIN information_schema.KEY_COLUMN_USAGE k ON k.TABLE_SCHEMA=c.TABLE_SCHEMA AND k.TABLE_NAME=c.TABLE_NAME AND k.COLUMN_NAME=c.COLUMN_NAME AND k.CONSTRAINT_NAME='PRIMARY' WHERE c.TABLE_SCHEMA=? AND c.TABLE_NAME=? ORDER BY c.ORDINAL_POSITION`, ref.Schema, ref.Name)
	case "postgres":
		if ref.Schema == "" {
			ref.Schema = "public"
		}
		var relationKind string
		err = db.QueryRowContext(ctx, `SELECT n.nspname,c.relname,c.relkind::text FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p','v','m','f')`, ref.Schema, ref.Name).Scan(&ref.Schema, &ref.Name, &relationKind)
		if err != nil {
			return d, tableLookupError(err)
		}
		d.Kind = "table"
		d.StorageEngine = "postgres"
		d.Transactional = "yes"
		if relationKind == "v" || relationKind == "m" {
			d.Kind = "view"
		}
		if relationKind == "f" {
			d.Kind = "foreign"
			d.Transactional = "unknown"
		}
		rows, err = db.QueryContext(ctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),NOT a.attnotnull,pg_catalog.pg_get_expr(ad.adbin,ad.adrelid),COALESCE((SELECT k.ordinality::integer FROM pg_catalog.pg_index i CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(number,ordinality) WHERE i.indrelid=c.oid AND i.indisprimary AND k.number=a.attnum AND k.ordinality<=i.indnkeyatts),0),CASE WHEN a.attgenerated='s' THEN 'stored' WHEN a.attgenerated='v' THEN 'virtual' WHEN a.attidentity<>'' THEN 'identity' ELSE 'none' END FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid=c.oid AND ad.adnum=a.attnum WHERE n.nspname=$1 AND c.relname=$2 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, ref.Schema, ref.Name)
	}
	if err != nil {
		return d, err
	}
	defer rows.Close()
	metadataBytes := len(ref.Name) + len(ref.Schema) + 512
	for rows.Next() {
		if len(d.Columns) >= 1024 {
			return d, public("result_too_large", "Zu viele Tabellenspalten.")
		}
		col := core.TableColumn{Generated: "none", ReadOnly: true}
		var defaultValue sql.NullString
		switch c.Driver {
		case "sqlite":
			var notNull, hidden int
			err = rows.Scan(&col.Name, &col.DatabaseType, &notNull, &defaultValue, &col.PKPosition, &hidden)
			col.Nullable = notNull == 0
			if hidden == 2 {
				col.Generated = "virtual"
			}
			if hidden == 3 {
				col.Generated = "stored"
			}
			if hidden == 1 {
				col.Generated = "hidden"
			}
		case "mysql":
			var nullable, extra string
			err = rows.Scan(&col.Name, &col.DatabaseType, &nullable, &defaultValue, &col.PKPosition, &extra)
			col.Nullable = nullable == "YES"
			switch {
			case strings.Contains(extra, "VIRTUAL GENERATED"):
				col.Generated = "virtual"
			case strings.Contains(extra, "STORED GENERATED"):
				col.Generated = "stored"
			case strings.Contains(extra, "auto_increment"):
				col.Generated = "identity"
			}
		case "postgres":
			err = rows.Scan(&col.Name, &col.DatabaseType, &col.Nullable, &defaultValue, &col.PKPosition, &col.Generated)
		}
		if err != nil {
			return d, err
		}
		if defaultValue.Valid {
			value := defaultValue.String
			col.DefaultValue = &value
		}
		if len(col.Name) > 1024 || len(col.DatabaseType) > MaxCellBytes || (col.DefaultValue != nil && len(*col.DefaultValue) > MaxCellBytes) {
			return d, public("result_too_large", "Spaltenmetadaten sind zu groß.")
		}
		encoded, _ := json.Marshal(col)
		metadataBytes += len(encoded) + 1
		if metadataBytes > MaxResponseBytes {
			return d, public("result_too_large", "Metadaten überschreiten die erlaubte Größe.")
		}
		d.Columns = append(d.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	if len(d.Columns) == 0 {
		return d, public("table_not_found", "Tabelle oder Spalten sind nicht verfügbar.")
	}
	rows.Close()
	d.Table = ref
	keys := []core.TableColumn{}
	for _, column := range d.Columns {
		if column.PKPosition > 0 {
			keys = append(keys, column)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].PKPosition < keys[j].PKPosition })
	if c.Driver == "sqlite" && len(keys) == 1 && strings.EqualFold(keys[0].DatabaseType, "INTEGER") {
		var primaryIndex int
		aliasErr := db.QueryRowContext(ctx, "SELECT 1 FROM pragma_index_list(?) WHERE origin='pk' LIMIT 1", ref.Name).Scan(&primaryIndex)
		if aliasErr != nil && aliasErr != sql.ErrNoRows {
			return d, aliasErr
		}
		if aliasErr == sql.ErrNoRows {
			for i := range d.Columns {
				if d.Columns[i].PKPosition == 1 {
					d.Columns[i].Nullable = false
					keys[0].Nullable = false
				}
			}
		}
	}
	d.StableOrder = len(keys) > 0
	for _, key := range keys {
		d.PrimaryKey = append(d.PrimaryKey, key.Name)
		if key.Nullable {
			d.StableOrder = false
		}
	}
	return d, nil
}
func tableLookupError(err error) error {
	if err == sql.ErrNoRows {
		return public("table_not_found", "Tabelle ist nicht verfügbar.")
	}
	return err
}
func quoteIdentifier(driver, value string) string {
	if driver == "mysql" {
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
