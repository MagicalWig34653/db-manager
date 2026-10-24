// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// ObjectOpsContractVersion is the API minor of the database and table operation routes.
const ObjectOpsContractVersion = "0.8.0"

// Limits of the object operation routes. They are part of the public contract.
const (
	ObjectOpDefaultTimeoutMS = 30000
	ObjectOpMaxTimeoutMS     = 60000
	ObjectOpMaxComment       = 2048
	ObjectOpMaxDependencies  = 100
	ObjectOpMaxAutoIncrement = 9007199254740991
)

// Action names. They are the route names, the capability keys and the value of
// confirm.action in an execute request.
const (
	OpDatabaseCreate    = "database_create"
	OpDatabaseDrop      = "database_drop"
	OpDatabaseCollation = "database_collation"
	OpTableRename       = "table_rename"
	OpTableCopy         = "table_copy"
	OpTableTruncate     = "table_truncate"
	OpTableDrop         = "table_drop"
	OpTableOptions      = "table_options"
	OpDependencies      = "dependencies"
)

// ObjectOpActions lists the planned actions in contract order.
var ObjectOpActions = []string{OpDatabaseCreate, OpDatabaseDrop, OpDatabaseCollation, OpTableRename, OpTableCopy, OpTableTruncate, OpTableDrop, OpTableOptions}

// ObjectOpRequest carries the structured input of every plan and execute
// request. Fields are pointers so that absent and empty values stay distinct;
// each action accepts only its own fields and rejects the rest. No field is SQL.
type ObjectOpRequest struct {
	Connection      Connection `json:"connection"`
	Database        *string    `json:"database,omitempty"`
	Table           *TableRef  `json:"table,omitempty"`
	NewName         *string    `json:"new_name,omitempty"`
	NewSchema       *string    `json:"new_schema,omitempty"`
	Mode            *string    `json:"mode,omitempty"`
	Collation       *string    `json:"collation,omitempty"`
	CollationMode   *string    `json:"collation_mode,omitempty"`
	Engine          *string    `json:"engine,omitempty"`
	Comment         *string    `json:"comment,omitempty"`
	AutoIncrement   *int64     `json:"auto_increment,omitempty"`
	RestartIdentity *bool      `json:"restart_identity,omitempty"`
	TimeoutMS       int        `json:"timeout_ms,omitempty"`
}

// ObjectOpConfirm is the explicit confirmation of an execute request.
// TargetName is the typed object name for destructive actions.
type ObjectOpConfirm struct {
	Action                     string `json:"action"`
	TargetName                 string `json:"target_name,omitempty"`
	AcknowledgeCurrentDatabase bool   `json:"acknowledge_current_database,omitempty"`
}

// ObjectOpExecuteRequest repeats the plan input and adds confirmation and the plan digest.
type ObjectOpExecuteRequest struct {
	ObjectOpRequest
	Confirm            ObjectOpConfirm `json:"confirm"`
	ExpectedPlanSHA256 string          `json:"expected_plan_sha256"`
}

// ObjectOpTarget names the object a plan acts on, resolved from the catalog.
type ObjectOpTarget struct {
	Kind   string `json:"kind"`
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
}

// ObjectOpConfirmation tells the client what the execute request must contain.
type ObjectOpConfirmation struct {
	Action          string `json:"action"`
	Destructive     bool   `json:"destructive"`
	TargetName      string `json:"target_name"`
	CurrentDatabase bool   `json:"current_database"`
}

// ObjectDependency is one object that is affected by an action on the target.
// Effect is blocks (the server refuses the action), breaks (the dependent
// object becomes invalid), follows (the server adjusts the reference) or info.
// Certainty is exact for catalog dependencies and textual for definition scans.
type ObjectDependency struct {
	Kind       string  `json:"kind"`
	Schema     string  `json:"schema"`
	Name       string  `json:"name"`
	Constraint *string `json:"constraint"`
	Effect     string  `json:"effect"`
	Certainty  string  `json:"certainty"`
}

