// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func mysqlCatalogColumn(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	var available bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='information_schema' AND TABLE_NAME=? AND COLUMN_NAME=?)`, table, column).Scan(&available)
	return available, err
}
func mysqlStructure(ctx context.Context, db *sql.DB, d core.TableDescription, result *core.TableStructure) error {
	expression, err := mysqlCatalogColumn(ctx, db, "STATISTICS", "EXPRESSION")
	if err != nil {
		return err
	}
	expressionSQL := "NULL"
	if expression {
		expressionSQL = "s.EXPRESSION IS NOT NULL"
	}
	rows, err := db.QueryContext(ctx, `SELECT s.INDEX_NAME,s.NON_UNIQUE,s.INDEX_TYPE,s.SEQ_IN_INDEX,s.COLUMN_NAME,s.COLLATION,s.SUB_PART,EXISTS(SELECT 1 FROM information_schema.TABLE_CONSTRAINTS t WHERE t.TABLE_SCHEMA=s.TABLE_SCHEMA AND t.TABLE_NAME=s.TABLE_NAME AND t.CONSTRAINT_NAME=s.INDEX_NAME AND t.CONSTRAINT_TYPE='PRIMARY KEY'),`+expressionSQL+`,c.COLLATION_NAME FROM information_schema.STATISTICS s LEFT JOIN information_schema.COLUMNS c ON c.TABLE_SCHEMA=s.TABLE_SCHEMA AND c.TABLE_NAME=s.TABLE_NAME AND c.COLUMN_NAME=s.COLUMN_NAME WHERE s.TABLE_SCHEMA=? AND s.TABLE_NAME=? ORDER BY s.INDEX_NAME,s.SEQ_IN_INDEX`, d.Table.Schema, d.Table.Name)
	if err != nil {
		return err
	}
	previous := ""
	first := true
	terms := 0
	for rows.Next() {
		var name, method string
		var nonUnique, position int
		var column, direction, coll sql.NullString
		var prefix sql.NullInt64
		var primary bool
		var expression sql.NullBool
		if err = rows.Scan(&name, &nonUnique, &method, &position, &column, &direction, &prefix, &primary, &expression, &coll); err != nil {
			rows.Close()
			return err
		}
		if first || name != previous {
			unique := "unknown"
			if nonUnique == 0 {
				unique = "yes"
			} else if nonUnique == 1 {
				unique = "no"
			}
			distinct := "unknown"
			if unique == "no" {
				distinct = "not_applicable"
			}
			result.Indexes = append(result.Indexes, core.StructureIndex{Name: name, Unique: unique, Primary: yesNo(primary), Method: method, MethodKnown: true, Partial: "unknown", Valid: "unknown", NullsDistinct: distinct, Terms: []core.IndexTerm{}})
			first = false
			previous = name
		}
		term := emptyTerm(position)
		term.Role = "key"
		term.PrefixState = "none"
		term.PrefixUnit = "not_applicable"
		if column.Valid {
			term.Kind = "column"
			value := column.String
			term.Column = &value
		} else if expression.Valid && expression.Bool {
			term.Kind = "expression"
		}
		switch direction.String {
		case "A":
			term.Direction = "asc"
		case "D":
			term.Direction = "desc"
		default:
			if !direction.Valid {
				term.Direction = "not_applicable"
			}
		}
		if coll.Valid {
			term.CollationState = "known"
			term.Collation = &core.TableRef{Name: coll.String}
		}
		if prefix.Valid {
			if prefix.Int64 < 1 || prefix.Int64 > MaxCellBytes {
				rows.Close()
				return public("metadata_error", "Indexpräfix ist nicht darstellbar.")
			}
			value := int(prefix.Int64)
			term.PrefixState = "present"
			term.PrefixUnit = "unknown"
			term.PrefixLength = &value
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
	enforced, err := mysqlCatalogColumn(ctx, db, "TABLE_CONSTRAINTS", "ENFORCED")
	if err != nil {
		return err
	}
	enforcedSQL := "NULL"
	if enforced {
		enforcedSQL = "t.ENFORCED"
	}
	// Constraint names are only unique within each type namespace. KCU has no type
	// column, so distinguish foreign keys by their non-NULL referenced table.
	rows, err = db.QueryContext(ctx, `SELECT t.CONSTRAINT_NAME,t.CONSTRAINT_TYPE,`+enforcedSQL+`,k.COLUMN_NAME,k.REFERENCED_TABLE_SCHEMA,k.REFERENCED_TABLE_NAME,k.REFERENCED_COLUMN_NAME,r.UPDATE_RULE,r.DELETE_RULE FROM information_schema.TABLE_CONSTRAINTS t LEFT JOIN information_schema.KEY_COLUMN_USAGE k ON k.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA AND k.TABLE_SCHEMA=t.TABLE_SCHEMA AND k.TABLE_NAME=t.TABLE_NAME AND k.CONSTRAINT_NAME=t.CONSTRAINT_NAME AND ((t.CONSTRAINT_TYPE IN ('PRIMARY KEY','UNIQUE') AND k.REFERENCED_TABLE_NAME IS NULL) OR (t.CONSTRAINT_TYPE='FOREIGN KEY' AND k.REFERENCED_TABLE_NAME IS NOT NULL)) LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON t.CONSTRAINT_TYPE='FOREIGN KEY' AND r.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA AND r.TABLE_NAME=t.TABLE_NAME AND r.CONSTRAINT_NAME=t.CONSTRAINT_NAME WHERE t.TABLE_SCHEMA=? AND t.TABLE_NAME=? ORDER BY t.CONSTRAINT_NAME,t.CONSTRAINT_TYPE,k.ORDINAL_POSITION`, d.Table.Schema, d.Table.Name)
	if err != nil {
		return err
	}
	defer rows.Close()
	first = true
	previous = ""
	previousType := ""
	for rows.Next() {
		var name, typ string
		var enforcement, column, schema, table, target, update, delete sql.NullString
		if err = rows.Scan(&name, &typ, &enforcement, &column, &schema, &table, &target, &update, &delete); err != nil {
			return err
		}
		if first || name != previous || typ != previousType {
			kind := "other"
			switch typ {
			case "PRIMARY KEY":
				kind = "primary_key"
			case "UNIQUE":
				kind = "unique"
			case "FOREIGN KEY":
				kind = "foreign_key"
			case "CHECK":
				kind = "check"
			}
			constraint := emptyConstraint(kind)
			constraint.Name = name
			constraint.NameKnown = true
			constraint.ColumnsKnown = kind == "primary_key" || kind == "unique" || kind == "foreign_key"
			if enforcement.Valid {
				switch enforcement.String {
				case "YES":
					constraint.Enforced = "yes"
				case "NO":
					constraint.Enforced = "no"
				}
			}
			if kind == "foreign_key" {
				constraint.OnUpdate = fkAction(update.String)
				constraint.OnDelete = fkAction(delete.String)
				constraint.ReferencedColumnsKnown = true
				if schema.Valid && table.Valid {
					constraint.ReferencedTable = &core.TableRef{Schema: schema.String, Name: table.String}
				}
			}
			result.Constraints = append(result.Constraints, constraint)
			first = false
			previous = name
			previousType = typ
		}
		constraint := &result.Constraints[len(result.Constraints)-1]
		if constraint.ColumnsKnown {
			if column.Valid {
				constraint.Columns = append(constraint.Columns, column.String)
			} else {
				constraint.ColumnsKnown = false
				constraint.Columns = []string{}
			}
		}
		if constraint.Kind == "foreign_key" {
			if target.Valid && constraint.ReferencedColumnsKnown {
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
