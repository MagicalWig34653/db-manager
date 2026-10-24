// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const MaintenanceContractVersion = "0.11.0"
const MaintenanceMaxItems = 50
const MaintenanceDefaultTimeoutMS = 10000
const MaintenanceMaxTimeoutMS = 60000

type MaintenanceItem struct {
	Table   TableRef `json:"table"`
	Action  string   `json:"action"`
	Full    bool     `json:"full,omitempty"`
	Analyze bool     `json:"analyze,omitempty"`
}
type MaintenanceExecuteItem struct {
	MaintenanceItem
	ExpectedPlanSHA256 string              `json:"expected_plan_sha256"`
	Confirm            dbplan.Confirmation `json:"confirm"`
}
type MaintenancePlanRequest struct {
	Connection Connection        `json:"connection"`
	Items      []MaintenanceItem `json:"items"`
	TimeoutMS  int               `json:"timeout_ms,omitempty"`
}
type MaintenanceExecuteRequest struct {
	Connection Connection               `json:"connection"`
	Items      []MaintenanceExecuteItem `json:"items"`
	TimeoutMS  int                      `json:"timeout_ms,omitempty"`
}
type MaintenancePreview struct {
	Table             TableRef     `json:"table"`
	Action            string       `json:"action"`
	State             string       `json:"state"`
	Reason            string       `json:"reason"`
	Plan              *dbplan.Plan `json:"plan"`
	TypedConfirmation bool         `json:"typed_confirmation"`
	TargetName        string       `json:"target_name"`
}
type MaintenancePlanResult struct {
	Items []MaintenancePreview `json:"items"`
}
type MaintenanceOutcome struct {
	Table      TableRef      `json:"table"`
	Action     string        `json:"action"`
	Outcome    dbplan.Status `json:"outcome"`
	Reason     string        `json:"reason"`
	Checksum   *string       `json:"checksum"`
	DurationMS int64         `json:"duration_ms"`
}
type MaintenanceResult struct {
	Items   []MaintenanceOutcome `json:"items"`
	Partial bool                 `json:"partial"`
}
type MaintenanceEngine interface {
	PlanMaintenance(context.Context, MaintenancePlanRequest) (MaintenancePlanResult, error)
	ExecuteMaintenance(context.Context, MaintenanceExecuteRequest) (MaintenanceResult, error)
}

func MaintenanceCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, action := range []string{"check", "analyze", "optimize", "repair", "checksum", "vacuum", "reindex"} {
			cap := dbplan.Capability{Support: dbplan.Unsupported, Reason: "Diese Aktion ist für diesen Treiber nicht verfügbar."}
			switch driver {
			case "sqlite":
				if action == "check" || action == "analyze" {
					cap = dbplan.Capability{Support: dbplan.Limited, Reason: "Nur main-Basistabellen; Integritätsprüfung ohne rohe Diagnosetexte."}
				}
				if action == "vacuum" {
					cap.Reason = "VACUUM ist unter ADR-0004 gesperrt; kein VACUUM INTO und keine Erweiterung der Dateisandbox."
				}
			case "mysql":
				if action == "check" || action == "analyze" || action == "optimize" || action == "repair" || action == "checksum" {
					cap = dbplan.Capability{Support: dbplan.Limited, Reason: "Engineabhängig; REPAIR nur MyISAM, Aria, ARCHIVE oder CSV. Serverrechte erforderlich; implizite Commits möglich."}
				}
			case "postgres":
				if action == "analyze" || action == "vacuum" || action == "reindex" {
					cap = dbplan.Capability{Support: dbplan.Limited, Reason: "Nur lokale Basistabellen ohne Vererbung oder Partitionierung; Eigentümer oder MAINTAIN-Recht (PostgreSQL ab 17). VACUUM unterstützt full und analyze."}
				}
			}
			c.Set(driver, action, cap)
		}
	}
	return c
}
