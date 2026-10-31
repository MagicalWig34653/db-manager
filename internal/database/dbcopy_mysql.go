// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// MySQL and MariaDB have no RENAME DATABASE. A copy creates the new schema,
// copies tables (CREATE TABLE LIKE plus INSERT ... SELECT), adds foreign keys
// after the data, then routines, views, triggers and events. A rename is a copy
// followed by a verified, lock-protected removal of the source.

const (
	dcMyDataMode   = "NO_AUTO_VALUE_ON_ZERO,NO_BACKSLASH_ESCAPES,NO_ENGINE_SUBSTITUTION"
	dcMaxColumns   = 200000
	dcMaxDefBytes  = 1 << 20
	dcMyFKUnnamed  = "unnamed"
)

var (
	dcModeRE    = regexp.MustCompile(`^[A-Za-z0-9_,]*$`)
	dcCollateRE = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	dcTZRE      = regexp.MustCompile(`^(SYSTEM|[+-][0-9]{1,2}:[0-9]{2}|[A-Za-z0-9_+/-]{1,64})$`)
	dcRuleOK    = map[string]bool{"RESTRICT": true, "CASCADE": true, "SET NULL": true, "NO ACTION": true, "SET DEFAULT": true}
)

// dcQuoter quotes identifiers and remembers the first error.
type dcQuoter struct {
	driver string
	err    error
}

func (q *dcQuoter) id(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted, err := dbplan.QuoteIdentifier(q.driver, part)
		if err != nil && q.err == nil {
			q.err = err
		}
		out = append(out, quoted)
	}
	return strings.Join(out, ".")
}

func (q *dcQuoter) list(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = q.id(n)
	}
	return strings.Join(quoted, ", ")
}

type myTable struct {
	name, typ, engine        string
	rows, dataBytes, idxBytes sql.NullInt64
}

type myColumn struct {
	table, name string
	generated   bool
	autoInc     bool
}

type myFK struct {
	name, table, refSchema, refTable string
	cols, refCols                    []string
	update, del                      string
}

type myDef struct {
	kind, name string
	text       string
	mode       string
	collation  string
	tz         string
	status     string
	routine    string // PROCEDURE or FUNCTION
}

func dcRows(ctx context.Context, q metadataQuerier, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err = each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// myShow runs a SHOW CREATE statement and returns its columns by name.
func myShow(ctx context.Context, q metadataQuerier, statement string) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	values := make([]sql.NullString, len(columns))
	targets := make([]any, len(columns))
	for i := range values {
		targets[i] = &values[i]
	}
	if err = rows.Scan(targets...); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, c := range columns {
		if values[i].Valid {
			out[c] = values[i].String
		}
	}
	return out, nil
}

func dcSchemaExists(ctx context.Context, q metadataQuerier, name string, foldCase bool) (bool, error) {
	exists := false
	err := dcRows(ctx, q, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE LOWER(SCHEMA_NAME) = LOWER(?) LIMIT 10", []any{name}, func(r *sql.Rows) error {
		var found string
		if err := r.Scan(&found); err != nil {
			return err
		}
		if found == name || foldCase && strings.EqualFold(found, name) {
			exists = true
		}
		return nil
	})
	return exists, err
}

