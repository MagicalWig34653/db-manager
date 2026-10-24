// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite "github.com/ncruces/go-sqlite3"
)

func invalidOp(message string) error { return public("invalid_object_op", message) }
func opNotFound() error {
	return public("object_not_found", "Das Zielobjekt ist nicht vorhanden oder für diese Verbindung nicht sichtbar.")
}
func opExists() error {
	return public("object_exists", "Ein Objekt mit dem Zielnamen ist bereits vorhanden.")
}

// opEnv is one pinned connection used for planning and, with a transaction
// where the driver supports transactional DDL, for execution.
type opEnv struct {
	c     core.Connection
	sess  *mutationSession
	maria bool
}

// opState is a built plan together with the data execution needs.
type opState struct {
	action      string
	plan        dbplan.Plan
	view        core.ObjectOpPlan
	target      string // typed name a destructive action must repeat
	destructive bool
	currentDB   bool
	transact    bool // statements may run in one transaction (PG/SQLite table DDL)
	src, dst    core.TableRef
	db          string
	srcRows     bool // observation counts rows (copy/truncate)
	expect      func(*core.ObjectOpObservation) bool
}

type opFieldSet map[string]bool

var opAllowed = map[string]opFieldSet{
	core.OpDatabaseCreate:    {"database": true, "collation": true},
	core.OpDatabaseDrop:      {"database": true},
	core.OpDatabaseCollation: {"database": true, "collation": true},
	core.OpTableRename:       {"table": true, "new_name": true, "new_schema": true},
	core.OpTableCopy:         {"table": true, "new_name": true, "new_schema": true, "mode": true},
	core.OpTableTruncate:     {"table": true, "restart_identity": true},
	core.OpTableDrop:         {"table": true},
	core.OpTableOptions:      {"table": true, "engine": true, "comment": true, "collation": true, "collation_mode": true, "auto_increment": true},
}

func opPresent(r core.ObjectOpRequest) []string {
	var out []string
	add := func(ok bool, name string) {
		if ok {
			out = append(out, name)
		}
	}
	add(r.Database != nil, "database")
	add(r.Table != nil, "table")
	add(r.NewName != nil, "new_name")
	add(r.NewSchema != nil, "new_schema")
	add(r.Mode != nil, "mode")
	add(r.Collation != nil, "collation")
	add(r.CollationMode != nil, "collation_mode")
	add(r.Engine != nil, "engine")
	add(r.Comment != nil, "comment")
	add(r.AutoIncrement != nil, "auto_increment")
	add(r.RestartIdentity != nil, "restart_identity")
	return out
}