// TableOptions are the observable table options. Null means "not available".
type TableOptions struct {
	Engine        *string `json:"engine"`
	Comment       *string `json:"comment"`
	Collation     *string `json:"collation"`
	AutoIncrement *int64  `json:"auto_increment"`
}

// ObjectOpPlan is the dry run of one action: exact statements, warnings,
// dependencies and the digest the execute request must repeat.
type ObjectOpPlan struct {
	Action                string               `json:"action"`
	Driver                string               `json:"driver"`
	Target                ObjectOpTarget       `json:"target"`
	Statements            []string             `json:"statements"`
	Warnings              []string             `json:"warnings"`
	AffectedObjects       []string             `json:"affected_objects"`
	Dependencies          []ObjectDependency   `json:"dependencies"`
	DependenciesTruncated bool                 `json:"dependencies_truncated"`
	CurrentOptions        *TableOptions        `json:"current_options"`
	DDLTransactional      string               `json:"ddl_transactional"`
	Confirmation          ObjectOpConfirmation `json:"confirmation"`
	PlanSHA256            string               `json:"plan_sha256"`
}

// ObjectOpObservation is what was read back after the statements ran. State is
// transactional (read inside the committing transaction), observed (separate
// read after the commit) or unavailable.
type ObjectOpObservation struct {
	State         string        `json:"state"`
	TargetExists  *bool         `json:"target_exists"`
	SourceExists  *bool         `json:"source_exists"`
	Rows          *int64        `json:"rows"`
	SourceRows    *int64        `json:"source_rows"`
	AutoIncrement *int64        `json:"auto_increment"`
	Collation     *string       `json:"collation"`
	Options       *TableOptions `json:"options"`
}

// ObjectOpResult is the answer of an execute request. Outcome is completed,
// rejected (nothing changed) or outcome_unknown (never retried automatically).
type ObjectOpResult struct {
	Action             string               `json:"action"`
	Outcome            string               `json:"outcome"`
	Reason             string               `json:"reason,omitempty"`
	Message            string               `json:"message"`
	StatementsTotal    int                  `json:"statements_total"`
	StatementsExecuted int                  `json:"statements_executed"`
	Atomic             bool                 `json:"atomic"`
	Observation        *ObjectOpObservation `json:"observation"`
}

