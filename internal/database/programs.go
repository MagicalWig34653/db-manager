// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func invalidProgram() error {
	return public("invalid_program", "Objektbezug oder strukturierte Programmeingabe ist ungültig.")
}
func programKind(kind string) bool {
	return kind == "view" || kind == "procedure" || kind == "function" || kind == "trigger" || kind == "event"
}
func programSupported(driver, kind string) error {
	if !programKind(kind) {
		return invalidProgram()
	}
	c := core.ProgramsCapabilities().Get(driver, kind)
	if c.Support == dbplan.Unsupported {
		return public("unsupported_action", c.Reason)
	}
	return nil
}
func newProgram(ref core.ProgramRef) core.Program {
	return core.Program{Object: ref, Parameters: []core.ProgramParameter{}, Properties: map[string]string{}, Dependencies: []core.ProgramDependency{}, Warnings: []string{}}
}
func programBound(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > 8*1024*1024 {
		return public("result_too_large", "Programmantwort überschreitet die erlaubte Größe.")
	}
	return nil
}
func validateProgramRef(ref core.ProgramRef) bool {
	if ref.Kind != "function" && ref.Kind != "procedure" && ref.Identity != "" || ref.Kind != "trigger" && ref.Table != nil {
		return false
	}
	return programKind(ref.Kind) && ref.Name != "" && len(ref.Name) <= 1024 && len(ref.Schema) <= 1024 && len(ref.Identity) <= 4096 && !strings.ContainsRune(ref.Name+ref.Schema+ref.Identity, 0) && (ref.Table == nil || validTableRef(*ref.Table))
}
func programSchema(ctx context.Context, env *opEnv, schema string) (string, error) {
	switch env.c.Driver {
	case "sqlite":
		if !sqliteSchemaOK(schema) {
			return "", invalidProgram()
		}
		return "main", nil
	case "mysql":
		if schema == "" {
			if err := env.sess.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
				return "", err
			}
		}
		var found string
		if err := env.sess.QueryRowContext(ctx, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE BINARY SCHEMA_NAME=BINARY ?", schema).Scan(&found); err != nil {
			return "", opNotFoundOr(err)
		}
		schema = found
	case "postgres":
		if schema == "" {
			if err := env.sess.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
				return "", err
			}
		}
		var found string
		if err := env.sess.QueryRowContext(ctx, "SELECT nspname FROM pg_namespace WHERE nspname=$1 AND has_schema_privilege(oid,'USAGE')", schema).Scan(&found); err != nil {
			return "", opNotFoundOr(err)
		}
		schema = found
	default:
		return "", invalidProgram()
	}
	if opSystemSchema(env.c.Driver, schema) {
		return "", invalidProgram()
	}
	return schema, nil
}