// validateOpRequest rejects fields that do not belong to the action and
// missing required fields before any connection is opened.
func validateOpRequest(action string, r core.ObjectOpRequest) error {
	allowed, ok := opAllowed[action]
	if !ok {
		return invalidOp("Unbekannte Aktion.")
	}
	for _, field := range opPresent(r) {
		if !allowed[field] {
			return invalidOp("Das Feld " + field + " gehört nicht zu dieser Aktion.")
		}
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > core.ObjectOpMaxTimeoutMS {
		return invalidOp("Zeitlimit ist ungültig.")
	}
	needDatabase := strings.HasPrefix(action, "database_")
	if needDatabase && r.Database == nil || !needDatabase && r.Table == nil {
		return invalidOp("Das Zielobjekt fehlt.")
	}
	if r.Table != nil && !validTableRef(*r.Table) {
		return invalidOp("Tabellenbezug ist ungültig.")
	}
	if r.Database != nil && (*r.Database == "" || len(*r.Database) > 1024 || strings.ContainsRune(*r.Database, 0)) {
		return invalidOp("Datenbankname ist ungültig.")
	}
	return nil
}

// opName validates a NEW object name the client chooses. It is always quoted
// with the driver quoting; the checks only keep obviously unusable names out.
func opName(driver, name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || name != strings.TrimSpace(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	switch driver {
	case "mysql":
		return utf8.RuneCountInString(name) <= 64
	case "postgres":
		return len(name) <= 63
	}
	return len(name) <= 255 && !strings.HasPrefix(strings.ToLower(name), "sqlite_")
}

func opSystemDatabase(driver, name string) bool {
	lower := strings.ToLower(name)
	if driver == "mysql" {
		return mysqlSystemSchemas[lower]
	}
	return lower == "postgres" || lower == "template0" || lower == "template1"
}

func opSystemSchema(driver, schema string) bool {
	lower := strings.ToLower(schema)
	switch driver {
	case "mysql":
		return mysqlSystemSchemas[lower]
	case "postgres":
		return strings.HasPrefix(lower, "pg_") || lower == "information_schema"
	}
	return false
}

// openOp opens the pinned connection and prepares the session: deterministic
// string literals (statements are shown to the user exactly as executed) and a
// short lock wait so that DDL fails definitely instead of hanging.
func (e *Engine) openOp(ctx context.Context, c core.Connection) (*opEnv, func(), error) {
	db, cleanup, err := e.open(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		cleanup()
		return nil, nil, safe(ctx, err)
	}
	closeAll := func() { conn.Close(); cleanup() }
	env := &opEnv{c: c, sess: &mutationSession{conn: conn, driver: c.Driver}}
	switch c.Driver {
	case "mysql":
		var modes, version string
		if err = conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode,VERSION()").Scan(&modes, &version); err == nil {
			env.maria = strings.Contains(version, "MariaDB")
			_, err = conn.ExecContext(ctx, "SET SESSION sql_mode = ?", modes+",NO_BACKSLASH_ESCAPES,STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION")
		}
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET NAMES utf8mb4")
		}
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET SESSION lock_wait_timeout = 10")
		}
		if err == nil && !env.maria {
			// Fresh statistics so AUTO_INCREMENT and similar values are read live (MySQL only).
			_, err = conn.ExecContext(ctx, "SET SESSION information_schema_stats_expiry = 0")
		}
	case "postgres":
		if _, err = conn.ExecContext(ctx, "SET standard_conforming_strings = on"); err == nil {
			_, err = conn.ExecContext(ctx, "SET lock_timeout = '10s'")
		}
	}
	if err != nil {
		closeAll()
		return nil, nil, safe(ctx, err)
	}
	return env, closeAll, nil
}

func opContext(ctx context.Context, timeoutMS int) (context.Context, context.CancelFunc) {
	if timeoutMS == 0 {
		timeoutMS = core.ObjectOpDefaultTimeoutMS
	}
	return context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
}

func opCapability(driver, action string) error {
	capability := core.ObjectOpCapabilities().Get(driver, action)
	if capability.Support == dbplan.Unsupported {
		return public("unsupported_action", capability.Reason)
	}
	return nil
}

// ObjectOpPlan builds the dry run of one action. Nothing is changed.
func (e *Engine) ObjectOpPlan(ctx context.Context, action string, r core.ObjectOpRequest) (core.ObjectOpPlan, error) {
	if err := validateOpRequest(action, r); err != nil {
		return core.ObjectOpPlan{}, err
	}
	if err := opCapability(r.Connection.Driver, action); err != nil {
		return core.ObjectOpPlan{}, err
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.ObjectOpPlan{}, err
	}
	defer closeAll()
	state, err := buildOp(ctx, env, action, r)
	if err != nil {
		return core.ObjectOpPlan{}, opPlanError(ctx, err)
	}
	if err = validateOverviewSize(state.view); err != nil {
		return core.ObjectOpPlan{}, err
	}
	return state.view, nil
}

// opPlanError keeps public codes and hides raw driver errors.
func opPlanError(ctx context.Context, err error) error {
	var p *core.Error
	if errors.As(err, &p) {
		return p
	}
	return safe(ctx, err)
}

// Dependencies lists the objects that depend on a base table.
func (e *Engine) Dependencies(ctx context.Context, r core.DependenciesRequest) (core.DependenciesResult, error) {
	if !validTableRef(r.Table) || r.TimeoutMS < 0 || r.TimeoutMS > core.ObjectOpMaxTimeoutMS {
		return core.DependenciesResult{}, invalidOp("Tabellenbezug oder Zeitlimit ist ungültig.")
	}
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.DependenciesResult{}, err
	}
	defer closeAll()
	desc, err := opSourceTable(ctx, env, r.Table)
	if err != nil {
		return core.DependenciesResult{}, opPlanError(ctx, err)
	}
	deps, truncated, err := tableDependencies(ctx, env, desc)
	if err != nil {
		return core.DependenciesResult{}, safe(ctx, err)
	}
	// The effects of the two structural actions are reported per dependency
	// (rename first, drop second is encoded in the notes; the plan routes
	// carry the effect for the concrete action).
	result := core.DependenciesResult{Driver: r.Connection.Driver, Table: core.ObjectOpTarget{Kind: "table", Schema: desc.Table.Schema, Name: desc.Table.Name}, Dependencies: opEffects(core.OpTableDrop, r.Connection.Driver, deps), Truncated: truncated, Notes: []string{"effect gilt für das Löschen der Tabelle; Pläne nennen die Wirkung für die jeweilige Aktion."}}
	if r.Connection.Driver != "postgres" {
		result.Notes = append(result.Notes, "Sichten werden anhand ihres Quelltextes erkannt (certainty=textual); Trigger werden nicht ausgewertet.")
	}
	return result, nil
}