// buildMySQLCopy reads the catalog of the source (and, when resuming, of the
// target) and assembles the phases, steps, objects, warnings and blockers.
func buildMySQLCopy(ctx context.Context, env *opEnv, action string, r core.DBCopyRequest) (*dcState, error) {
	sess := env.sess
	src, dst := *r.Source, *r.NewName
	rename := action == core.OpDatabaseRename
	resume := dcBool(r.Resume, false)
	withData := rename || r.Content == nil || *r.Content == "structure_and_data"
	incViews, incRoutines, incTriggers, incEvents := true, true, true, true
	if !rename && r.Include != nil {
		incViews, incRoutines, incTriggers, incEvents = dcBool(r.Include.Views, true), dcBool(r.Include.Routines, true), dcBool(r.Include.Triggers, true), dcBool(r.Include.Events, true)
	}
	fold := mysqlCaseInsensitive(ctx, sess)
	if src == dst || fold && strings.EqualFold(src, dst) {
		return nil, invalidCopy("Quell- und Zielname sind gleich.")
	}
	var charset, collation sql.NullString
	err := sess.QueryRowContext(ctx, "SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", src).Scan(&charset, &collation)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, opNotFound()
	}
	if err != nil {
		return nil, err
	}
	targetExists, err := dcSchemaExists(ctx, sess, dst, fold)
	if err != nil {
		return nil, err
	}
	switch {
	case targetExists && !resume:
		return nil, opExists()
	case !targetExists && resume:
		return nil, opNotFound()
	}
	if !charset.Valid || !collation.Valid || !dcCollateRE.MatchString(charset.String) || !dcCollateRE.MatchString(collation.String) {
		return nil, invalidCopy("Zeichensatz oder Kollation der Quelle ist nicht lesbar.")
	}
	isCurrent, err := opIsCurrentDatabase(ctx, env, src)
	if err != nil {
		return nil, err
	}

	// Catalog of the source.
	var tables []myTable
	err = dcRows(ctx, sess, "SELECT TABLE_NAME, TABLE_TYPE, ENGINE, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? LIMIT ?", []any{src, core.DBCopyMaxObjects + 1}, func(rows *sql.Rows) error {
		var t myTable
		var engine sql.NullString
		if err := rows.Scan(&t.name, &t.typ, &engine, &t.rows, &t.dataBytes, &t.idxBytes); err != nil {
			return err
		}
		t.engine = engine.String
		tables = append(tables, t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].name < tables[j].name })
	var base, views []myTable
	var unsupported []myTable
	for _, t := range tables {
		switch t.typ {
		case "BASE TABLE":
			base = append(base, t)
		case "VIEW":
			views = append(views, t)
		default:
			unsupported = append(unsupported, t)
		}
	}
	if len(tables) > core.DBCopyMaxObjects || len(base) > core.DBCopyMaxTables {
		return nil, dcLimit(fmt.Sprintf("Die Datenbank hat zu viele Objekte (Grenze: %d Objekte, %d Tabellen).", core.DBCopyMaxObjects, core.DBCopyMaxTables))
	}
	columns, err := myColumns(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	fks, err := myForeignKeys(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	routines, err := myRoutines(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	triggers, err := myNamed(ctx, sess, "SELECT TRIGGER_NAME FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = ? ORDER BY TRIGGER_NAME LIMIT ?", src)
	if err != nil {
		return nil, err
	}
	events, err := myEvents(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	if len(tables)+len(routines)+len(triggers)+len(events) > core.DBCopyMaxObjects {
		return nil, dcLimit(fmt.Sprintf("Die Datenbank hat zu viele Objekte (Grenze: %d).", core.DBCopyMaxObjects))
	}

	q := &dcQuoter{driver: "mysql"}
	st := &dcState{action: action, scope: "database", driver: "mysql", source: src, target: dst, withData: withData, resume: resume, rename: rename,
		expect: map[string][]string{}, srcObjects: map[string][]string{}, currentDB: rename && isCurrent, destructive: rename}
	if rename {
		st.typed = src
	}
	content := "structure_and_data"
	if !withData {
		content = "structure"
	}
	st.view = core.DBCopyPlan{Action: action, Driver: "mysql", Scope: "database", Source: src, Target: dst, Content: content, Resume: resume,
		AffectedObjects: []string{"database:" + dst}, Warnings: []string{}, Blockers: []string{}}
	if rename {
		st.view.AffectedObjects = append(st.view.AffectedObjects, "database:"+src)
	}
	block := func(format string, args ...any) { st.view.Blockers = append(st.view.Blockers, fmt.Sprintf(format, args...)) }
	warn := func(format string, args ...any) { st.view.Warnings = append(st.view.Warnings, fmt.Sprintf(format, args...)) }

	// Resume: what already exists in the target.
	var tgtTables map[string]myTable
	var tgtSets map[string]map[string]bool
	if resume {
		var list []myTable
		err = dcRows(ctx, sess, "SELECT TABLE_NAME, TABLE_TYPE, ENGINE, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? LIMIT ?", []any{dst, core.DBCopyMaxObjects + 1}, func(rows *sql.Rows) error {
			var t myTable
			var engine sql.NullString
			if err := rows.Scan(&t.name, &t.typ, &engine, &t.rows, &t.dataBytes, &t.idxBytes); err != nil {
				return err
			}
			list = append(list, t)
			return nil
		})
		if err != nil {
			return nil, err
		}
		tgtTables = map[string]myTable{}
		for _, t := range list {
			tgtTables[t.name] = t
		}
		if tgtSets, err = myTargetSets(ctx, sess, dst); err != nil {
			return nil, err
		}
		warn("Fortsetzen ergänzt fehlende Objekte in der vorhandenen Zieldatenbank %s. Vorhandene Tabellen müssen in der Struktur mit der Quelle übereinstimmen; Sichten, Routinen, Trigger und Ereignisse mit gleichem Namen werden nicht inhaltlich verglichen und nicht verändert.", dst)
	}

	// Phases in execution order.
	st.phases = []dcPhase{{"create_database", "Zieldatenbank anlegen", false}, {"tables", "Tabellen anlegen", false}}
	if withData {
		st.phases = append(st.phases, dcPhase{"data", "Daten kopieren und Zähler setzen", false})
	}
	st.phases = append(st.phases, dcPhase{"foreign_keys", "Fremdschlüssel anlegen", false})
	if incRoutines {
		st.phases = append(st.phases, dcPhase{"routines", "Routinen anlegen", false})
	}
	if incViews {
		st.phases = append(st.phases, dcPhase{"views", "Sichten anlegen", false})
	}
	if incTriggers {
		st.phases = append(st.phases, dcPhase{"triggers", "Trigger anlegen", false})
	}
	if incEvents {
		st.phases = append(st.phases, dcPhase{"events", "Ereignisse deaktiviert anlegen", false})
	}
	st.phases = append(st.phases, dcPhase{"verify", "Kopie verifizieren", false})
	if rename {
		st.phases = append(st.phases, dcPhase{"drop_source", "Quelle löschen (Sperre, Prüfsummen, DROP)", true}, dcPhase{"enable_events", "Ereignisse aktivieren", false})
	}

	if !resume {
		st.steps = append(st.steps, dcStep{phase: "create_database", kind: "database", name: dst,
			stmts: []string{"CREATE DATABASE " + q.id(dst) + " CHARACTER SET " + charset.String + " COLLATE " + collation.String}})
	}

	// Tables in foreign key order (parents first).
	order := myTableOrder(base, fks)
	colsOf := map[string][]myColumn{}
	for _, c := range columns {
		colsOf[c.table] = append(colsOf[c.table], c)
	}
	var estRows, estData, estIndex int64
	dataSeen := false
	tableAction := map[string]string{}
	for _, name := range order {
		var t myTable
		for _, b := range base {
			if b.name == name {
				t = b
			}
		}
		action := "create"
		note := ""
		if resume {
			if existing, ok := tgtTables[name]; ok {
				a, why := myResumeTable(ctx, sess, q, src, dst, name, existing)
				action = a
				if a == "conflict" {
					block("Die Tabelle %s existiert im Ziel und weicht von der Quelle ab oder ist nur teilweise gefüllt (%s). Zieltabelle prüfen oder löschen.", name, why)
					action, note = "unsupported", why
				} else if a == "skip_complete" {
					note = why
				}
			}
		}
		tableAction[name] = action
		var rowsEst *int64
		if t.rows.Valid {
			v := t.rows.Int64
			rowsEst = &v
			estRows += v
			dataSeen = true
		}
		var bytesEst *int64
		if t.dataBytes.Valid {
			v := t.dataBytes.Int64 + t.idxBytes.Int64
			bytesEst = &v
			estData += t.dataBytes.Int64
			estIndex += t.idxBytes.Int64
			dataSeen = true
		}
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "table", Name: name, Action: action, RowsEstimate: rowsEst, BytesEstimate: bytesEst, Note: note})
		if action == "create" {
			st.steps = append(st.steps, dcStep{phase: "tables", kind: "table", name: name, stmts: []string{"CREATE TABLE " + q.id(dst, name) + " LIKE " + q.id(src, name)}})
		}
		if withData && (action == "create" || action == "fill") {
			var names []string
			auto := false
			for _, c := range colsOf[name] {
				if !c.generated {
					names = append(names, c.name)
				}
				auto = auto || c.autoInc
			}
			if len(names) == 0 {
				continue
			}
			stmts := []string{"SET SESSION sql_mode = '" + dcMyDataMode + "'",
				"INSERT INTO " + q.id(dst, name) + " (" + q.list(names) + ") SELECT " + q.list(names) + " FROM " + q.id(src, name)}
			step := dcStep{phase: "data", kind: "table", name: name, stmts: stmts, restore: true, rows: t.rows.Int64}
			if auto {
				step.stmts = append(step.stmts, "ALTER TABLE "+q.id(dst, name)+" AUTO_INCREMENT = "+dcAutoIncrement)
				step.aiSchema, step.aiTable = src, name
			}
			st.steps = append(st.steps, step)
		}
		if action != "unsupported" {
			st.tables = append(st.tables, name)
		}
	}
	for _, t := range unsupported {
		st.skipped++
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "table", Name: t.name, Action: "unsupported", Note: "Tabellenart " + t.typ + " wird nicht kopiert."})
		if rename {
			block("Das Objekt %s (Art %s) kann nicht kopiert werden; das Löschen der Quelle würde es vernichten.", t.name, t.typ)
		} else {
			warn("Das Objekt %s (Art %s) wird nicht kopiert.", t.name, t.typ)
		}
	}
	st.srcObjects["table"] = namesOf(base)
	st.expect["table"] = append([]string{}, st.tables...)

	// Foreign keys after the data.
	skippedTable := map[string]bool{}
	for _, t := range unsupported {
		skippedTable[t.name] = true
	}
	fkCount := 0
	for _, fk := range fks {
		ref := fk.refSchema
		if ref == src {
			ref = dst
		}
		if skippedTable[fk.refTable] && fk.refSchema == src || tableAction[fk.table] == "unsupported" {
			warn("Der Fremdschlüssel %s.%s wird nicht angelegt, weil ein beteiligtes Objekt nicht kopiert wird.", fk.table, fk.name)
			continue
		}
		if resume && tgtSets["fk"][fk.table+"\x00"+fk.name] {
			continue
		}
		stmt := "ALTER TABLE " + q.id(dst, fk.table) + " ADD CONSTRAINT " + q.id(fk.name) + " FOREIGN KEY (" + q.list(fk.cols) + ") REFERENCES " + q.id(ref, fk.refTable) + " (" + q.list(fk.refCols) + ")" +
			" ON DELETE " + fk.del + " ON UPDATE " + fk.update
		st.steps = append(st.steps, dcStep{phase: "foreign_keys", kind: "foreign_key", name: fk.table + "." + fk.name, stmts: []string{stmt}})
		st.expect["fk"] = append(st.expect["fk"], fk.table+"\x00"+fk.name)
		fkCount++
		if fk.refSchema != src {
			warn("Der Fremdschlüssel %s.%s verweist auf die andere Datenbank %s und bleibt dort verankert.", fk.table, fk.name, fk.refSchema)
		}
	}
	st.srcObjects["fk"] = nil
	for _, fk := range fks {
		st.srcObjects["fk"] = append(st.srcObjects["fk"], fk.table+"\x00"+fk.name)
	}
	_ = fkCount

	viewMode, err := myViewMode(ctx, sess)
	if err != nil {
		return nil, err
	}
	st.viewMode = viewMode
	mentions := []string{}
	useDst := "USE " + q.id(dst)
	totalDef := 0
	addDef := func(phase string, d myDef, include bool, kindLabel string, retry bool) error {
		inTarget := resume && tgtSets[d.kind][d.name]
		if !include {
			st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: kindLabel, Name: d.name, Action: "skip_option"})
			return nil
		}
		st.srcObjects[d.kind] = append(st.srcObjects[d.kind], d.name)
		if d.text == "" {
			st.skipped++
			st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: kindLabel, Name: d.name, Action: "unsupported", Note: "Die Definition ist für diesen Benutzer nicht lesbar."})
			if rename {
				block("Die Definition von %s %s ist nicht lesbar; das Löschen der Quelle würde sie vernichten.", kindLabel, d.name)
			} else {
				warn("Die Definition von %s %s ist nicht lesbar; das Objekt wird nicht kopiert.", kindLabel, d.name)
			}
			return nil
		}
		totalDef += len(d.text)
		if totalDef > core.DBCopyMaxPlanBytes {
			return public("result_too_large", "Die Definitionen der Datenbank überschreiten die zulässige Plangröße.")
		}
		if dcMentionsSchema(d.text, src) {
			mentions = append(mentions, kindLabel+" "+d.name)
		}
		if inTarget {
			st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: kindLabel, Name: d.name, Action: "skip_complete", Note: "Im Ziel vorhanden (nicht inhaltlich verglichen)."})
			st.expect[d.kind] = append(st.expect[d.kind], d.name)
			return nil
		}
		stmts := []string{useDst}
		mode := d.mode
		if d.kind == "view" {
			mode = viewMode
		}
		if mode != "" {
			if !dcModeRE.MatchString(mode) {
				return invalidCopy("Der gespeicherte sql_mode von " + d.name + " ist ungültig.")
			}
			stmts = append(stmts, "SET SESSION sql_mode = '"+mode+"'")
		}
		if d.collation != "" {
			if !dcCollateRE.MatchString(d.collation) {
				return invalidCopy("Die gespeicherte Kollation von " + d.name + " ist ungültig.")
			}
			stmts = append(stmts, "SET SESSION collation_connection = '"+d.collation+"'")
		}
		if d.kind == "event" && d.tz != "" && d.tz != "SYSTEM" {
			if !dcTZRE.MatchString(d.tz) {
				return invalidCopy("Die gespeicherte Zeitzone von " + d.name + " ist ungültig.")
			}
			stmts = append(stmts, "SET SESSION time_zone = '"+d.tz+"'")
		}
		stmts = append(stmts, d.text)
		if d.kind == "event" {
			stmts = append(stmts, "ALTER EVENT "+q.id(dst, d.name)+" DISABLE")
		}
		st.steps = append(st.steps, dcStep{phase: phase, kind: kindLabel, name: d.name, stmts: stmts, tolerate: true, retry: retry, restore: true})
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: kindLabel, Name: d.name, Action: "create"})
		st.expect[d.kind] = append(st.expect[d.kind], d.name)
		return nil
	}
	// Routines first (views may call functions), then views, triggers, events.
	for i := range routines {
		d, err := myRoutineDef(ctx, sess, q, src, routines[i])
		if err != nil {
			return nil, err
		}
		if err = addDef("routines", d, incRoutines, strings.ToLower(d.routine), false); err != nil {
			return nil, err
		}
	}
	for _, v := range views {
		d, err := myViewDef(ctx, sess, q, src, v.name)
		if err != nil {
			return nil, err
		}
		if err = addDef("views", d, incViews, "view", true); err != nil {
			return nil, err
		}
	}
	for _, name := range triggers {
		d, err := myTriggerDef(ctx, sess, q, src, name)
		if err != nil {
			return nil, err
		}
		if err = addDef("triggers", d, incTriggers, "trigger", false); err != nil {
			return nil, err
		}
	}
	for _, ev := range events {
		d, err := myEventDef(ctx, sess, q, src, ev)
		if err != nil {
			return nil, err
		}
		if err = addDef("events", d, incEvents, "event", false); err != nil {
			return nil, err
		}
		if incEvents && ev.status == "ENABLED" {
			st.enableEvt = append(st.enableEvt, ev.name)
		}
	}
	// Routine kinds share the expectation key.
	if q.err != nil {
		return nil, invalidCopy("Ein Objektname ist nicht verwendbar.")
	}
	if len(mentions) > 0 {
		sort.Strings(mentions)
		if len(mentions) > 20 {
			mentions = append(mentions[:20], "...")
		}
		warn("Folgende Objekte nennen die Quelldatenbank %s ausdrücklich im Text (textuell erkannt) und verweisen auch in der Kopie dorthin: %s.", src, strings.Join(mentions, ", "))
	}

	// Drop phase for a rename.
	if rename {
		if len(st.tables) > 0 || len(base) > 0 {
			var lock []string
			for _, name := range namesOf(base) {
				lock = append(lock, q.id(src, name)+" WRITE")
			}
			for _, name := range st.tables {
				lock = append(lock, q.id(dst, name)+" READ")
			}
			var drop []string
			for _, name := range namesOf(base) {
				drop = append(drop, q.id(src, name))
			}
			st.steps = append(st.steps, dcStep{phase: "drop_source", kind: "tables", name: src, stmts: []string{"LOCK TABLES " + strings.Join(lock, ", "), "DROP TABLE " + strings.Join(drop, ", "), "UNLOCK TABLES"}})
		}
		st.steps = append(st.steps, dcStep{phase: "drop_source", kind: "database", name: src, stmts: []string{"DROP DATABASE " + q.id(src)}})
		for _, name := range st.enableEvt {
			st.steps = append(st.steps, dcStep{phase: "enable_events", kind: "event", name: name, stmts: []string{"ALTER EVENT " + q.id(dst, name) + " ENABLE"}, tolerate: true})
		}
	}
	if q.err != nil {
		return nil, invalidCopy("Ein Objektname ist nicht verwendbar.")
	}
	st.srcObjects["unsupported"] = nil

	// Estimates, dependencies, warnings.
	st.view.Estimate = core.DBCopyEstimate{Tables: len(st.tables), Views: len(views), Routines: len(routines), Triggers: len(triggers), Events: len(events)}
	if dataSeen {
		st.view.Estimate.Rows, st.view.Estimate.DataBytes, st.view.Estimate.IndexBytes = &estRows, &estData, &estIndex
	}
	deps, truncated, err := myExternalRefs(ctx, sess, src, rename)
	if err != nil {
		return nil, err
	}
	st.view.Dependencies, st.view.DependenciesTruncated = deps, truncated
	for _, d := range deps {
		if d.Effect == "blocks" {
			block("Der Fremdschlüssel %s.%s.%s verweist auf Tabellen der Quelle; DROP würde scheitern. Fremdschlüssel zuerst entfernen.", d.Schema, d.Name, valueOr(d.Constraint))
		}
	}
	warn("Rechte (Benutzer-, Datenbank-, Tabellen- und Spaltenrechte) werden nicht kopiert; sie müssen für die neue Datenbank bei Bedarf neu vergeben werden.")
	warn("Die Kopie ist nicht atomar: MySQL und MariaDB führen DDL mit implizitem Commit aus. Bei einem Fehler bleibt ein Teilzustand, der je Phase und Objekt gemeldet wird.")
	if withData {
		warn("Daten werden je Tabelle mit einer Anweisung (INSERT ... SELECT) kopiert, nicht aus einem gemeinsamen Snapshot. Die Anweisung kann Zeilen der Quelle sperren; gleichzeitige Schreibzugriffe warten oder machen die Verifikation hinfällig.")
		warn("Fremdschlüssel werden nach den Daten angelegt und prüfen sie dabei; die Reihenfolge der Tabellen folgt den Fremdschlüsseln (Eltern zuerst).")
	} else {
		warn("Nur die Struktur wird kopiert: Tabellen sind leer, AUTO_INCREMENT-Zähler beginnen neu.")
	}
	warn("Definer von Sichten, Routinen, Triggern und Ereignissen bleiben unverändert; fehlt dafür das Recht, scheitert nur dieses Objekt.")
	if incEvents && len(events) > 0 {
		if rename {
			warn("Ereignisse werden deaktiviert angelegt und erst nach dem Löschen der Quelle aktiviert, damit sie nicht doppelt laufen. Zwischen Anlegen und Deaktivieren liegt ein kurzes Zeitfenster.")
		} else {
			warn("Ereignisse der Kopie werden deaktiviert angelegt und müssen bei Bedarf selbst aktiviert werden. Zwischen Anlegen und Deaktivieren liegt ein kurzes Zeitfenster.")
		}
	}
	if rename {
		warn("Umbenennen kopiert alles, verifiziert Zeilenzahlen und Prüfsummen unter einer Schreibsperre und löscht die Quelle erst danach; getippter Quellname und Plan-Hash sind erforderlich. Rechte am alten Namen bleiben im Server bestehen. Anwendungen, die den alten Namen verwenden, brechen. Ein Rest-Risiko besteht für Objekte, die zwischen der letzten Prüfung und DROP DATABASE neu angelegt werden.")
		if isCurrent {
			warn("Die Quelle ist die Datenbank der aktuellen Verbindung; sie erfordert eine zusätzliche ausdrückliche Bestätigung.")
		}
	}
	return st, nil
}

func valueOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func namesOf(tables []myTable) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.name
	}
	return out
}

func myColumns(ctx context.Context, q metadataQuerier, schema string) ([]myColumn, error) {
	var out []myColumn
	err := dcRows(ctx, q, "SELECT TABLE_NAME, COLUMN_NAME, EXTRA, GENERATION_EXPRESSION FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME, ORDINAL_POSITION LIMIT ?", []any{schema, dcMaxColumns + 1}, func(r *sql.Rows) error {
		var c myColumn
		var extra, gen sql.NullString
		if err := r.Scan(&c.table, &c.name, &extra, &gen); err != nil {
			return err
		}
		e := strings.ToLower(extra.String)
		c.generated = strings.Contains(e, "generated") || strings.TrimSpace(gen.String) != ""
		c.autoInc = strings.Contains(e, "auto_increment")
		out = append(out, c)
		return nil
	})
	if err == nil && len(out) > dcMaxColumns {
		return nil, dcLimit("Die Datenbank hat zu viele Spalten.")
	}
	return out, err
}

func myForeignKeys(ctx context.Context, q metadataQuerier, schema string) ([]myFK, error) {
	index := map[string]int{}
	var out []myFK
	err := dcRows(ctx, q, "SELECT k.CONSTRAINT_NAME, k.TABLE_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME, r.UPDATE_RULE, r.DELETE_RULE "+
		"FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME AND r.TABLE_NAME = k.TABLE_NAME "+
		"WHERE k.TABLE_SCHEMA = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL ORDER BY k.TABLE_NAME, k.CONSTRAINT_NAME, k.ORDINAL_POSITION LIMIT ?", []any{schema, 100000}, func(r *sql.Rows) error {
		var name, table, col, refSchema, refTable, refCol, update, del string
		if err := r.Scan(&name, &table, &col, &refSchema, &refTable, &refCol, &update, &del); err != nil {
			return err
		}
		if !dcRuleOK[update] || !dcRuleOK[del] {
			return invalidCopy("Eine Fremdschlüsselregel ist unbekannt.")
		}
		key := table + "\x00" + name
		i, ok := index[key]
		if !ok {
			i = len(out)
			index[key] = i
			out = append(out, myFK{name: name, table: table, refSchema: refSchema, refTable: refTable, update: update, del: del})
		}
		out[i].cols = append(out[i].cols, col)
		out[i].refCols = append(out[i].refCols, refCol)
		return nil
	})
	return out, err
}

