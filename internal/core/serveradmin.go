// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const ServerAdminContractVersion = "0.15.0"
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
	ID       string `json:"id"`
	User     string `json:"user"`
	Database string `json:"database"`
	Seconds  int64  `json:"seconds"`
	Self     bool   `json:"self"`
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
type ServerAdminEngine interface {
	ServerAdminRead(context.Context, string, ServerAdminReadRequest) (ServerAdminReadResult, error)
}

func ServerAdminCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, action := range []string{"status", "variables", "processes"} {
			cap := dbplan.Capability{Support: dbplan.Limited, Reason: "Sichtbarkeit hängt von Serverrechten ab; SQL-Texte werden nicht ausgegeben."}
			if driver == "sqlite" {
				cap = dbplan.Capability{Support: dbplan.Unsupported, Reason: "SQLite hat keine Serververwaltung."}
				if action == "status" {
					cap = dbplan.Capability{Support: dbplan.Limited, Reason: "Nur SQLite-Version ohne Pfade."}
				}
			}
			if driver == "postgres" && action == "status" {
				cap.Reason = "Nur feste pg_stat_database-Zähler der verbundenen Datenbank; keine MySQL-Anweisungstypzähler."
			}
			if driver == "postgres" && action == "variables" {
				cap.Reason = "Nur wirksame SESSION-Werte; sensible und unbekannte Werte maskiert, kein GLOBAL."
			}
			c.Set(driver, action, cap)
		}
	}
	return c
}
