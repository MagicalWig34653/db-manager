// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerIndexes(mux *http.ServeMux) {
	mux.Handle("GET /api/v13/capabilities", a.auth(http.HandlerFunc(a.indexesCapabilities)))
	mux.Handle("POST /api/v13/indexes/plan", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.indexes(w, r, false) })))
	mux.Handle("POST /api/v13/indexes/alter", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.indexes(w, r, true) })))
}
func (a *API) indexesCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.IndexesEngine); ok {
		features = append(features, "indexes_alter")
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.IndexesContractVersion, "features": features, "drivers": core.IndexesCapabilities(), "max_timeout_ms": 60000, "max_request_bytes": MaxRequestBytes})
}
func (a *API) indexes(w http.ResponseWriter, r *http.Request, execute bool) {
	var input core.IndexesRequest
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	if !execute && (input.Confirm != nil || input.ExpectedPlanSHA256 != "") {
		fail(w, 400, "invalid_indexes", "Vorschau enthält keine Ausführungsbestätigung.")
		return
	}

	engine, ok := a.engine.(core.IndexesEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Schlüsseländerungen sind nicht verfügbar.")
		return
	}
	var result any
	var err error
	if execute {
		result, err = engine.ExecuteIndexes(r.Context(), input)
	} else {
		result, err = engine.PlanIndexes(r.Context(), input)
	}
	if err != nil {
		var p *core.Error
		if errors.As(err, &p) {
			status := 0
			switch p.Code {
			case "invalid_indexes", "invalid_table", "confirmation_mismatch":
				status = 400
			case "column_not_found", "table_not_found", "object_not_found":
				status = 404
			case "object_exists", "plan_mismatch":
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
