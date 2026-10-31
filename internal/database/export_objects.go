// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// DUMP-001: optional views, routines, triggers and events in the SQL dump.
//
// The object lists are read once before the first byte of a database is written
// (names only, bounded). Definitions are read and written one object at a time, so
// memory is bounded by one definition (core.ExportMaxObjectBytes), not by the
// number of objects. Any object that cannot be read aborts the dump: the stream ends
// without the completion marker and the trailer reports incomplete.

type exportObject struct {
	class       string // views, routines, triggers, events
	kind        string // view, matview, function, procedure, trigger, event
	schema      string // PostgreSQL schema
	name        string
	identity    string // PostgreSQL routine identity arguments
	table       string // trigger table
	tableSchema string
	oid         int64 // PostgreSQL object id (snapshot-local)
}

func objectUnreadable() error {
	return public("export_object_unreadable", "Ein gewähltes Datenbankobjekt ist nicht vollständig lesbar; Export ist unvollständig.")
}

// objectFailure keeps cancellation and public errors, hides driver details otherwise.
func objectFailure(ctx context.Context, err error) error {
	var p *core.Error
	if errors.As(err, &p) {
		return err
	}
	if ctx.Err() != nil {
		return safe(ctx, ctx.Err())
	}
	return objectUnreadable()
}

func validObjectName(s string) bool {
	return s != "" && len(s) <= 1024 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// listExportObjects returns the objects in creation (dependency) order:
// views, routines, triggers, events; PostgreSQL views and routines are ordered
// together by their catalog dependencies.
func listExportObjects(ctx context.Context, conn *sql.Conn, c core.Connection, r core.ExportRequest) ([]exportObject, error) {
	if len(r.Objects) == 0 {
		return nil, nil
	}
	var objects []exportObject
	var err error
	if c.Driver == "postgres" {
		// Identity arguments and type names render fully qualified.
		if _, err = conn.ExecContext(ctx, "SET LOCAL search_path=pg_catalog"); err != nil {
			return nil, objectFailure(ctx, err)
		}
	}
	switch c.Driver {
	case "sqlite":
		objects, err = sqliteObjectList(ctx, conn, r)
	case "mysql":
		objects, err = mysqlObjectList(ctx, conn, r)
	default:
		objects, err = postgresObjectList(ctx, conn, r)
	}
	if err != nil {
		return nil, objectFailure(ctx, err)
	}
	if len(objects) > core.ExportMaxObjects {
		return nil, public("export_object_limit", "Zu viele Datenbankobjekte für einen Export.")
	}
	for _, o := range objects {
		if !validObjectName(o.name) || !utf8.ValidString(o.schema) || !utf8.ValidString(o.table) || !utf8.ValidString(o.identity) {
			return nil, public("unsupported_export", "Objektname ist nicht als gültiges UTF-8 exportierbar.")
		}
	}
	return objects, nil
}

// collectNames reads a bounded list of rows.
func collectObjects(ctx context.Context, conn *sql.Conn, query string, scan func(*sql.Rows) (exportObject, error), objects *[]exportObject) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			return err
		}
		*objects = append(*objects, o)
		if len(*objects) > core.ExportMaxObjects {
			return public("export_object_limit", "Zu viele Datenbankobjekte für einen Export.")
		}
	}
	return rows.Err()
}

// ---------------------------------------------------------------- SQLite

func sqliteObjectList(ctx context.Context, conn *sql.Conn, r core.ExportRequest) ([]exportObject, error) {
	var types []string
	if r.HasExportObject("views") {
		types = append(types, "'view'")
	}
	if r.HasExportObject("triggers") {
		types = append(types, "'trigger'")
	}
	var objects []exportObject
	err := collectObjects(ctx, conn, "SELECT type,name,tbl_name FROM main.sqlite_schema WHERE type IN ("+strings.Join(types, ",")+") AND name NOT GLOB 'sqlite_*' ORDER BY type DESC,name", func(rows *sql.Rows) (exportObject, error) {
		var o exportObject
		var kind, table string
		if err := rows.Scan(&kind, &o.name, &table); err != nil {
			return o, err
		}
		o.kind = kind
		o.class = "views"
		if kind == "trigger" {
			o.class = "triggers"
			o.table = table
		}
		return o, nil
	}, &objects)
	return objects, err
}

