// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// Database copy and rename (OBJ-004, API 0.26).
//
// The plan is built from the catalog on a pinned connection and contains the
// exact statements. The execute request repeats the input; the plan is rebuilt
// and compared with the confirmed digest before anything is written. Confirmed
// plans are single use (planLedger, key "dbcopy:<digest>").

// dcAutoIncrement is replaced at execution time by the AUTO_INCREMENT counter
// of the source table (contract: the only placeholder of a statement list).
const dcAutoIncrement = "{source_auto_increment}"

const dcDigestVersion = "dbcopy/v1"

// Hooks lets tests inject faults and concurrent writes between phases and steps.
// It is nil in production.
type dbCopyHooks struct {
	beforeStep func(phase, kind, name string)
	beforeLock func()
}

type dcPhase struct {
	id, label   string
	destructive bool
}

// dcStep is a group of statements that belong to one object. A failure of a
// step is reported for its object.
type dcStep struct {
	phase, kind, name string
	stmts             []string
	tolerate          bool // a definite failure does not stop the phase
	retry             bool // views: retry later when a dependency does not exist yet
	aiSchema, aiTable string
	restore           bool // definition step: restore the session context afterwards
	rows              int64
}

// dcTable is a base table that takes part in the verification.
type dcTable struct {
	name string
	rows int64
}

// dcState is a built plan together with the data execution needs.
type dcState struct {
	action      string
	scope       string
	driver      string
	source      string
	target      string
	withData    bool
	resume      bool
	rename      bool
	steps       []dcStep
	phases      []dcPhase
	view        core.DBCopyPlan
	destructive bool
	typed       string // typed name a destructive action must repeat
	currentDB   bool
	oneTx       bool // all steps run in one transaction (PostgreSQL schema copy)
	tables      []string
	expect      map[string][]string // kind -> names that must exist in the target afterwards
	srcObjects  map[string][]string // kind -> names that exist in the source
	enableEvt   []string
	skipped     int
	restoreSQL  []string
	viewMode    string
}

func invalidCopy(message string) error { return public("invalid_object_op", message) }

func dcLimit(message string) error { return public("limit_exceeded", message) }

func dcBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// validateDBCopyRequest rejects unusable input before any connection is opened.
func validateDBCopyRequest(action string, r core.DBCopyRequest, execute bool) (scope string, err error) {
	if action != core.OpDatabaseCopy && action != core.OpDatabaseRename {
		return "", invalidCopy("Unbekannte Aktion.")
	}
	driver := r.Connection.Driver
	if capability := core.DBCopyCapabilities().Get(driver, action); capability.Support == dbplan.Unsupported {
		return "", public("unsupported_action", capability.Reason)
	}
	if r.Source == nil || r.NewName == nil {
		return "", invalidCopy("Quell- und Zielname sind erforderlich.")
	}
	scopes := core.DBCopyScopes()[driver]
	scope = scopes[0]
	if r.Scope != nil {
		found := false
		for _, s := range scopes {
			found = found || s == *r.Scope
		}
		if !found {
			return "", invalidCopy("Der Geltungsbereich (scope) ist für diesen Treiber ungültig.")
		}
		scope = *r.Scope
	}
	if action == core.OpDatabaseRename && (r.Content != nil || r.Include != nil) {
		return "", invalidCopy("Umbenennen kopiert immer vollständig; content und include gehören nicht zu dieser Aktion.")
	}
	if r.Content != nil && *r.Content != "structure" && *r.Content != "structure_and_data" {
		return "", invalidCopy("content muss structure oder structure_and_data sein.")
	}
	if r.Resume != nil && *r.Resume && !(driver == "mysql") {
		return "", invalidCopy("Fortsetzen gibt es nur für MySQL und MariaDB; PostgreSQL arbeitet in einer Transaktion oder mit einer einzelnen Anweisung.")
	}
	maxTimeout := core.DBCopyMaxPlanTimeoutMS
	if execute {
		maxTimeout = core.DBCopyMaxTimeoutMS
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > maxTimeout {
		return "", invalidCopy("Zeitlimit ist ungültig.")
	}
	if !dcName(driver, scope, *r.Source, true) {
		return "", invalidCopy("Quellname ist ungültig.")
	}
	if !dcName(driver, scope, *r.NewName, false) {
		return "", invalidCopy("Zielname ist ungültig oder reserviert.")
	}
	return scope, nil
}

// dcName validates an object name chosen by the client. Names are always
// quoted by the engine; the rules only keep unusable and reserved names out.
func dcName(driver, scope, name string, source bool) bool {
	if name == "" || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || name != strings.TrimSpace(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '�' {
			return false
		}
	}
	lower := strings.ToLower(name)
	switch driver {
	case "mysql":
		if utf8.RuneCountInString(name) > 64 || strings.ContainsAny(name, "/\\.") {
			return false
		}
		return !mysqlSystemSchemas[lower]
	case "postgres":
		if len(name) > 63 {
			return false
		}
		if scope == "schema" {
			return !strings.HasPrefix(lower, "pg_") && lower != "information_schema"
		}
		return lower != "postgres" && lower != "template0" && lower != "template1"
	}
	return false
}

func dcContext(ctx context.Context, timeoutMS int, execute bool) (context.Context, context.CancelFunc) {
	if timeoutMS == 0 {
		timeoutMS = core.DBCopyDefaultPlanTimeoutMS
		if execute {
			timeoutMS = core.DBCopyDefaultTimeoutMS
		}
	}
	return context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
}

func dcTimeoutMS(r core.DBCopyRequest, execute bool) int {
	if r.TimeoutMS != 0 {
		return r.TimeoutMS
	}
	if execute {
		return core.DBCopyDefaultTimeoutMS
	}
	return core.DBCopyDefaultPlanTimeoutMS
}

// DatabaseCopyPlan builds the dry run of a database copy or rename. Nothing is changed.
func (e *Engine) DatabaseCopyPlan(ctx context.Context, action string, r core.DBCopyRequest) (core.DBCopyPlan, error) {
	scope, err := validateDBCopyRequest(action, r, false)
	if err != nil {
		return core.DBCopyPlan{}, err
	}
	ctx, cancel := dcContext(ctx, r.TimeoutMS, false)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return core.DBCopyPlan{}, err
	}
	defer closeAll()
	st, err := buildDBCopy(ctx, env, action, scope, r)
	if err != nil {
		return core.DBCopyPlan{}, opPlanError(ctx, err)
	}
	if core.IsPlanPreflight(ctx) && e.plans.isConsumed("dbcopy:"+st.view.PlanSHA256) {
		return core.DBCopyPlan{}, public("plan_mismatch", "Der Plan ist verbraucht (ein Ausführungsversuch hat begonnen). Neue Vorschau erforderlich.")
	}
	if !core.IsPlanPreflight(ctx) {
		e.plans.arm("dbcopy:" + st.view.PlanSHA256)
	}
	return st.view, nil
}

