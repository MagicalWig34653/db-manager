// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// mysqlSchemaInfo looks a database up by exact name (bound parameter) and
// returns its default collation.
func mysqlSchemaInfo(ctx context.Context, q metadataQuerier, name string) (collation string, found bool, err error) {
	var schema string
	var coll sql.NullString
	err = q.QueryRowContext(ctx, "SELECT SCHEMA_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", name).Scan(&schema, &coll)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil || schema != name {
		return "", false, err
	}
	return coll.String, true, nil
}

func mysqlCollation(ctx context.Context, q metadataQuerier, name string) (charset, collation string, ok bool, err error) {
	err = q.QueryRowContext(ctx, "SELECT CHARACTER_SET_NAME, COLLATION_NAME FROM information_schema.COLLATIONS WHERE COLLATION_NAME = ?", name).Scan(&charset, &collation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	return charset, collation, err == nil, err
}

func mysqlEngine(ctx context.Context, q metadataQuerier, name string) (string, bool, error) {
	var engine string
	err := q.QueryRowContext(ctx, "SELECT ENGINE FROM information_schema.ENGINES WHERE ENGINE = ? AND SUPPORT IN ('YES','DEFAULT')", name).Scan(&engine)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return engine, err == nil, err
}

func mysqlTableOptions(ctx context.Context, q metadataQuerier, ref core.TableRef) (core.TableOptions, error) {
	var engine, comment, collation sql.NullString
	var auto sql.NullInt64
	err := q.QueryRowContext(ctx, "SELECT ENGINE, TABLE_COMMENT, TABLE_COLLATION, AUTO_INCREMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", ref.Schema, ref.Name).Scan(&engine, &comment, &collation, &auto)
	if err != nil {
		return core.TableOptions{}, err
	}
	return core.TableOptions{Engine: nullString(engine), Comment: nullString(comment), Collation: nullString(collation), AutoIncrement: nullInt(auto)}, nil
}

// mysqlDatabaseContents lists up to 50 objects of a database and the foreign
// keys of other databases that reference its tables.
func mysqlDatabaseContents(ctx context.Context, q metadataQuerier, name string) (objects []string, more bool, crossing []core.ObjectDependency, err error) {
	rows, err := q.QueryContext(ctx, "SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME LIMIT 51", name)
	if err != nil {
		return nil, false, nil, err
	}
	for rows.Next() {
		var table, kind string
		if err = rows.Scan(&table, &kind); err != nil {
			rows.Close()
			return nil, false, nil, err
		}
		if len(objects) == 50 {
			more = true
			break
		}
		label := "table:"
		if kind == "VIEW" {
			label = "view:"
		}
		objects = append(objects, label+name+"."+table)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, false, nil, err
	}
	crossing, _, err = mysqlForeignKeys(ctx, q, "SELECT CONSTRAINT_SCHEMA, TABLE_NAME, CONSTRAINT_NAME FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE UNIQUE_CONSTRAINT_SCHEMA = ? AND CONSTRAINT_SCHEMA <> ? ORDER BY CONSTRAINT_SCHEMA, TABLE_NAME, CONSTRAINT_NAME LIMIT 101", "foreign_key_in", name, name)
	return objects, more, crossing, err
}

func mysqlForeignKeys(ctx context.Context, q metadataQuerier, query, kind string, args ...any) ([]core.ObjectDependency, bool, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []core.ObjectDependency
	for rows.Next() {
		var schema, table, constraint string
		if err = rows.Scan(&schema, &table, &constraint); err != nil {
			return nil, false, err
		}
		out = append(out, core.ObjectDependency{Kind: kind, Schema: schema, Name: table, Constraint: &constraint, Certainty: "exact"})
	}
	return out, len(out) > core.ObjectOpMaxDependencies, rows.Err()
}

func mysqlDependencies(ctx context.Context, q metadataQuerier, desc core.TableDescription) ([]core.ObjectDependency, error) {
	ref := desc.Table
	in, _, err := mysqlForeignKeys(ctx, q, "SELECT CONSTRAINT_SCHEMA, TABLE_NAME, CONSTRAINT_NAME FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE UNIQUE_CONSTRAINT_SCHEMA = ? AND REFERENCED_TABLE_NAME = ? AND NOT (CONSTRAINT_SCHEMA = ? AND TABLE_NAME = ?) ORDER BY CONSTRAINT_SCHEMA, TABLE_NAME, CONSTRAINT_NAME LIMIT 101", "foreign_key_in", ref.Schema, ref.Name, ref.Schema, ref.Name)
	if err != nil {
		return nil, err
	}
	out, _, err := mysqlForeignKeys(ctx, q, "SELECT UNIQUE_CONSTRAINT_SCHEMA, REFERENCED_TABLE_NAME, CONSTRAINT_NAME FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE CONSTRAINT_SCHEMA = ? AND TABLE_NAME = ? AND NOT (UNIQUE_CONSTRAINT_SCHEMA = ? AND REFERENCED_TABLE_NAME = ?) ORDER BY 1, 2, 3 LIMIT 101", "foreign_key_out", ref.Schema, ref.Name, ref.Schema, ref.Name)
	if err != nil {
		return nil, err
	}
	needle, err := dbplan.QualifiedName("mysql", ref.Schema, ref.Name)
	if err != nil {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, "SELECT TABLE_SCHEMA, TABLE_NAME FROM information_schema.VIEWS WHERE LOCATE(?, VIEW_DEFINITION) > 0 ORDER BY TABLE_SCHEMA, TABLE_NAME LIMIT 101", needle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var views []core.ObjectDependency
	for rows.Next() {
		var schema, name string
		if err = rows.Scan(&schema, &name); err != nil {
			return nil, err
		}
		views = append(views, core.ObjectDependency{Kind: "view", Schema: schema, Name: name, Certainty: "textual"})
	}
	return append(append(in, out...), views...), rows.Err()
}

func mysqlSubObjects(ctx context.Context, q metadataQuerier, ref core.TableRef) (subObjects, error) {
	var s subObjects
	rows, err := q.QueryContext(ctx, "SELECT PARTITION_NAME FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND PARTITION_NAME IS NOT NULL ORDER BY PARTITION_ORDINAL_POSITION LIMIT 51", ref.Schema, ref.Name)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return s, err
		}
		if len(s.names) == 50 {
			s.truncated = true
			break
		}
		s.names = append(s.names, name)
	}
	s.partitioned = len(s.names) > 0
	return s, rows.Err()
}

// opIsCurrentDatabase compares with the database the server reports for the
// session, not with the spelling the client sent.
func opIsCurrentDatabase(ctx context.Context, env *opEnv, name string) (bool, error) {
	if env.c.Driver != "mysql" {
		return env.c.Database == name, nil
	}
	var current sql.NullString
	if err := env.sess.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&current); err != nil {
		return false, err
	}
	return env.c.Database == name || current.Valid && strings.EqualFold(current.String, name) && mysqlCaseInsensitive(ctx, env.sess), nil
}

func mysqlCaseInsensitive(ctx context.Context, q metadataQuerier) bool {
	var mode int
	return q.QueryRowContext(ctx, "SELECT @@lower_case_table_names").Scan(&mode) == nil && mode != 0
}
