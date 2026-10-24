// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// opDatabaseInfo reports whether a database exists (exact name) and whether it
// is a system database.
func opDatabaseInfo(ctx context.Context, env *opEnv, name string) (exists, system bool, err error) {
	switch env.c.Driver {
	case "mysql":
		_, found, err := mysqlSchemaInfo(ctx, env.sess, name)
		return found, opSystemDatabase("mysql", name), err
	case "postgres":
		found, system, _, err := postgresDatabaseInfo(ctx, env.sess, name)
		return found, system, err
	}
	return false, false, public("unsupported_action", "Datenbankoperationen sind für diesen Treiber nicht verfügbar.")
}

// opDatabaseCollationClause validates a collation against the server catalog
// and returns the clause that follows the database name.
func opDatabaseCollationClause(ctx context.Context, env *opEnv, collation, action string) (string, error) {
	switch env.c.Driver {
	case "mysql":
		charset, coll, ok, err := mysqlCollation(ctx, env.sess, collation)
		if err != nil {
			return "", err
		}
		if !ok || !opToken.MatchString(charset) || !opToken.MatchString(coll) {
			return "", invalidOp("Die Kollation ist auf diesem Server nicht verfügbar.")
		}
		return " CHARACTER SET " + charset + " COLLATE " + coll, nil
	case "postgres":
		if action != core.OpDatabaseCreate {
			return "", public("unsupported_action", "PostgreSQL legt die Kollation einer Datenbank bei der Anlage fest.")
		}
		locale, ok, err := postgresLibcLocale(ctx, env.sess, collation)
		if err != nil {
			return "", err
		}
		if !ok || !opLocale.MatchString(locale) {
			return "", invalidOp("Die Kollation ist auf diesem Server nicht als libc-Locale verfügbar.")
		}
		literal, err := opLiteral(locale)
		if err != nil {
			return "", err
		}
		return " TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE " + literal + " LC_CTYPE " + literal, nil
	}
	return "", public("unsupported_action", "Datenbankoperationen sind für diesen Treiber nicht verfügbar.")
}

func tableOptions(ctx context.Context, env *opEnv, ref core.TableRef) (core.TableOptions, error) {
	switch env.c.Driver {
	case "mysql":
		return mysqlTableOptions(ctx, env.sess, ref)
	case "postgres":
		return postgresOptions(ctx, env.sess, ref)
	}
	return core.TableOptions{}, nil
}

// tableDependencies lists foreign keys and views, capped at the contract limit.
func tableDependencies(ctx context.Context, env *opEnv, desc core.TableDescription) ([]core.ObjectDependency, bool, error) {
	var deps []core.ObjectDependency
	var err error
	switch env.c.Driver {
	case "mysql":
		deps, err = mysqlDependencies(ctx, env.sess, desc)
	case "postgres":
		deps, err = postgresDependencies(ctx, env.sess, desc)
	default:
		deps, err = sqliteDependencies(ctx, env.sess, desc)
	}
	if err != nil {
		return nil, false, err
	}
	truncated := false
	if len(deps) > core.ObjectOpMaxDependencies {
		deps, truncated = deps[:core.ObjectOpMaxDependencies], true
	}
	return deps, truncated, nil
}

func boolPointer(value bool) *bool { return &value }

func countRows(ctx context.Context, env *opEnv, ref core.TableRef) *int64 {
	name, err := opQualified(env.c.Driver, ref.Schema, ref.Name)
	if err != nil {
		return nil
	}
	var count int64
	if err = env.sess.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+name).Scan(&count); err != nil {
		return nil
	}
	if count > maxSafeInteger {
		count = maxSafeInteger
	}
	return &count
}

func observationMatches(st *opState, o *core.ObjectOpObservation) bool {
	return st.expect == nil || st.expect(o)
}

// observeOp reads the result back after the statements ran. For PostgreSQL
// and SQLite table DDL this happens inside the committing transaction (state
// transactional); otherwise it is a separate read after the implicit commit.
func observeOp(ctx, parent context.Context, env *opEnv, st *opState, state string) *core.ObjectOpObservation {
	if state != "transactional" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, 10*time.Second)
		defer cancel()
	}
	o := &core.ObjectOpObservation{State: state}
	failed := false
	check := func(err error) {
		if err != nil {
			failed = true
		}
	}
	driver := env.c.Driver
	exists := func(ref core.TableRef) *bool {
		found, err := opTableExists(ctx, env.sess, env.c, ref)
		check(err)
		if err != nil {
			return nil
		}
		return boolPointer(found)
	}
	switch st.action {
	case core.OpDatabaseCreate, core.OpDatabaseCollation, core.OpDatabaseDrop:
		found, _, err := opDatabaseInfo(ctx, env, st.db)
		check(err)
		if err == nil {
			o.TargetExists = boolPointer(found)
			if found && st.action != core.OpDatabaseDrop {
				var collation string
				if driver == "mysql" {
					collation, _, err = mysqlSchemaInfo(ctx, env.sess, st.db)
				} else {
					_, _, collation, err = postgresDatabaseInfo(ctx, env.sess, st.db)
				}
				check(err)
				if err == nil && collation != "" {
					o.Collation = &collation
				}
			}
		}
	case core.OpTableRename:
		o.SourceExists = exists(st.src)
		o.TargetExists = exists(st.dst)
	case core.OpTableCopy:
		o.TargetExists = exists(st.dst)
		if st.srcRows && !failed {
			o.Rows = countRows(ctx, env, st.dst)
			o.SourceRows = countRows(ctx, env, st.src)
		}
	case core.OpTableTruncate:
		o.TargetExists = exists(st.src)
		o.Rows = countRows(ctx, env, st.src)
		failed = failed || o.Rows == nil
		switch driver {
		case "mysql":
			if options, err := mysqlTableOptions(ctx, env.sess, st.src); err == nil {
				o.AutoIncrement = options.AutoIncrement
			}
		case "sqlite":
			if seq, err := sqliteAutoIncrement(ctx, env.sess, st.src.Name); err == nil {
				o.AutoIncrement = seq
			}
		}
	case core.OpTableDrop:
		o.TargetExists = exists(st.src)
	case core.OpTableOptions:
		options, err := tableOptions(ctx, env, st.src)
		check(err)
		if err == nil {
			o.Options = &options
		}
	}
	if failed {
		o.State = "unavailable"
	}
	return o
}
