// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func validatePrefix(r core.PrefixRequest, read bool) error {
	if r.Prefix == "" || len(r.Prefix) > 255 || len(r.Schema) > 1024 || len(r.NewPrefix) > 255 || strings.ContainsRune(r.Prefix+r.NewPrefix+r.Schema, 0) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 || opSystemSchema(r.Connection.Driver, r.Schema) {
		return invalidOp("Präfix, Schema oder Zeitlimit ist ungültig.")
	}
	if read {
		if r.Action != "" || r.NewPrefix != "" {
			return invalidOp("Leseanfrage enthält Änderungsfelder.")
		}
		return nil
	}
	switch r.Action {
	case "drop", "truncate", "remove":
		if r.NewPrefix != "" {
			return invalidOp("Diese Aktion akzeptiert kein neues Präfix.")
		}
	case "replace", "add", "copy":
		if r.NewPrefix == "" {
			return invalidOp("Neues Präfix fehlt.")
		}
	default:
		return invalidOp("Unbekannte Präfixaktion.")
	}
	return nil
}

// prefixTables reads the full matching set with a bound, never a truncated batch.
// Prefixes are compared literally, including SQL LIKE wildcard characters.
func prefixTables(ctx context.Context, env *opEnv, r core.PrefixRequest) (core.PrefixResult, error) {
	out := core.PrefixResult{Tables: []core.TableRef{}}
	driver := env.c.Driver
	schema := r.Schema
	var query string
	var args []any
	switch driver {
	case "sqlite":
		if schema != "" && schema != "main" {
			return out, invalidOp("SQLite unterstützt nur main.")
		}
		schema = "main"
		query = `SELECT name FROM sqlite_schema WHERE type='table' AND substr(name,1,?)=? AND substr(lower(name),1,7)<>'sqlite_' ORDER BY name LIMIT 101`
		args = []any{len([]rune(r.Prefix)), r.Prefix}
	case "mysql":
		if schema == "" {
			if err := env.sess.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
				return out, err
			}
		}
		query = `SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE' AND BINARY LEFT(TABLE_NAME,?)=BINARY ? ORDER BY TABLE_NAME LIMIT 101`
		args = []any{schema, len([]rune(r.Prefix)), r.Prefix}
	case "postgres":
		if schema == "" {
			schema = "public"
		}
		query = `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relkind IN ('r','p') AND left(c.relname,$2)=$3 ORDER BY c.relname LIMIT 101`
		args = []any{schema, len([]rune(r.Prefix)), r.Prefix}
	default:
		return out, public("unsupported_action", "Unbekannter Treiber.")
	}
	if opSystemSchema(driver, schema) {
		return out, invalidOp("Systemschemas sind ausgeschlossen.")
	}
	rows, err := env.sess.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return out, err
		}
		if !strings.HasPrefix(name, r.Prefix) {
			continue
		}
		out.Tables = append(out.Tables, core.TableRef{Schema: schema, Name: name})
		if len(out.Tables) > core.PrefixMaxTables {
			return out, public("result_too_large", "Mehr als 100 passende Tabellen: Präfix enger wählen.")
		}
	}
	return out, rows.Err()
}
func (e *Engine) PrefixTables(ctx context.Context, r core.PrefixRequest) (core.PrefixResult, error) {
	if err := validatePrefix(r, true); err != nil {
		return core.PrefixResult{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.PrefixResult{}, err
	}
	defer closeAll()
	out, err := prefixTables(ctx, env, r)
	return out, opPlanError(ctx, err)
}

// prefixGroup records how many plan statements belong to one batch table, in plan order.
type prefixGroup struct {
	table core.TableRef
	count int
}

func buildPrefix(ctx context.Context, env *opEnv, r core.PrefixRequest) (core.StructurePlan, []prefixGroup, error) {
	empty := core.StructurePlan{}
	groups := []prefixGroup{}
	tables, err := prefixTables(ctx, env, r)
	if err != nil {
		return empty, nil, err
	}
	if len(tables.Tables) == 0 {
		return empty, nil, opNotFound()
	}
	statements, warnings, affected := []string{}, []string{"Der Stapel umfasst alle passenden Basistabellen im Schema. Kein automatischer Retry; MySQL/MariaDB können Teiländerungen behalten."}, []string{}
	for _, ref := range tables.Tables {
		req := core.ObjectOpRequest{Connection: r.Connection, Table: &ref}
		action := core.OpTableRename
		switch r.Action {
		case "drop":
			action = core.OpTableDrop
		case "truncate":
			action = core.OpTableTruncate
		default:
			name := r.NewPrefix + strings.TrimPrefix(ref.Name, r.Prefix)
			if r.Action == "add" {
				name = r.NewPrefix + ref.Name
			}
			if r.Action == "copy" {
				action = core.OpTableCopy
				req.Mode = spPrefix("both")
			}
			req.NewName = &name
		}
		// Reuse W-P07 catalog resolution, destination checks, child-object and
		// dependency warnings, quoting and exact statement generation.
		st, err := buildOp(ctx, env, action, req)
		if err != nil {
			return empty, nil, err
		}
		if st.view.DependenciesTruncated {
			return empty, nil, public("result_too_large", "Abhängigkeitsliste ist gekürzt; Stapel wird nicht ausgeführt.")
		}
		groups = append(groups, prefixGroup{table: ref, count: len(st.plan.Statements)})
		statements = append(statements, st.plan.Statements...)
		warnings = append(warnings, st.plan.Warnings...)
		affected = append(affected, st.plan.Affected...)
		for _, dep := range st.view.Dependencies {
			warnings = append(warnings, "Abhängigkeit ("+dep.Effect+"): "+dep.Schema+"."+dep.Name)
		}
	}
	// The typed source prefix is derived from the common prefix of catalog names.
	target := tables.Tables[0].Name[:len(r.Prefix)]
	plan, err := structurePlan(env.c.Driver, "prefix_"+r.Action, target, statements, warnings, affected, r.Action != "copy")
	return plan, groups, err
}
func spPrefix(s string) *string { return &s }
func (e *Engine) PlanPrefix(ctx context.Context, r core.PrefixRequest) (core.StructurePlan, error) {
	if err := validatePrefix(r, false); err != nil {
		return core.StructurePlan{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.StructurePlan{}, err
	}
	defer closeAll()
	p, _, err := buildPrefix(ctx, env, r)
	return p, opPlanError(ctx, err)
}
func (e *Engine) ExecutePrefix(ctx context.Context, r core.PrefixExecuteRequest) (core.PrefixExecuteResult, error) {
	none := core.PrefixExecuteResult{}
	if err := validatePrefix(r.PrefixRequest, false); err != nil {
		return none, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return none, err
	}
	defer closeAll()
	p, groups, err := buildPrefix(ctx, env, r.PrefixRequest)
	if err != nil {
		return none, opPlanError(ctx, err)
	}
	if err = checkedStructure(p, r.ExpectedPlanSHA256, r.Confirm); err != nil {
		return none, err
	}
	res := executeStructure(ctx, env, p)
	tables := prefixTableResults(env.c.Driver, res, groups)
	if res.Outcome != "completed" {
		for _, t := range tables {
			if t.Status == "applied" {
				res.Message += " Teilzustand: Der Stapel wurde nicht vollständig ausgeführt, siehe Ergebnis je Tabelle."
				break
			}
		}
	}
	return core.PrefixExecuteResult{ObjectOpResult: res, Tables: tables}, nil
}

// prefixTableResults derives an explicit per-table state. It never claims more
// than the executor knows: the table containing the first unexecuted statement
// is "unknown" after any non-definite stop on MySQL/MariaDB (no transaction).
func prefixTableResults(driver string, res core.ObjectOpResult, groups []prefixGroup) []core.PrefixTableResult {
	out := make([]core.PrefixTableResult, 0, len(groups))
	remaining := res.StatementsExecuted
	transactional := driver != "mysql"
	firstOpen := true
	for _, g := range groups {
		item := core.PrefixTableResult{Table: g.table, StatementsTotal: g.count}
		switch {
		case res.Outcome == "completed":
			item.Status, item.StatementsExecuted = "applied", g.count
		case transactional:
			// One transaction: either everything is rolled back or the commit result is unknown.
			item.Status = "not_applied"
			if res.Outcome == "outcome_unknown" {
				item.Status = "unknown"
			}
		case remaining >= g.count:
			item.Status, item.StatementsExecuted = "applied", g.count
			remaining -= g.count
		case firstOpen && res.Outcome == "outcome_unknown":
			item.Status, item.StatementsExecuted = "unknown", remaining
			remaining, firstOpen = 0, false
		default:
			item.Status = "not_applied"
			if firstOpen && remaining > 0 {
				// Partly executed multi-statement table (copy): state of the rest is unknown.
				item.Status, item.StatementsExecuted = "unknown", remaining
			}
			remaining, firstOpen = 0, false
		}
		out = append(out, item)
	}
	return out
}