func sqliteObjectDDL(ctx context.Context, conn *sql.Conn, o exportObject, ifNotExists bool) (string, error) {
	var def sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT sql FROM main.sqlite_schema WHERE type=? AND name=?", o.kind, o.name).Scan(&def); err != nil || !def.Valid || strings.TrimSpace(def.String) == "" {
		return "", objectUnreadable()
	}
	text := def.String
	if ifNotExists {
		prefix := "CREATE VIEW "
		if o.kind == "trigger" {
			prefix = "CREATE TRIGGER "
		}
		if len(text) < len(prefix) || !strings.EqualFold(text[:len(prefix)], prefix) {
			return "", public("unsupported_export", "Objektdefinition hat ein unerwartetes Format.")
		}
		text = text[:len(prefix)] + "IF NOT EXISTS " + text[len(prefix):]
	}
	return text, nil
}

// ---------------------------------------------------------------- MySQL / MariaDB

var mysqlSQLMode = regexp.MustCompile(`^[A-Za-z0-9_,]{0,1000}$`)
var mysqlSimpleName = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
var mysqlTimeZone = regexp.MustCompile(`^[A-Za-z0-9_+:/-]{1,64}$`)

// mysqlGrantee renders CURRENT_USER() in the GRANTEE format of information_schema.
const mysqlGrantee = `CONCAT("'",SUBSTRING_INDEX(CURRENT_USER(),'@',1),"'@'",SUBSTRING_INDEX(CURRENT_USER(),'@',-1),"'")`

