// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// PostgreSQL: a schema rename is one ALTER SCHEMA statement. A schema copy is
// built from the catalog and runs in one REPEATABLE READ transaction, so it is
// consistent and all-or-nothing. A database copy is CREATE DATABASE TEMPLATE,
// a database rename is ALTER DATABASE RENAME; both are single statements and
// fail while other sessions use the source.

type pgRel struct {
	oid          int64
	name, kind   string
	rows, bytes  int64
	partition    bool
	inheritance  bool
	reloptions   sql.NullString
	cols, insert []string
}

type pgSeq struct {
	oid                                  int64
	name, typ                            string
	start, inc, min, max, cache          int64
	cycle                                bool
	ownerTable, ownerCol                 string
	identity                             bool
}

type pgDef struct{ name, table, text string }

func pgNamespaceExists(ctx context.Context, q metadataQuerier, name string) (bool, error) {
	var found string
	err := q.QueryRowContext(ctx, "SELECT nspname FROM pg_namespace WHERE nspname = $1", name).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil && found == name, err
}

func buildPostgresCopy(ctx context.Context, env *opEnv, action, scope string, r core.DBCopyRequest) (*dcState, error) {
	if scope == "database" {
		return buildPostgresDatabaseCopy(ctx, env, action, r)
	}
	return buildPostgresSchemaCopy(ctx, env, action, r)
}

func buildPostgresDatabaseCopy(ctx context.Context, env *opEnv, action string, r core.DBCopyRequest) (*dcState, error) {
	sess := env.sess
	src, dst := *r.Source, *r.NewName
	rename := action == core.OpDatabaseRename
	if src == dst {
		return nil, invalidCopy("Quell- und Zielname sind gleich.")
	}
	if r.Content != nil && *r.Content != "structure_and_data" {
		return nil, invalidCopy("PostgreSQL kopiert eine Datenbank nur vollständig (Vorlage); nur Struktur ist für eine Datenbank nicht möglich.")
	}
	if r.Include != nil {
		return nil, invalidCopy("include gilt nicht für PostgreSQL-Datenbanken; sie werden vollständig kopiert.")
	}
	found, system, _, err := postgresDatabaseInfo(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, opNotFound()
	}
	if system || opSystemDatabase("postgres", src) {
		return nil, invalidCopy("Systemdatenbanken werden nicht kopiert oder umbenannt.")
	}
	exists, _, _, err := postgresDatabaseInfo(ctx, sess, dst)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, opExists()
	}
	var current string
	if err = sess.QueryRowContext(ctx, "SELECT current_database()").Scan(&current); err != nil {
		return nil, err
	}
	q := &dcQuoter{driver: "postgres"}
	st := &dcState{action: action, scope: "database", driver: "postgres", source: src, target: dst, withData: true, rename: rename,
		currentDB: false, oneTx: false, expect: map[string][]string{}, srcObjects: map[string][]string{}}
	st.view = core.DBCopyPlan{Action: action, Driver: "postgres", Scope: "database", Source: src, Target: dst, Content: "structure_and_data",
		AffectedObjects: []string{"database:" + dst}, Warnings: []string{}, Blockers: []string{}}
	block := func(format string, args ...any) { st.view.Blockers = append(st.view.Blockers, fmt.Sprintf(format, args...)) }
	warn := func(format string, args ...any) { st.view.Warnings = append(st.view.Warnings, fmt.Sprintf(format, args...)) }
	var statement string
	if rename {
		st.phases = []dcPhase{{"rename_database", "Datenbank umbenennen", false}}
		statement = "ALTER DATABASE " + q.id(src) + " RENAME TO " + q.id(dst)
		st.steps = []dcStep{{phase: "rename_database", kind: "database", name: src, stmts: []string{statement}}}
		st.view.AffectedObjects = append(st.view.AffectedObjects, "database:"+src)
	} else {
		st.phases = []dcPhase{{"create_database", "Datenbank aus Vorlage anlegen", false}}
		statement = "CREATE DATABASE " + q.id(dst) + " TEMPLATE " + q.id(src)
		st.steps = []dcStep{{phase: "create_database", kind: "database", name: dst, stmts: []string{statement}}}
	}
	if q.err != nil {
		return nil, invalidCopy("Ein Datenbankname ist nicht verwendbar.")
	}
	st.phases = append(st.phases, dcPhase{"verify", "Ergebnis prüfen", false})
	st.expect["database"] = []string{dst}
	// Size, sessions, privileges.
	var size sql.NullInt64
	_ = sess.QueryRowContext(ctx, "SELECT pg_database_size($1::name)", src).Scan(&size)
	st.view.Estimate = core.DBCopyEstimate{Exact: false}
	if size.Valid {
		v := size.Int64
		st.view.Estimate.DataBytes = &v
	}
	st.view.Objects = []core.DBCopyObject{{Kind: "database", Name: src, Action: "create", BytesEstimate: nullIntPtr(size)}}
	var others int64
	if err = sess.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", src).Scan(&others); err != nil {
		return nil, err
	}
	if current == src {
		block("Diese Verbindung liegt auf der Datenbank %s; PostgreSQL verlangt, dass die Quelle von keiner Sitzung benutzt wird. Mit einer anderen Datenbank (zum Beispiel postgres) verbinden.", src)
	}
	if others > 0 {
		block("Auf der Datenbank %s sind %d weitere Sitzungen verbunden (nur sichtbar, soweit die Rolle sie sehen darf); PostgreSQL verweigert Kopieren und Umbenennen, solange Sitzungen bestehen. Sitzungen beenden und neu planen.", src, others)
	}
	var create, super bool
	if err = sess.QueryRowContext(ctx, "SELECT rolcreatedb, rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&create, &super); err != nil {
		return nil, err
	}
	if !create && !super {
		block("Der Datenbankbenutzer hat weder das Recht CREATEDB noch Superuser-Rechte.")
	}
	if rename {
		var owned bool
		if err = sess.QueryRowContext(ctx, "SELECT pg_has_role(current_user, datdba, 'USAGE') FROM pg_database WHERE datname = $1", src).Scan(&owned); err == nil && !owned && !super {
			block("Nur der Eigentümer der Datenbank darf sie umbenennen.")
		}
		warn("ALTER DATABASE RENAME ist eine einzelne Anweisung: sie wirkt ganz oder gar nicht. PostgreSQL verweigert sie, solange Sitzungen mit der Datenbank verbunden sind; nicht jede Sitzung ist für diese Rolle sichtbar, die Anweisung scheitert dann mit Meldung und ändert nichts.")
		warn("Anwendungen, Rollen-/Datenbankeinstellungen (ALTER DATABASE ... SET), Replikation und Verbindungszeichenfolgen, die den alten Namen verwenden, müssen angepasst werden.")
		st.view.Estimate.TimeRisk = "low"
	} else {
		warn("CREATE DATABASE kopiert die gesamte Datenbank einschließlich Rechten (ACLs) und Eigentümern von der Vorlage; Rechte werden hier nicht gefiltert und die Kopie ist nicht transaktional (Anweisung ganz oder gar nicht).")
		warn("Die Quelle darf während der Kopie von keiner anderen Sitzung benutzt werden; es wird nicht gewartet, sondern abgelehnt. Geschätzte Größe: %s.", fmtBytes(size))
	}
	st.view.Dependencies = []core.ObjectDependency{}
	return st, nil
}