type myRoutine struct{ name, typ string }

func myRoutines(ctx context.Context, q metadataQuerier, schema string) ([]myRoutine, error) {
	var out []myRoutine
	err := dcRows(ctx, q, "SELECT ROUTINE_NAME, ROUTINE_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? ORDER BY ROUTINE_TYPE, ROUTINE_NAME LIMIT ?", []any{schema, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var x myRoutine
		if err := r.Scan(&x.name, &x.typ); err != nil {
			return err
		}
		if x.typ != "PROCEDURE" && x.typ != "FUNCTION" {
			return nil
		}
		out = append(out, x)
		return nil
	})
	return out, err
}

func myNamed(ctx context.Context, q metadataQuerier, query, schema string) ([]string, error) {
	var out []string
	err := dcRows(ctx, q, query, []any{schema, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var n string
		if err := r.Scan(&n); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	return out, err
}

type myEvent struct{ name, status string }

func myEvents(ctx context.Context, q metadataQuerier, schema string) ([]myEvent, error) {
	var out []myEvent
	err := dcRows(ctx, q, "SELECT EVENT_NAME, STATUS FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ? ORDER BY EVENT_NAME LIMIT ?", []any{schema, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var e myEvent
		if err := r.Scan(&e.name, &e.status); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	return out, err
}

// myTargetSets lists what already exists in the target schema (resume).
func myTargetSets(ctx context.Context, q metadataQuerier, schema string) (map[string]map[string]bool, error) {
	sets := map[string]map[string]bool{"view": {}, "procedure": {}, "function": {}, "trigger": {}, "event": {}, "fk": {}}
	add := func(kind, name string) { sets[kind][name] = true }
	if err := dcRows(ctx, q, "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'VIEW' LIMIT 100000", []any{schema}, func(r *sql.Rows) error {
		var n string
		if err := r.Scan(&n); err != nil {
			return err
		}
		add("view", n)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := dcRows(ctx, q, "SELECT ROUTINE_NAME, ROUTINE_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? LIMIT 100000", []any{schema}, func(r *sql.Rows) error {
		var n, t string
		if err := r.Scan(&n, &t); err != nil {
			return err
		}
		add(strings.ToLower(t), n)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := dcRows(ctx, q, "SELECT TRIGGER_NAME FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = ? LIMIT 100000", []any{schema}, func(r *sql.Rows) error {
		var n string
		if err := r.Scan(&n); err != nil {
			return err
		}
		add("trigger", n)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := dcRows(ctx, q, "SELECT EVENT_NAME FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ? LIMIT 100000", []any{schema}, func(r *sql.Rows) error {
		var n string
		if err := r.Scan(&n); err != nil {
			return err
		}
		add("event", n)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := dcRows(ctx, q, "SELECT DISTINCT TABLE_NAME, CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = ? AND CONSTRAINT_TYPE = 'FOREIGN KEY' LIMIT 100000", []any{schema}, func(r *sql.Rows) error {
		var t, n string
		if err := r.Scan(&t, &n); err != nil {
			return err
		}
		add("fk", t+"\x00"+n)
		return nil
	}); err != nil {
		return nil, err
	}
	return sets, nil
}

// myViewMode is the global sql_mode without NO_BACKSLASH_ESCAPES: stored view
// text escapes quotes with backslashes and would be misread otherwise.
func myViewMode(ctx context.Context, q metadataQuerier) (string, error) {
	var mode sql.NullString
	if err := q.QueryRowContext(ctx, "SELECT @@GLOBAL.sql_mode").Scan(&mode); err != nil {
		return "", err
	}
	var kept []string
	for _, m := range strings.Split(mode.String, ",") {
		if m != "" && m != "NO_BACKSLASH_ESCAPES" {
			kept = append(kept, m)
		}
	}
	return strings.Join(kept, ","), nil
}

func myRoutineDef(ctx context.Context, q metadataQuerier, qq *dcQuoter, schema string, r myRoutine) (myDef, error) {
	row, err := myShow(ctx, q, "SHOW CREATE "+r.typ+" "+qq.id(schema, r.name))
	if err != nil {
		return myDef{}, dcDefErr(err)
	}
	key := "Create Procedure"
	kind := "procedure"
	if r.typ == "FUNCTION" {
		key, kind = "Create Function", "function"
	}
	d := myDef{kind: kind, name: r.name, routine: r.typ, text: row[key], mode: row["sql_mode"], collation: row["collation_connection"]}
	return d, dcDefSize(d)
}

func myViewDef(ctx context.Context, q metadataQuerier, qq *dcQuoter, schema, name string) (myDef, error) {
	row, err := myShow(ctx, q, "SHOW CREATE VIEW "+qq.id(schema, name))
	if err != nil {
		return myDef{}, dcDefErr(err)
	}
	d := myDef{kind: "view", name: name, text: row["Create View"], collation: row["collation_connection"]}
	return d, dcDefSize(d)
}

func myTriggerDef(ctx context.Context, q metadataQuerier, qq *dcQuoter, schema, name string) (myDef, error) {
	row, err := myShow(ctx, q, "SHOW CREATE TRIGGER "+qq.id(schema, name))
	if err != nil {
		return myDef{}, dcDefErr(err)
	}
	d := myDef{kind: "trigger", name: name, text: row["SQL Original Statement"], mode: row["sql_mode"], collation: row["collation_connection"]}
	return d, dcDefSize(d)
}

func myEventDef(ctx context.Context, q metadataQuerier, qq *dcQuoter, schema string, ev myEvent) (myDef, error) {
	row, err := myShow(ctx, q, "SHOW CREATE EVENT "+qq.id(schema, ev.name))
	if err != nil {
		return myDef{}, dcDefErr(err)
	}
	d := myDef{kind: "event", name: ev.name, text: row["Create Event"], mode: row["sql_mode"], collation: row["collation_connection"], tz: row["time_zone"], status: ev.status}
	return d, dcDefSize(d)
}

// dcDefErr keeps a missing privilege on a definition out of the failure path:
// the object is then reported as unreadable instead of failing the plan.
func dcDefErr(err error) error {
	if reason, ok := opReason(err); ok && (reason == "permission_denied" || reason == "object_not_found") {
		return nil
	}
	return err
}

func dcDefSize(d myDef) error {
	if len(d.text) > dcMaxDefBytes {
		return public("result_too_large", "Die Definition von "+d.name+" ist zu groß.")
	}
	return nil
}

// dcMentionsSchema reports a textual qualified reference to the schema.
func dcMentionsSchema(text, schema string) bool {
	if strings.Contains(text, "`"+strings.ReplaceAll(schema, "`", "``")+"`.") {
		return true
	}
	for _, r := range schema {
		if !(r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	needle := schema + "."
	for start := 0; ; {
		i := strings.Index(text[start:], needle)
		if i < 0 {
			return false
		}
		at := start + i
		if at == 0 || !isIdentByte(text[at-1]) {
			return true
		}
		start = at + 1
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b == '`' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// myTableOrder sorts tables so that referenced tables come first; cycles fall
// back to name order.
func myTableOrder(tables []myTable, fks []myFK) []string {
	names := namesOf(tables)
	in := map[string]bool{}
	for _, n := range names {
		in[n] = true
	}
	deps := map[string]map[string]bool{}
	for _, fk := range fks {
		if in[fk.table] && in[fk.refTable] && fk.table != fk.refTable {
			if deps[fk.table] == nil {
				deps[fk.table] = map[string]bool{}
			}
			deps[fk.table][fk.refTable] = true
		}
	}
	done := map[string]bool{}
	var out []string
	for len(out) < len(names) {
		progressed := false
		for _, n := range names {
			if done[n] {
				continue
			}
			ready := true
			for p := range deps[n] {
				if !done[p] {
					ready = false
				}
			}
			if ready {
				done[n] = true
				out = append(out, n)
				progressed = true
			}
		}
		if !progressed {
			for _, n := range names {
				if !done[n] {
					done[n] = true
					out = append(out, n)
				}
			}
		}
	}
	return out
}

// myResumeTable decides what to do with a table that already exists in the target.
func myResumeTable(ctx context.Context, q metadataQuerier, qq *dcQuoter, src, dst, name string, existing myTable) (action, why string) {
	if existing.typ != "BASE TABLE" {
		return "conflict", "im Ziel ist es keine Tabelle"
	}
	a, err := myShow(ctx, q, "SHOW CREATE TABLE "+qq.id(src, name))
	if err != nil {
		return "conflict", "Struktur der Quelle nicht lesbar"
	}
	b, err := myShow(ctx, q, "SHOW CREATE TABLE "+qq.id(dst, name))
	if err != nil {
		return "conflict", "Struktur des Ziels nicht lesbar"
	}
	if strings.Join(myNormalizeCreate(a["Create Table"]), "\n") != strings.Join(myNormalizeCreate(b["Create Table"]), "\n") {
		return "conflict", "Struktur weicht ab"
	}
	var srcRows, dstRows int64
	if err = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qq.id(src, name)).Scan(&srcRows); err != nil {
		return "conflict", "Zeilenzahl der Quelle nicht lesbar"
	}
	if err = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qq.id(dst, name)).Scan(&dstRows); err != nil {
		return "conflict", "Zeilenzahl des Ziels nicht lesbar"
	}
	switch {
	case dstRows == srcRows:
		return "skip_complete", fmt.Sprintf("Struktur gleich, %d Zeilen in Quelle und Ziel", srcRows)
	case dstRows == 0:
		return "fill", "Struktur gleich, Ziel leer"
	}
	return "conflict", fmt.Sprintf("Quelle %d Zeilen, Ziel %d Zeilen", srcRows, dstRows)
}

var (
	myAutoIncRE = regexp.MustCompile(` AUTO_INCREMENT=[0-9]+`)
)

// myNormalizeCreate returns the sorted lines of a CREATE TABLE text without the
// table name, the AUTO_INCREMENT counter and foreign key lines.
func myNormalizeCreate(text string) []string {
	text = myAutoIncRE.ReplaceAllString(text, "")
	var lines []string
	for i, line := range strings.Split(text, "\n") {
		if i == 0 {
			continue
		}
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ","))
		if strings.Contains(line, "FOREIGN KEY") {
			continue
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

// myExternalRefs finds objects of other databases that refer to the source.
func myExternalRefs(ctx context.Context, q metadataQuerier, schema string, rename bool) ([]core.ObjectDependency, bool, error) {
	var out []core.ObjectDependency
	truncated := false
	add := func(d core.ObjectDependency) {
		if len(out) >= core.DBCopyMaxDependencies {
			truncated = true
			return
		}
		out = append(out, d)
	}
	system := "('information_schema','mysql','performance_schema','sys')"
	err := dcRows(ctx, q, "SELECT DISTINCT k.TABLE_SCHEMA, k.TABLE_NAME, k.CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE k WHERE k.REFERENCED_TABLE_SCHEMA = ? AND k.TABLE_SCHEMA <> ? AND k.TABLE_SCHEMA NOT IN "+system+" LIMIT 200", []any{schema, schema}, func(r *sql.Rows) error {
		var s, t, c string
		if err := r.Scan(&s, &t, &c); err != nil {
			return err
		}
		effect := "info"
		if rename {
			effect = "blocks"
		}
		add(core.ObjectDependency{Kind: "foreign_key", Schema: s, Name: t, Constraint: &c, Effect: effect, Certainty: "exact"})
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	needle := "`" + strings.ReplaceAll(schema, "`", "``") + "`."
	scan := func(kind, query string) error {
		return dcRows(ctx, q, query, []any{needle, schema}, func(r *sql.Rows) error {
			var s, n string
			if err := r.Scan(&s, &n); err != nil {
				return err
			}
			effect := "info"
			if rename {
				effect = "breaks"
			}
			add(core.ObjectDependency{Kind: kind, Schema: s, Name: n, Effect: effect, Certainty: "textual"})
			return nil
		})
	}
	if err = scan("view", "SELECT TABLE_SCHEMA, TABLE_NAME FROM information_schema.VIEWS WHERE LOCATE(?, VIEW_DEFINITION) > 0 AND TABLE_SCHEMA <> ? AND TABLE_SCHEMA NOT IN "+system+" LIMIT 100"); err != nil {
		return nil, false, err
	}
	if err = scan("routine", "SELECT ROUTINE_SCHEMA, ROUTINE_NAME FROM information_schema.ROUTINES WHERE LOCATE(?, ROUTINE_DEFINITION) > 0 AND ROUTINE_SCHEMA <> ? AND ROUTINE_SCHEMA NOT IN "+system+" LIMIT 100"); err != nil {
		return nil, false, err
	}
	if err = scan("trigger", "SELECT TRIGGER_SCHEMA, TRIGGER_NAME FROM information_schema.TRIGGERS WHERE LOCATE(?, ACTION_STATEMENT) > 0 AND TRIGGER_SCHEMA <> ? AND TRIGGER_SCHEMA NOT IN "+system+" LIMIT 100"); err != nil {
		return nil, false, err
	}
	if err = scan("event", "SELECT EVENT_SCHEMA, EVENT_NAME FROM information_schema.EVENTS WHERE LOCATE(?, EVENT_DEFINITION) > 0 AND EVENT_SCHEMA <> ? AND EVENT_SCHEMA NOT IN "+system+" LIMIT 100"); err != nil {
		return nil, false, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Schema != b.Schema {
			return a.Schema < b.Schema
		}
		return a.Name < b.Name
	})
	return out, truncated, nil
}