// mysqlObjectPrivileges proves that the account may read every requested class
// completely. information_schema lists only objects the account may see, so a
// missing privilege would otherwise produce a silently incomplete dump.
func mysqlObjectPrivileges(ctx context.Context, conn *sql.Conn, r core.ExportRequest) error {
	global, schema := map[string]bool{}, map[string]bool{}
	for _, part := range []struct {
		target map[string]bool
		query  string
	}{
		{global, "SELECT PRIVILEGE_TYPE FROM information_schema.USER_PRIVILEGES WHERE GRANTEE=" + mysqlGrantee},
		{schema, "SELECT PRIVILEGE_TYPE FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE=" + mysqlGrantee + " AND DATABASE() LIKE TABLE_SCHEMA"},
	} {
		rows, err := conn.QueryContext(ctx, part.query)
		if err != nil {
			return err
		}
		for rows.Next() {
			var privilege string
			if err = rows.Scan(&privilege); err != nil {
				rows.Close()
				return err
			}
			part.target[strings.ToUpper(strings.ReplaceAll(privilege, " ", "_"))] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	has := func(names ...string) bool {
		for _, name := range names {
			if global[name] || schema[name] {
				return true
			}
		}
		return false
	}
	ok := true
	if r.HasExportObject("views") && !has("SHOW_VIEW") {
		ok = false
	}
	if r.HasExportObject("triggers") && !has("TRIGGER") {
		ok = false
	}
	if r.HasExportObject("events") && !has("EVENT") {
		ok = false
	}
	if r.HasExportObject("routines") && !(has("EXECUTE", "ALTER_ROUTINE") || global["SHOW_ROUTINE"] || global["SELECT"]) {
		ok = false
	}
	if !ok {
		return public("export_object_privilege", "Die Berechtigung zum vollständigen Lesen der gewählten Objektklassen ist nicht nachweisbar; Export wird nicht begonnen.")
	}
	return nil
}

func mysqlObjectList(ctx context.Context, conn *sql.Conn, r core.ExportRequest) ([]exportObject, error) {
	if err := mysqlObjectPrivileges(ctx, conn, r); err != nil {
		return nil, err
	}
	// Stored routine bodies are resolved when they run, but a view that calls a stored
	// function is checked at CREATE VIEW: MySQL/MariaDB therefore restore routines first.
	var objects []exportObject
	if r.HasExportObject("routines") {
		if err := collectObjects(ctx, conn, "SELECT ROUTINE_TYPE,ROUTINE_NAME FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE() ORDER BY ROUTINE_TYPE,ROUTINE_NAME", func(rows *sql.Rows) (exportObject, error) {
			var kind string
			o := exportObject{class: "routines"}
			err := rows.Scan(&kind, &o.name)
			o.kind = strings.ToLower(kind)
			if o.kind != "function" && o.kind != "procedure" {
				err = objectUnreadable()
			}
			return o, err
		}, &objects); err != nil {
			return nil, err
		}
	}
	if r.HasExportObject("views") {
		views, err := mysqlViewList(ctx, conn)
		if err != nil {
			return nil, err
		}
		objects = append(objects, views...)
	}
	if r.HasExportObject("triggers") {
		if err := collectObjects(ctx, conn, "SELECT TRIGGER_NAME,EVENT_OBJECT_TABLE FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA=DATABASE() ORDER BY EVENT_OBJECT_TABLE,ACTION_TIMING,EVENT_MANIPULATION,ACTION_ORDER,CREATED,TRIGGER_NAME", func(rows *sql.Rows) (exportObject, error) {
			o := exportObject{class: "triggers", kind: "trigger"}
			return o, rows.Scan(&o.name, &o.table)
		}, &objects); err != nil {
			return nil, err
		}
	}
	if r.HasExportObject("events") {
		if err := collectObjects(ctx, conn, "SELECT EVENT_NAME FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE() ORDER BY EVENT_NAME", func(rows *sql.Rows) (exportObject, error) {
			o := exportObject{class: "events", kind: "event"}
			return o, rows.Scan(&o.name)
		}, &objects); err != nil {
			return nil, err
		}
	}
	return objects, nil
}

// mysqlViewList orders views so that a view follows the views it reads. The server
// writes every reference as `database`.`name` into VIEW_DEFINITION; the references are
// extracted with a quote-aware scan, never by rewriting the definition.
func mysqlViewList(ctx context.Context, conn *sql.Conn) ([]exportObject, error) {
	var database string
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, "SELECT TABLE_NAME,VIEW_DEFINITION FROM information_schema.VIEWS WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var views []exportObject
	var texts []string
	index := map[string]int{}
	total := 0
	for rows.Next() {
		var name string
		var text sql.NullString
		if err = rows.Scan(&name, &text); err != nil {
			return nil, err
		}
		if !text.Valid || strings.TrimSpace(text.String) == "" || len(text.String) > core.ExportMaxObjectBytes {
			return nil, objectUnreadable()
		}
		total += len(text.String)
		if total > 64<<20 || len(views) >= core.ExportMaxObjects {
			return nil, public("export_object_limit", "Zu viele oder zu große Sichten für einen Export.")
		}
		index[name] = len(views)
		views = append(views, exportObject{class: "views", kind: "view", name: name})
		texts = append(texts, text.String)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	deps := make([][]int, len(views))
	for i, text := range texts {
		for _, ref := range mysqlViewReferences(text, database) {
			if j, ok := index[ref]; ok && j != i {
				deps[i] = append(deps[i], j)
			}
		}
	}
	order, err := dependencyOrder(len(views), deps)
	if err != nil {
		return nil, err
	}
	sorted := make([]exportObject, len(views))
	for i, j := range order {
		sorted[i] = views[j]
	}
	return sorted, nil
}

// mysqlViewReferences returns the object names that follow `database`. in text,
// skipping string literals and comments. Backtick quoting doubles backticks.
func mysqlViewReferences(text, database string) []string {
	prefix := "`" + strings.ReplaceAll(database, "`", "``") + "`.`"
	var refs []string
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '\'' || c == '"':
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == c {
					if i+1 < len(text) && text[i+1] == c {
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
		case c == '`':
			if strings.HasPrefix(text[i:], prefix) {
				name, end, ok := scanBacktick(text, i+len(prefix)-1)
				if ok {
					refs = append(refs, name)
					i = end
					continue
				}
			}
			_, end, ok := scanBacktick(text, i)
			if !ok {
				return refs
			}
			i = end
		case c == '#' || c == '-' && strings.HasPrefix(text[i:], "-- "):
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case c == '/' && strings.HasPrefix(text[i:], "/*"):
			end := strings.Index(text[i+2:], "*/")
			if end < 0 {
				return refs
			}
			i += end + 4
		default:
			i++
		}
	}
	return refs
}

// scanBacktick reads a backtick-quoted identifier that starts at text[start] and
// returns the unquoted name and the index after the closing backtick.
func scanBacktick(text string, start int) (string, int, bool) {
	var b strings.Builder
	for i := start + 1; i < len(text); i++ {
		if text[i] == '`' {
			if i+1 < len(text) && text[i+1] == '`' {
				b.WriteByte('`')
				i++
				continue
			}
			return b.String(), i + 1, true
		}
		b.WriteByte(text[i])
	}
	return "", 0, false
}

// dependencyOrder returns node indexes so that every node follows its dependencies.
// Ties keep the input order. A cycle cannot be restored and aborts the export.
func dependencyOrder(n int, deps [][]int) ([]int, error) {
	done := make([]bool, n)
	order := make([]int, 0, n)
	for len(order) < n {
		progress := false
		for i := 0; i < n; i++ {
			if done[i] {
				continue
			}
			ready := true
			for _, d := range deps[i] {
				if !done[d] {
					ready = false
					break
				}
			}
			if ready {
				done[i] = true
				order = append(order, i)
				progress = true
				// restart from the front so that tie order stays the input order
				break
			}
		}
		if !progress {
			return nil, public("export_object_cycle", "Zirkuläre Abhängigkeit zwischen Datenbankobjekten; Export ist nicht möglich.")
		}
	}
	return order, nil
}

type mysqlObjectCreateResult struct {
	statement string
	sqlMode   string
	charset   string
	collation string
	timeZone  string
}

// mysqlObjectCreate reads the authoritative SHOW CREATE output of one object.
func mysqlObjectCreate(ctx context.Context, conn *sql.Conn, o exportObject) (mysqlObjectCreateResult, error) {
	var out mysqlObjectCreateResult
	keyword := strings.ToUpper(o.kind)
	rows, err := conn.QueryContext(ctx, "SHOW CREATE "+keyword+" "+quoteIdentifier("mysql", o.name))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return out, err
	}
	if !rows.Next() {
		return out, objectUnreadable()
	}
	values := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err = rows.Scan(ptrs...); err != nil {
		return out, err
	}
	createColumn := map[string]string{"view": "Create View", "procedure": "Create Procedure", "function": "Create Function", "trigger": "SQL Original Statement", "event": "Create Event"}[o.kind]
	found := false
	for i, col := range cols {
		v := values[i]
		switch strings.ToLower(col) {
		case strings.ToLower(createColumn):
			if !v.Valid || strings.TrimSpace(v.String) == "" {
				return out, objectUnreadable()
			}
			out.statement = v.String
			found = true
		case "sql_mode":
			out.sqlMode = v.String
		case "character_set_client":
			out.charset = v.String
		case "collation_connection":
			out.collation = v.String
		case "time_zone":
			out.timeZone = v.String
		}
	}
	if !found || rows.Next() {
		return out, objectUnreadable()
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if len(out.statement) > core.ExportMaxObjectBytes {
		return out, public("export_object_too_large", "Eine Objektdefinition überschreitet die Größengrenze; Export ist unvollständig.")
	}
	if !mysqlSQLMode.MatchString(out.sqlMode) || out.charset != "" && !mysqlSimpleName.MatchString(out.charset) || out.collation != "" && !mysqlSimpleName.MatchString(out.collation) || out.timeZone != "" && !mysqlTimeZone.MatchString(out.timeZone) {
		return out, public("unsupported_export", "Sitzungsmerkmale eines Objekts sind nicht sicher exportierbar.")
	}
	return out, nil
}

// mysqlRewriteCreate parses the fixed server-generated prefix of a CREATE statement
// (CREATE [ALGORITHM=..] [DEFINER=..] [SQL SECURITY ..] <KIND> ...), drops the DEFINER
// clause unless preserved and inserts IF NOT EXISTS after the kind. The statement body
// is never touched; an unexpected prefix aborts the export.
func mysqlRewriteCreate(statement, kind string, preserveDefiner, ifNotExists bool) (string, error) {
	fail := public("unsupported_export", "Objektdefinition hat ein unerwartetes Format.")
	keyword := strings.ToUpper(kind)
	if !strings.HasPrefix(statement, "CREATE ") {
		return "", fail
	}
	i := len("CREATE ")
	var kept strings.Builder
	for {
		rest := statement[i:]
		switch {
		case len(rest) > len(keyword) && strings.EqualFold(rest[:len(keyword)], keyword) && rest[len(keyword)] == ' ':
			head := "CREATE " + kept.String() + keyword
			if ifNotExists {
				head += " IF NOT EXISTS"
			}
			return head + statement[i+len(keyword):], nil
		case strings.HasPrefix(rest, "ALGORITHM="):
			end := strings.IndexByte(rest, ' ')
			if end < 0 {
				return "", fail
			}
			kept.WriteString(rest[:end+1])
			i += end + 1
		case strings.HasPrefix(rest, "SQL SECURITY DEFINER "), strings.HasPrefix(rest, "SQL SECURITY INVOKER "):
			n := len("SQL SECURITY DEFINER ")
			kept.WriteString(rest[:n])
			i += n
		case strings.HasPrefix(rest, "DEFINER="):
			end, ok := skipMysqlAccount(rest[len("DEFINER="):])
			if !ok || len(rest) <= len("DEFINER=")+end || rest[len("DEFINER=")+end] != ' ' {
				return "", fail
			}
			n := len("DEFINER=") + end + 1
			if preserveDefiner {
				kept.WriteString(rest[:n])
			}
			i += n
		default:
			return "", fail
		}
	}
}

// skipMysqlAccount returns the length of `user`[@`host`]. The server quotes with
// backticks, or with double quotes when the object was created under ANSI_QUOTES.
func skipMysqlAccount(s string) (int, bool) {
	part := func(from int) (int, bool) {
		if from >= len(s) || s[from] != '`' && s[from] != '"' {
			return 0, false
		}
		q := s[from]
		for i := from + 1; i < len(s); i++ {
			if s[i] == q {
				if i+1 < len(s) && s[i+1] == q {
					i++
					continue
				}
				return i + 1, true
			}
		}
		return 0, false
	}
	end, ok := part(0)
	if !ok {
		return 0, false
	}
	if end < len(s) && s[end] == '@' {
		return part(end + 1)
	}
	return end, true
}

var delimiterCandidates = []string{";;", "$$", "//", "%%", "@@", "!!", "^^", "~~", ";;;", "$$$", "///", "%%%", "@@@", "^^^", "!!!"}

// mysqlDelimiter picks a client delimiter that occurs nowhere in the statement.
func mysqlDelimiter(statement string) (string, error) {
	for _, line := range strings.Split(statement, "\n") {
		if len(strings.TrimLeft(line, " \t\r")) >= 9 && strings.EqualFold(strings.TrimLeft(line, " \t\r")[:9], "delimiter") {
			return "", public("unsupported_export", "Objektdefinition enthält eine Zeile, die wie ein Client-DELIMITER-Befehl beginnt.")
		}
	}
	for _, candidate := range delimiterCandidates {
		if !strings.Contains(statement, candidate) {
			return candidate, nil
		}
	}
	return "", public("unsupported_export", "Für die Objektdefinition ist kein sicheres Client-Trennzeichen verfügbar.")
}

// ---------------------------------------------------------------- PostgreSQL

const pgObjectSchemas = `n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp\_%'`

func postgresObjectList(ctx context.Context, conn *sql.Conn, r core.ExportRequest) ([]exportObject, error) {
	var mixed []exportObject
	if r.HasExportObject("routines") {
		var aggregates int
		if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prokind IN ('a','w') AND `+pgObjectSchemas+` AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e')`).Scan(&aggregates); err != nil {
			return nil, err
		}
		if aggregates > 0 {
			return nil, public("unsupported_export", "Aggregat- und Fensterfunktionen sind nicht exportierbar.")
		}
	}
	if r.HasExportObject("views") {
		if err := collectObjects(ctx, conn, `SELECT c.oid::int8,n.nspname,c.relname,c.relkind::text FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('v','m') AND `+pgObjectSchemas+` AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e') ORDER BY n.nspname,c.relname`, func(rows *sql.Rows) (exportObject, error) {
			o := exportObject{class: "views"}
			var relkind string
			err := rows.Scan(&o.oid, &o.schema, &o.name, &relkind)
			o.kind = "view"
			if relkind == "m" {
				o.kind = "matview"
			}
			return o, err
		}, &mixed); err != nil {
			return nil, err
		}
	}
	if r.HasExportObject("routines") {
		if err := collectObjects(ctx, conn, `SELECT p.oid::int8,n.nspname,p.proname,p.prokind::text,pg_get_function_identity_arguments(p.oid) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prokind IN ('f','p') AND `+pgObjectSchemas+` AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e') ORDER BY n.nspname,p.proname,pg_get_function_identity_arguments(p.oid)`, func(rows *sql.Rows) (exportObject, error) {
			o := exportObject{class: "routines"}
			var prokind string
			err := rows.Scan(&o.oid, &o.schema, &o.name, &prokind, &o.identity)
			o.kind = "function"
			if prokind == "p" {
				o.kind = "procedure"
			}
			return o, err
		}, &mixed); err != nil {
			return nil, err
		}
	}
	if len(mixed) > 1 {
		index := map[int64]int{}
		for i, o := range mixed {
			index[o.oid] = i
		}
		deps := make([][]int, len(mixed))
		add := func(from, to int64) {
			if a, ok := index[from]; ok {
				if b, ok := index[to]; ok && a != b {
					deps[a] = append(deps[a], b)
				}
			}
		}
		// A view follows the views and functions its rewrite rule uses.
		if err := pgEdges(ctx, conn, `SELECT r.ev_class::int8,d.refobjid::int8 FROM pg_rewrite r JOIN pg_class c ON c.oid=r.ev_class JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_depend d ON d.classid='pg_rewrite'::regclass AND d.objid=r.oid WHERE c.relkind IN ('v','m') AND d.refclassid IN ('pg_class'::regclass,'pg_proc'::regclass) AND d.refobjid<>r.ev_class AND `+pgObjectSchemas, add); err != nil {
			return nil, err
		}
		// A routine follows the views whose row types it takes or returns.
		if err := pgEdges(ctx, conn, `SELECT p.oid::int8,c.oid::int8 FROM pg_depend d JOIN pg_proc p ON d.classid='pg_proc'::regclass AND d.objid=p.oid JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_type t ON d.refclassid='pg_type'::regclass AND d.refobjid=t.oid JOIN pg_class c ON c.reltype=t.oid AND c.relkind IN ('v','m') WHERE `+pgObjectSchemas, add); err != nil {
			return nil, err
		}
		order, err := dependencyOrder(len(mixed), deps)
		if err != nil {
			return nil, err
		}
		sorted := make([]exportObject, len(mixed))
		for i, j := range order {
			sorted[i] = mixed[j]
		}
		mixed = sorted
	}
	objects := mixed
	if r.HasExportObject("triggers") {
		if err := collectObjects(ctx, conn, `SELECT t.oid::int8,t.tgname,n.nspname,c.relname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND `+pgObjectSchemas+` ORDER BY n.nspname,c.relname,t.tgname`, func(rows *sql.Rows) (exportObject, error) {
			o := exportObject{class: "triggers", kind: "trigger"}
			err := rows.Scan(&o.oid, &o.name, &o.tableSchema, &o.table)
			o.schema = o.tableSchema
			return o, err
		}, &objects); err != nil {
			return nil, err
		}
	}
	return objects, nil
}

func pgEdges(ctx context.Context, conn *sql.Conn, query string, add func(from, to int64)) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var from, to int64
		if err = rows.Scan(&from, &to); err != nil {
			return err
		}
		add(from, to)
	}
	return rows.Err()
}

