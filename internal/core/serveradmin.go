// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const ServerAdminContractVersion = "0.17.0"
const ServerAdminMaxLimit = 200
const ServerAdminMaxTimeoutMS = 10000

type ServerAdminReadRequest struct {
	Connection Connection `json:"connection"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
	Filter     string     `json:"filter,omitempty"`
	Scope      string     `json:"scope,omitempty"`
}
type ServerAdminValue struct {
	Name   string  `json:"name"`
	Value  *string `json:"value"`
	Masked bool    `json:"masked"`
}
type ServerAdminProcess struct {
	ID            string `json:"id"`
	User          string `json:"user"`
	Database      string `json:"database"`
	Seconds       int64  `json:"seconds"`
	Self          bool   `json:"self"`
	Identity      string `json:"identity"`
	QueryIdentity string `json:"query_identity"`
}
type ServerAdminReadResult struct {
	Driver    string               `json:"driver"`
	Kind      string               `json:"kind"`
	Scope     string               `json:"scope"`
	SampledAt string               `json:"sampled_at"`
	Values    []ServerAdminValue   `json:"values"`
	Processes []ServerAdminProcess `json:"processes"`
	Limit     int                  `json:"limit"`
	Offset    int                  `json:"offset"`
	HasMore   bool                 `json:"has_more"`
}
type ServerAdminRequest struct {
	Connection Connection `json:"connection"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
	ID         string     `json:"id,omitempty"`
	Identity   string     `json:"identity,omitempty"`
	Mode       string     `json:"mode,omitempty"`
	Name       string     `json:"name,omitempty"`
	Scope      string     `json:"scope,omitempty"`
	Value      string     `json:"value,omitempty"`
}
type ServerAdminPlan struct {
	dbplan.Plan
	Action     string `json:"action"`
	TargetName string `json:"target_name"`
}
type ServerAdminExecuteRequest struct {
	ServerAdminRequest
	Confirm            dbplan.Confirmation `json:"confirm"`
	ExpectedPlanSHA256 string              `json:"expected_plan_sha256"`
}
type ServerAdminResult struct {
	Outcome       dbplan.Status `json:"outcome"`
	Message       string        `json:"message"`
	Reason        string        `json:"reason"`
	ObservedValue *string       `json:"observed_value"`
}
type ServerAdminEngine interface {
	ServerAdminRead(context.Context, string, ServerAdminReadRequest) (ServerAdminReadResult, error)
	ServerAdminPlan(context.Context, string, ServerAdminRequest) (ServerAdminPlan, error)
	ServerAdminExecute(context.Context, string, ServerAdminExecuteRequest) (ServerAdminResult, error)
}

func ServerAdminCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, action := range []string{"status", "variables", "processes", "kill", "set_variable", "flush"} {
			cap := dbplan.Capability{Support: dbplan.Limited, Reason: "Sichtbarkeit und Ausführung hängen von Serverrechten ab; SQL-Texte werden nicht ausgegeben."}
			if driver == "sqlite" {
				cap = dbplan.Capability{Support: dbplan.Unsupported, Reason: "SQLite hat keine Serververwaltung."}
				if action == "status" {
					cap = dbplan.Capability{Support: dbplan.Limited, Reason: "Nur SQLite-Version ohne Pfade."}
				}
			}
			if driver == "mysql" && action == "set_variable" {
				cap.Reason = "Nur tmp_table_size/max_heap_table_size in SESSION/GLOBAL; Rechte prüft der Server. SESSION endet mit dieser Anfrage."
			}
			if driver == "postgres" && action == "status" {
				cap.Reason = "Nur feste pg_stat_database-Zähler der verbundenen Datenbank; keine MySQL-Anweisungstypzähler."
			}
			if driver == "postgres" && action == "variables" {
				cap.Reason = "Nur wirksame SESSION-Werte; sensible und unbekannte Werte maskiert, kein GLOBAL."
			}
			if action == "flush" {
				cap = dbplan.Capability{Support: dbplan.Unsupported, Reason: "FLUSH ist in diesem Paket nicht freigegeben."}
			}
			if driver == "postgres" && action == "set_variable" {
				cap.Reason = "Nur SESSION statement_timeout und lock_timeout; Sitzung endet mit dieser Anfrage. GLOBAL wird nicht unterstützt."
			}
			c.Set(driver, action, cap)
		}
	}
	return c
}
