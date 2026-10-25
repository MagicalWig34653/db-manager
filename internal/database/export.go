// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

var exportNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func invalidExport() error {
	return public("invalid_export", "Exportoptionen sind ungültig oder nicht unterstützt.")
}

type exportWriter struct {
	ctx       context.Context
	out       io.Writer
	remaining int64
}

func (w *exportWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > w.remaining {
		return 0, public("export_byte_limit", "Export ist wegen des Bytelimits unvollständig.")
	}
	n, err := w.out.Write(p)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

type exportTable struct {
	description core.TableDescription
	definition  string
	query       string
}

func (e *Engine) Export(ctx context.Context, input core.ExportRequest, out io.Writer) error {
	r, err := core.NormalizeExport(input)
	if err != nil {
		return err
	}
	select {
	case e.exports <- struct{}{}:
		defer func() { <-e.exports }()
	default:
		return public("export_busy", "Zu viele gleichzeitige Exporte. Später erneut versuchen.")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	w := &exportWriter{ctx: ctx, out: out, remaining: r.MaxBytes}
	f := newExportFormat(w, r)
	databases := []string{r.Connection.Database}
	if r.Scope == "server" {
		databases = r.Databases
	}
	remainingRows := r.MaxRows
	started := false
	for _, name := range databases {
		c := r.Connection
		c.Database = name
		if err = e.exportDatabase(ctx, c, r, f, &remainingRows, &started); err != nil {
			return safeExport(ctx, err)
		}
	}
	if !started {
		if err = f.start(); err != nil {
			return err
		}
	}
	return f.finish()
}
func safeExport(ctx context.Context, err error) error {
	var p *core.Error
	if errors.As(err, &p) {
		return err
	}
	if ctx.Err() != nil {
		return safe(ctx, ctx.Err())
	}
	return public("export_failed", "Export ist unvollständig; Datenbank oder Download ist fehlgeschlagen.")
}

func (e *Engine) exportDatabase(ctx context.Context, c core.Connection, r core.ExportRequest, f *exportFormat, remaining *int64, started *bool) error {
	db, cleanup, err := e.open(ctx, c)
	if err != nil {
		return err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	// Pin the entire catalog/data read to one connection and one snapshot.
	setup := []string{"PRAGMA query_only=ON", "BEGIN"}
	if c.Driver == "mysql" {
		setup = []string{"SET NAMES utf8mb4", "SET SESSION time_zone='+00:00'", "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ", "START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY"}
	}
	if c.Driver == "postgres" {
		setup = []string{"BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY", "SET LOCAL TIME ZONE 'UTC'", "SET LOCAL DateStyle='ISO, YMD'"}
	}
	for _, q := range setup {
		if _, err = conn.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanupCtx, "ROLLBACK")
	}()
	tables, err := exportTables(ctx, conn, c, r)
	if err != nil {
		return err
	}
	if (r.Format == "csv" || r.Format == "excel-csv") && len(tables) != 1 {
		return invalidExport()
	}
	if !*started {
		if err = f.start(); err != nil {
			return err
		}
		*started = true
	}
	if r.Scope == "server" && r.Format == "sql" {
		if c.Driver == "mysql" {
			if err = f.write("CREATE DATABASE IF NOT EXISTS " + quoteIdentifier(c.Driver, c.Database) + ";\nUSE " + quoteIdentifier(c.Driver, c.Database) + ";\n"); err != nil {
				return err
			}
		} else {
			return public("unsupported_export", "SQL-Serverexport ist nur für MySQL/MariaDB verfügbar; JSON unterstützt getrennte Datenbanken.")
		}
	}
	// Structure precedes data. DDL is bounded metadata, never table contents.
	if *r.Structure {
		for _, table := range tables {
			if err = f.structure(c, table); err != nil {
				return err
			}
		}
	}
	if *r.Data {
		for _, table := range tables {
			if err = f.table(c, table.description); err != nil {
				return err
			}
			query := table.query
			if query == "" {
				continue
			}
			emit := func(row []core.Cell) error {
				if *remaining == 0 {
					return public("export_row_limit", "Export ist wegen des Zeilenlimits unvollständig.")
				}
				*remaining = *remaining - 1
				return f.row(c.Driver, row)
			}
			if err = streamExportRows(ctx, conn, c.Driver, query, emit); err != nil {
				return err
			}
			if err = f.endTable(); err != nil {
				return err
			}
		}
	}
	return nil
}

func exportTables(ctx context.Context, conn *sql.Conn, c core.Connection, r core.ExportRequest) ([]exportTable, error) {
	if r.Scope == "query" {
		return exportQueryTable(ctx, conn, c, r.SQL)
	}
	refs := r.Tables
	if r.Scope != "tables" {
		var query string
		switch c.Driver {
		case "sqlite":
			query = "SELECT 'main',name FROM main.sqlite_schema WHERE type='table' AND name NOT GLOB 'sqlite_*' ORDER BY name"
		case "mysql":
			query = "SELECT TABLE_SCHEMA,TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_TYPE='BASE TABLE' ORDER BY TABLE_NAME"
		case "postgres":
			query = "SELECT n.nspname,c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p') AND " + pgVisible + " ORDER BY n.nspname,c.relname"
		}
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ref core.TableRef
			if err = rows.Scan(&ref.Schema, &ref.Name); err != nil {
				rows.Close()
				return nil, err
			}
			refs = append(refs, ref)
			if len(refs) > core.ExportMaxTables {
				rows.Close()
				return nil, invalidExport()
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	result := make([]exportTable, 0, len(refs))
	seen := map[core.TableRef]bool{}
	size := 0
	for _, ref := range refs {
		d, err := describe(ctx, conn, c, ref)
		if err != nil {
			return nil, err
		}
		if d.Kind != "table" || (c.Driver == "mysql" && d.Transactional != "yes") {
			return nil, public("unsupported_export", "Nur reguläre transaktionale Tabellen werden exportiert.")
		}

		if c.Driver == "postgres" {
			var regular bool
			if err = conn.QueryRowContext(ctx, `SELECT c.relkind='r' AND NOT c.relispartition AND NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&regular); err != nil {
				return nil, err
			}
			if !regular {
				return nil, public("unsupported_export", "Partitionierung und Tabellenvererbung sind nicht für diesen Export freigegeben.")
			}
		}
		if seen[d.Table] {
			return nil, invalidExport()
		}
		seen[d.Table] = true
		cols := []string{}
		for _, col := range d.Columns {
			if (r.Format != "sql" && col.Generated != "hidden") || col.Generated == "none" || col.Generated == "identity" {
				cols = append(cols, quoteIdentifier(c.Driver, col.Name))
			}
		}
		if len(cols) == 0 {
			return nil, invalidExport()
		}
		t := exportTable{description: d, query: "SELECT " + strings.Join(cols, ",") + " FROM " + qualifiedTable(c.Driver, d.Table)}

		if c.Driver == "mysql" && r.Format == "sql" && d.Table.Schema != c.Database {
			return nil, invalidExport()
		}
		lockRows, lockErr := conn.QueryContext(ctx, t.query+" LIMIT 0")
		if lockErr != nil {
			return nil, lockErr
		}
		lockRows.Close()
		current, describeErr := describe(ctx, conn, c, d.Table)
		if describeErr != nil {
			return nil, describeErr
		}
		if !sameExportColumns(d, current) {
			return nil, public("export_schema_changed", "Tabellenstruktur hat sich während der Exportvorbereitung geändert.")
		}
		if *r.Structure {
			t.definition, err = exportDefinition(ctx, conn, c, d, r.IfNotExists)
			if err != nil {
				return nil, err
			}
			if len(t.definition) > core.OverviewMaxDefinition {
				return nil, public("result_too_large", "Tabellendefinition ist zu groß.")
			}
		}
		size += len(t.definition) + len(t.query)
		if size > MaxResponseBytes {
			return nil, public("result_too_large", "Exportmetadaten sind zu groß.")
		}
		result = append(result, t)
	}
	return result, nil
}

func exportDefinition(ctx context.Context, conn *sql.Conn, c core.Connection, d core.TableDescription, ifNotExists bool) (string, error) {
	if c.Driver == "sqlite" {
		var def string
		err := conn.QueryRowContext(ctx, "SELECT sql FROM main.sqlite_schema WHERE name=? AND type='table'", d.Table.Name).Scan(&def)
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(def)), "CREATE VIRTUAL") {
			return "", invalidExport()
		}

		if err != nil {
			return "", err
		}
		rows, err := conn.QueryContext(ctx, "SELECT sql FROM main.sqlite_schema WHERE type='index' AND tbl_name=? AND sql IS NOT NULL ORDER BY name", d.Table.Name)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		for rows.Next() {
			var index string
			if err = rows.Scan(&index); err != nil {
				return "", err
			}
			if ifNotExists {
				index = indexIfNotExists(index)
			}
			def = strings.TrimSuffix(def, ";") + ";\n" + index
			if len(def) > core.OverviewMaxDefinition {
				return "", public("result_too_large", "Tabellendefinition ist zu groß.")
			}
		}
		return def, rows.Err()
	}
	if c.Driver == "mysql" {
		var name, def string
		err := conn.QueryRowContext(ctx, "SHOW CREATE TABLE "+qualifiedTable(c.Driver, d.Table)).Scan(&name, &def)
		return def, err
	}
	// The existing catalog reconstruction preserves constraints and indexes and
	// explicitly documents the omitted PostgreSQL object classes.
	def, err := postgresDefinitionIdx(ctx, conn, d.Table, ifNotExists)
	return def.Definition, err
}

func exportQueryTable(ctx context.Context, conn *sql.Conn, c core.Connection, query string) ([]exportTable, error) {
	// Preparing first rejects batches; the surrounding read-only transaction is
	// the authority for functions/CTEs with side effects, not an SQL prefix check.
	cols, err := exportQueryColumns(ctx, conn, c.Driver, query)
	if err != nil {
		return nil, err
	}
	return []exportTable{{description: core.TableDescription{Table: core.TableRef{Name: "query"}, Columns: cols}, query: query}}, nil
}

func qualifiedTable(driver string, ref core.TableRef) string {
	if ref.Schema == "" {
		return quoteIdentifier(driver, ref.Name)
	}
	return quoteIdentifier(driver, ref.Schema) + "." + quoteIdentifier(driver, ref.Name)
}

func sameExportColumns(a, b core.TableDescription) bool {
	if a.Kind != b.Kind || a.StorageEngine != b.StorageEngine || len(a.Columns) != len(b.Columns) {
		return false
	}
	for i, col := range a.Columns {
		other := b.Columns[i]
		if col.Name != other.Name || col.DatabaseType != other.DatabaseType || col.Generated != other.Generated {
			return false
		}
	}
	return true
}

// indexIfNotExists inserts IF NOT EXISTS after the leading CREATE [UNIQUE] INDEX
// of a catalog-provided index statement; the rest of the statement is untouched.
func indexIfNotExists(statement string) string {
	for _, prefix := range []string{"CREATE UNIQUE INDEX ", "CREATE INDEX "} {
		if strings.HasPrefix(statement, prefix) {
			return prefix + "IF NOT EXISTS " + strings.TrimPrefix(statement, prefix)
		}
	}
	return statement
}