var pgReloption = regexp.MustCompile(`^[a-z_.]{1,63}=[A-Za-z0-9_.-]{1,63}$`)

// postgresObjectStatements returns the CREATE statements (without terminator) of one object.
func postgresObjectStatements(ctx context.Context, conn *sql.Conn, o exportObject) ([]string, error) {
	switch o.kind {
	case "view", "matview":
		var def sql.NullString
		if err := conn.QueryRowContext(ctx, "SELECT pg_get_viewdef($1::int8::oid,true)", o.oid).Scan(&def); err != nil || !def.Valid {
			return nil, objectUnreadable()
		}
		var options []string
		rows, err := conn.QueryContext(ctx, "SELECT unnest(reloptions) FROM pg_class WHERE oid=$1::int8::oid", o.oid)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var option string
			if err = rows.Scan(&option); err != nil {
				rows.Close()
				return nil, err
			}
			if !pgReloption.MatchString(option) {
				rows.Close()
				return nil, public("unsupported_export", "Eine Sichtoption ist nicht sicher exportierbar.")
			}
			options = append(options, option)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		name, quoteErr := dbplan.QualifiedName("postgres", o.schema, o.name)
		if quoteErr != nil {
			return nil, objectUnreadable()
		}
		with := ""
		if len(options) > 0 {
			with = " WITH (" + strings.Join(options, ",") + ")"
		}
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(def.String), ";"))
		if o.kind == "view" {
			return []string{"CREATE VIEW " + name + with + " AS " + body}, nil
		}
		statements := []string{"CREATE MATERIALIZED VIEW " + name + with + " AS " + body + " WITH DATA"}
		idx, err := conn.QueryContext(ctx, "SELECT pg_get_indexdef(i.indexrelid) FROM pg_index i WHERE i.indrelid=$1::int8::oid ORDER BY (SELECT relname FROM pg_class WHERE oid=i.indexrelid)", o.oid)
		if err != nil {
			return nil, err
		}
		defer idx.Close()
		for idx.Next() {
			var def string
			if err = idx.Scan(&def); err != nil {
				return nil, err
			}
			statements = append(statements, def)
		}
		return statements, idx.Err()
	case "function", "procedure":
		var def sql.NullString
		if err := conn.QueryRowContext(ctx, "SELECT pg_get_functiondef($1::int8::oid)", o.oid).Scan(&def); err != nil || !def.Valid || strings.TrimSpace(def.String) == "" {
			return nil, objectUnreadable()
		}
		return []string{strings.TrimSpace(def.String)}, nil
	default:
		var def sql.NullString
		var enabled string
		if err := conn.QueryRowContext(ctx, "SELECT pg_get_triggerdef($1::int8::oid,true),tgenabled::text FROM pg_trigger WHERE oid=$1::int8::oid", o.oid).Scan(&def, &enabled); err != nil || !def.Valid {
			return nil, objectUnreadable()
		}
		statements := []string{strings.TrimSpace(def.String)}
		table, err := dbplan.QualifiedName("postgres", o.tableSchema, o.table)
		trigger, err2 := dbplan.QuoteIdentifier("postgres", o.name)
		if err != nil || err2 != nil {
			return nil, objectUnreadable()
		}
		state := map[string]string{"D": "DISABLE", "R": "ENABLE REPLICA", "A": "ENABLE ALWAYS"}[enabled]
		if state != "" {
			statements = append(statements, "ALTER TABLE "+table+" "+state+" TRIGGER "+trigger)
		}
		return statements, nil
	}
}