// DependenciesRequest asks for the objects that depend on a table.
type DependenciesRequest struct {
	Connection Connection `json:"connection"`
	Table      TableRef   `json:"table"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

// DependenciesResult lists dependent objects with the effect of a rename and a drop.
type DependenciesResult struct {
	Driver       string             `json:"driver"`
	Table        ObjectOpTarget     `json:"table"`
	Dependencies []ObjectDependency `json:"dependencies"`
	Truncated    bool               `json:"truncated"`
	Notes        []string           `json:"notes"`
}

// ObjectOpsEngine is the optional database and table operation capability of an engine.
type ObjectOpsEngine interface {
	ObjectOpPlan(ctx context.Context, action string, r ObjectOpRequest) (ObjectOpPlan, error)
	ObjectOpExecute(ctx context.Context, action string, r ObjectOpExecuteRequest) (ObjectOpResult, error)
	Dependencies(ctx context.Context, r DependenciesRequest) (DependenciesResult, error)
}

// ObjectOpCapabilities is the static per-driver capability table. MariaDB is
// driver "mysql". Unsupported and limited entries always carry a reason.
func ObjectOpCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	sup := dbplan.Capability{Support: dbplan.Supported}
	lim := func(reason string) dbplan.Capability {
		return dbplan.Capability{Support: dbplan.Limited, Reason: reason}
	}
	uns := func(reason string) dbplan.Capability {
		return dbplan.Capability{Support: dbplan.Unsupported, Reason: reason}
	}
	managed := "SQLite-Datenbanken sind verwaltete Kennungen der Verbindung; sie werden nicht über SQL angelegt, geändert oder gelöscht."
	c.Set("sqlite", OpDatabaseCreate, uns(managed))
	c.Set("sqlite", OpDatabaseDrop, uns(managed))
	c.Set("sqlite", OpDatabaseCollation, uns(managed))
	c.Set("sqlite", OpTableRename, lim("Nur innerhalb der Datenbank main; Verschieben in ein anderes Schema entfällt. SQLite führt Verweise in Fremdschlüsseln und Sichten nach."))
	c.Set("sqlite", OpTableCopy, lim("Struktur wird mit CREATE TABLE AS vereinfacht kopiert: Spaltennamen und Affinitäten, aber keine Schlüssel, Indizes, Standardwerte oder Einschränkungen."))
	c.Set("sqlite", OpTableTruncate, lim("Wird als DELETE FROM ausgeführt; ein AUTOINCREMENT-Zähler bleibt erhalten."))
	c.Set("sqlite", OpTableDrop, sup)
	c.Set("sqlite", OpTableOptions, uns("SQLite kennt keine Tabellenoptionen (Engine, Kommentar, Kollation, Auto-Inkrement)."))
	c.Set("sqlite", OpDependencies, lim("Sichten werden anhand ihres Quelltextes erkannt (textuell); Trigger werden nicht ausgewertet."))
	c.Set("mysql", OpDatabaseCreate, sup)
	c.Set("mysql", OpDatabaseDrop, lim("Entfernt alle Objekte der Datenbank; DDL ist nicht transaktional (implizites Commit). Die Verbindungsdatenbank verlangt eine zusätzliche Bestätigung."))
	c.Set("mysql", OpDatabaseCollation, lim("Ändert nur den Standard der Datenbank, nicht bestehende Tabellen oder Spalten (dafür Tabellenoptionen mit Umwandlung)."))
	c.Set("mysql", OpTableRename, lim("DDL ist nicht transaktional (implizites Commit). Sichten werden nicht nachgeführt und können ungültig werden."))
	c.Set("mysql", OpTableCopy, lim("CREATE TABLE LIKE kopiert Spalten und Indizes, keine Fremdschlüssel oder Trigger. Struktur und Daten sind zwei nicht atomare Schritte."))
	c.Set("mysql", OpTableTruncate, lim("Implizites Commit; der Auto-Inkrement-Zähler wird zurückgesetzt; scheitert, wenn Fremdschlüssel auf die Tabelle verweisen."))
	c.Set("mysql", OpTableDrop, lim("Implizites Commit; kein CASCADE; scheitert, wenn Fremdschlüssel auf die Tabelle verweisen."))
	c.Set("mysql", OpTableOptions, sup)
	c.Set("mysql", OpDependencies, lim("Sichten werden anhand ihrer Definition erkannt (textuell); Trigger werden nicht ausgewertet."))
	c.Set("postgres", OpDatabaseCreate, lim("Kollation nur aus den libc-Locales des Servers; erfordert das Recht CREATEDB; nicht transaktional."))
	c.Set("postgres", OpDatabaseDrop, lim("Nicht die verbundene Datenbank und nicht mit offenen Sitzungen; der Inhalt einer anderen Datenbank ist von dieser Verbindung aus nicht einsehbar; nicht transaktional."))
	c.Set("postgres", OpDatabaseCollation, uns("PostgreSQL legt die Kollation einer Datenbank bei der Anlage fest."))
	c.Set("postgres", OpTableRename, sup)
	c.Set("postgres", OpTableCopy, lim("LIKE ... INCLUDING ALL: serial-Standardwerte teilen die Sequenz der Quelle, Zähler der Kopie werden nicht nachgeführt; Fremdschlüssel und Trigger werden nicht kopiert."))
	c.Set("postgres", OpTableTruncate, sup)
	c.Set("postgres", OpTableDrop, sup)
	c.Set("postgres", OpTableOptions, lim("Nur der Tabellenkommentar; Engine, Kollation und Auto-Inkrement sind in PostgreSQL keine Tabellenoptionen."))
	c.Set("postgres", OpDependencies, sup)
	return c
}

// ObjectOpTableOptionFields lists the table option fields each driver accepts.
func ObjectOpTableOptionFields() map[string][]string {
	return map[string][]string{
		"sqlite":   {},
		"mysql":    {"engine", "comment", "collation", "auto_increment"},
		"postgres": {"comment"},
	}
}
