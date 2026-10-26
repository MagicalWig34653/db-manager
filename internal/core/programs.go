// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const ProgramsContractVersion = "0.16.0"
const ProgramsMaxBodyBytes = 262144
const ProgramsMaxObjects = 200

// ProgramRef identifies overloaded routines by their catalog identity arguments.
// Trigger names in PostgreSQL are scoped to a table.
type ProgramRef struct {
	Kind     string    `json:"kind"`
	Schema   string    `json:"schema,omitempty"`
	Name     string    `json:"name"`
	Identity string    `json:"identity,omitempty"`
	Table    *TableRef `json:"table,omitempty"`
}
type ProgramParameter struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
	Type string `json:"type"`
}
type ProgramDefiner struct {
	Name string `json:"name"`
	Host string `json:"host,omitempty"`
}

// ProgramInput separates structural choices from explicitly marked user SQL.
type ProgramInput struct {
	Body          string             `json:"body"`
	Parameters    []ProgramParameter `json:"parameters,omitempty"`
	Returns       string             `json:"returns,omitempty"`
	Language      string             `json:"language,omitempty"`
	Security      string             `json:"security,omitempty"`
	Definer       *ProgramDefiner    `json:"definer,omitempty"`
	Deterministic bool               `json:"deterministic,omitempty"`
	Timing        string             `json:"timing,omitempty"`
	Event         string             `json:"event,omitempty"`
	Function      *ProgramRef        `json:"function,omitempty"`
	Schedule      *ProgramSchedule   `json:"schedule,omitempty"`
	Status        string             `json:"status,omitempty"`
	CheckOption   string             `json:"check_option,omitempty"`
}
type ProgramSchedule struct {
	At       string `json:"at,omitempty"`
	Every    int    `json:"every,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Starts   string `json:"starts,omitempty"`
	Ends     string `json:"ends,omitempty"`
	Preserve bool   `json:"preserve"`
}
type ProgramsRequest struct {
	Connection Connection  `json:"connection"`
	Kind       string      `json:"kind,omitempty"`
	Schema     string      `json:"schema,omitempty"`
	Table      *TableRef   `json:"table,omitempty"`
	Object     *ProgramRef `json:"object,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
	TimeoutMS  int         `json:"timeout_ms,omitempty"`
}
type Program struct {
	Object                ProgramRef          `json:"object"`
	Definition            *string             `json:"definition"`
	Body                  *string             `json:"body"`
	Parameters            []ProgramParameter  `json:"parameters"`
	Properties            map[string]string   `json:"properties"`
	Definer               *ProgramDefiner     `json:"definer"`
	Security              *string             `json:"security"`
	Dependencies          []ProgramDependency `json:"dependencies"`
	DependenciesTruncated bool                `json:"dependencies_truncated"`
	Warnings              []string            `json:"warnings"`
}
type ProgramDependency struct {
	Object    ProgramRef `json:"object"`
	Direction string     `json:"direction"`
	Certainty string     `json:"certainty"`
}
type ProgramsResult struct {
	Driver           string    `json:"driver"`
	Objects          []Program `json:"objects"`
	HasMore          bool      `json:"has_more"`
	Scheduler        *string   `json:"scheduler"`
	EventErrorsState string    `json:"event_errors_state"`
	Warnings         []string  `json:"warnings"`
}
type ProgramChangeRequest struct {
	Connection Connection    `json:"connection"`
	Action     string        `json:"action"`
	Object     ProgramRef    `json:"object"`
	Input      *ProgramInput `json:"input,omitempty"`
	TimeoutMS  int           `json:"timeout_ms,omitempty"`
}
type ProgramExecuteRequest struct {
	ProgramChangeRequest
	Confirm            dbplan.Confirmation `json:"confirm"`
	ExpectedPlanSHA256 string              `json:"expected_plan_sha256"`
}
type ProgramPlan struct {
	dbplan.Plan
	Action                string              `json:"action"`
	Object                ProgramRef          `json:"object"`
	UserSQL               bool                `json:"user_sql"`
	Destructive           bool                `json:"destructive"`
	Transactional         bool                `json:"transactional"`
	Dependencies          []ProgramDependency `json:"dependencies"`
	DependenciesTruncated bool                `json:"dependencies_truncated"`
}
type ProgramResult struct {
	Outcome            string `json:"outcome"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message"`
	StatementsExecuted int    `json:"statements_executed"`
	StatementsTotal    int    `json:"statements_total"`
	Atomic             bool   `json:"atomic"`
	Observation        string `json:"observation"`
}
type ProgramsEngine interface {
	Programs(context.Context, ProgramsRequest) (ProgramsResult, error)
	ProgramPlan(context.Context, ProgramChangeRequest) (ProgramPlan, error)
	ProgramExecute(context.Context, ProgramExecuteRequest) (ProgramResult, error)
}

func ProgramsCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, kind := range []string{"view", "procedure", "function", "trigger", "event"} {
			v := dbplan.Capability{Support: dbplan.Limited, Reason: "Nutzer-SQL nur mit Plan und Bestätigung; tatsächliche Datenbankrechte gelten. Abhängigkeiten können unvollständig sein."}
			if driver == "sqlite" && (kind == "procedure" || kind == "function" || kind == "event") || driver == "postgres" && kind == "event" {
				v = dbplan.Capability{Support: dbplan.Unsupported, Reason: "Dieser Treiber unterstützt diese Objektart nicht."}
			}
			c.Set(driver, kind, v)
		}
		for _, action := range []string{"routine_execute", "scheduler_write", "sqlite_rebuild", "event_errors"} {
			c.Set(driver, action, dbplan.Capability{Support: dbplan.Unsupported, Reason: "Nicht Bestandteil dieses Pakets; keine Ausführung, Scheduleränderung, Tabellen-Neuaufbau oder portabel verfügbare Eventfehlerliste."})
		}
	}
	return c
}