// ---------------------------------------------------------------- output

func objectLabel(o exportObject) string {
	label := o.kind + " "
	if o.schema != "" {
		label += o.schema + "."
	}
	label += o.name
	if o.table != "" {
		label += " on " + o.table
	}
	return label
}

func objectQualified(driver string, o exportObject) (string, error) {
	if driver == "postgres" {
		return dbplan.QualifiedName(driver, o.schema, o.name)
	}
	return dbplan.QuoteIdentifier(driver, o.name)
}

// objectDrops emits the DROP statements in reverse creation order. They precede the
// table section so that dependent objects never block dropping the tables (PostgreSQL).
func (f *exportFormat) objectDrops(c core.Connection, objects []exportObject) error {
	for i := len(objects) - 1; i >= 0; i-- {
		o := objects[i]
		name, err := objectQualified(c.Driver, o)
		if err != nil {
			return public("unsupported_export", "Objektname ist nicht exportierbar.")
		}
		var statement string
		switch o.kind {
		case "view":
			statement = "DROP VIEW IF EXISTS " + name
		case "matview":
			statement = "DROP MATERIALIZED VIEW IF EXISTS " + name
		case "function":
			statement = "DROP FUNCTION IF EXISTS " + name
		case "procedure":
			statement = "DROP PROCEDURE IF EXISTS " + name
		case "event":
			statement = "DROP EVENT IF EXISTS " + name
		case "trigger":
			if c.Driver == "postgres" {
				table, tableErr := dbplan.QualifiedName(c.Driver, o.tableSchema, o.table)
				trigger, triggerErr := dbplan.QuoteIdentifier(c.Driver, o.name)
				if tableErr != nil || triggerErr != nil {
					return public("unsupported_export", "Objektname ist nicht exportierbar.")
				}
				statement = "DROP TRIGGER IF EXISTS " + trigger + " ON " + table
			} else {
				statement = "DROP TRIGGER IF EXISTS " + name
			}
		}
		if c.Driver == "postgres" && (o.kind == "function" || o.kind == "procedure") {
			statement += "(" + o.identity + ")"
		}
		if err = f.write(statement + ";\n"); err != nil {
			return err
		}
	}
	return nil
}

