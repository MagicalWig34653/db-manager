// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func columnQuote(driver, name string) string { q, _ := dbplan.QuoteIdentifier(driver, name); return q }
func columnLiteral(value string) string      { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
func columnDefault(d core.ColumnDefinition) (string, error) {
	if d.Default == nil || d.Nullable == nil {
		return "", invalidColumns()
	}
	if d.Type.Kind == "int64" || d.Type.Kind == "text" || d.Type.Kind == "binary" || d.Type.Kind == "decimal" {
		literal, err := createDefaultSQL(core.CreateColumn{Type: core.CreateType{Kind: d.Type.Kind, Length: d.Type.Length, Precision: d.Type.Precision, Scale: d.Type.Scale}, Nullable: *d.Nullable, Default: *d.Default})
		if err != nil {
			return "", invalidColumns()
		}
		return literal, nil
	}
	def := d.Default
	if def.Mode == "none" && def.Value == nil {
		return "", nil
	}
	if def.Mode == "null" && def.Value == nil && *d.Nullable {
		return "NULL", nil
	}
	if def.Mode != "value" || def.Value == nil {
		return "", invalidColumns()
	}
	value := def.Value.Value
	switch d.Type.Kind {
	case "boolean":
		if def.Value.Kind == "boolean" && (value == "true" || value == "false") {
			return strings.ToUpper(value), nil
		}
	case "date", "timestamp":
		// Dates/timestamps use ISO constants only. The engine validates the calendar.
		if def.Value.Kind != "text" || len(value) < 10 || len(value) > 32 {
			break
		}
		for _, ch := range value {
			if !strings.ContainsRune("0123456789-: T.+Z", ch) {
				return "", invalidColumns()
			}
		}
		return columnLiteral(value), nil
	case "enum", "set":
		if def.Value.Kind != "text" {
			break
		}
		values := []string{value}
		if d.Type.Kind == "set" {
			values = strings.Split(value, ",")
		}
		for _, v := range values {
			found := false
			for _, allowed := range d.Type.Values {
				if v == allowed {
					found = true
				}
			}
			if !found {
				return "", invalidColumns()
			}
		}
		return columnLiteral(value), nil
	}
	return "", invalidColumns()
}

func buildColumns(ctx context.Context, s *mutationSession, r core.ColumnsRequest) (core.ColumnsPlan, error) {
	empty := core.ColumnsPlan{}
	d, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return empty, err
	}
	if d.Kind != "table" || d.Table.Name != r.Table.Name || r.Table.Schema != "" && d.Table.Schema != r.Table.Schema {
		return empty, invalidColumns()
	}
	driver := r.Connection.Driver
	lower := strings.ToLower(d.Table.Schema)
	if driver == "postgres" && (strings.HasPrefix(lower, "pg_") || lower == "information_schema") || driver == "mysql" && mysqlSystemSchemas[lower] {
		return empty, invalidColumns()
	}
	if driver == "mysql" && !strings.EqualFold(d.StorageEngine, "InnoDB") {
		return empty, unsupportedColumns()
	}
	if driver == "postgres" {
		var special bool
		err = s.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&special)
		if err != nil {
			return empty, err
		}
		if special {
			return empty, unsupportedColumns()
		}
	}
	table, err := dbplan.QualifiedName(driver, d.Table.Schema, d.Table.Name)
	if err != nil {
		return empty, invalidColumns()
	}
	columns := map[string]core.TableColumn{}
	for _, c := range d.Columns {
		columns[c.Name] = c
	}
	touched := map[string]bool{}
	newNames := map[string]bool{}
	statements := []string{}
	clauses := []string{}
	warnings := []string{"Strukturänderungen können die Tabelle sperren. Zeitlimit und DB-Rechte gelten.", "Plan-Hash ist kein Replay-Schutz; nach Abschluss oder unbekanntem Ausgang neu planen und bestätigen."}
	affected := []string{d.Table.Schema + "." + d.Table.Name}
	deps := []string{}
	destructive := false
	for _, ch := range r.Changes {
		target := ch.Column
		if ch.Action == "add" {
			target = ch.Definition.Name
		}
		if touched[target] {
			return empty, invalidColumns()
		}
		touched[target] = true
		old, exists := columns[ch.Column]
		if ch.Action != "add" && !exists {
			return empty, public("column_not_found", "Die Spalte ist nicht vorhanden.")
		}
		if ch.Action != "add" && old.Generated != "none" {
			return empty, unsupportedColumns()
		}
		if driver == "mysql" && (ch.Action == "change" || ch.Action == "reorder") {
			// Reject attributes not modeled by the explicit replacement definition.
			var extra string
			if err := s.QueryRowContext(ctx, `SELECT EXTRA FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=?`, d.Table.Schema, d.Table.Name, old.Name).Scan(&extra); err != nil {
				return empty, err
			}
			if extra != "" || ch.Definition.Comment == nil || ch.Definition.Type.Kind == "text" && ch.Definition.Collation == nil {
				return empty, unsupportedColumns()
			}
		}
		name := ch.NewName
		if ch.Definition != nil {
			name = ch.Definition.Name
		}
		if name != "" && (name != ch.Column || ch.Action == "add") {
			for existing := range columns {
				if strings.EqualFold(existing, name) {
					return empty, public("column_exists", "Eine Spalte mit diesem Namen ist bereits vorhanden.")
				}
			}
			if newNames[strings.ToLower(name)] {
				return empty, invalidColumns()
			}
			newNames[strings.ToLower(name)] = true
		}
		qold := columnQuote(driver, old.Name)
		prefix := "ALTER TABLE " + table + " "
		switch ch.Action {
		case "drop":
			destructive = true
			warnings = append(warnings, "Spalte "+old.Name+": alle Werte werden gelöscht. Kein CASCADE.")
			dd, err := columnDependencies(ctx, s, r.Connection, d, old.Name)
			if err != nil {
				return empty, err
			}
			deps = append(deps, dd...)
			warnings = append(warnings, columnDependencyWarnings(driver, old.Name, "drop", dd)...)
			for _, dep := range dd[1:] {
				affected = append(affected, dep)
			}
			clauses = append(clauses, "DROP COLUMN "+qold)
			statements = append(statements, prefix+"DROP COLUMN "+qold)
		case "rename":
			dd, err := columnDependencies(ctx, s, r.Connection, d, old.Name)
			if err != nil {
				return empty, err
			}
			deps = append(deps, dd...)
			warnings = append(warnings, "Spalte "+old.Name+": Abfragen, Sichten, Trigger und Programme, die den alten Namen verwenden, funktionieren danach nicht mehr.")
			warnings = append(warnings, columnDependencyWarnings(driver, old.Name, "rename", dd)...)
			clauses = append(clauses, "RENAME COLUMN "+qold+" TO "+columnQuote(driver, name))
			statements = append(statements, prefix+"RENAME COLUMN "+qold+" TO "+columnQuote(driver, name))
		case "add", "change", "reorder":
			def := *ch.Definition
			typ, _ := dbplan.TypeSQL(driver, def.Type)
			literal, err := columnDefault(def)
			if err != nil {
				return empty, err
			}
			qname := columnQuote(driver, def.Name)
			collate, err := columnCollation(ctx, s, d.Table, driver, def)
			if err != nil {
				return empty, err
			}
			definition := qname + " " + typ + collate
			if *def.Nullable {
				definition += " NULL"
			} else {
				definition += " NOT NULL"
			}
			if def.Default.Mode != "none" {
				definition += " DEFAULT " + literal
			}
			if driver == "mysql" && def.Comment != nil {
				definition += " COMMENT " + columnLiteral(*def.Comment)
			}
			position, err := columnPosition(driver, ch, d)
			if err != nil {
				return empty, err
			}
			definition += position
			if ch.Action == "add" {
				if _, exists := columns[def.Name]; exists {
					return empty, invalidColumns()
				}
				if driver == "sqlite" && !*def.Nullable && (def.Default.Mode == "none" || def.Default.Mode == "null") {
					return empty, unsupportedColumns()
				}
				clauses = append(clauses, "ADD COLUMN "+definition)
				statements = append(statements, prefix+"ADD COLUMN "+definition)
			} else {
				destructive = true
				warnings = append(warnings, "Spalte "+old.Name+": Typ-/Attributänderung kann Daten verlieren; vollständige neue Definition ersetzt die Attribute.")
				if old.DefaultValue != nil {
					text := "Spalte " + old.Name + ": der bisherige Standardwert " + *old.DefaultValue + " wird durch die neue Definition ersetzt"
					if def.Default.Mode == "none" {
						text += " und entfällt"
					}
					if strings.Contains(*old.DefaultValue, "nextval(") {
						text += "; die zugehörige Sequenz bleibt verwaist und neue Zeilen ohne Wert erhalten keinen automatischen Wert mehr"
					}
					warnings = append(warnings, text+".")
				}
				if !*def.Nullable {
					// Existence check instead of a counting full scan; stops at the first NULL.
					var found bool
					if err := s.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE "+qold+" IS NULL)").Scan(&found); err != nil {
						return empty, err
					}
					if found {
						warnings = append(warnings, fmt.Sprintf("Spalte %s enthält NULL-Werte; NOT NULL kann scheitern.", old.Name))
					}
				}
				if ch.Action == "reorder" && def.Name != old.Name {
					return empty, invalidColumns()
				}
				if driver == "mysql" {
					clauses = append(clauses, "CHANGE COLUMN "+qold+" "+definition)
				} else {
					// The cast target is generated from TypeSQL's fixed dialect catalog.
					statements = append(statements, prefix+"ALTER COLUMN "+qold+" DROP DEFAULT", prefix+"ALTER COLUMN "+qold+" TYPE "+typ+collate+" USING "+qold+"::"+typ)
					nullable := "SET NOT NULL"
					if *def.Nullable {
						nullable = "DROP NOT NULL"
					}
					statements = append(statements, prefix+"ALTER COLUMN "+qold+" "+nullable)
					if def.Default.Mode != "none" {
						statements = append(statements, prefix+"ALTER COLUMN "+qold+" SET DEFAULT "+literal)
					}
					if name != old.Name {
						statements = append(statements, prefix+"RENAME COLUMN "+qold+" TO "+qname)
					}
				}
			}
			if driver == "postgres" && def.Comment != nil {
				statements = append(statements, "COMMENT ON COLUMN "+table+"."+qname+" IS "+columnLiteral(*def.Comment))
			}
		}
		affected = append(affected, d.Table.Schema+"."+d.Table.Name+"."+target)
	}
	if driver == "mysql" {
		statements = []string{"ALTER TABLE " + table + " " + strings.Join(clauses, ", ")}
	}
	if len(deps) > 256 {
		return empty, public("result_too_large", "Zu viele Spaltenabhängigkeiten.")
	}
	p, err := dbplan.NewPlan(driver, statements, warnings, affected)
	if err != nil {
		return empty, invalidColumns()
	}
	return core.ColumnsPlan{Plan: p, Table: d.Table, Destructive: destructive, TargetName: d.Table.Name, Dependencies: deps, DDLTransactional: driver != "mysql"}, nil
}

