// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func (a *API) registerColumns(mux *http.ServeMux) {
	mux.Handle("GET /api/v9/capabilities", a.auth(http.HandlerFunc(a.columnsCapabilities)))
	mux.Handle("POST /api/v9/columns/plan", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.columns(w, r, false) })))
	mux.Handle("POST /api/v9/columns/alter", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.columns(w, r, true) })))
}
func (a *API) columnsCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.ColumnsEngine); ok {
		features = append(features, "columns_alter")
	}
	types := map[string][]string{}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		types[driver] = dbplan.TypeCatalog(driver)
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.ColumnsContractVersion, "features": features, "drivers": core.ColumnsCapabilities(), "types": types, "max_changes": 16, "max_timeout_ms": 60000, "max_request_bytes": MaxRequestBytes})
}
func (a *API) columns(w http.ResponseWriter, r *http.Request, execute bool) {
	var wire struct {
		Connection core.Connection      `json:"connection"`
		Table      core.TableRef        `json:"table"`
		Changes    []core.ColumnChange  `json:"changes"`
		TimeoutMS  int                  `json:"timeout_ms,omitempty"`
		Confirm    *dbplan.Confirmation `json:"confirm,omitempty"`
		Expected   *string              `json:"expected_plan_sha256,omitempty"`
	}
	wire.Connection.TLSMode = "verify-full"
	if !decode(w, r, &wire) || !validConnection(w, wire.Connection) {
		return
	}
	if !execute && (wire.Confirm != nil || wire.Expected != nil) {
		fail(w, 400, "invalid_columns", "Vorschau enthält keine Ausführungsbestätigung.")
		return
	}
	input := core.ColumnsRequest{Connection: wire.Connection, Table: wire.Table, Changes: wire.Changes, TimeoutMS: wire.TimeoutMS, Confirm: wire.Confirm}
	if wire.Expected != nil {
		input.ExpectedPlanSHA256 = *wire.Expected
	}
	engine, ok := a.engine.(core.ColumnsEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Spaltenänderungen sind nicht verfügbar.")
		return
	}
	var result any
	var err error
	if execute {
		result, err = engine.ExecuteColumns(r.Context(), input)
	} else {
		result, err = engine.PlanColumns(r.Context(), input)
	}
	if err != nil {
		var p *core.Error
		if errors.As(err, &p) {
			status := 0
			switch p.Code {
			case "invalid_columns", "invalid_table", "confirmation_mismatch":
				status = 400
			case "column_not_found", "table_not_found":
				status = 404
			case "column_exists", "plan_mismatch":
				status = 409
			case "unsupported_action":
				status = 422
			}
			if status != 0 {
				fail(w, status, p.Code, p.Message)
				return
			}
		}
		mutationError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
