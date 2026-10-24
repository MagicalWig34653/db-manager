// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func postgresDatabaseInfo(ctx context.Context, q metadataQuerier, name string) (found, system bool, collation string, err error) {
	var datname string
	var template bool
	var coll sql.NullString
	err = q.QueryRowContext(ctx, "SELECT datname, datistemplate, COALESCE(datcollate, datlocale) FROM pg_database WHERE datname = $1 AND datallowconn", name).Scan(&datname, &template, &coll)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "", nil
	}
	if err != nil || datname != name {
		return false, false, "", err
	}
	return true, template || name == "postgres", coll.String, nil
}

// postgresLibcLocale validates a database collation against the libc
// collations of the server and returns the locale string.
func postgresLibcLocale(ctx context.Context, q metadataQuerier, name string) (string, bool, error) {
	var locale sql.NullString
	err := q.QueryRowContext(ctx, "SELECT collcollate FROM pg_collation WHERE collname = $1 AND collprovider = 'c' AND collencoding IN (-1, 6) ORDER BY collencoding DESC LIMIT 1", name).Scan(&locale)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !locale.Valid {
		return "", false, nil
	}
	return locale.String, err == nil, err
}

func postgresOptions(ctx context.Context, q metadataQuerier, ref core.TableRef) (core.TableOptions, error) {
	var comment sql.NullString
	err := q.QueryRowContext(ctx, "SELECT obj_description(c.oid, 'pg_class') FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2", ref.Schema, ref.Name).Scan(&comment)
	if err != nil {
		return core.TableOptions{}, err
	}
	return core.TableOptions{Comment: nullString(comment)}, nil
}

func postgresDependencies(ctx context.Context, q metadataQuerier, desc core.TableDescription) ([]core.ObjectDependency, error) {
	ref := desc.Table
	var oid int64
	if err := q.QueryRowContext(ctx, "SELECT c.oid::bigint FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2", ref.Schema, ref.Name).Scan(&oid); err != nil {
		return nil, err
	}
	var out []core.ObjectDependency
	collect := func(query, kind, certainty string) error {
		rows, err := q.QueryContext(ctx, query, oid)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var schema, name string
			var constraint sql.NullString
			if err = rows.Scan(&schema, &name, &constraint); err != nil {
				return err
			}
			out = append(out, core.ObjectDependency{Kind: kind, Schema: schema, Name: name, Constraint: nullString(constraint), Certainty: certainty})
		}
		return rows.Err()
	}
	if err := collect(`SELECT cn.nspname, c.relname, con.conname FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace cn ON cn.oid = c.relnamespace WHERE con.contype = 'f' AND con.confrelid = $1::oid AND con.conrelid <> con.confrelid ORDER BY 1, 2, 3 LIMIT 101`, "foreign_key_in", "exact"); err != nil {
		return nil, err
	}
	if err := collect(`SELECT cn.nspname, c.relname, con.conname FROM pg_constraint con JOIN pg_class c ON c.oid = con.confrelid JOIN pg_namespace cn ON cn.oid = c.relnamespace WHERE con.contype = 'f' AND con.conrelid = $1::oid AND con.conrelid <> con.confrelid ORDER BY 1, 2, 3 LIMIT 101`, "foreign_key_out", "exact"); err != nil {
		return nil, err
	}
	if err := collect(`SELECT DISTINCT vn.nspname, v.relname, NULL::text FROM pg_depend d JOIN pg_rewrite r ON r.oid = d.objid JOIN pg_class v ON v.oid = r.ev_class JOIN pg_namespace vn ON vn.oid = v.relnamespace WHERE d.classid = 'pg_rewrite'::regclass AND d.refclassid = 'pg_class'::regclass AND d.refobjid = $1::oid AND v.oid <> $1::oid ORDER BY 1, 2 LIMIT 101`, "view", "exact"); err != nil {
		return nil, err
	}
	return out, nil
}

// subObjects describes child tables of a target: PostgreSQL inheritance
// children and partitions, MySQL/MariaDB partitions.
type subObjects struct {
	labels      []string
	inherits    bool // PostgreSQL ordinary table with inheritance children
	partitioned bool
	truncated   bool
	names       []string
}

func (s subObjects) warnings(action string) []string {
	if len(s.names) == 0 && !s.partitioned {
		return nil
	}
	list := strings.Join(s.names, ", ")
	if s.truncated {
		list += " und weitere"
	}
	switch {
	case s.inherits:
		out := []string{"Die Tabelle hat Vererbungs-Kindtabellen (" + list + ")."}
		if action == core.OpTableTruncate {
			out = append(out, "TRUNCATE ONLY leert nur diese Tabelle; die Kindtabellen bleiben unverändert.")
		}
		if action == core.OpTableDrop {
			out = append(out, "Kindtabellen verhindern das Löschen (kein CASCADE).")
		}
		return out
	case action == core.OpTableTruncate:
		return []string{"Die partitionierte Tabelle wird samt ihren Partitionen geleert (" + list + ")."}
	case action == core.OpTableDrop:
		return []string{"Beim Löschen werden auch die Partitionen entfernt und ihre Daten gelöscht (" + list + ")."}
	}
	return []string{"Die Tabelle ist partitioniert (" + list + ")."}
}

func postgresSubObjects(ctx context.Context, q metadataQuerier, ref core.TableRef) (subObjects, error) {
	var s subObjects
	var kind string
	if err := q.QueryRowContext(ctx, "SELECT c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2", ref.Schema, ref.Name).Scan(&kind); err != nil {
		return s, err
	}
	rows, err := q.QueryContext(ctx, `WITH RECURSIVE t(oid, depth) AS (
 SELECT c.oid, 0 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2
 UNION ALL SELECT i.inhrelid, t.depth + 1 FROM pg_inherits i JOIN t ON i.inhparent = t.oid WHERE t.depth < 8)
SELECT n.nspname, c.relname FROM t JOIN pg_class c ON c.oid = t.oid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE t.depth > 0 ORDER BY 1, 2 LIMIT 51`, ref.Schema, ref.Name)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var schema, name string
		if err = rows.Scan(&schema, &name); err != nil {
			return s, err
		}
		if len(s.names) == 50 {
			s.truncated = true
			break
		}
		s.names = append(s.names, schema+"."+name)
		s.labels = append(s.labels, "table:"+schema+"."+name)
	}
	s.partitioned = kind == "p"
	s.inherits = kind == "r" && len(s.names) > 0
	return s, rows.Err()
}
