// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func sqliteStructure(ctx context.Context, db *sql.DB, d core.TableDescription, result *core.TableStructure) error {
	result.Coverage.Constraints = "partial"
	result.Coverage.Checks = "unavailable"
	type listedIndex struct {
		name, origin    string
		unique, partial bool
	}
	listed := []listedIndex{}
	rows, err := db.QueryContext(ctx, `SELECT name,origin,"unique",partial FROM pragma_index_list(?) ORDER BY seq`, d.Table.Name)
	if err != nil {
		return err
	}
	for rows.Next() {
		var index listedIndex
		if err = rows.Scan(&index.name, &index.origin, &index.unique, &index.partial); err != nil {
			rows.Close()
			return err
		}
		listed = append(listed, index)
		if len(listed) > MaxStructureObjects {
			rows.Close()
			return public("result_too_large", "Zu viele Indizes.")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(d.PrimaryKey) > 0 {
		constraint := emptyConstraint("primary_key")
		constraint.Columns = append(constraint.Columns, d.PrimaryKey...)
		constraint.ColumnsKnown = true
		result.Constraints = append(result.Constraints, constraint)
	}
	for _, listed := range listed {
		index := core.StructureIndex{Name: listed.name, Unique: yesNo(listed.unique), Primary: yesNo(listed.origin == "pk"), Method: "unknown", Partial: yesNo(listed.partial), Valid: "unknown", NullsDistinct: "unknown", Terms: []core.IndexTerm{}}
		rows, err = db.QueryContext(ctx, `SELECT seqno,cid,name,"desc",coll,"key" FROM pragma_index_xinfo(?) ORDER BY seqno`, listed.name)
		if err != nil {
			return err
		}
		for rows.Next() {
			var position, cid int
			var name, coll sql.NullString
			var descending, key bool
			if err = rows.Scan(&position, &cid, &name, &descending, &coll, &key); err != nil {
				rows.Close()
				return err
			}
			term := emptyTerm(position + 1)
			term.Role = "auxiliary"
			if key {
				term.Role = "key"
			}
			term.Direction = "asc"
			if descending {
				term.Direction = "desc"
			}
			term.PrefixState = "none"
			if cid == -1 {
				term.Kind = "rowid"
			} else if cid == -2 {
				term.Kind = "expression"
			} else if cid >= 0 && name.Valid {
				term.Kind = "column"
				value := name.String
				term.Column = &value
			}
			if coll.Valid {
				term.CollationState = "known"
				term.Collation = &core.TableRef{Name: coll.String}
			}
			index.Terms = append(index.Terms, term)
			if len(index.Terms) > MaxStructureObjects {
				rows.Close()
				return public("result_too_large", "Zu viele Indexterme.")
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		result.Indexes = append(result.Indexes, index)
		if listed.origin == "u" {
			constraint := emptyConstraint("unique")
			constraint.ColumnsKnown = true
			for _, term := range index.Terms {
				if term.Role == "key" {
					if term.Column == nil {
						constraint.ColumnsKnown = false
					} else {
						constraint.Columns = append(constraint.Columns, *term.Column)
					}
				}
			}
			if !constraint.ColumnsKnown {
				constraint.Columns = []string{}
			}
			result.Constraints = append(result.Constraints, constraint)
		}
		if err = structureBudget(*result); err != nil {
			return err
		}
	}
	rows, err = db.QueryContext(ctx, `SELECT id,seq,"table","from","to",on_update,on_delete FROM pragma_foreign_key_list(?) ORDER BY id,seq`, d.Table.Name)
	if err != nil {
		return err
	}
	defer rows.Close()
	previous := -1
	for rows.Next() {
		var id, seq int
		var target, source string
		var column sql.NullString
		var update, delete string
		if err = rows.Scan(&id, &seq, &target, &source, &column, &update, &delete); err != nil {
			return err
		}
		if id != previous {
			constraint := emptyConstraint("foreign_key")
			constraint.ColumnsKnown = true
			constraint.ReferencedColumnsKnown = true
			constraint.ReferencedTable = &core.TableRef{Schema: "main", Name: target}
			constraint.OnUpdate = fkAction(update)
			constraint.OnDelete = fkAction(delete)
			result.Constraints = append(result.Constraints, constraint)
			previous = id
		}
		constraint := &result.Constraints[len(result.Constraints)-1]
		constraint.Columns = append(constraint.Columns, source)
		if !column.Valid {
			constraint.ReferencedColumnsKnown = false
			constraint.ReferencedColumns = []string{}
		} else if constraint.ReferencedColumnsKnown {
			constraint.ReferencedColumns = append(constraint.ReferencedColumns, column.String)
		}
		if err = structureBudget(*result); err != nil {
			return err
		}
	}
	return rows.Err()
}