func columnPosition(driver string, ch core.ColumnChange, d core.TableDescription) (string, error) {
	if ch.Position == nil {
		return "", nil
	}
	if driver != "mysql" {
		return "", unsupportedColumns()
	}
	p := ch.Position
	if p.Mode == "first" {
		return " FIRST", nil
	}
	for i, c := range d.Columns {
		if c.Name != p.Column {
			continue
		}
		if c.Name == ch.Column {
			return "", invalidColumns()
		}
		if p.Mode == "before" {
			if i == 0 {
				return " FIRST", nil
			}
			c = d.Columns[i-1]
			if c.Name == ch.Column {
				if i == 1 {
					return " FIRST", nil
				}
				c = d.Columns[i-2]
			}
		}
		return " AFTER " + columnQuote(driver, c.Name), nil
	}
	return "", public("column_not_found", "Die Positionsspalte fehlt.")
}

func columnCollation(ctx context.Context, s *mutationSession, table core.TableRef, driver string, d core.ColumnDefinition) (string, error) {
	if d.Collation == nil {
		return "", nil
	}
	if d.Type.Kind != "text" {
		return "", invalidColumns()
	}
	var schema, name string
	switch driver {
	case "mysql":
		err := s.QueryRowContext(ctx, `SELECT CHARACTER_SET_NAME,COLLATION_NAME FROM information_schema.COLLATIONS WHERE COLLATION_NAME=?`, *d.Collation).Scan(&schema, &name)
		if err != nil {
			return "", invalidColumns()
		}
		return " CHARACTER SET " + columnQuote(driver, schema) + " COLLATE " + columnQuote(driver, name), nil
	case "postgres":
		// Only a named collation in pg_catalog or the table schema is selectable.
		err := s.QueryRowContext(ctx, `SELECT n.nspname,c.collname FROM pg_catalog.pg_collation c JOIN pg_catalog.pg_namespace n ON n.oid=c.collnamespace WHERE c.collname=$1 AND n.nspname IN ($2,'pg_catalog') AND c.collencoding IN (-1,pg_catalog.pg_char_to_encoding('UTF8')) ORDER BY (n.nspname=$2) DESC LIMIT 1`, *d.Collation, table.Schema).Scan(&schema, &name)
		if err != nil {
			return "", invalidColumns()
		}
		q, _ := dbplan.QualifiedName(driver, schema, name)
		return " COLLATE " + q, nil
	}
	return "", unsupportedColumns()
}

