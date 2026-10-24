// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

var opToken = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
var opLocale = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,64}$`)

// opSourceTable resolves a base table from catalog metadata. The returned
// description carries the catalog spelling of schema and name; only that
// spelling is ever quoted into statements or used as a confirmation target.
func opSourceTable(ctx context.Context, env *opEnv, ref core.TableRef) (core.TableDescription, error) {
	if opSystemSchema(env.c.Driver, ref.Schema) {
		return core.TableDescription{}, invalidOp("Systemobjekte sind von diesen Aktionen ausgeschlossen.")
	}
	desc, err := describe(ctx, env.sess, env.c, ref)
	if err != nil {
		if p, ok := err.(*core.Error); ok {
			switch p.Code {
			case "table_not_found":
				return desc, opNotFound()
			case "invalid_table":
				if env.c.Driver == "sqlite" {
					return desc, opNotFound()
				}
				return desc, invalidOp(p.Message)
			}
		}
		return desc, err
	}
	if desc.Table.Name != ref.Name || ref.Schema != "" && desc.Table.Schema != ref.Schema {
		return desc, opNotFound()
	}
	if opSystemSchema(env.c.Driver, desc.Table.Schema) {
		return desc, invalidOp("Systemobjekte sind von diesen Aktionen ausgeschlossen.")
	}
	if desc.Kind != "table" {
		return desc, invalidOp("Nur Basistabellen werden unterstützt (keine Sichten, virtuellen oder externen Tabellen).")
	}
	return desc, nil
}

// opTableExists reports whether any table-like object with this name exists.
func opTableExists(ctx context.Context, q metadataQuerier, c core.Connection, ref core.TableRef) (bool, error) {
	_, err := describe(ctx, q, c, ref)
	if err == nil {
		return true, nil
	}
	if p, ok := err.(*core.Error); ok && p.Code == "table_not_found" {
		return false, nil
	}
	return false, err
}

// opDestinationSchema resolves the schema of a rename/copy target.
func opDestinationSchema(ctx context.Context, env *opEnv, source string, want *string) (string, error) {
	if want == nil || *want == "" || *want == source {
		return source, nil
	}
	switch env.c.Driver {
	case "sqlite":
		return "", public("unsupported_action", "SQLite kennt nur die Datenbank main; ein anderes Zielschema ist nicht möglich.")
	case "mysql":
		if _, found, err := mysqlSchemaInfo(ctx, env.sess, *want); err != nil {
			return "", err
		} else if !found {
			return "", opNotFound()
		}
	case "postgres":
		var name string
		err := env.sess.QueryRowContext(ctx, "SELECT nspname FROM pg_namespace WHERE nspname = $1", *want).Scan(&name)
		if err != nil {
			return "", opNotFoundOr(err)
		}
	}
	if opSystemSchema(env.c.Driver, *want) {
		return "", invalidOp("Systemschemas sind als Ziel ausgeschlossen.")
	}
	return *want, nil
}

func opNotFoundOr(err error) error {
	if err.Error() == "sql: no rows in result set" {
		return opNotFound()
	}
	return err
}

func opQualified(driver, schema, name string) (string, error) {
	if driver == "sqlite" {
		return dbplan.QuoteIdentifier(driver, name)
	}
	return dbplan.QualifiedName(driver, schema, name)
}

func opLiteral(value string) (string, error) {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return "", invalidOp("Text enthält ungültige Zeichen.")
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'", nil
}

// buildOp resolves the targets from the catalog and builds the plan.
func buildOp(ctx context.Context, env *opEnv, action string, r core.ObjectOpRequest) (*opState, error) {
	driver := env.c.Driver
	st := &opState{action: action}
	var statements, warnings, affected []string
	var deps []core.ObjectDependency
	truncated := false
	var current *core.TableOptions
	switch action {
	case core.OpDatabaseCreate, core.OpDatabaseDrop, core.OpDatabaseCollation:
		name := *r.Database
		st.db = name
		st.target = name
		switch action {
		case core.OpDatabaseCreate:
			if !opName(driver, name) || opSystemDatabase(driver, name) {
				return nil, invalidOp("Datenbankname ist ungültig oder reserviert.")
			}
			exists, _, err := opDatabaseInfo(ctx, env, name)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, opExists()
			}
			quoted, err := dbplan.QuoteIdentifier(driver, name)
			if err != nil {
				return nil, invalidOp("Datenbankname ist ungültig.")
			}
			statement := "CREATE DATABASE " + quoted
			if r.Collation != nil && *r.Collation != "" {
				suffix, err := opDatabaseCollationClause(ctx, env, *r.Collation, action)
				if err != nil {
					return nil, err
				}
				statement += suffix
			}
			statements = []string{statement}
			affected = []string{"database:" + name}
			warnings = []string{"Die Datenbank wird neu angelegt; sie enthält danach keine Objekte."}
			if driver == "postgres" {
				warnings = append(warnings, "CREATE DATABASE läuft in PostgreSQL außerhalb einer Transaktion und erfordert das Recht CREATEDB.")
			}
		case core.OpDatabaseDrop:
			exists, system, err := opDatabaseInfo(ctx, env, name)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, opNotFound()
			}
			if system || opSystemDatabase(driver, name) {
				return nil, invalidOp("Systemdatenbanken können nicht gelöscht werden.")
			}
			isCurrent := env.c.Database == name
			if isCurrent && driver == "postgres" {
				return nil, invalidOp("PostgreSQL kann die Datenbank der aktuellen Verbindung nicht löschen. Mit einer anderen Datenbank verbinden.")
			}
			quoted, err := dbplan.QuoteIdentifier(driver, name)
			if err != nil {
				return nil, invalidOp("Datenbankname ist ungültig.")
			}
			statements = []string{"DROP DATABASE " + quoted}
			st.destructive, st.currentDB = true, isCurrent
			affected = []string{"database:" + name}
			warnings = []string{"Die Datenbank und alle darin enthaltenen Objekte und Daten werden unwiderruflich gelöscht."}
			if isCurrent {
				warnings = append(warnings, "Dies ist die Datenbank der aktuellen Verbindung; sie erfordert eine zusätzliche ausdrückliche Bestätigung.")
			}
			if driver == "mysql" {
				objects, more, crossing, err := mysqlDatabaseContents(ctx, env.sess, name)
				if err != nil {
					return nil, err
				}
				affected = append(affected, objects...)
				if more {
					warnings = append(warnings, "Die Liste der betroffenen Objekte ist gekürzt.")
				}
				deps = opEffects(action, driver, crossing)
				truncated = more
				warnings = append(warnings, fmt.Sprintf("Die Datenbank enthält %d Tabellen oder Sichten.", len(objects)))
			} else {
				warnings = append(warnings, "Der Inhalt einer anderen Datenbank ist von dieser Verbindung aus nicht einsehbar; Offene Sitzungen verhindern das Löschen.")
			}
		case core.OpDatabaseCollation:
			exists, system, err := opDatabaseInfo(ctx, env, name)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, opNotFound()
			}
			if system || opSystemDatabase(driver, name) {
				return nil, invalidOp("Systemdatenbanken werden nicht geändert.")
			}
			if r.Collation == nil || *r.Collation == "" {
				return nil, invalidOp("Die Kollation fehlt.")
			}
			quoted, err := dbplan.QuoteIdentifier(driver, name)
			if err != nil {
				return nil, invalidOp("Datenbankname ist ungültig.")
			}
			suffix, err := opDatabaseCollationClause(ctx, env, *r.Collation, action)
			if err != nil {
				return nil, err
			}
			statements = []string{"ALTER DATABASE " + quoted + suffix}
			affected = []string{"database:" + name}
			warnings = []string{"Nur der Standard der Datenbank ändert sich; bestehende Tabellen und Spalten behalten ihre Kollation."}
		}
		st.dst = core.TableRef{Name: name}
		st.view.Target = core.ObjectOpTarget{Kind: "database", Name: name}
	default:
		desc, err := opSourceTable(ctx, env, *r.Table)
		if err != nil {
			return nil, err
		}
		st.src = desc.Table
		st.dst = desc.Table
		st.target = desc.Table.Name
		st.view.Target = core.ObjectOpTarget{Kind: "table", Schema: desc.Table.Schema, Name: desc.Table.Name}
		st.transact = driver != "mysql"
		source, err := opQualified(driver, desc.Table.Schema, desc.Table.Name)
		if err != nil {
			return nil, invalidOp("Tabellenname ist ungültig.")
		}
		affected = []string{"table:" + opLabel(desc.Table)}
		deps, truncated, err = tableDependencies(ctx, env, desc)
		if err != nil {
			return nil, err
		}
		deps = opEffects(action, driver, deps)
		switch action {
		case core.OpTableRename:
			st.destructive = true
			statements, warnings, err = planRename(ctx, env, st, desc, source, r)
		case core.OpTableCopy:
			statements, warnings, err = planCopy(ctx, env, st, desc, source, r)
		case core.OpTableTruncate:
			st.destructive = true
			statements, warnings, err = planTruncate(env, st, desc, source, r)
		case core.OpTableDrop:
			st.destructive = true
			statements = []string{"DROP TABLE " + source}
			warnings = []string{"Die Tabelle und alle ihre Daten werden unwiderruflich gelöscht."}
		case core.OpTableOptions:
			statements, warnings, current, err = planOptions(ctx, env, st, desc, source, r)
		}
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, opDependencyWarnings(deps, truncated)...)
		if driver == "mysql" {
			warnings = append(warnings, "MySQL und MariaDB führen DDL mit implizitem Commit aus; die Aktion ist nicht transaktional.")
		}
	}
	plan, err := dbplan.NewPlan(driver, statements, warnings, affected)
	if err != nil {
		return nil, invalidOp("Es ergibt sich keine ausführbare Aktion.")
	}
	st.plan = plan
	ddl := "yes"
	if driver == "mysql" || !st.transact {
		ddl = "no"
	}
	if deps == nil {
		deps = []core.ObjectDependency{}
	}
	st.view = core.ObjectOpPlan{Action: action, Driver: driver, Target: st.view.Target, Statements: plan.Statements, Warnings: plan.Warnings, AffectedObjects: plan.Affected,
		Dependencies: deps, DependenciesTruncated: truncated, CurrentOptions: current, DDLTransactional: ddl,
		Confirmation: core.ObjectOpConfirmation{Action: action, Destructive: st.destructive, TargetName: st.target, CurrentDatabase: st.currentDB}, PlanSHA256: plan.SHA256}
	return st, nil
}

func opLabel(ref core.TableRef) string {
	if ref.Schema == "" || ref.Schema == "main" {
		return ref.Name
	}
	return ref.Schema + "." + ref.Name
}

func planRename(ctx context.Context, env *opEnv, st *opState, desc core.TableDescription, source string, r core.ObjectOpRequest) ([]string, []string, error) {
	driver := env.c.Driver
	newName := desc.Table.Name
	if r.NewName != nil {
		newName = *r.NewName
	}
	if !opName(driver, newName) {
		return nil, nil, invalidOp("Der neue Tabellenname ist ungültig.")
	}
	schema, err := opDestinationSchema(ctx, env, desc.Table.Schema, r.NewSchema)
	if err != nil {
		return nil, nil, err
	}
	if newName == desc.Table.Name && schema == desc.Table.Schema {
		return nil, nil, invalidOp("Name und Schema entsprechen bereits dem Ziel.")
	}
	st.dst = core.TableRef{Schema: schema, Name: newName}
	exists, err := opTableExists(ctx, env.sess, env.c, st.dst)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, opExists()
	}
	target, err := opQualified(driver, schema, newName)
	if err != nil {
		return nil, nil, invalidOp("Der neue Tabellenname ist ungültig.")
	}
	var statements []string
	warnings := []string{"Programme und Abfragen, die den alten Namen verwenden, funktionieren danach nicht mehr."}
	switch driver {
	case "mysql":
		statements = []string{"RENAME TABLE " + source + " TO " + target}
	case "sqlite":
		quoted, _ := dbplan.QuoteIdentifier(driver, newName)
		statements = []string{"ALTER TABLE " + source + " RENAME TO " + quoted}
	case "postgres":
		current := source
		if newName != desc.Table.Name {
			quoted, _ := dbplan.QuoteIdentifier(driver, newName)
			statements = append(statements, "ALTER TABLE "+source+" RENAME TO "+quoted)
			current, _ = opQualified(driver, desc.Table.Schema, newName)
		}
		if schema != desc.Table.Schema {
			quoted, _ := dbplan.QuoteIdentifier(driver, schema)
			statements = append(statements, "ALTER TABLE "+current+" SET SCHEMA "+quoted)
			warnings = append(warnings, "Indizes und Sequenzen wechseln mit der Tabelle das Schema; Rechte bleiben erhalten.")
		}
	}
	st.expect = func(o *core.ObjectOpObservation) bool {
		return o.TargetExists != nil && *o.TargetExists && o.SourceExists != nil && !*o.SourceExists
	}
	return statements, warnings, nil
}

// copyColumns returns the source columns that can be written explicitly.
func copyColumns(driver string, desc core.TableDescription) (names []string, identity bool, err error) {
	for _, column := range desc.Columns {
		switch column.Generated {
		case "stored", "virtual", "hidden":
			continue
		case "identity":
			identity = true
		}
		quoted, err := dbplan.QuoteIdentifier(driver, column.Name)
		if err != nil {
			return nil, false, invalidOp("Spaltenname ist ungültig.")
		}
		names = append(names, quoted)
	}
	if len(names) == 0 {
		return nil, false, invalidOp("Die Tabelle hat keine kopierbaren Spalten.")
	}
	return names, identity, nil
}

func planCopy(ctx context.Context, env *opEnv, st *opState, desc core.TableDescription, source string, r core.ObjectOpRequest) ([]string, []string, error) {
	driver := env.c.Driver
	mode := "both"
	if r.Mode != nil {
		mode = *r.Mode
	}
	if mode != "structure" && mode != "data" && mode != "both" {
		return nil, nil, invalidOp("Modus muss structure, data oder both sein.")
	}
	if r.NewName == nil || !opName(driver, *r.NewName) {
		return nil, nil, invalidOp("Der Name der Kopie ist ungültig.")
	}
	schema, err := opDestinationSchema(ctx, env, desc.Table.Schema, r.NewSchema)
	if err != nil {
		return nil, nil, err
	}
	st.dst = core.TableRef{Schema: schema, Name: *r.NewName}
	if st.dst == desc.Table {
		return nil, nil, invalidOp("Quelle und Ziel sind dieselbe Tabelle.")
	}
	target, err := opQualified(driver, schema, *r.NewName)
	if err != nil {
		return nil, nil, invalidOp("Der Name der Kopie ist ungültig.")
	}
	st.srcRows = mode != "structure"
	var statements, warnings []string
	warnings = append(warnings, "Fremdschlüssel und Trigger werden nicht kopiert.")
	var targetDesc core.TableDescription
	if mode == "data" {
		targetDesc, err = opSourceTable(ctx, env, st.dst)
		if err != nil {
			return nil, nil, err
		}
		st.dst = targetDesc.Table
		have := map[string]bool{}
		for _, column := range targetDesc.Columns {
			have[column.Name] = true
		}
		for _, column := range desc.Columns {
			if column.Generated == "stored" || column.Generated == "virtual" || column.Generated == "hidden" {
				continue
			}
			if !have[column.Name] {
				return nil, nil, invalidOp("Die Zieltabelle enthält die Spalte " + column.Name + " nicht.")
			}
		}
		warnings = append(warnings, "Die Zeilen werden an die vorhandene Zieltabelle angehängt; Schlüsselkonflikte lehnt der Server ab.")
	} else {
		exists, err := opTableExists(ctx, env.sess, env.c, st.dst)
		if err != nil {
			return nil, nil, err
		}
		if exists {
			return nil, nil, opExists()
		}
	}
	if mode != "data" {
		switch driver {
		case "mysql":
			statements = append(statements, "CREATE TABLE "+target+" LIKE "+source)
		case "postgres":
			statements = append(statements, "CREATE TABLE "+target+" (LIKE "+source+" INCLUDING ALL)")
		case "sqlite":
			tail := ""
			if mode == "structure" {
				tail = " WHERE 0"
			}
			statements = append(statements, "CREATE TABLE "+target+" AS SELECT * FROM "+source+tail)
			warnings = append(warnings, "SQLite kopiert nur Spaltennamen und Affinitäten, keine Schlüssel, Indizes, Standardwerte oder Einschränkungen.")
		}
	}
	insert := mode == "data" || mode == "both" && driver != "sqlite"
	if insert {
		columns, identity, err := copyColumns(driver, desc)
		if err != nil {
			return nil, nil, err
		}
		list := strings.Join(columns, ", ")
		override := ""
		if driver == "postgres" && identity {
			override = " OVERRIDING SYSTEM VALUE"
		}
		statements = append(statements, "INSERT INTO "+target+" ("+list+")"+override+" SELECT "+list+" FROM "+source)
	}
	if driver == "postgres" && mode != "data" {
		for _, column := range desc.Columns {
			if column.DefaultValue != nil && strings.Contains(*column.DefaultValue, "nextval(") {
				warnings = append(warnings, "Die Spalte "+column.Name+" verwendet nextval(): die Kopie teilt die Sequenz der Quelle.")
			}
			if column.Generated == "identity" && mode != "structure" {
				warnings = append(warnings, "Der Identity-Zähler der Spalte "+column.Name+" in der Kopie wird nicht nachgeführt; neue Zeilen können mit kopierten Werten kollidieren.")
			}
		}
	}
	if mode == "structure" {
		warnings = append(warnings, "Es werden keine Zeilen kopiert.")
	}
	st.expect = func(o *core.ObjectOpObservation) bool { return o.TargetExists != nil && *o.TargetExists }
	return statements, warnings, nil
}

func planTruncate(env *opEnv, st *opState, desc core.TableDescription, source string, r core.ObjectOpRequest) ([]string, []string, error) {
	driver := env.c.Driver
	restart := r.RestartIdentity != nil && *r.RestartIdentity
	identity := false
	for _, column := range desc.Columns {
		identity = identity || column.Generated == "identity" || column.DefaultValue != nil && strings.Contains(*column.DefaultValue, "nextval(")
	}
	warnings := []string{"Alle Zeilen der Tabelle werden unwiderruflich entfernt; die Tabelle selbst bleibt bestehen."}
	var statement string
	switch driver {
	case "mysql":
		if restart {
			return nil, nil, public("unsupported_action", "MySQL und MariaDB setzen den Auto-Inkrement-Zähler bei TRUNCATE immer zurück; restart_identity entfällt.")
		}
		statement = "TRUNCATE TABLE " + source
		warnings = append(warnings, "TRUNCATE ist DDL: implizites Commit, der Auto-Inkrement-Zähler wird zurückgesetzt, ON DELETE-Trigger laufen nicht.")
	case "postgres":
		statement = "TRUNCATE TABLE " + source
		if restart {
			statement += " RESTART IDENTITY"
			warnings = append(warnings, "RESTART IDENTITY setzt die Sequenzen der Tabelle auf ihren Startwert zurück.")
		} else if identity {
			warnings = append(warnings, "Sequenzen und Identity-Zähler bleiben erhalten (kein RESTART IDENTITY); neue Zeilen setzen die Zählung fort.")
		}
	case "sqlite":
		if restart {
			return nil, nil, public("unsupported_action", "SQLite erlaubt das Zurücksetzen des AUTOINCREMENT-Zählers hier nicht.")
		}
		statement = "DELETE FROM " + source
		warnings = append(warnings, "SQLite hat kein TRUNCATE; die Tabelle wird mit DELETE FROM geleert, ein AUTOINCREMENT-Zähler (sqlite_sequence) bleibt erhalten.")
	}
	st.srcRows = true
	st.expect = func(o *core.ObjectOpObservation) bool { return o.Rows != nil && *o.Rows == 0 }
	return []string{statement}, warnings, nil
}

func planOptions(ctx context.Context, env *opEnv, st *opState, desc core.TableDescription, source string, r core.ObjectOpRequest) ([]string, []string, *core.TableOptions, error) {
	driver := env.c.Driver
	current, err := tableOptions(ctx, env, desc.Table)
	if err != nil {
		return nil, nil, nil, err
	}
	if driver == "postgres" && (r.Engine != nil || r.Collation != nil || r.CollationMode != nil || r.AutoIncrement != nil) {
		return nil, nil, nil, public("unsupported_action", "PostgreSQL unterstützt nur den Tabellenkommentar als Tabellenoption.")
	}
	if r.CollationMode != nil && r.Collation == nil {
		return nil, nil, nil, invalidOp("collation_mode erfordert collation.")
	}
	var specs, warnings []string
	if r.Engine != nil {
		engine, ok, err := mysqlEngine(ctx, env.sess, *r.Engine)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok || !opToken.MatchString(engine) {
			return nil, nil, nil, invalidOp("Die Speicher-Engine ist auf diesem Server nicht verfügbar.")
		}
		if current.Engine == nil || !strings.EqualFold(*current.Engine, engine) {
			specs = append(specs, "ENGINE = "+engine)
			warnings = append(warnings, "Ein Engine-Wechsel baut die Tabelle neu auf; das kann bei großen Tabellen lange dauern und Funktionen der alten Engine (zum Beispiel Fremdschlüssel) entfernen.")
		}
	}
	if r.AutoIncrement != nil {
		value := *r.AutoIncrement
		if value < 1 || value > core.ObjectOpMaxAutoIncrement {
			return nil, nil, nil, invalidOp("Der Auto-Inkrement-Wert ist ungültig.")
		}
		has := false
		for _, column := range desc.Columns {
			has = has || column.Generated == "identity"
		}
		if !has {
			return nil, nil, nil, invalidOp("Die Tabelle hat keine Auto-Inkrement-Spalte.")
		}
		if current.AutoIncrement == nil || *current.AutoIncrement != value {
			specs = append(specs, fmt.Sprintf("AUTO_INCREMENT = %d", value))
			warnings = append(warnings, "Ein Wert unterhalb des höchsten vorhandenen Schlüssels wird vom Server angehoben.")
		}
	}
	var postgresStatements []string
	if r.Comment != nil {
		if utf8.RuneCountInString(*r.Comment) > core.ObjectOpMaxComment {
			return nil, nil, nil, invalidOp("Der Kommentar ist zu lang.")
		}
		for _, c := range *r.Comment {
			if unicode.IsControl(c) && c != '\n' && c != '\t' && c != '\r' {
				return nil, nil, nil, invalidOp("Der Kommentar enthält Steuerzeichen.")
			}
		}
		literal, err := opLiteral(*r.Comment)
		if err != nil {
			return nil, nil, nil, err
		}
		if current.Comment == nil && *r.Comment != "" || current.Comment != nil && *current.Comment != *r.Comment {
			if driver == "mysql" {
				specs = append(specs, "COMMENT = "+literal)
			} else {
				postgresStatements = append(postgresStatements, "COMMENT ON TABLE "+source+" IS "+literal)
			}
		}
	}
	if r.Collation != nil {
		charset, collation, ok, err := mysqlCollation(ctx, env.sess, *r.Collation)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok || !opToken.MatchString(charset) || !opToken.MatchString(collation) {
			return nil, nil, nil, invalidOp("Die Kollation ist auf diesem Server nicht verfügbar.")
		}
		mode := "default"
		if r.CollationMode != nil {
			mode = *r.CollationMode
		}
		switch mode {
		case "default":
			if current.Collation == nil || *current.Collation != collation {
				specs = append(specs, "DEFAULT CHARACTER SET = "+charset+" COLLATE = "+collation)
				warnings = append(warnings, "Nur der Standard für neue Spalten ändert sich; vorhandene Spalten behalten ihre Kollation.")
			}
		case "convert":
			specs = append(specs, "CONVERT TO CHARACTER SET "+charset+" COLLATE "+collation)
			warnings = append(warnings, "Alle Textspalten werden umgewandelt und die Tabelle neu aufgebaut; bei Zeichensatzwechseln kann Text verändert werden oder Daten gehen verloren.")
		default:
			return nil, nil, nil, invalidOp("collation_mode muss default oder convert sein.")
		}
	}
	statements := postgresStatements
	if len(specs) > 0 {
		statements = append([]string{"ALTER TABLE " + source + " " + strings.Join(specs, ", ")}, statements...)
	}
	if len(statements) == 0 {
		return nil, nil, nil, invalidOp("Keine der angegebenen Optionen weicht vom aktuellen Wert ab.")
	}
	st.expect = func(o *core.ObjectOpObservation) bool {
		if driver != "postgres" || r.Comment == nil || o.Options == nil {
			return true
		}
		return o.Options.Comment != nil && *o.Options.Comment == *r.Comment || *r.Comment == "" && o.Options.Comment == nil
	}
	return statements, warnings, &current, nil
}

// opEffects fills the effect of an action on each dependency.
func opEffects(action, driver string, deps []core.ObjectDependency) []core.ObjectDependency {
	out := make([]core.ObjectDependency, len(deps))
	for i, d := range deps {
		d.Effect = "info"
		switch d.Kind {
		case "foreign_key_in":
			switch action {
			case core.OpTableRename:
				d.Effect = "follows"
			case core.OpTableDrop, core.OpTableTruncate, core.OpDatabaseDrop:
				d.Effect = "blocks"
			}
		case "view":
			switch action {
			case core.OpTableRename:
				d.Effect = "breaks"
				if driver != "mysql" {
					d.Effect = "follows"
				}
			case core.OpTableDrop:
				d.Effect = "breaks"
				if driver == "postgres" {
					d.Effect = "blocks"
				}
			}
		}
		out[i] = d
	}
	return out
}

func opDependencyWarnings(deps []core.ObjectDependency, truncated bool) []string {
	var out []string
	for _, d := range deps {
		label := d.Name
		if d.Schema != "" && d.Schema != "main" {
			label = d.Schema + "." + d.Name
		}
		suffix := ""
		if d.Certainty == "textual" {
			suffix = " (anhand des Quelltextes erkannt)"
		}
		switch {
		case d.Kind == "foreign_key_in" && d.Effect == "blocks":
			out = append(out, "Die Tabelle "+label+" verweist per Fremdschlüssel auf diese Tabelle; der Server lehnt die Aktion ab (kein CASCADE, Fremdschlüsselprüfung bleibt aktiv).")
		case d.Kind == "foreign_key_in" && d.Effect == "follows":
			out = append(out, "Der Fremdschlüssel der Tabelle "+label+" wird vom Server auf den neuen Namen umgestellt.")
		case d.Kind == "view" && d.Effect == "breaks":
			out = append(out, "Die Sicht "+label+" verwendet diese Tabelle und wird ungültig"+suffix+".")
		case d.Kind == "view" && d.Effect == "blocks":
			out = append(out, "Die Sicht "+label+" verwendet diese Tabelle und verhindert das Löschen (kein CASCADE).")
		case d.Kind == "view" && d.Effect == "follows":
			out = append(out, "Die Sicht "+label+" wird vom Server auf den neuen Namen umgestellt"+suffix+".")
		}
		if len(out) >= 20 {
			out = append(out, "Weitere Abhängigkeiten stehen in der Liste der Abhängigkeiten.")
			break
		}
	}
	if truncated {
		out = append(out, fmt.Sprintf("Es werden höchstens %d Abhängigkeiten angezeigt; weitere können bestehen.", core.ObjectOpMaxDependencies))
	}
	return out
}