// objectsBegin writes the fixed banner and session preparation of the object section.
func (f *exportFormat) objectsBegin(c core.Connection) error {
	s := "-- Objects: " + strings.Join(f.r.Objects, ", ") + "\n"
	switch c.Driver {
	case "mysql":
		if f.r.Definer == "preserve" {
			s += "-- WARNING: DEFINER clauses are preserved. The accounts must exist and the restoring account needs SET_USER_ID or SUPER.\n"
		} else {
			s += "-- DEFINER clauses are omitted: the restoring account becomes the definer; SQL SECURITY DEFINER objects run with its rights.\n"
		}
		if f.r.HasExportObject("views") {
			s += "-- View definitions keep the schema qualification of database " + safeExportComment(c.Database) + ": restore into a database of the same name.\n"
		}
		s += "SET @dbm_export_dump_sql_mode=@@SESSION.sql_mode;\n"
	case "postgres":
		s += "-- Not included: ownership, privileges, comments, rules, sequences, types, event triggers. Objects are schema-qualified.\n"
		if f.r.HasExportObject("routines") {
			s += "SET check_function_bodies=false;\n"
		}
	}
	return f.write(s)
}

// objectsEnd restores the session settings the table and data statements rely on.
func (f *exportFormat) objectsEnd(c core.Connection) error {
	switch c.Driver {
	case "mysql":
		return f.write("SET SESSION sql_mode=@dbm_export_dump_sql_mode;\nSET NAMES utf8mb4;\nSET time_zone='+00:00';\n")
	case "postgres":
		if f.r.HasExportObject("routines") {
			return f.write("RESET check_function_bodies;\n")
		}
	}
	return nil
}

