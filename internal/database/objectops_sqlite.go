// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func sqliteDependencies(ctx context.Context, q metadataQuerier, desc core.TableDescription) ([]core.ObjectDependency, error) {
	name := desc.Table.Name
	var out []core.ObjectDependency
	rows, err := q.QueryContext(ctx, `SELECT m.name, CAST(f.id AS TEXT) FROM sqlite_schema m, pragma_foreign_key_list(m.name) f WHERE m.type = 'table' AND m.name NOT GLOB 'sqlite_*' AND m.name <> ?1 AND lower(f."table") = lower(?1) ORDER BY m.name, f.id LIMIT 101`, name)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var table, id string
		if err = rows.Scan(&table, &id); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, core.ObjectDependency{Kind: "foreign_key_in", Schema: "main", Name: table, Certainty: "exact"})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT DISTINCT "table" FROM pragma_foreign_key_list(?1) WHERE lower("table") <> lower(?1) ORDER BY 1 LIMIT 101`, name)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, core.ObjectDependency{Kind: "foreign_key_out", Schema: "main", Name: table, Certainty: "exact"})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'view' AND instr(lower(COALESCE(sql, '')), lower(?1)) > 0 ORDER BY name LIMIT 101`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var view string
		if err = rows.Scan(&view); err != nil {
			return nil, err
		}
		out = append(out, core.ObjectDependency{Kind: "view", Schema: "main", Name: view, Certainty: "textual"})
	}
	return out, rows.Err()
}

// sqliteAutoIncrement reads the AUTOINCREMENT counter if the table has one.
func sqliteAutoIncrement(ctx context.Context, q metadataQuerier, name string) (*int64, error) {
	var exists int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_schema WHERE name = 'sqlite_sequence'").Scan(&exists); err != nil || exists == 0 {
		return nil, err
	}
	var seq sql.NullInt64
	err := q.QueryRowContext(ctx, "SELECT seq FROM sqlite_sequence WHERE name = ?", name).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return nullInt(seq), err
}