func nullIntPtr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func fmtBytes(v sql.NullInt64) string {
	if !v.Valid {
		return "unbekannt"
	}
	return strconv.FormatInt(v.Int64>>20, 10) + " MiB"
}

func buildPostgresSchemaCopy(ctx context.Context, env *opEnv, action string, r core.DBCopyRequest) (*dcState, error) {
	sess := env.sess
	src, dst := *r.Source, *r.NewName
	rename := action == core.OpDatabaseRename
	if src == dst {
		return nil, invalidCopy("Quell- und Zielname sind gleich.")
	}
	found, err := pgNamespaceExists(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, opNotFound()
	}
	exists, err := pgNamespaceExists(ctx, sess, dst)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, opExists()
	}
	var owned, create bool
	if err = sess.QueryRowContext(ctx, "SELECT pg_has_role(current_user, nspowner, 'USAGE') OR (SELECT rolsuper FROM pg_roles WHERE rolname = current_user), has_database_privilege(current_user, current_database(), 'CREATE') FROM pg_namespace WHERE nspname = $1", src).Scan(&owned, &create); err != nil {
		return nil, err
	}
	q := &dcQuoter{driver: "postgres"}
	st := &dcState{action: action, scope: "schema", driver: "postgres", source: src, target: dst, withData: true, rename: rename,
		expect: map[string][]string{}, srcObjects: map[string][]string{}}
	content := "structure_and_data"
	withData := true
	if !rename && r.Content != nil && *r.Content == "structure" {
		content, withData = "structure", false
	}
	st.withData = withData
	st.view = core.DBCopyPlan{Action: action, Driver: "postgres", Scope: "schema", Source: src, Target: dst, Content: content,
		AffectedObjects: []string{"schema:" + dst}, Warnings: []string{}, Blockers: []string{}}
	block := func(format string, args ...any) { st.view.Blockers = append(st.view.Blockers, fmt.Sprintf(format, args...)) }
	warn := func(format string, args ...any) { st.view.Warnings = append(st.view.Warnings, fmt.Sprintf(format, args...)) }
	if !create {
		block("Der Datenbankbenutzer hat kein CREATE-Recht auf der Datenbank.")
	}
	if rename {
		if !owned {
			block("Nur der Eigentümer des Schemas darf es umbenennen.")
		}
		st.phases = []dcPhase{{"rename_schema", "Schema umbenennen", false}, {"verify", "Ergebnis prüfen", false}}
		st.steps = []dcStep{{phase: "rename_schema", kind: "schema", name: src, stmts: []string{"ALTER SCHEMA " + q.id(src) + " RENAME TO " + q.id(dst)}}}
		st.expect["schema"] = []string{dst}
		st.view.AffectedObjects = append(st.view.AffectedObjects, "schema:"+src)
		st.view.Objects = []core.DBCopyObject{{Kind: "schema", Name: src, Action: "create", Note: "Umbenennung, keine Kopie"}}
		st.view.Estimate = core.DBCopyEstimate{TimeRisk: "low"}
		st.view.Dependencies = []core.ObjectDependency{}
		warn("ALTER SCHEMA RENAME ist eine einzelne, transaktionale Anweisung (ganz oder gar nicht). Objekte im Schema, Fremdschlüssel und Sichten (auch anderer Schemas) folgen dem neuen Namen, weil PostgreSQL sie über Kennungen verknüpft.")
		warn("Nicht nachgeführt werden Namen in Texten: Funktionsrümpfe, dynamisches SQL, Anwendungen, search_path-Einstellungen von Rollen und Datenbanken sowie Verbindungseinstellungen. Der Vorgang sperrt das Schema kurz exklusiv.")
		var settings int
		if err = sess.QueryRowContext(ctx, "SELECT count(*) FROM pg_db_role_setting s, unnest(s.setconfig) AS c WHERE c LIKE 'search_path=%' AND position($1 IN c) > 0", src).Scan(&settings); err == nil && settings > 0 {
			warn("%d Rollen- oder Datenbankeinstellungen (search_path) erwähnen den Schemanamen und werden nicht angepasst.", settings)
		}
		if q.err != nil {
			return nil, invalidCopy("Ein Schemaname ist nicht verwendbar.")
		}
		return st, nil
	}
	// Search path of the source for definition text without qualification noise.
	if _, err = sess.exec(ctx, "SET search_path TO "+q.id(src)); err != nil {
		return nil, err
	}
	defer sess.exec(context.WithoutCancel(ctx), "RESET search_path")

	rels, err := pgRelations(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	seqs, err := pgSequences(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	var tables, views []pgRel
	var skipped []pgRel
	tableNames := map[string]bool{}
	for _, rel := range rels {
		if rel.kind == "r" && !rel.partition && !rel.inheritance {
			tableNames[rel.name] = true
		}
	}
	for i := range seqs {
		if seqs[i].ownerTable != "" && !tableNames[seqs[i].ownerTable] {
			seqs[i].ownerTable, seqs[i].ownerCol, seqs[i].identity = "", "", false
		}
	}
	for _, rel := range rels {
		switch {
		case rel.kind == "r" && !rel.partition && !rel.inheritance:
			tables = append(tables, rel)
		case rel.kind == "v":
			views = append(views, rel)
		case rel.kind == "S":
		default:
			skipped = append(skipped, rel)
		}
	}
	if len(rels) > core.DBCopyMaxObjects {
		return nil, dcLimit(fmt.Sprintf("Das Schema hat zu viele Objekte (Grenze: %d).", core.DBCopyMaxObjects))
	}
	st.srcObjects["table"] = pgNames(tables)
	// Unsupported kinds and user defined types.
	for _, rel := range skipped {
		why := map[string]string{"m": "materialisierte Sicht", "p": "partitionierte Tabelle", "f": "Fremdtabelle", "r": "Tabelle mit Vererbung oder Partition"}[rel.kind]
		if why == "" {
			why = "Relationsart " + rel.kind
		}
		st.skipped++
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "table", Name: rel.name, Action: "unsupported", Note: why + " wird nicht kopiert."})
		warn("%s %s wird nicht kopiert.", strings.ToUpper(why[:1])+why[1:], rel.name)
	}
	var types int
	if err = sess.QueryRowContext(ctx, "SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace WHERE n.nspname = $1 AND t.typtype IN ('e','d','r','m') OR n.nspname = $1 AND t.typtype = 'c' AND EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = t.typrelid AND c.relkind = 'c')", src).Scan(&types); err != nil {
		return nil, err
	}
	if types > 0 {
		block("Das Schema enthält %d benutzerdefinierte Typen (Enum, Domäne, Bereich, zusammengesetzt). Die Kopie würde auf die Typen der Quelle verweisen; das wird nicht unterstützt.", types)
	}
	if len(tables) > core.DBCopyMaxTables {
		return nil, dcLimit(fmt.Sprintf("Das Schema hat zu viele Tabellen (Grenze: %d).", core.DBCopyMaxTables))
	}
	if err = pgColumns(ctx, sess, src, tables); err != nil {
		return nil, err
	}
	deps, err := pgSequenceUses(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	fks, err := pgNamedDefs(ctx, sess, "SELECT con.conname, rel.relname, pg_get_constraintdef(con.oid, true) FROM pg_constraint con JOIN pg_class rel ON rel.oid = con.conrelid JOIN pg_namespace n ON n.oid = rel.relnamespace WHERE n.nspname = $1 AND con.contype = 'f' ORDER BY rel.relname, con.conname LIMIT 100000", src)
	if err != nil {
		return nil, err
	}
	funcs, err := pgFunctions(ctx, sess, q, src, dst)
	if err != nil {
		return nil, err
	}
	trigs, err := pgTriggers(ctx, sess, q, src)
	if err != nil {
		return nil, err
	}
	viewDefs, err := pgViews(ctx, sess, src, views)
	if err != nil {
		return nil, err
	}
	var rules, policies int
	_ = sess.QueryRowContext(ctx, "SELECT count(*) FROM pg_rewrite r JOIN pg_class c ON c.oid = r.ev_class JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relkind = 'r'", src).Scan(&rules)
	_ = sess.QueryRowContext(ctx, "SELECT count(*) FROM pg_policy p JOIN pg_class c ON c.oid = p.polrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1", src).Scan(&policies)
	if rules > 0 {
		warn("%d Regeln (RULE) auf Tabellen werden nicht kopiert.", rules)
	}
	if policies > 0 {
		warn("%d Sicherheitsrichtlinien (Row Level Security) werden nicht kopiert.", policies)
	}

	// Plan statements.
	st.phases = []dcPhase{{"create_schema", "Schema anlegen", false}, {"sequences", "Sequenzen anlegen", false}, {"tables", "Tabellen anlegen", false}, {"functions", "Funktionen anlegen", false}}
	if withData {
		st.phases = append(st.phases, dcPhase{"data", "Daten kopieren und Zähler setzen", false})
	}
	st.phases = append(st.phases, dcPhase{"foreign_keys", "Fremdschlüssel anlegen", false}, dcPhase{"views", "Sichten anlegen", false}, dcPhase{"triggers", "Trigger anlegen", false}, dcPhase{"verify", "Kopie verifizieren", false})
	st.steps = append(st.steps, dcStep{phase: "create_schema", kind: "schema", name: dst, stmts: []string{"CREATE SCHEMA " + q.id(dst), "SET LOCAL search_path TO " + q.id(dst)}})
	seqByOid := map[int64]pgSeq{}
	for _, s := range seqs {
		seqByOid[s.oid] = s
	}
	for _, s := range seqs {
		if s.identity {
			continue // created with the table (LIKE ... INCLUDING IDENTITY)
		}
		stmt := fmt.Sprintf("CREATE SEQUENCE %s AS %s INCREMENT BY %d MINVALUE %d MAXVALUE %d START WITH %d CACHE %d %s", q.id(dst, s.name), s.typ, s.inc, s.min, s.max, s.start, s.cache, map[bool]string{true: "CYCLE", false: "NO CYCLE"}[s.cycle])
		st.steps = append(st.steps, dcStep{phase: "sequences", kind: "sequence", name: s.name, stmts: []string{stmt}})
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "sequence", Name: s.name, Action: "create"})
		st.expect["sequence"] = append(st.expect["sequence"], s.name)
	}
	var estRows, estBytes int64
	for _, t := range tables {
		stmts := []string{"CREATE TABLE " + q.id(dst, t.name) + " (LIKE " + q.id(src, t.name) + " INCLUDING ALL)"}
		for _, d := range deps {
			if d.table != t.name {
				continue
			}
			seq, ok := seqByOid[d.seqOid]
			if !ok || seq.identity {
				continue
			}
			lit, err := opLiteral(q.id(dst, seq.name))
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, "ALTER TABLE "+q.id(dst, t.name)+" ALTER COLUMN "+q.id(d.column)+" SET DEFAULT nextval("+lit+"::regclass)")
		}
		for _, s := range seqs {
			if s.ownerTable == t.name && !s.identity {
				stmts = append(stmts, "ALTER SEQUENCE "+q.id(dst, s.name)+" OWNED BY "+q.id(dst, t.name)+"."+q.id(s.ownerCol))
			}
		}
		st.steps = append(st.steps, dcStep{phase: "tables", kind: "table", name: t.name, stmts: stmts, rows: t.rows})
		rows, bytes := t.rows, t.bytes
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "table", Name: t.name, Action: "create", RowsEstimate: &rows, BytesEstimate: &bytes})
		estRows += max64(t.rows, 0)
		estBytes += t.bytes
		st.tables = append(st.tables, t.name)
	}
	st.expect["table"] = append([]string{}, st.tables...)
	for _, f := range funcs {
		if f.text == "" {
			st.skipped++
			warn("Die Funktion %s kann nicht kopiert werden (Definition nicht umschreibbar, Aggregat oder Erweiterung).", f.name)
			st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "function", Name: f.name, Action: "unsupported"})
			continue
		}
		st.steps = append(st.steps, dcStep{phase: "functions", kind: "function", name: f.name, stmts: []string{f.text}})
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "function", Name: f.name, Action: "create"})
		st.expect["function"] = append(st.expect["function"], f.name)
	}
	if withData {
		for _, t := range tables {
			if len(t.insert) == 0 {
				continue
			}
			list := q.list(t.insert)
			stmt := "INSERT INTO " + q.id(dst, t.name) + " (" + list + ") OVERRIDING SYSTEM VALUE SELECT " + list + " FROM " + q.id(src, t.name)
			st.steps = append(st.steps, dcStep{phase: "data", kind: "table", name: t.name, stmts: []string{stmt}, rows: t.rows})
		}
		for _, s := range seqs {
			var stmt string
			if s.identity {
				lt, _ := opLiteral(q.id(dst, s.ownerTable))
				lc, _ := opLiteral(s.ownerCol)
				stmt = "SELECT setval(pg_get_serial_sequence(" + lt + ", " + lc + "), last_value, is_called) FROM " + q.id(src, s.name)
			} else {
				lit, _ := opLiteral(q.id(dst, s.name))
				stmt = "SELECT setval(" + lit + "::regclass, last_value, is_called) FROM " + q.id(src, s.name)
			}
			st.steps = append(st.steps, dcStep{phase: "data", kind: "sequence", name: s.name, stmts: []string{stmt}})
		}
	}
	for _, fk := range fks {
		st.steps = append(st.steps, dcStep{phase: "foreign_keys", kind: "foreign_key", name: fk.table + "." + fk.name, stmts: []string{"ALTER TABLE " + q.id(dst, fk.table) + " ADD CONSTRAINT " + q.id(fk.name) + " " + fk.text}})
		st.expect["fk"] = append(st.expect["fk"], fk.table+"\x00"+fk.name)
	}
	for _, v := range viewDefs {
		st.steps = append(st.steps, dcStep{phase: "views", kind: "view", name: v.name, stmts: []string{"CREATE VIEW " + q.id(dst, v.name) + " AS " + v.text}})
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "view", Name: v.name, Action: "create"})
		st.expect["view"] = append(st.expect["view"], v.name)
		if v.table != "" {
			warn("Die Sicht %s hat Optionen (%s), die nicht kopiert werden.", v.name, v.table)
		}
	}
	for _, t := range trigs {
		if t.text == "" {
			st.skipped++
			warn("Der Trigger %s auf %s kann nicht kopiert werden (Definition nicht umschreibbar).", t.name, t.table)
			st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "trigger", Name: t.name, Action: "unsupported"})
			continue
		}
		st.steps = append(st.steps, dcStep{phase: "triggers", kind: "trigger", name: t.table + "." + t.name, stmts: []string{t.text}})
		st.view.Objects = append(st.view.Objects, core.DBCopyObject{Kind: "trigger", Name: t.name, Action: "create"})
		st.expect["trigger"] = append(st.expect["trigger"], t.table+"\x00"+t.name)
	}
	if q.err != nil {
		return nil, invalidCopy("Ein Objektname ist nicht verwendbar.")
	}
	st.oneTx = true
	st.view.Estimate = core.DBCopyEstimate{Tables: len(tables), Views: len(views), Triggers: len(trigs), Routines: len(funcs), Exact: false}
	st.view.Estimate.Rows, st.view.Estimate.DataBytes = &estRows, &estBytes
	ext, extTrunc, err := pgExternalRefs(ctx, sess, src)
	if err != nil {
		return nil, err
	}
	st.view.Dependencies, st.view.DependenciesTruncated = ext, extTrunc
	warn("Rechte (ACLs), Eigentümer, Kommentare an Sichten und Funktionen, Statistikobjekte und Berechtigungen auf Sequenzen werden nicht kopiert; Tabellen übernehmen Indizes, Standardwerte, CHECK-Bedingungen, Identitäten und Tabellenkommentare.")
	warn("Die Kopie läuft in einer einzigen Transaktion mit konsistentem Schnappschuss (REPEATABLE READ): ganz oder gar nicht, bei Fehler oder Abbruch bleibt nichts zurück. Große Schemas können lange Sperren auf neuen Objekten und viel Speicher für das WAL benötigen.")
	warn("Sequenzzähler werden zum Zeitpunkt der Kopie übernommen; die Quelle behält ihre eigenen Zähler. Fremdschlüssel, Trigger und Sichten werden nach den Daten angelegt.")
	if withData {
		warn("Daten werden pro Tabelle mit INSERT ... SELECT kopiert und anschließend über Zeilenzahl und Prüfsumme gegen die Quelle verglichen.")
	}
	return st, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func pgNames(rels []pgRel) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = r.name
	}
	return out
}