// writeObjects reads and writes the object definitions one by one.
func (f *exportFormat) writeObjects(ctx context.Context, conn *sql.Conn, c core.Connection, objects []exportObject) error {
	if len(objects) == 0 {
		return nil
	}
	if c.Driver == "mysql" {
		// Quote and render SHOW CREATE output independent of the account's session mode.
		if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode=''"); err != nil {
			return objectFailure(ctx, err)
		}
	}
	if c.Driver == "postgres" {
		// Names are written fully qualified: no dependency on the restoring search_path.
		if _, err := conn.ExecContext(ctx, "SET LOCAL search_path=pg_catalog"); err != nil {
			return objectFailure(ctx, err)
		}
	}
	if err := f.objectsBegin(c); err != nil {
		return err
	}
	for _, o := range objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if f.r.Comments {
			if err := f.write("-- Catalog " + safeExportComment(objectLabel(o)) + "\n"); err != nil {
				return err
			}
		}
		var err error
		switch c.Driver {
		case "sqlite":
			err = f.writeSQLiteObject(ctx, conn, o)
		case "mysql":
			err = f.writeMySQLObject(ctx, conn, o)
		default:
			err = f.writePostgresObject(ctx, conn, o)
		}
		if err != nil {
			return objectFailure(ctx, err)
		}
	}
	return f.objectsEnd(c)
}

