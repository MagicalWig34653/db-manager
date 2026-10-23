// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func pgCatalogColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var available bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_attribute WHERE attrelid=$1::regclass AND attname=$2 AND NOT attisdropped)`, "pg_catalog."+table, column).Scan(&available)
	return available, err
}
func postgresStructure(ctx context.Context, db *sql.DB, d core.TableDescription, result *core.TableStructure) error {
	distinct, err := pgCatalogColumn(ctx, db, "pg_index", "indnullsnotdistinct")
	if err != nil {
		return err
	}
	distinctSQL := "NULL::boolean"
	if distinct {
		distinctSQL = "i.indnullsnotdistinct"
	}
	rows, err := db.QueryContext(ctx, `SELECT ic.relname,i.indisunique,i.indisprimary,am.amname,i.indpred IS NOT NULL,i.indisvalid,`+distinctSQL+`,k.ordinality,k.attnum,a.attname,k.ordinality<=i.indnkeyatts,pg_catalog.pg_index_column_has_property(i.indexrelid,k.ordinality::integer,'asc'),pg_catalog.pg_index_column_has_property(i.indexrelid,k.ordinality::integer,'desc'),i.indcollation[(k.ordinality-1)::integer]::bigint,cn.nspname,co.collname FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class t ON t.oid=i.indrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace JOIN pg_catalog.pg_class ic ON ic.oid=i.indexrelid JOIN pg_catalog.pg_am am ON am.oid=ic.relam CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum,ordinality) LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.attnum LEFT JOIN pg_catalog.pg_collation co ON co.oid=i.indcollation[(k.ordinality-1)::integer] LEFT JOIN pg_catalog.pg_namespace cn ON cn.oid=co.collnamespace WHERE n.nspname=$1 AND t.relname=$2 ORDER BY ic.relname,k.ordinality`, d.Table.Schema, d.Table.Name)
	if err != nil {
		return err
	}
	previous := ""
	first := true
	terms := 0
	for rows.Next() {
		var name, method string
		var unique, primary, partial, valid, key bool
		var nulls, asc, desc sql.NullBool
		var position, attnum int
		var column, collSchema, collName sql.NullString
		var collID sql.NullInt64
		if err = rows.Scan(&name, &unique, &primary, &method, &partial, &valid, &nulls, &position, &attnum, &column, &key, &asc, &desc, &collID, &collSchema, &collName); err != nil {
			rows.Close()
			return err
		}
		if first || name != previous {
			nullsDistinct := "not_applicable"
			if unique {
				nullsDistinct = "unknown"
				if nulls.Valid {
					nullsDistinct = yesNo(!nulls.Bool)
				}
			}
			result.Indexes = append(result.Indexes, core.StructureIndex{Name: name, Unique: yesNo(unique), Primary: yesNo(primary), Method: method, MethodKnown: true, Partial: yesNo(partial), Valid: yesNo(valid), NullsDistinct: nullsDistinct, Terms: []core.IndexTerm{}})
			previous = name
			first = false
		}
		term := emptyTerm(position)
		term.PrefixState = "none"
		term.PrefixUnit = "not_applicable"
		term.Role = "include"
		if key {
			term.Role = "key"
		}
		if attnum == 0 {
			term.Kind = "expression"
		} else if column.Valid {
			term.Kind = "column"
			value := column.String
			term.Column = &value
		}
		if !key {
			term.Direction = "not_applicable"
			term.CollationState = "not_applicable"
		} else {
			if desc.Valid && desc.Bool {
				term.Direction = "desc"
			} else if asc.Valid && asc.Bool {
				term.Direction = "asc"
			}
			if collID.Valid && collID.Int64 == 0 {
				term.CollationState = "not_applicable"
			} else if collName.Valid && collSchema.Valid {
				term.CollationState = "known"
				term.Collation = &core.TableRef{Schema: collSchema.String, Name: collName.String}
			}
		}
		index := &result.Indexes[len(result.Indexes)-1]
		index.Terms = append(index.Terms, term)
		terms++
		if len(result.Indexes) > MaxStructureObjects || len(index.Terms) > MaxStructureObjects || terms > MaxStructureTerms {
			rows.Close()
			return public("result_too_large", "Zu viele Indexmetadaten.")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = structureBudget(*result); err != nil {
		return err
	}
	enforced, err := pgCatalogColumn(ctx, db, "pg_constraint", "conenforced")
	if err != nil {
		return err
	}
	enforcedSQL := "NULL::boolean"
	if enforced {
		enforcedSQL = "c.conenforced"
	}
	rows, err = db.QueryContext(ctx, `SELECT c.oid::text,c.conname,c.contype::text,c.condeferrable,c.condeferred,`+enforcedSQL+`,c.convalidated,c.conkey[s.position],a.attname,rn.nspname,rt.relname,c.confkey[s.position],ra.attname,c.confupdtype::text,c.confdeltype::text FROM pg_catalog.pg_constraint c JOIN pg_catalog.pg_class t ON t.oid=c.conrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace LEFT JOIN pg_catalog.pg_class rt ON rt.oid=c.confrelid LEFT JOIN pg_catalog.pg_namespace rn ON rn.oid=rt.relnamespace CROSS JOIN LATERAL generate_series(1,GREATEST(COALESCE(array_length(c.conkey,1),0),COALESCE(array_length(c.confkey,1),0),1)) s(position) LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid=t.oid AND a.attnum=c.conkey[s.position] LEFT JOIN pg_catalog.pg_attribute ra ON ra.attrelid=rt.oid AND ra.attnum=c.confkey[s.position] WHERE n.nspname=$1 AND t.relname=$2 ORDER BY c.conname,c.oid,s.position`, d.Table.Schema, d.Table.Name)
	if err != nil {
		return err
	}
	defer rows.Close()
	previous = ""
	first = true
	for rows.Next() {
		var id, name, typ, update, delete string
		var deferrable, deferred, validated bool
		var enforced sql.NullBool
		var attnum, refnum sql.NullInt64
		var column, schema, table, target sql.NullString
		if err = rows.Scan(&id, &name, &typ, &deferrable, &deferred, &enforced, &validated, &attnum, &column, &schema, &table, &refnum, &target, &update, &delete); err != nil {
			return err
		}
		if first || id != previous {
			kind := "other"
			switch typ {
			case "p":
				kind = "primary_key"
			case "u":
				kind = "unique"
			case "f":
				kind = "foreign_key"
			case "c":
				kind = "check"
			case "n":
				kind = "not_null"
			}
			constraint := emptyConstraint(kind)
			constraint.Name = name
			constraint.NameKnown = true
			constraint.Deferrable = yesNo(deferrable)
			constraint.InitiallyDeferred = yesNo(deferred)
			constraint.Validated = yesNo(validated)
			if enforced.Valid {
				constraint.Enforced = yesNo(enforced.Bool)
			}
			constraint.ColumnsKnown = kind != "other"
			if kind == "foreign_key" {
				constraint.OnUpdate = fkAction(update)
				constraint.OnDelete = fkAction(delete)
				constraint.ReferencedColumnsKnown = true
				if schema.Valid && table.Valid {
					constraint.ReferencedTable = &core.TableRef{Schema: schema.String, Name: table.String}
				}
			}
			result.Constraints = append(result.Constraints, constraint)
			previous = id
			first = false
		}
		constraint := &result.Constraints[len(result.Constraints)-1]
		if constraint.ColumnsKnown && attnum.Valid {
			if attnum.Int64 > 0 && column.Valid {
				constraint.Columns = append(constraint.Columns, column.String)
			} else {
				constraint.ColumnsKnown = false
				constraint.Columns = []string{}
			}
		}
		if constraint.Kind == "foreign_key" {
			if constraint.ReferencedColumnsKnown && refnum.Valid && refnum.Int64 > 0 && target.Valid {
				constraint.ReferencedColumns = append(constraint.ReferencedColumns, target.String)
			} else {
				constraint.ReferencedColumnsKnown = false
				constraint.ReferencedColumns = []string{}
			}
		}
		if err = structureBudget(*result); err != nil {
			return err
		}
	}
	return rows.Err()
}