func pgRelations(ctx context.Context, q metadataQuerier, schema string) ([]pgRel, error) {
	var out []pgRel
	err := dcRows(ctx, q, `SELECT c.oid::bigint, c.relname, c.relkind::text, c.reltuples::bigint, pg_total_relation_size(c.oid), c.relispartition,
		EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid = c.oid OR i.inhparent = c.oid), array_to_string(c.reloptions, ',')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind IN ('r','p','v','m','f','S') AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
		ORDER BY c.relname LIMIT $2`, []any{schema, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var x pgRel
		if err := r.Scan(&x.oid, &x.name, &x.kind, &x.rows, &x.bytes, &x.partition, &x.inheritance, &x.reloptions); err != nil {
			return err
		}
		out = append(out, x)
		return nil
	})
	return out, err
}

func pgColumns(ctx context.Context, q metadataQuerier, schema string, tables []pgRel) error {
	index := map[string]*pgRel{}
	for i := range tables {
		index[tables[i].name] = &tables[i]
	}
	return dcRows(ctx, q, `SELECT c.relname, a.attname, a.attgenerated::text FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'r' AND a.attnum > 0 AND NOT a.attisdropped ORDER BY c.relname, a.attnum LIMIT $2`, []any{schema, dcMaxColumns + 1}, func(r *sql.Rows) error {
		var table, name, generated string
		if err := r.Scan(&table, &name, &generated); err != nil {
			return err
		}
		if t := index[table]; t != nil {
			t.cols = append(t.cols, name)
			if generated == "" {
				t.insert = append(t.insert, name)
			}
		}
		return nil
	})
}

