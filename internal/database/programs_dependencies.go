// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func programDependencies(ctx context.Context, env *opEnv, p *core.Program) error {
	if p.Object.Table != nil {
		p.Dependencies = append(p.Dependencies, core.ProgramDependency{Object: core.ProgramRef{Kind: "table", Schema: p.Object.Table.Schema, Name: p.Object.Table.Name}, Direction: "outgoing", Certainty: "exact"})
	}
	if env.c.Driver == "postgres" {
		return postgresProgramDependencies(ctx, env, p)
	}
	// Textual scans are intentionally labeled. They may find false positives
	// and cannot discover dynamic SQL or references hidden by server privileges.
	query := `SELECT type,name,COALESCE(sql,'') FROM main.sqlite_schema WHERE type IN ('table','view','trigger') AND name NOT GLOB 'sqlite_*' ORDER BY type,name LIMIT 1001`
	args := []any{}
	if env.c.Driver == "mysql" {
		query = `SELECT 'view',TABLE_NAME,COALESCE(VIEW_DEFINITION,'') FROM information_schema.VIEWS WHERE BINARY TABLE_SCHEMA=BINARY ?
 UNION ALL SELECT 'table',TABLE_NAME,'' FROM information_schema.TABLES WHERE BINARY TABLE_SCHEMA=BINARY ? AND TABLE_TYPE='BASE TABLE'
 UNION ALL SELECT LOWER(ROUTINE_TYPE),ROUTINE_NAME,COALESCE(ROUTINE_DEFINITION,'') FROM information_schema.ROUTINES WHERE BINARY ROUTINE_SCHEMA=BINARY ?
 UNION ALL SELECT 'trigger',TRIGGER_NAME,ACTION_STATEMENT FROM information_schema.TRIGGERS WHERE BINARY TRIGGER_SCHEMA=BINARY ?
 UNION ALL SELECT 'event',EVENT_NAME,EVENT_DEFINITION FROM information_schema.EVENTS WHERE BINARY EVENT_SCHEMA=BINARY ? LIMIT 1001`
		for i := 0; i < 5; i++ {
			args = append(args, p.Object.Schema)
		}
	}
	rows, err := env.sess.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	text := ""
	if p.Body != nil {
		text = *p.Body
	} else if p.Definition != nil {
		text = *p.Definition
	}
	count := 0
	for rows.Next() {
		count++
		if count > 1000 {
			p.DependenciesTruncated = true
			break
		}
		var kind, name, definition string
		if err = rows.Scan(&kind, &name, &definition); err != nil {
			return err
		}
		if name == p.Object.Name && kind == p.Object.Kind {
			continue
		}
		add := func(direction string) {
			if len(p.Dependencies) >= 100 {
				p.DependenciesTruncated = true
				return
			}
			p.Dependencies = append(p.Dependencies, core.ProgramDependency{Object: core.ProgramRef{Kind: kind, Schema: p.Object.Schema, Name: name}, Direction: direction, Certainty: "textual"})
		}
		if strings.Contains(strings.ToLower(definition), strings.ToLower(p.Object.Name)) {
			add("incoming")
		}
		if strings.Contains(strings.ToLower(text), strings.ToLower(name)) {
			add("outgoing")
		}
	}
	p.Warnings = append(p.Warnings, "Abhängigkeiten sind textuelle Hinweise innerhalb des Schemas (maximal 1000 Kandidaten, 100 Treffer), keine vollständige Analyse. Fehlende Katalogrechte und dynamisches SQL können Verweise verbergen.")
	return rows.Err()
}
func postgresProgramDependencies(ctx context.Context, env *opEnv, p *core.Program) error {
	// pg_depend records references of views through pg_rewrite, triggers through
	// pg_trigger, and routines through pg_proc. String routine bodies can have no
	// catalog dependencies and are explicitly reported as incomplete below.
	rows, err := env.sess.QueryContext(ctx, `WITH target AS (
 SELECT 'pg_class'::regclass AS classid,c.oid FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE $1='view' AND n.nspname=$2 AND c.relname=$3
 UNION ALL SELECT 'pg_proc'::regclass,p.oid FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE $1 IN ('function','procedure') AND n.nspname=$2 AND p.proname=$3 AND pg_get_function_identity_arguments(p.oid)=$4
 UNION ALL SELECT 'pg_trigger'::regclass,t.oid FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE $1='trigger' AND n.nspname=$2 AND t.tgname=$3 AND c.relname=$5
 ), links AS (
 SELECT d.classid,d.objid,d.refclassid,d.refobjid FROM pg_depend d
 UNION ALL SELECT 'pg_class'::regclass,r.ev_class,d.refclassid,d.refobjid FROM pg_depend d JOIN pg_rewrite r ON d.classid='pg_rewrite'::regclass AND d.objid=r.oid
 ), objects AS (
 SELECT 'pg_class'::regclass AS classid,c.oid,n.nspname AS schema_name,c.relname AS name,CASE WHEN c.relkind='v' THEN 'view' ELSE 'table' END AS kind,'' AS identity_args,'' AS table_name FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v') AND (pg_has_role(c.relowner,'USAGE') OR has_table_privilege(c.oid,'SELECT,TRIGGER'))
 UNION ALL SELECT 'pg_proc'::regclass,p.oid,n.nspname,p.proname,CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END,pg_get_function_identity_arguments(p.oid),'' FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prokind IN ('f','p') AND has_function_privilege(p.oid,'EXECUTE')
 UNION ALL SELECT 'pg_trigger'::regclass,t.oid,n.nspname,t.tgname,'trigger','',c.relname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND has_table_privilege(c.oid,'TRIGGER')
 ) SELECT DISTINCT o.kind,o.schema_name,o.name,o.identity_args,o.table_name,'outgoing' AS direction FROM target t JOIN links d ON d.classid=t.classid AND d.objid=t.oid JOIN objects o ON o.classid=d.refclassid AND o.oid=d.refobjid WHERE NOT(o.classid=t.classid AND o.oid=t.oid) AND o.schema_name NOT IN ('pg_catalog','information_schema')
 UNION SELECT DISTINCT o.kind,o.schema_name,o.name,o.identity_args,o.table_name,'incoming' FROM target t JOIN links d ON d.refclassid=t.classid AND d.refobjid=t.oid JOIN objects o ON o.classid=d.classid AND o.oid=d.objid WHERE NOT(o.classid=t.classid AND o.oid=t.oid) AND o.schema_name NOT IN ('pg_catalog','information_schema')
 ORDER BY direction,kind,schema_name,name,identity_args,table_name LIMIT 101`, p.Object.Kind, p.Object.Schema, p.Object.Name, p.Object.Identity, programTableName(p.Object))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var dep core.ProgramDependency
		var table string
		if err = rows.Scan(&dep.Object.Kind, &dep.Object.Schema, &dep.Object.Name, &dep.Object.Identity, &table, &dep.Direction); err != nil {
			return err
		}
		dep.Certainty = "exact"
		if table != "" {
			dep.Object.Table = &core.TableRef{Schema: dep.Object.Schema, Name: table}
		}
		if len(p.Dependencies) >= 100 {
			p.DependenciesTruncated = true
			break
		}
		p.Dependencies = append(p.Dependencies, dep)
	}
	p.Warnings = append(p.Warnings, "Katalogabhängigkeiten sind exakt, aber unvollständig: String-Körper, dynamisches SQL, Erweiterungen und unsichtbare Objekte können weitere Verweise enthalten.")
	return rows.Err()
}
func programTableName(ref core.ProgramRef) string {
	if ref.Table == nil {
		return ""
	}
	return ref.Table.Name
}