// ObjectOpExecute recomputes the plan, checks confirmation and digest and only
// then runs the statements. Failures after a started change never retry.
func (e *Engine) ObjectOpExecute(ctx context.Context, action string, r core.ObjectOpExecuteRequest) (core.ObjectOpResult, error) {
	empty := core.ObjectOpResult{}
	if err := validateOpRequest(action, r.ObjectOpRequest); err != nil {
		return empty, err
	}
	if err := opCapability(r.Connection.Driver, action); err != nil {
		return empty, err
	}
	parent := ctx
	ctx, cancel := opContext(ctx, r.TimeoutMS)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeAll()
	state, err := buildOp(ctx, env, action, r.ObjectOpRequest)
	if err != nil {
		return empty, opPlanError(ctx, err)
	}
	switch err = dbplan.Check(state.plan, r.ExpectedPlanSHA256, dbplan.Confirmation{Action: r.Confirm.Action, TargetName: r.Confirm.TargetName}, action, state.target, state.destructive); {
	case err == nil:
	case errors.Is(err, dbplan.ErrInvalidDigest):
		return empty, invalidOp("expected_plan_sha256 ist kein gültiger SHA-256-Hexwert.")
	case errors.Is(err, dbplan.ErrPlanMismatch):
		return empty, public("plan_mismatch", "Der Plan hat sich seit der Vorschau geändert. Plan neu laden und erneut bestätigen.")
	default:
		return empty, public("confirmation_mismatch", "Aktion oder getippter Zielname stimmen nicht mit dem Plan überein.")
	}
	if state.currentDB && !r.Confirm.AcknowledgeCurrentDatabase {
		return empty, public("confirmation_mismatch", "Die Verbindungsdatenbank erfordert die zusätzliche ausdrückliche Bestätigung.")
	}
	return executeOp(ctx, parent, env, state), nil
}

// opReason maps driver errors to a small public set. definite reports that the
// server answered with an error, so the failed statement did not take effect.
func opReason(err error) (reason string, definite bool) {
	var my *mysql.MySQLError
	var pg *pgconn.PgError
	var lite *sqlite.Error
	switch {
	case errors.As(err, &my):
		switch my.Number {
		case 1044, 1045, 1142, 1143, 1227, 1370, 1410:
			return "permission_denied", true
		case 1007, 1050, 1304:
			return "object_exists", true
		case 1008, 1051, 1049, 1146, 1109:
			return "object_not_found", true
		case 1217, 1451, 1701, 3730:
			return "dependency_blocks", true
		case 1205, 1213:
			return "lock_timeout", true
		case 1273, 1286, 1253, 1115, 1067, 1064:
			return "invalid_value", true
		}
		return "database_error", true
	case errors.As(err, &pg):
		switch pg.Code {
		case "42501":
			return "permission_denied", true
		case "42P07", "42P04", "42710":
			return "object_exists", true
		case "42P01", "3D000", "42704":
			return "object_not_found", true
		case "2BP01", "23503":
			return "dependency_blocks", true
		case "55006":
			return "in_use", true
		case "55P03":
			return "lock_timeout", true
		case "22023", "22P02", "42602", "42601":
			return "invalid_value", true
		}
		return "database_error", true
	case errors.As(err, &lite):
		switch lite.Code() {
		case sqlite.CONSTRAINT:
			return "dependency_blocks", true
		case sqlite.PERM, sqlite.READONLY, sqlite.AUTH:
			return "permission_denied", true
		}
		return "database_error", true
	}
	return "", false
}