// columnDependencies returns names only; catalog SQL/default expressions never
// become executable fragments or public errors. Trigger/view coverage is textual.
func columnDependencies(ctx context.Context, s *mutationSession, c core.Connection, d core.TableDescription, column string) ([]string, error) {
	deps := []string{"Trigger und externe Verweise sind nicht vollständig erfasst; die Engine prüft ohne CASCADE."}
	var query string
	var args []any
	switch c.Driver {
	case "sqlite":
		query = `SELECT name FROM main.sqlite_schema WHERE (tbl_name=? AND type IN ('index','trigger')) OR (type IN ('view','table') AND instr(lower(sql),lower(?))>0) ORDER BY name LIMIT 257`
		args = []any{d.Table.Name, d.Table.Name}
	case "mysql":
		query = `SELECT CONCAT('constraint:',CONSTRAINT_SCHEMA,'.',TABLE_NAME,'.',CONSTRAINT_NAME) FROM information_schema.KEY_COLUMN_USAGE WHERE (TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=?) OR (REFERENCED_TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME=? AND REFERENCED_COLUMN_NAME=?) UNION SELECT CONCAT('index:',INDEX_NAME) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=? UNION SELECT CONCAT('view:',TABLE_SCHEMA,'.',TABLE_NAME) FROM information_schema.VIEWS WHERE LOCATE(?,VIEW_DEFINITION)>0 AND LOCATE(?,VIEW_DEFINITION)>0 ORDER BY 1 LIMIT 257`
		tableNeedle, _ := dbplan.QualifiedName("mysql", d.Table.Schema, d.Table.Name)
		args = []any{d.Table.Schema, d.Table.Name, column, d.Table.Schema, d.Table.Name, column, d.Table.Schema, d.Table.Name, column, tableNeedle, columnQuote("mysql", column)}
	case "postgres":
		query = `SELECT DISTINCT pg_catalog.pg_describe_object(dep.classid,dep.objid,dep.objsubid) FROM pg_catalog.pg_depend dep JOIN pg_catalog.pg_class c ON c.oid=dep.refobjid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum=dep.refobjsubid WHERE dep.refclassid='pg_catalog.pg_class'::regclass AND n.nspname=$1 AND c.relname=$2 AND a.attname=$3 ORDER BY 1 LIMIT 257`
		args = []any{d.Table.Schema, d.Table.Name, column}
	}
	rows, err := s.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		deps = append(deps, name)
		if len(deps) > 256 {
			return nil, public("result_too_large", "Zu viele Spaltenabhängigkeiten.")
		}
	}
	return deps, rows.Err()
}