func (f *exportFormat) writeSQLiteObject(ctx context.Context, conn *sql.Conn, o exportObject) error {
	text, err := sqliteObjectDDL(ctx, conn, o, f.r.IfNotExists)
	if err != nil {
		return err
	}
	if len(text) > core.ExportMaxObjectBytes {
		return public("export_object_too_large", "Eine Objektdefinition überschreitet die Größengrenze; Export ist unvollständig.")
	}
	// The newline protects the terminator from a trailing line comment.
	return f.write(text + "\n;\n")
}

func (f *exportFormat) writePostgresObject(ctx context.Context, conn *sql.Conn, o exportObject) error {
	if o.schema != "" && o.schema != "public" {
		schema, err := dbplan.QuoteIdentifier("postgres", o.schema)
		if err != nil {
			return objectUnreadable()
		}
		if err = f.write("CREATE SCHEMA IF NOT EXISTS " + schema + ";\n"); err != nil {
			return err
		}
	}
	statements, err := postgresObjectStatements(ctx, conn, o)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if len(statement) > core.ExportMaxObjectBytes {
			return public("export_object_too_large", "Eine Objektdefinition überschreitet die Größengrenze; Export ist unvollständig.")
		}
		if err = f.write(statement + ";\n"); err != nil {
			return err
		}
	}
	return nil
}

func (f *exportFormat) writeMySQLObject(ctx context.Context, conn *sql.Conn, o exportObject) error {
	create, err := mysqlObjectCreate(ctx, conn, o)
	if err != nil {
		return err
	}
	statement, err := mysqlRewriteCreate(create.statement, o.kind, f.r.Definer == "preserve", f.r.IfNotExists)
	if err != nil {
		return err
	}
	s := ""
	if o.kind != "view" {
		s += "SET SESSION sql_mode='" + create.sqlMode + "';\n"
	}
	if create.charset != "" {
		s += "SET character_set_client='" + create.charset + "';\n"
	}
	if create.collation != "" {
		s += "SET collation_connection='" + create.collation + "';\n"
	}
	if o.kind == "event" && create.timeZone != "" {
		s += "SET time_zone='" + create.timeZone + "';\n"
	}
	if o.kind == "view" {
		return f.write(s + statement + ";\n")
	}
	delimiter, err := mysqlDelimiter(statement)
	if err != nil {
		return err
	}
	// The delimiter sits on its own line, so no statement byte can join it.
	return f.write(s + "DELIMITER " + delimiter + "\n" + statement + "\n" + delimiter + "\nDELIMITER ;\n")
}

// objectNamesEqual reports whether two object lists name the same objects (used
// to detect concurrent object DDL between the listing and the end of the section).
func objectNamesEqual(a, b []exportObject) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(o exportObject) string {
		return o.kind + "\x00" + o.schema + "\x00" + o.name + "\x00" + o.table + "\x00" + o.identity + "\x00" + strconv.FormatInt(o.oid, 10)
	}
	ka, kb := make([]string, len(a)), make([]string, len(b))
	for i := range a {
		ka[i], kb[i] = key(a[i]), key(b[i])
	}
	sort.Strings(ka)
	sort.Strings(kb)
	for i := range ka {
		if ka[i] != kb[i] {
			return false
		}
	}
	return true
}