var reasonText = map[string]string{
	"permission_denied": "Der Datenbankbenutzer hat dafür kein Recht.",
	"object_exists":     "Ein Objekt mit dem Zielnamen ist bereits vorhanden.",
	"object_not_found":  "Ein beteiligtes Objekt ist nicht mehr vorhanden.",
	"dependency_blocks": "Abhängige Objekte (zum Beispiel Fremdschlüssel) verhindern die Aktion.",
	"lock_timeout":      "Das Objekt ist durch eine andere Sitzung gesperrt.",
	"in_use":            "Das Objekt wird von anderen Sitzungen verwendet.",
	"invalid_value":     "Der Server hat einen Wert abgelehnt.",
	"database_error":    "Der Server hat die Anweisung abgelehnt.",
}

func executeOp(ctx, parent context.Context, env *opEnv, st *opState) core.ObjectOpResult {
	sess := env.sess
	total := len(st.plan.Statements)
	result := core.ObjectOpResult{Action: st.action, StatementsTotal: total}
	transactional := st.transact && env.c.Driver != "mysql"
	finishFail := func(index int, err error) core.ObjectOpResult {
		reason, definite := opReason(err)
		switch {
		case definite && (transactional || index == 0):
			if transactional {
				sess.rollback()
			}
			result.Outcome, result.Reason = string(dbplan.StatusRejected), reason
			result.Message = reasonText[reason] + " Es wurde nichts geändert."
		case definite:
			result.Outcome, result.Reason = string(dbplan.StatusOutcomeUnknown), reason
			result.Message = fmt.Sprintf("%s Zuvor ausgeführte Anweisungen (%d von %d) sind nicht rückgängig gemacht. Zustand prüfen; nicht automatisch wiederholen.", reasonText[reason], index, total)
		default:
			if transactional {
				sess.rollback()
			}
			result.Outcome = string(dbplan.StatusOutcomeUnknown)
			result.Message = "Das Ergebnis ist unbekannt (Verbindung oder Zeitlimit). Zustand prüfen; nicht automatisch wiederholen."
		}
		result.StatementsExecuted = index
		return result
	}
	if transactional {
		if err := sess.begin(ctx); err != nil {
			result.Outcome = string(dbplan.StatusRejected)
			result.Message = "Transaktion konnte nicht gestartet werden. Es wurde nichts geändert."
			return result
		}
		defer sess.rollback()
	}
	for index, statement := range st.plan.Statements {
		if err := ctx.Err(); err != nil {
			if index == 0 {
				// Nothing was sent yet, so nothing can have changed.
				result.Outcome, result.Reason = string(dbplan.StatusRejected), "canceled"
				result.Message = "Abgebrochen, bevor etwas ausgeführt wurde. Es wurde nichts geändert."
				return result
			}
			return finishFail(index, err)
		}
		if _, err := sess.exec(ctx, statement); err != nil {
			return finishFail(index, err)
		}
	}
	result.StatementsExecuted = total
	state := "observed"
	if transactional {
		state = "transactional"
	}
	observation := observeOp(ctx, parent, env, st, state)
	result.Observation = observation
	if transactional {
		if observation.State == "unavailable" || !observationMatches(st, observation) {
			sess.rollback()
			result.Outcome, result.Reason = string(dbplan.StatusRejected), "observation_mismatch"
			result.Message = "Die Aktion wurde vor dem Commit abgebrochen: das Ergebnis konnte in derselben Transaktion nicht bestätigt werden. Es wurde nichts geändert."
			result.StatementsExecuted = 0
			result.Observation = nil
			return result
		}
		if err := sess.commit(ctx); err != nil {
			result.Outcome = string(dbplan.StatusOutcomeUnknown)
			result.Message = "Das Commit-Ergebnis ist unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			result.Observation = nil
			return result
		}
		result.Atomic = true
		result.Outcome = string(dbplan.StatusCompleted)
		result.Message = "Ausgeführt und in derselben Transaktion bestätigt."
		return result
	}
	result.Outcome = string(dbplan.StatusCompleted)
	result.Message = "Ausgeführt; das Ergebnis wurde getrennt beobachtet (kein atomarer Snapshot)."
	if env.c.Driver == "mysql" {
		result.Message = "Ausgeführt (DDL mit implizitem Commit); das Ergebnis wurde getrennt beobachtet (kein atomarer Snapshot)."
	}
	if observation.State == "unavailable" {
		result.Message = "Ausgeführt; die Beobachtung des Ergebnisses ist nicht verfügbar."
	}
	return result
}