// Programs returns a bounded list or one exact catalog object. Definitions are
// exportable as text; no routine is invoked and no scheduler setting is changed.
func (e *Engine) Programs(ctx context.Context, r core.ProgramsRequest) (core.ProgramsResult, error) {
	result := core.ProgramsResult{Driver: r.Connection.Driver, Objects: []core.Program{}, EventErrorsState: "unavailable", Warnings: []string{"Eventfehlerprotokolle sind über diese API nicht verfügbar. Abhängigkeiten aus dynamischem SQL sind nicht vollständig erkennbar."}}
	if r.TimeoutMS < 0 || r.TimeoutMS > 60000 || r.Limit < 0 || r.Limit > core.ProgramsMaxObjects || r.Offset < 0 || r.Offset > 100000 || r.Table != nil && !validTableRef(*r.Table) {
		return result, invalidProgram()
	}
	if r.Object != nil {
		if r.Connection.Driver != "postgres" && r.Object.Identity != "" {
			return result, invalidProgram()
		}
		if !validateProgramRef(*r.Object) || r.Kind != "" || r.Schema != "" || r.Table != nil || r.Limit != 0 || r.Offset != 0 {
			return result, invalidProgram()
		}
		r.Kind, r.Schema = r.Object.Kind, r.Object.Schema
	}
	if r.Table != nil && r.Kind != "trigger" {
		return result, invalidProgram()
	}
	if r.Kind != "" {
		if err := programSupported(r.Connection.Driver, r.Kind); err != nil {
			return result, err
		}
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return result, err
	}
	defer closeAll()
	schema, err := programSchema(ctx, env, r.Schema)
	if err != nil {
		return result, opPlanError(ctx, err)
	}
	r.Schema = schema
	if r.Limit == 0 {
		r.Limit = 100
	}
	if r.Object != nil {
		object := *r.Object
		object.Schema = schema
		r.Object = &object
		r.Limit = 1
		r.Offset = 0
	}
	objects, more, err := programCatalog(ctx, env, r)
	if err != nil {
		return result, opPlanError(ctx, err)
	}
	if r.Object != nil && len(objects) == 0 {
		return result, opNotFound()
	}
	result.Objects, result.HasMore = objects, more
	if env.c.Driver == "mysql" {
		var scheduler string
		if err = env.sess.QueryRowContext(ctx, "SELECT @@GLOBAL.event_scheduler").Scan(&scheduler); err != nil {
			return result, safe(ctx, err)
		}
		result.Scheduler = &scheduler
	}
	if err = programBound(result); err != nil {
		return core.ProgramsResult{}, err
	}
	return result, nil
}
func programCatalog(ctx context.Context, env *opEnv, r core.ProgramsRequest) ([]core.Program, bool, error) {
	switch env.c.Driver {
	case "sqlite":
		return sqlitePrograms(ctx, env, r)
	case "mysql":
		return mysqlPrograms(ctx, env, r)
	case "postgres":
		return postgresPrograms(ctx, env, r)
	}
	return nil, false, invalidProgram()
}
func programFind(ctx context.Context, env *opEnv, ref core.ProgramRef) (*core.Program, error) {
	items, _, err := programCatalog(ctx, env, core.ProgramsRequest{Object: &ref, Schema: ref.Schema, Kind: ref.Kind, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, opNotFound()
	}
	return &items[0], nil
}

func (e *Engine) ProgramPlan(ctx context.Context, r core.ProgramChangeRequest) (core.ProgramPlan, error) {
	if err := validateProgramChange(r); err != nil {
		return core.ProgramPlan{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.ProgramPlan{}, err
	}
	defer closeAll()
	plan, err := buildProgramPlan(ctx, env, r)
	return plan, opPlanError(ctx, err)
}
func validateProgramChange(r core.ProgramChangeRequest) error {
	if !validateProgramRef(r.Object) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 || (r.Action != "create" && r.Action != "replace" && r.Action != "drop") {
		return invalidProgram()
	}
	if r.Connection.Driver != "postgres" && r.Object.Identity != "" {
		return invalidProgram()
	}
	if err := programSupported(r.Connection.Driver, r.Object.Kind); err != nil {
		return err
	}
	if r.Action == "drop" {
		if r.Input != nil {
			return invalidProgram()
		}
		return nil
	}
	if r.Input == nil || len(r.Input.Body) > core.ProgramsMaxBodyBytes || !utf8.ValidString(r.Input.Body) || strings.ContainsRune(r.Input.Body, 0) || len(r.Input.Parameters) > 32 {
		return invalidProgram()
	}
	return nil
}
func buildProgramPlan(ctx context.Context, env *opEnv, r core.ProgramChangeRequest) (core.ProgramPlan, error) {
	p := core.ProgramPlan{Action: r.Action, Object: r.Object, UserSQL: r.Action != "drop", Destructive: r.Action != "create", Transactional: env.c.Driver != "mysql", Dependencies: []core.ProgramDependency{}}
	schema, err := programSchema(ctx, env, r.Object.Schema)
	if err != nil {
		return p, err
	}
	r.Object.Schema = schema
	p.Object = r.Object
	if r.Action == "create" && r.Object.Kind == "trigger" {
		if r.Object.Table == nil || r.Object.Table.Schema != "" && r.Object.Table.Schema != schema {
			return p, invalidProgram()
		}
		tableRef := *r.Object.Table
		tableRef.Schema = schema
		desc, err := describe(ctx, env.sess, env.c, tableRef)
		if err != nil {
			return p, err
		}
		if desc.Table.Name != tableRef.Name {
			return p, opNotFound()
		}
		r.Object.Table = &desc.Table
		p.Object = r.Object
	}
	var existing *core.Program
	if r.Action != "create" {
		existing, err = programFind(ctx, env, r.Object)
		if err != nil {
			return p, err
		}
		r.Object = existing.Object
		p.Object = existing.Object
		p.Dependencies = existing.Dependencies
		p.DependenciesTruncated = existing.DependenciesTruncated
	} else if !opName(env.c.Driver, r.Object.Name) || r.Object.Identity != "" {
		return p, invalidProgram()
	}
	warnings := []string{"Keine automatische Wiederholung. Ein bestätigter Plan ist keine einmalige Freigabe; nach Abschluss oder unbekanntem Ausgang ist eine neue Vorschau erforderlich.", "Zwischen Katalogprüfung und Ausführung können andere Sitzungen Objekte ändern. Abhängigkeiten sind auf sichtbare Katalogobjekte begrenzt; dynamisches SQL, String-Körper und externe Verweise sind nicht vollständig erkennbar."}
	if p.UserSQL {
		warnings = append(warnings, "Ohne expliziten Sicherheitskontext gelten die Dialektvorgaben; insbesondere können Sichten und MySQL-Objekte mit Definer-Rechten arbeiten.")
		warnings = append(warnings, "Nutzer-SQL: Der Körper wird mit den Rechten der Verbindung beziehungsweise des Definers gespeichert. Keine Parser- oder Unbedenklichkeitszusage. DELIMITER ist kein SQL und wird nicht unterstützt.")
	}
	if env.c.Driver == "mysql" {
		warnings = append(warnings, "DDL bestätigt implizit. Ersetzen kann aus DROP und CREATE bestehen; bei Teilfehlern ist das Ziel möglicherweise entfernt. Definer und Rechte nach dem Ersetzen prüfen.")
	}
	if r.Object.Kind == "event" {
		warnings = append(warnings, "Events können nach dem Speichern zeitversetzt Code ausführen. Zeitangaben verwenden die Sitzungszeitzone. Der globale Event-Scheduler wird ausschließlich gelesen.")
	}
	var statements []string
	if r.Action == "drop" {
		statement, err := programDrop(env, r.Object)
		if err != nil {
			return p, err
		}
		statements = []string{statement}
	} else {
		statements, err = programCreate(ctx, env, r, existing)
		if err != nil {
			return p, err
		}
		if r.Input.Definer != nil || r.Input.Security == "definer" {
			warnings = append(warnings, "Privilegierte Aktion: expliziter Definer beziehungsweise SQL SECURITY DEFINER kann den Sicherheitskontext wechseln; tatsächliche Datenbankrechte gelten.")
		}
		if existing != nil {
			warnings = append(warnings, "Ersetzen ist destruktiv. Bestehende Eigenschaften, Privilegien und Abhängigkeiten müssen anhand der exakten Anweisungen geprüft werden.")
		}
	}
	if r.Action == "create" {
		candidate := newProgram(p.Object)
		candidate.Body = &r.Input.Body
		if err = programDependencies(ctx, env, &candidate); err != nil {
			return p, err
		}
		p.Dependencies = candidate.Dependencies
		p.DependenciesTruncated = candidate.DependenciesTruncated
		if r.Input.Function != nil && env.c.Driver == "postgres" {
			functionRef := *r.Input.Function
			functionRef.Schema, err = programSchema(ctx, env, functionRef.Schema)
			if err != nil {
				return p, err
			}
			fn, err := programFind(ctx, env, functionRef)
			if err != nil {
				return p, err
			}
			if len(p.Dependencies) < 100 {
				p.Dependencies = append(p.Dependencies, core.ProgramDependency{Object: fn.Object, Direction: "outgoing", Certainty: "exact"})
			} else {
				p.DependenciesTruncated = true
			}
		}
	}
	affected := []string{r.Object.Kind + ":" + schema + "." + r.Object.Name}
	for _, d := range p.Dependencies {
		affected = append(affected, d.Direction+":"+d.Object.Kind+":"+d.Object.Schema+"."+d.Object.Name)
	}
	p.Plan, err = dbplan.NewPlan(env.c.Driver, statements, warnings, affected)
	if err != nil {
		return p, err
	}
	if err = programBound(p); err != nil {
		return p, err
	}
	return p, nil
}
func programDrop(env *opEnv, ref core.ProgramRef) (string, error) {
	q, err := opQualified(env.c.Driver, ref.Schema, ref.Name)
	if err != nil {
		return "", err
	}
	kind := strings.ToUpper(ref.Kind)
	if ref.Kind == "trigger" && env.c.Driver == "postgres" {
		if ref.Table == nil {
			return "", invalidProgram()
		}
		table, err := opQualified(env.c.Driver, ref.Table.Schema, ref.Table.Name)
		if err != nil {
			return "", err
		}
		triggerName, err := dbplan.QuoteIdentifier(env.c.Driver, ref.Name)
		if err != nil {
			return "", err
		}
		return "DROP TRIGGER " + triggerName + " ON " + table, nil
	}
	if env.c.Driver == "postgres" && (ref.Kind == "function" || ref.Kind == "procedure") {
		q += "(" + ref.Identity + ")"
	}
	return "DROP " + kind + " " + q, nil
}

func (e *Engine) ProgramExecute(ctx context.Context, r core.ProgramExecuteRequest) (core.ProgramResult, error) {
	result := core.ProgramResult{Observation: "unavailable"}
	if err := validateProgramChange(r.ProgramChangeRequest); err != nil {
		return result, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return result, err
	}
	defer closeAll()
	plan, err := buildProgramPlan(ctx, env, r.ProgramChangeRequest)
	if err != nil {
		return result, opPlanError(ctx, err)
	}
	err = dbplan.Check(plan.Plan, r.ExpectedPlanSHA256, r.Confirm, r.Action, plan.Object.Name, plan.Destructive)
	if err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) {
			return result, public("plan_mismatch", "Der Plan weicht von der bestätigten Vorschau ab.")
		}
		return result, public("confirmation_mismatch", "Plan-Hash, Aktion oder getippter Objektname sind ungültig.")
	}
	result.StatementsTotal = len(plan.Statements)
	result.Atomic = plan.Transactional
	var tx *sql.Tx
	if plan.Transactional {
		tx, err = env.sess.conn.BeginTx(ctx, nil)
		if err != nil {
			return result, safe(ctx, err)
		}
		defer tx.Rollback()
	}
	for _, statement := range plan.Statements {
		prepareFailed := false
		notSent := ctx.Err() != nil
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if env.c.Driver == "mysql" {
			// MultiStatements=false on the connector. Compound bodies are one server statement.
			_, err = env.sess.conn.ExecContext(ctx, statement)
		} else {
			// Prepare uses the server/SQLite parser to reject trailing statements. It
			// preserves compound trigger bodies and dollar-quoted routine definitions.
			var prepared *sql.Stmt
			prepared, err = tx.PrepareContext(ctx, statement)
			prepareFailed = err != nil
			if err == nil {
				_, err = prepared.ExecContext(ctx)
				prepared.Close()
			}
		}
		if err != nil {
			reason, definite := opReason(err)
			result.Outcome = "outcome_unknown"
			result.Reason = reason
			result.Message = "Ausgang unbekannt oder Teiländerung möglich. Zustand prüfen; nicht automatisch wiederholen."
			if (definite || prepareFailed || notSent) && (plan.Transactional || result.StatementsExecuted == 0) {
				result.Outcome = "rejected"
				result.Message = "Der Server hat die Änderung abgelehnt. Es wurde nichts geändert."
			}
			if tx != nil {
				tx.Rollback()
			}
			return result, nil
		}
		result.StatementsExecuted++
	}
	if tx != nil {
		if err = tx.Commit(); err != nil {
			result.Outcome = "outcome_unknown"
			result.Message = "Commit-Ausgang unbekannt. Nicht automatisch wiederholen."
			return result, nil
		}
	}
	result.Outcome = "completed"
	result.Message = "Die Anweisungen wurden bestätigt abgeschlossen. Eine anschließende Katalogbeobachtung ist separat erforderlich."
	return result, nil
}