func pgSequences(ctx context.Context, q metadataQuerier, schema string) ([]pgSeq, error) {
	var out []pgSeq
	err := dcRows(ctx, q, `SELECT c.oid::bigint, c.relname, format_type(s.seqtypid, NULL), s.seqstart, s.seqincrement, s.seqmin, s.seqmax, s.seqcache, s.seqcycle,
		COALESCE(t.relname, ''), COALESCE(a.attname, ''), COALESCE(d.deptype = 'i', false)
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace JOIN pg_sequence s ON s.seqrelid = c.oid
		LEFT JOIN pg_depend d ON d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.refclassid = 'pg_class'::regclass AND d.deptype IN ('a','i')
		LEFT JOIN pg_class t ON t.oid = d.refobjid LEFT JOIN pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
		WHERE n.nspname = $1 AND c.relkind = 'S' ORDER BY c.relname LIMIT $2`, []any{schema, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var s pgSeq
		if err := r.Scan(&s.oid, &s.name, &s.typ, &s.start, &s.inc, &s.min, &s.max, &s.cache, &s.cycle, &s.ownerTable, &s.ownerCol, &s.identity); err != nil {
			return err
		}
		switch s.typ {
		case "smallint", "integer", "bigint":
		default:
			return invalidCopy("Eine Sequenz hat einen unbekannten Datentyp.")
		}
		out = append(out, s)
		return nil
	})
	return out, err
}