func buildDBCopy(ctx context.Context, env *opEnv, action, scope string, r core.DBCopyRequest) (*dcState, error) {
	var st *dcState
	var err error
	switch env.c.Driver {
	case "mysql":
		st, err = buildMySQLCopy(ctx, env, action, r)
	case "postgres":
		st, err = buildPostgresCopy(ctx, env, action, scope, r)
	default:
		return nil, public("unsupported_action", core.DBCopyCapabilities().Get(env.c.Driver, action).Reason)
	}
	if err != nil {
		return nil, err
	}
	if err = dcFinish(st, r); err != nil {
		return nil, err
	}
	return st, nil
}

// dcFinish assembles the public plan, the digest and the confirmation.
func dcFinish(st *dcState, r core.DBCopyRequest) error {
	var statements []string
	size := 0
	phases := make([]core.DBCopyPhasePlan, 0, len(st.phases))
	for _, phase := range st.phases {
		p := core.DBCopyPhasePlan{ID: phase.id, Label: phase.label, Destructive: phase.destructive}
		for _, step := range st.steps {
			if step.phase != phase.id {
				continue
			}
			p.Objects++
			p.Statements += len(step.stmts)
			statements = append(statements, step.stmts...)
			for _, s := range step.stmts {
				size += len(s)
			}
		}
		phases = append(phases, p)
	}
	if size > core.DBCopyMaxPlanBytes {
		return public("result_too_large", "Der Plan überschreitet die zulässige Größe (Definitionen von Sichten, Routinen und Triggern). Die Datenbank ist für einen Plan zu groß.")
	}
	if len(statements) == 0 {
		// A copy of an empty database still creates it; resume with nothing left to do has no statements.
		return public("nothing_to_do", "Der Plan enthält keine Anweisung: Es ist nichts zu kopieren oder fortzusetzen.")
	}
	st.view.Statements = statements
	st.view.Phases = phases
	st.view.Executable = len(st.view.Blockers) == 0
	st.view.Atomic = st.oneTx
	if st.view.Warnings == nil {
		st.view.Warnings = []string{}
	}
	if st.view.Blockers == nil {
		st.view.Blockers = []string{}
	}
	if st.view.Dependencies == nil {
		st.view.Dependencies = []core.ObjectDependency{}
	}
	if st.view.Objects == nil {
		st.view.Objects = []core.DBCopyObject{}
	}
	if len(st.view.Objects) > core.DBCopyMaxListed {
		st.view.Objects = st.view.Objects[:core.DBCopyMaxListed]
		st.view.ObjectsTruncated = true
	}
	st.view.Estimate.TimeoutMS = dcTimeoutMS(r, true)
	st.view.Estimate.TimeRisk = dcTimeRisk(st.view.Estimate)
	if st.view.Estimate.TimeRisk == "high" {
		st.view.Warnings = append(st.view.Warnings, fmt.Sprintf("Die geschätzte Datenmenge ist im Verhältnis zum Zeitlimit von %d Sekunden groß; bei Überschreitung wird abgebrochen und ein Teilzustand gemeldet. Ein größeres timeout_ms (höchstens %d) wählen.", st.view.Estimate.TimeoutMS/1000, core.DBCopyMaxTimeoutMS))
	}
	st.view.Confirmation = core.ObjectOpConfirmation{Action: st.action, Destructive: st.destructive, TargetName: st.typed, CurrentDatabase: st.currentDB}
	sum, err := dcDigest(st, r, statements)
	if err != nil {
		return err
	}
	st.view.PlanSHA256 = sum
	return nil
}