// columnDependencyWarnings turns the dependency list into driver-specific,
// honest warnings. The first entry of deps is the generic coverage notice.
func columnDependencyWarnings(driver, column, action string, deps []string) []string {
	var out []string
	for _, dep := range deps {
		kind, name, _ := strings.Cut(dep, ":")
		switch driver {
		case "mysql":
			switch kind {
			case "view":
				out = append(out, "Die Sicht "+name+" verwendet die Spalte "+column+" (anhand des Quelltextes erkannt) und wird danach ungültig; der Server lehnt die Änderung nicht ab.")
			case "index":
				if action == "drop" {
					out = append(out, "Der Index "+name+" entfällt oder wird verkürzt, wenn die Spalte entfernt wird.")
				}
			case "constraint":
				out = append(out, "Der Schlüssel/Constraint "+name+" betrifft die Spalte "+column+"; ein Fremdschlüssel verhindert das Ändern oder Entfernen.")
			}
		case "postgres":
			switch {
			case strings.HasPrefix(dep, "view "), strings.HasPrefix(dep, "materialized view "), strings.HasPrefix(dep, "rule ") && strings.Contains(dep, " on view "), strings.HasPrefix(dep, "rule ") && strings.Contains(dep, " on materialized view "):
				if action == "drop" {
					out = append(out, "Die Sicht ("+dep+") verwendet die Spalte und verhindert das Entfernen (kein CASCADE).")
				}
			case strings.HasPrefix(dep, "index "):
				if action == "drop" {
					out = append(out, "Der Index ("+dep+") wird mit der Spalte entfernt.")
				}
			case strings.HasPrefix(dep, "constraint "):
				out = append(out, "Constraint betroffen: "+dep+"; Fremdschlüssel verhindern das Entfernen, andere Constraints entfallen mit der Spalte.")
			}
		default:
			// SQLite names are not typed: its engine refuses to drop a column that an index, view, trigger or constraint uses.
		}
		if len(out) >= 20 {
			break
		}
	}
	if driver == "sqlite" && action == "drop" && len(deps) > 1 {
		out = append(out, "SQLite lehnt das Entfernen ab, wenn Index, Sicht, Trigger oder Einschränkung die Spalte verwenden.")
	}
	return out
}
