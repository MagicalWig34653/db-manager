// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// DBCopyContractVersion is the API minor of the database copy and rename routes (OBJ-004).
const DBCopyContractVersion = "0.26.0"

// Limits of the database copy routes. They are part of the public contract.
const (
	DBCopyDefaultPlanTimeoutMS = 30000
	DBCopyMaxPlanTimeoutMS     = 60000
	DBCopyDefaultTimeoutMS     = 600000
	DBCopyMaxTimeoutMS         = 3600000
	DBCopyMaxObjects           = 2000
	DBCopyMaxTables            = 1000
	DBCopyMaxPlanBytes         = 4 << 20
	DBCopyMaxFailures          = 100
	DBCopyMaxListed            = 2000
	DBCopyMaxDependencies      = 100
)

// Action names. They are the route names, the capability keys and the value of confirm.action.
const (
	OpDatabaseCopy   = "database_copy"
	OpDatabaseRename = "database_rename"
)

// DBCopyActions lists the actions in contract order.
var DBCopyActions = []string{OpDatabaseCopy, OpDatabaseRename}

// DBCopyInclude selects the object kinds that are copied besides tables.
// Absent values default to true. Tables are always part of a copy.
type DBCopyInclude struct {
	Views    *bool `json:"views,omitempty"`
	Routines *bool `json:"routines,omitempty"`
	Triggers *bool `json:"triggers,omitempty"`
	Events   *bool `json:"events,omitempty"`
}

// DBCopyRequest carries the structured input of every plan and execute request.
// Names are data: they are checked, quoted by the engine and never executed as SQL.
type DBCopyRequest struct {
	Connection Connection     `json:"connection"`
	Source     *string        `json:"source,omitempty"`
	NewName    *string        `json:"new_name,omitempty"`
	Scope      *string        `json:"scope,omitempty"`
	Content    *string        `json:"content,omitempty"`
	Include    *DBCopyInclude `json:"include,omitempty"`
	Resume     *bool          `json:"resume,omitempty"`
	TimeoutMS  int            `json:"timeout_ms,omitempty"`
}

// DBCopyExecuteRequest repeats the plan input and adds confirmation and the plan digest.
type DBCopyExecuteRequest struct {
	DBCopyRequest
	Confirm            ObjectOpConfirm `json:"confirm"`
	ExpectedPlanSHA256 string          `json:"expected_plan_sha256"`
}

// DBCopyPhasePlan describes one phase of the plan.
type DBCopyPhasePlan struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Objects     int    `json:"objects"`
	Statements  int    `json:"statements"`
	Destructive bool   `json:"destructive"`
}

// DBCopyObject is one source object and what the plan does with it. Action is
// create, fill, skip_complete (resume), skip_option, skip_unsupported or unsupported.
type DBCopyObject struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Action        string `json:"action"`
	RowsEstimate  *int64 `json:"rows_estimate"`
	BytesEstimate *int64 `json:"bytes_estimate"`
	Note          string `json:"note,omitempty"`
}

// DBCopyEstimate is the size estimate from catalog statistics (never exact).
type DBCopyEstimate struct {
	Tables     int    `json:"tables"`
	Views      int    `json:"views"`
	Routines   int    `json:"routines"`
	Triggers   int    `json:"triggers"`
	Events     int    `json:"events"`
	Rows       *int64 `json:"rows"`
	DataBytes  *int64 `json:"data_bytes"`
	IndexBytes *int64 `json:"index_bytes"`
	Exact      bool   `json:"exact"`
	TimeoutMS  int    `json:"timeout_ms"`
	TimeRisk   string `json:"time_risk"`
}

// DBCopyPlan is the dry run: exact statements, phases, objects, warnings,
// blockers and the digest the execute request must repeat.
type DBCopyPlan struct {
	Action                string               `json:"action"`
	Driver                string               `json:"driver"`
	Scope                 string               `json:"scope"`
	Source                string               `json:"source"`
	Target                string               `json:"target"`
	Content               string               `json:"content"`
	Resume                bool                 `json:"resume"`
	Statements            []string             `json:"statements"`
	Warnings              []string             `json:"warnings"`
	AffectedObjects       []string             `json:"affected_objects"`
	Phases                []DBCopyPhasePlan    `json:"phases"`
	Objects               []DBCopyObject       `json:"objects"`
	ObjectsTruncated      bool                 `json:"objects_truncated"`
	Estimate              DBCopyEstimate       `json:"estimate"`
	Dependencies          []ObjectDependency   `json:"dependencies"`
	DependenciesTruncated bool                 `json:"dependencies_truncated"`
	Blockers              []string             `json:"blockers"`
	Executable            bool                 `json:"executable"`
	Atomic                bool                 `json:"atomic"`
	Confirmation          ObjectOpConfirmation `json:"confirmation"`
	PlanSHA256            string               `json:"plan_sha256"`
}