// dcDigest binds the mode (names, scope, content, options, resume) and the exact statements.
func dcDigest(st *dcState, r core.DBCopyRequest, statements []string) (string, error) {
	header, err := json.Marshal([]any{dcDigestVersion, st.action, st.driver, st.scope, st.source, st.target, st.view.Content, st.resume,
		dcBool(includeField(r, "views"), true), dcBool(includeField(r, "routines"), true), dcBool(includeField(r, "triggers"), true), dcBool(includeField(r, "events"), true)})
	if err != nil {
		return "", err
	}
	all := append([]string{"-- " + string(header)}, statements...)
	return dbplan.Digest(st.driver, all)
}

func includeField(r core.DBCopyRequest, kind string) *bool {
	if r.Include == nil {
		return nil
	}
	switch kind {
	case "views":
		return r.Include.Views
	case "routines":
		return r.Include.Routines
	case "triggers":
		return r.Include.Triggers
	case "events":
		return r.Include.Events
	}
	return nil
}

// dcTimeRisk is a heuristic: bytes of data and indexes against an assumed
// throughput of 20 MiB/s. It is documented as an estimate, never a promise.
func dcTimeRisk(e core.DBCopyEstimate) string {
	if e.DataBytes == nil {
		return "unknown"
	}
	total := *e.DataBytes
	if e.IndexBytes != nil {
		total += *e.IndexBytes
	}
	seconds := float64(total) / float64(20<<20)
	limit := float64(e.TimeoutMS) / 1000
	switch {
	case limit <= 0:
		return "unknown"
	case seconds >= limit*0.8:
		return "high"
	case seconds >= limit*0.3:
		return "medium"
	}
	return "low"
}

