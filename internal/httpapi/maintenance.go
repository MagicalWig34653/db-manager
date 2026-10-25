// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerMaintenance(mux *http.ServeMux) {
	mux.Handle("GET /api/v10/capabilities", a.auth(http.HandlerFunc(a.maintenanceCapabilities)))
	mux.Handle("POST /api/v10/tables/maintenance/plan", a.auth(http.HandlerFunc(a.maintenancePlan)))
	mux.Handle("POST /api/v10/tables/maintenance", a.auth(http.HandlerFunc(a.maintenanceExecute)))
}
func (a *API) maintenanceCapabilities(w http.ResponseWriter, r *http.Request) {
	_, available := a.engine.(core.MaintenanceEngine)
	writeJSON(w, 200, map[string]any{"contract_version": core.MaintenanceContractVersion, "available": available, "drivers": core.MaintenanceCapabilities(), "max_items": core.MaintenanceMaxItems, "default_timeout_ms": core.MaintenanceDefaultTimeoutMS, "max_timeout_ms": core.MaintenanceMaxTimeoutMS, "max_request_bytes": MaxRequestBytes})
}
func maintenanceInput(w http.ResponseWriter, items []core.MaintenanceItem, timeout int) bool {
	if len(items) < 1 || len(items) > core.MaintenanceMaxItems || timeout < 0 || timeout > core.MaintenanceMaxTimeoutMS {
		fail(w, 400, "invalid_maintenance", "Tabellenanzahl oder Zeitlimit ist ungültig.")
		return false
	}
	for _, item := range items {
		switch item.Action {
		case "check", "analyze", "optimize", "repair", "checksum", "vacuum", "reindex":
		default:
			fail(w, 400, "invalid_maintenance", "Wartungsaktion ist ungültig.")
			return false
		}
		if item.Table.Name == "" || len(item.Table.Name) > 1024 || len(item.Table.Schema) > 1024 {
			fail(w, 400, "invalid_maintenance", "Tabellenbezug ist erforderlich.")
			return false
		}
	}
	return true
}
func maintenanceFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) && p.Code == "invalid_maintenance" {
		fail(w, 400, p.Code, p.Message)
		return
	}
	engineError(w, err)
}
func (a *API) maintenancePlan(w http.ResponseWriter, r *http.Request) {
	input := core.MaintenancePlanRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) || !maintenanceInput(w, input.Items, input.TimeoutMS) {
		return
	}
	engine, ok := a.engine.(core.MaintenanceEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Tabellenwartung ist nicht verfügbar.")
		return
	}
	result, err := engine.PlanMaintenance(r.Context(), input)
	if err != nil {
		maintenanceFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) maintenanceExecute(w http.ResponseWriter, r *http.Request) {
	input := core.MaintenanceExecuteRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	items := make([]core.MaintenanceItem, len(input.Items))
	for i, item := range input.Items {
		items[i] = item.MaintenanceItem
		if item.ExpectedPlanSHA256 == "" || item.Confirm.Action == "" {
			fail(w, 400, "invalid_maintenance", "Plan-Hash und ausdrückliche Bestätigung sind erforderlich.")
			return
		}
	}
	if !maintenanceInput(w, items, input.TimeoutMS) {
		return
	}
	engine, ok := a.engine.(core.MaintenanceEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Tabellenwartung ist nicht verfügbar.")
		return
	}
	result, err := engine.ExecuteMaintenance(r.Context(), input)
	if err != nil {
		maintenanceFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