type pgSeqUse struct {
	table, column string
	seqOid        int64
}

// pgSequenceUses lists plain "nextval('seq'::regclass)" defaults that point at
// a sequence of the same schema, so that the copy gets a default on its own sequence.
func pgSequenceUses(ctx context.Context, q metadataQuerier, schema string) ([]pgSeqUse, error) {
	var out []pgSeqUse
	err := dcRows(ctx, q, `SELECT c.relname, a.attname, seq.oid::bigint, pg_get_expr(ad.adbin, ad.adrelid)
		FROM pg_attrdef ad JOIN pg_class c ON c.oid = ad.adrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = ad.adrelid AND a.attnum = ad.adnum
		JOIN pg_depend d ON d.classid = 'pg_attrdef'::regclass AND d.objid = ad.oid AND d.refclassid = 'pg_class'::regclass AND d.deptype = 'n'
		JOIN pg_class seq ON seq.oid = d.refobjid AND seq.relkind = 'S' JOIN pg_namespace sn ON sn.oid = seq.relnamespace
		WHERE n.nspname = $1 AND sn.nspname = $1 AND c.relkind = 'r' ORDER BY c.relname, a.attnum LIMIT 100000`, []any{schema}, func(r *sql.Rows) error {
		var u pgSeqUse
		var expr string
		if err := r.Scan(&u.table, &u.column, &u.seqOid, &expr); err != nil {
			return err
		}
		if strings.HasPrefix(expr, "nextval('") && strings.HasSuffix(expr, "'::regclass)") && strings.Count(expr, "nextval(") == 1 {
			out = append(out, u)
		}
		return nil
	})
	return out, err
}