// DatabaseCopyExecute recomputes the plan, checks confirmation and digest and
// only then runs the statements. Failures after a started change never retry.
func (e *Engine) DatabaseCopyExecute(ctx context.Context, action string, r core.DBCopyExecuteRequest, progress func(core.DBCopyProgress)) (core.DBCopyResult, error) {
	empty := core.DBCopyResult{}
	scope, err := validateDBCopyRequest(action, r.DBCopyRequest, true)
	if err != nil {
		return empty, err
	}
	parent := ctx
	ctx, cancel := dcContext(ctx, r.TimeoutMS, true)
	defer cancel()
	env, closeAll, err := e.openOp(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeAll()
	st, err := buildDBCopy(ctx, env, action, scope, r.DBCopyRequest)
	if err != nil {
		return empty, opPlanError(ctx, err)
	}
	switch err = dbplan.Check(dbplan.Plan{SHA256: st.view.PlanSHA256}, r.ExpectedPlanSHA256, dbplan.Confirmation{Action: r.Confirm.Action, TargetName: r.Confirm.TargetName}, action, st.typed, st.destructive); {
	case err == nil:
	case errors.Is(err, dbplan.ErrInvalidDigest):
		return empty, invalidCopy("expected_plan_sha256 ist kein gültiger SHA-256-Hexwert.")
	case errors.Is(err, dbplan.ErrPlanMismatch):
		return empty, public("plan_mismatch", "Der Plan hat sich seit der Vorschau geändert. Plan neu laden und erneut bestätigen.")
	default:
		return empty, public("confirmation_mismatch", "Aktion oder getippter Name stimmen nicht mit dem Plan überein.")
	}
	if st.currentDB && !r.Confirm.AcknowledgeCurrentDatabase {
		return empty, public("confirmation_mismatch", "Die Verbindungsdatenbank erfordert die zusätzliche ausdrückliche Bestätigung.")
	}
	if len(st.view.Blockers) > 0 {
		result := dcNewResult(st)
		result.Outcome, result.Reason = string(dbplan.StatusRejected), "blocked"
		result.Message = "Der Plan ist blockiert: " + st.view.Blockers[0] + " Es wurde nichts geändert."
		result.NextStep = "Blockade beheben und eine neue Vorschau laden."
		return result, nil
	}
	// Single use: consumed before the first write, also when the outcome stays unknown.
	if !e.plans.consume("dbcopy:" + st.view.PlanSHA256) {
		return empty, public("plan_mismatch", "Der Plan ist verbraucht (ein Ausführungsversuch hat begonnen; auch ein fehlgeschlagener verbraucht ihn). Neue Vorschau erforderlich.")
	}
	run := &dcRun{e: e, env: env, st: st, parent: parent, progress: progress}
	if progress == nil {
		run.progress = func(core.DBCopyProgress) {}
	}
	if env.c.Driver == "postgres" {
		return run.postgres(ctx), nil
	}
	return run.mysql(ctx), nil
}

// dcNewResult prepares a result with the phase list of the plan.
func dcNewResult(st *dcState) core.DBCopyResult {
	res := core.DBCopyResult{Action: st.action, SourceState: "intact", TargetState: "absent", Phases: []core.DBCopyPhaseResult{}, Failures: []core.DBCopyFailure{}, Atomic: st.oneTx}
	res.Verification = core.DBCopyVerification{State: "not_run", Mismatches: []core.DBCopyMismatch{}}
	if st.resume {
		res.TargetState = "partial"
	}
	for _, phase := range st.phases {
		total := 0
		for _, step := range st.steps {
			if step.phase == phase.id {
				total++
			}
		}
		res.Phases = append(res.Phases, core.DBCopyPhaseResult{ID: phase.id, Status: "not_started", ObjectsTotal: total})
	}
	for _, step := range st.steps {
		res.StatementsTotal += len(step.stmts)
	}
	return res
}

// dcRun is the state of one execute request.
type dcRun struct {
	e        *Engine
	env      *opEnv
	st       *dcState
	parent   context.Context
	progress func(core.DBCopyProgress)
	res      core.DBCopyResult
	executed int
	wrote    bool
}

func (d *dcRun) phase(id string) *core.DBCopyPhaseResult {
	for i := range d.res.Phases {
		if d.res.Phases[i].ID == id {
			return &d.res.Phases[i]
		}
	}
	return nil
}

func (d *dcRun) report(phase string) {
	p := d.phase(phase)
	if p == nil {
		return
	}
	d.progress(core.DBCopyProgress{Phase: phase, ObjectsDone: p.ObjectsDone, ObjectsTotal: p.ObjectsTotal, StatementsExecuted: d.executed, StatementsTotal: d.res.StatementsTotal})
}

func (d *dcRun) fail(phase string, step dcStep, reason string) {
	if len(d.res.Failures) >= core.DBCopyMaxFailures {
		d.res.FailuresTruncated = true
		return
	}
	d.res.Failures = append(d.res.Failures, core.DBCopyFailure{Phase: phase, Kind: step.kind, Name: step.name, Reason: reason, Message: dcReasonText(reason)})
}

var dcReasonTexts = map[string]string{
	"canceled":           "Abgebrochen.",
	"timeout":            "Das Zeitlimit wurde erreicht.",
	"connection_error":   "Die Verbindung wurde unterbrochen; der Zustand dieses Objekts ist unbekannt.",
	"verification_failed": "Die Verifikation der Kopie ist fehlgeschlagen.",
	"source_changed":     "Die Quelle hat sich während der Kopie geändert.",
	"blocked":            "Der Plan ist blockiert.",
}

func dcReasonText(reason string) string {
	if t, ok := reasonText[reason]; ok {
		return t
	}
	if t, ok := dcReasonTexts[reason]; ok {
		return t
	}
	return "Das Objekt konnte nicht verarbeitet werden."
}

// dcClassify returns the public reason of a failed statement and whether the
// server definitely rejected it (so it had no effect).
func dcClassify(ctx context.Context, err error) (reason string, definite bool) {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "timeout", false
		}
		return "canceled", false
	}
	if r, ok := opReason(err); ok {
		return r, true
	}
	return "connection_error", false
}

