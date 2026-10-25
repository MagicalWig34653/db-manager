// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"strings"
)

func buildIndexes(ctx context.Context, s *mutationSession, r core.IndexesRequest) (core.IndexesPlan, error) {
	empty := core.IndexesPlan{}
	d, err := indexTable(ctx, s, r.Connection, r.Table)
	if err != nil {
		return empty, err
	}
	driver := s.driver
	q := func(n string) string { v, _ := dbplan.QuoteIdentifier(driver, n); return v }
	qualified := func(ref core.TableRef) string { v, _ := dbplan.QualifiedName(driver, ref.Schema, ref.Name); return v }
	table := qualified(d.Table)
	objects, err := indexObjects(ctx, s, d)
	if err != nil {
		return empty, err
	}
	old := indexObject{}
	for _, o := range objects {
		if (r.Action == "primary_set" || r.Action == "primary_drop") && o.kind == "primary" || o.name == r.Name && (strings.HasPrefix(r.Action, "foreign_key_") == (o.kind == "foreign")) {
			old = o
		}
	}
	needsOld := r.Action == "index_drop" || r.Action == "index_alter" || r.Action == "index_rename" || r.Action == "foreign_key_drop" || r.Action == "foreign_key_alter" || r.Action == "primary_drop"
	if needsOld && old.name == "" {
		return empty, public("object_not_found", "Der Schlüssel ist nicht vorhanden.")
	}
	if strings.HasPrefix(r.Action, "index_") && old.name != "" && old.kind != "index" {
		return empty, unsupportedIndexes()
	}
	// New names are validated declarations; existing names always come from metadata.
	name := r.NewName
	if r.Index != nil {
		name = r.Index.Name
	}
	if r.ForeignKey != nil {
		name = r.ForeignKey.Name
	}
	for _, o := range objects {
		if name != "" && strings.EqualFold(o.name, name) && o.name != old.name {
			return empty, public("object_exists", "Ein Schlüssel mit diesem Namen ist bereits vorhanden.")
		}
	}
	if driver == "sqlite" && strings.HasPrefix(strings.ToLower(name), "sqlite_") {
		return empty, invalidIndexes()
	}
	p := core.IndexesPlan{Table: d.Table, TargetName: d.Table.Name, Dependencies: []core.ObjectDependency{}, DDLTransactional: driver != "mysql", Destructive: needsOld || r.Action == "primary_set"}
	affected := []string{d.Table.Schema + "." + d.Table.Name}
	statements := []string{}
	clauses := []string{}
	warnings := []string{"Schlüsseländerungen können die Tabelle sperren und lange dauern; vollständige Scans und Indexaufbau benötigen Zeit und Speicher.", "Entfernen oder Ersetzen verliert Indexstrukturen und Integritätsregeln, keine Tabellenzeilen; neue Eindeutigkeit kann an vorhandenen Daten scheitern.", "Kein CASCADE. Trigger, externe Verweise und gleichnamig ersetzte Objekte sind nicht vollständig erfasst; Plan-Hash ist kein Replay- oder ABA-Schutz. Nach Abschluss oder unbekanntem Ausgang neu planen und bestätigen."}
	deps, truncated, err := tableDependencies(ctx, &opEnv{c: r.Connection, sess: s}, d)
	if err != nil {
		return empty, err
	}
	if truncated {
		return empty, public("result_too_large", "Zu viele Abhängigkeiten für eine vollständige Vorschau.")
	}
	for i := range deps {
		deps[i].Effect = "info"
	}
	p.Dependencies = append([]core.ObjectDependency{}, deps...)
	for _, dep := range deps {
		if dep.Kind == "foreign_key_in" || dep.Kind == "foreign_key_out" {
			warnings = append(warnings, "Fremdschlüssel betroffen: "+dep.Schema+"."+dep.Name+"; Entfernen benötigter Schlüssel kann blockiert werden. Selbstreferenzen prüft zusätzlich die Engine.")
		}
	}
	addAlter := func(clause string) {
		clauses = append(clauses, clause)
		statements = append(statements, "ALTER TABLE "+table+" "+clause)
	}
	if old.name != "" && r.Action != "index_rename" {
		switch old.kind {
		case "foreign":
			clause := "DROP CONSTRAINT " + q(old.name)
			if driver == "mysql" {
				clause = "DROP FOREIGN KEY " + q(old.name)
			}
			addAlter(clause)
		case "primary":
			clause := "DROP CONSTRAINT " + q(old.name)
			if driver == "mysql" {
				clause = "DROP PRIMARY KEY"
			}
			addAlter(clause)
		default:
			if driver == "mysql" {
				addAlter("DROP INDEX " + q(old.name))
			} else {
				statements = append(statements, "DROP INDEX "+qualified(core.TableRef{Schema: d.Table.Schema, Name: old.name}))
			}
		}
	}
	if r.Action == "index_rename" && name == old.name {
		return empty, invalidIndexes()
	}
	if r.Action == "index_rename" {
		if driver == "mysql" {
			addAlter("RENAME INDEX " + q(old.name) + " TO " + q(name))
		} else {
			statements = append(statements, "ALTER INDEX "+qualified(core.TableRef{Schema: d.Table.Schema, Name: old.name})+" RENAME TO "+q(name))
		}
	}
	if def := r.Index; def != nil {
		terms := []string{}
		groups := []string{}
		nonnull := []string{}
		nulls := []string{}
		seen := map[string]bool{}
		for _, term := range def.Terms {
			c, err := indexColumn(d, term.Column)
			if err != nil {
				return empty, err
			}
			if seen[c.Name] {
				return empty, invalidIndexes()
			}
			seen[c.Name] = true
			expression := q(c.Name)
			group := expression
			if term.Expression != "" {
				if !indexTextType(c) {
					return empty, invalidIndexes()
				}
				expression = "pg_catalog." + term.Expression + "(" + expression + ")"
				group = expression
				expression = "(" + expression + ")"
			}
			if term.PrefixLength > 0 {
				typ := strings.ToLower(c.DatabaseType)
				if !indexTextType(c) && !strings.Contains(typ, "binary") && !strings.Contains(typ, "blob") {
					return empty, invalidIndexes()
				}
				expression += fmt.Sprintf("(%d)", term.PrefixLength)
				group = fmt.Sprintf("LEFT(%s,%d)", q(c.Name), term.PrefixLength)
			}
			if def.Kind == "fulltext" && !indexTextType(c) {
				return empty, invalidIndexes()
			}
			if def.Kind == "spatial" && (len(def.Terms) != 1 || c.Nullable) {
				return empty, invalidIndexes()
			}
			groups = append(groups, group)
			nonnull = append(nonnull, group+" IS NOT NULL")
			nulls = append(nulls, q(c.Name)+" IS NULL")
			if term.Direction != "" {
				expression += " " + strings.ToUpper(term.Direction)
			}
			terms = append(terms, expression)
		}
		predicate := ""
		if def.Predicate != nil {
			c, err := indexColumn(d, def.Predicate.Column)
			if err != nil {
				return empty, err
			}
			predicate = q(c.Name) + " IS NULL"
			if def.Predicate.Operator == "is_not_null" {
				predicate = q(c.Name) + " IS NOT NULL"
			}
		}
		termSQL := "(" + strings.Join(terms, ", ") + ")"
		if def.Kind == "primary" {
			addAlter("ADD CONSTRAINT " + q(def.Name) + " PRIMARY KEY " + termSQL)
		} else if driver == "mysql" {
			prefix := map[string]string{"index": "INDEX", "unique": "UNIQUE INDEX", "fulltext": "FULLTEXT INDEX", "spatial": "SPATIAL INDEX"}[def.Kind]
			addAlter("ADD " + prefix + " " + q(def.Name) + " " + termSQL)
		} else {
			prefix := "CREATE INDEX "
			if def.Kind == "unique" {
				prefix = "CREATE UNIQUE INDEX "
			}
			// SQLite requires the schema on the index name, not on the table name.
			on := table
			indexName := q(def.Name)
			if driver == "sqlite" {
				on = q(d.Table.Name)
				indexName = qualified(core.TableRef{Schema: d.Table.Schema, Name: def.Name})
			}
			statement := prefix + indexName + " ON " + on + " " + termSQL
			if predicate != "" {
				statement += " WHERE " + predicate
			}
			statements = append(statements, statement)
		}
		if def.Kind == "primary" || def.Kind == "unique" {
			where := strings.Join(nonnull, " AND ")
			if predicate != "" {
				where += " AND (" + predicate + ")"
			}
			var count int64
			err = s.QueryRowContext(ctx, "SELECT COUNT(*) FROM (SELECT 1 FROM "+table+" WHERE "+where+" GROUP BY "+strings.Join(groups, ", ")+" HAVING COUNT(*)>1) AS dbm_duplicates").Scan(&count)
			if err != nil {
				return empty, err
			}
			p.DuplicateGroups = &count
			warnings = append(warnings, fmt.Sprintf("Vorprüfung: %d Gruppen mit doppelten Schlüsselwerten; NULL-Werte bei UNIQUE ausgenommen. Konkurrenz bleibt möglich.", count))
			if def.Kind == "primary" {
				err = s.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE "+strings.Join(nulls, " OR ")+")").Scan(&p.ContainsNull)
				if err != nil {
					return empty, err
				}
				if p.ContainsNull {
					warnings = append(warnings, "Vorhandene NULL-Werte verhindern den Primärschlüssel.")
				}
			}
		}
	}
	if f := r.ForeignKey; f != nil {
		ref, err := indexTable(ctx, s, r.Connection, f.ReferencedTable)
		if err != nil {
			return empty, err
		}
		affected = append(affected, ref.Table.Schema+"."+ref.Table.Name)
		local, remote := []string{}, []string{}
		seen := map[string]bool{}
		seenRef := map[string]bool{}
		for i, n := range f.Columns {
			lc, err := indexColumn(d, n)
			if err != nil {
				return empty, err
			}
			rc, err := indexColumn(ref, f.ReferencedColumns[i])
			if err != nil {
				return empty, err
			}
			// Deliberately conservative: exact catalog types, including length/sign.
			if !strings.EqualFold(lc.DatabaseType, rc.DatabaseType) || seen[lc.Name] || seenRef[rc.Name] {
				return empty, invalidIndexes()
			}
			if (f.OnDelete == "set_null" || f.OnUpdate == "set_null") && !lc.Nullable {
				return empty, invalidIndexes()
			}
			seen[lc.Name] = true
			seenRef[rc.Name] = true
			local = append(local, q(lc.Name))
			remote = append(remote, q(rc.Name))
		}
		action := func(value string) string { return strings.ToUpper(strings.ReplaceAll(value, "_", " ")) }
		addAlter("ADD CONSTRAINT " + q(f.Name) + " FOREIGN KEY (" + strings.Join(local, ", ") + ") REFERENCES " + qualified(ref.Table) + " (" + strings.Join(remote, ", ") + ") ON DELETE " + action(f.OnDelete) + " ON UPDATE " + action(f.OnUpdate))
		warnings = append(warnings, "Referenzschlüssel, Kollation, Operatorverträglichkeit und vorhandene Daten prüft die Engine. CASCADE/SET NULL/SET DEFAULT können bei späteren Datenänderungen Zeilen ändern oder löschen.")
	}
	if driver == "mysql" {
		statements = []string{"ALTER TABLE " + table + " " + strings.Join(clauses, ", ")}
	}
	if old.name != "" {
		affected = append(affected, old.name)
	}
	if name != "" {
		affected = append(affected, name)
	}
	p.Plan, err = dbplan.NewPlan(driver, statements, warnings, affected)
	if err != nil {
		return empty, invalidIndexes()
	}
	encoded, err := json.Marshal(p)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Schlüsselplan ist zu groß.")
	}
	return p, nil
}