func pgNamedDefs(ctx context.Context, q metadataQuerier, query, schema string) ([]pgDef, error) {
	var out []pgDef
	err := dcRows(ctx, q, query, []any{schema}, func(r *sql.Rows) error {
		var d pgDef
		if err := r.Scan(&d.name, &d.table, &d.text); err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

// pgFunctions returns the function definitions with the header rewritten to
// the target schema. A definition whose header cannot be recognised exactly is
// returned with empty text (reported as unsupported).
func pgFunctions(ctx context.Context, sess metadataQuerier, q *dcQuoter, src, dst string) ([]pgDef, error) {
	var out []pgDef
	err := dcRows(ctx, sess, `SELECT p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')', quote_ident(n.nspname) || '.' || quote_ident(p.proname), quote_ident(p.proname), p.prokind::text, CASE WHEN p.prokind IN ('f','p') THEN pg_get_functiondef(p.oid) ELSE '' END
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = $1 AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e')
		ORDER BY p.proname, p.oid LIMIT $2`, []any{src, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var label, qualified, quoted, kind, def string
		if err := r.Scan(&label, &qualified, &quoted, &kind, &def); err != nil {
			return err
		}
		d := pgDef{name: label}
		if def != "" {
			word := "FUNCTION"
			if kind == "p" {
				word = "PROCEDURE"
			}
			prefix := "CREATE OR REPLACE " + word + " " + qualified + "("
			if strings.HasPrefix(def, prefix) {
				d.text = "CREATE " + word + " " + q.id(dst) + "." + quoted + "(" + strings.TrimPrefix(def, prefix)
			}
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

// pgTriggers returns trigger definitions whose target table is rewritten.
func pgTriggers(ctx context.Context, sess metadataQuerier, q *dcQuoter, src string) ([]pgDef, error) {
	var out []pgDef
	err := dcRows(ctx, sess, `SELECT t.tgname, c.relname, quote_ident(n.nspname) || '.' || quote_ident(c.relname), pg_get_triggerdef(t.oid, false)
		FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND NOT t.tgisinternal AND c.relkind = 'r' ORDER BY c.relname, t.tgname LIMIT $2`, []any{src, core.DBCopyMaxObjects + 1}, func(r *sql.Rows) error {
		var name, table, qualified, def string
		if err := r.Scan(&name, &table, &qualified, &def); err != nil {
			return err
		}
		d := pgDef{name: name, table: table}
		// With the source in the search_path the table is printed unqualified; both
		// spellings are accepted. Function names resolve through the search_path of the target.
		quotedTable := q.id(table)
		switch {
		case strings.Count(def, " ON "+qualified+" ") == 1:
			d.text = strings.Replace(def, " ON "+qualified+" ", " ON "+quotedTable+" ", 1)
		case strings.Count(def, " ON "+quotedTable+" ") == 1:
			d.text = def
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

// pgViews returns view bodies in dependency order. The body is produced with
// the source in the search path, so references to its objects are unqualified
// and resolve to the copies when run with the target as search path.
func pgViews(ctx context.Context, sess metadataQuerier, src string, views []pgRel) ([]pgDef, error) {
	if len(views) == 0 {
		return nil, nil
	}
	bodies := map[string]pgDef{}
	for _, v := range views {
		var text string
		if err := sess.QueryRowContext(ctx, "SELECT pg_get_viewdef($1::oid, true)", v.oid).Scan(&text); err != nil {
			return nil, err
		}
		d := pgDef{name: v.name, text: strings.TrimSuffix(strings.TrimSpace(text), ";")}
		if v.reloptions.Valid && v.reloptions.String != "" {
			d.table = v.reloptions.String
		}
		bodies[v.name] = d
	}
	// Dependencies between views of the schema.
	deps := map[string]map[string]bool{}
	err := dcRows(ctx, sess, `SELECT DISTINCT v.relname, ref.relname FROM pg_rewrite rw JOIN pg_class v ON v.oid = rw.ev_class JOIN pg_namespace n ON n.oid = v.relnamespace
		JOIN pg_depend d ON d.classid = 'pg_rewrite'::regclass AND d.objid = rw.oid AND d.refclassid = 'pg_class'::regclass
		JOIN pg_class ref ON ref.oid = d.refobjid AND ref.relkind = 'v' AND ref.oid <> v.oid JOIN pg_namespace rn ON rn.oid = ref.relnamespace
		WHERE n.nspname = $1 AND rn.nspname = $1 AND v.relkind = 'v'`, []any{src}, func(r *sql.Rows) error {
		var a, b string
		if err := r.Scan(&a, &b); err != nil {
			return err
		}
		if deps[a] == nil {
			deps[a] = map[string]bool{}
		}
		deps[a][b] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bodies))
	for n := range bodies {
		names = append(names, n)
	}
	sort.Strings(names)
	done := map[string]bool{}
	var out []pgDef
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
				out = append(out, bodies[n])
				progressed = true
			}
		}
		if !progressed {
			return nil, invalidCopy("Die Sichten des Schemas haben zirkuläre Abhängigkeiten.")
		}
	}
	return out, nil
}

func pgExternalRefs(ctx context.Context, sess metadataQuerier, src string) ([]core.ObjectDependency, bool, error) {
	var out []core.ObjectDependency
	truncated := false
	err := dcRows(ctx, sess, `SELECT DISTINCT cn.nspname, c.relname, con.conname FROM pg_constraint con JOIN pg_class c ON c.oid = con.conrelid JOIN pg_namespace cn ON cn.oid = c.relnamespace
		JOIN pg_class ref ON ref.oid = con.confrelid JOIN pg_namespace rn ON rn.oid = ref.relnamespace WHERE con.contype = 'f' AND rn.nspname = $1 AND cn.nspname <> $1 LIMIT 101`, []any{src}, func(r *sql.Rows) error {
		var s, t, c string
		if err := r.Scan(&s, &t, &c); err != nil {
			return err
		}
		if len(out) >= core.DBCopyMaxDependencies {
			truncated = true
			return nil
		}
		out = append(out, core.ObjectDependency{Kind: "foreign_key", Schema: s, Name: t, Constraint: &c, Effect: "info", Certainty: "exact"})
		return nil
	})
	return out, truncated, err
}