// DBCopyPhaseResult is the outcome of one phase. Status is done, failed,
// skipped, not_started or unknown.
type DBCopyPhaseResult struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ObjectsTotal int    `json:"objects_total"`
	ObjectsDone  int    `json:"objects_done"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
}

// DBCopyFailure is one object that could not be processed.
type DBCopyFailure struct {
	Phase   string `json:"phase"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// DBCopyMismatch is one verification difference.
type DBCopyMismatch struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// DBCopyVerification is the read-back check. State is passed, failed,
// unavailable or not_run.
type DBCopyVerification struct {
	State              string           `json:"state"`
	TablesCompared     int              `json:"tables_compared"`
	Checksums          bool             `json:"checksums"`
	Mismatches         []DBCopyMismatch `json:"mismatches"`
	MismatchesTruncated bool            `json:"mismatches_truncated"`
}

// DBCopyResult is the answer of an execute request. Outcome is completed,
// rejected (nothing changed), partial (some changes made, the state is
// reported per phase) or outcome_unknown (never retried automatically).
// SourceState is intact, partially_removed, removed or unknown; TargetState is
// absent, partial, complete, renamed or unknown.
type DBCopyResult struct {
	Action             string             `json:"action"`
	Outcome            string             `json:"outcome"`
	Reason             string             `json:"reason,omitempty"`
	Message            string             `json:"message"`
	SourceState        string             `json:"source_state"`
	TargetState        string             `json:"target_state"`
	Phases             []DBCopyPhaseResult `json:"phases"`
	Failures           []DBCopyFailure    `json:"failures"`
	FailuresTruncated  bool               `json:"failures_truncated"`
	Verification       DBCopyVerification `json:"verification"`
	StatementsTotal    int                `json:"statements_total"`
	StatementsExecuted int                `json:"statements_executed"`
	Atomic             bool               `json:"atomic"`
	CanResume          bool               `json:"can_resume"`
	NextStep           string             `json:"next_step"`
}

// DBCopyProgress is reported while an execute request runs.
type DBCopyProgress struct {
	Phase              string `json:"phase"`
	ObjectsDone        int    `json:"objects_done"`
	ObjectsTotal       int    `json:"objects_total"`
	StatementsExecuted int    `json:"statements_executed"`
	StatementsTotal    int    `json:"statements_total"`
}

// DBCopyEngine is the optional database copy and rename capability of an engine.
type DBCopyEngine interface {
	DatabaseCopyPlan(ctx context.Context, action string, r DBCopyRequest) (DBCopyPlan, error)
	DatabaseCopyExecute(ctx context.Context, action string, r DBCopyExecuteRequest, progress func(DBCopyProgress)) (DBCopyResult, error)
}

// DBCopyCapabilities is the static per-driver capability table. MariaDB is
// driver "mysql". Unsupported and limited entries always carry a reason.
func DBCopyCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	managed := "SQLite: eine Datei ist eine Datenbank. Datenbanken sind verwaltete Kennungen der Verbindung und werden nicht über SQL kopiert oder umbenannt."
	c.Set("sqlite", OpDatabaseCopy, dbplan.Capability{Support: dbplan.Unsupported, Reason: managed})
	c.Set("sqlite", OpDatabaseRename, dbplan.Capability{Support: dbplan.Unsupported, Reason: managed})
	c.Set("mysql", OpDatabaseCopy, dbplan.Capability{Support: dbplan.Limited, Reason: "Neue Datenbank, Tabellen mit Daten, Fremdschlüssel, Sichten, Routinen, Trigger und Ereignisse; nicht atomar (DDL mit implizitem Commit), Teilzustände werden je Phase gemeldet. Rechte werden nicht kopiert."})
	c.Set("mysql", OpDatabaseRename, dbplan.Capability{Support: dbplan.Limited, Reason: "MySQL und MariaDB kennen kein RENAME DATABASE: Kopieren, Prüfen, danach die Quelle löschen. Die Quelle wird nur nach vollständig verifizierter Kopie und getipptem Quellnamen gelöscht."})
	c.Set("postgres", OpDatabaseCopy, dbplan.Capability{Support: dbplan.Limited, Reason: "Schema: in einer Transaktion (alles oder nichts), nur ohne benutzerdefinierte Typen, Partitionen, Vererbung und ähnliche Objekte. Datenbank: CREATE DATABASE TEMPLATE, scheitert bei offenen Sitzungen auf der Quelle."})
	c.Set("postgres", OpDatabaseRename, dbplan.Capability{Support: dbplan.Limited, Reason: "Schema: ALTER SCHEMA RENAME (transaktional). Datenbank: ALTER DATABASE RENAME scheitert, solange Sitzungen mit ihr verbunden sind; diese Verbindung darf nicht auf der umzubenennenden Datenbank liegen."})
	return c
}

// DBCopyScopes lists the scopes each driver accepts (first entry is the default).
func DBCopyScopes() map[string][]string {
	return map[string][]string{
		"sqlite":   {},
		"mysql":    {"database"},
		"postgres": {"schema", "database"},
	}
}
