// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerPrograms(mux *http.ServeMux) {
	mux.Handle("GET /api/v16/capabilities", a.auth(http.HandlerFunc(a.programCapabilities)))
	for _, route := range []string{"list", "describe", "dependencies"} {
		mux.Handle("POST /api/v16/programs/"+route, a.auth(a.programRead(route)))
	}
	mux.Handle("POST /api/v16/programs/plan", a.auth(http.HandlerFunc(a.programPlan)))
	mux.Handle("POST /api/v16/programs/execute", a.auth(http.HandlerFunc(a.programExecute)))
}
func (a *API) programCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.ProgramsEngine); ok {
		features = []string{"list", "describe", "dependencies", "plan", "execute"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.ProgramsContractVersion, "features": features, "drivers": core.ProgramsCapabilities(), "max_body_bytes": core.ProgramsMaxBodyBytes, "max_objects": core.ProgramsMaxObjects, "max_request_bytes": MaxRequestBytes, "default_timeout_ms": 30000, "max_timeout_ms": 60000, "routine_execution": false, "scheduler_write": false})
}
func (a *API) programsEngine(w http.ResponseWriter) (core.ProgramsEngine, bool) {
	e, ok := a.engine.(core.ProgramsEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Programmierobjekte sind nicht verfügbar.")
	}
	return e, ok
}
func programFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "invalid_program", "ambiguous_object", "confirmation_mismatch":
			fail(w, 400, p.Code, p.Message)
			return
		}
	}
	objectOpFail(w, err)
}
func (a *API) programRead(route string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input core.ProgramsRequest
		input.Connection.TLSMode = "verify-full"
		if !decode(w, r, &input) || !validConnection(w, input.Connection) {
			return
		}
		if (route == "list") != (input.Object == nil) {
			fail(w, 400, "invalid_program", "Für diese Route fehlt der passende Objektbezug.")
			return
		}
		e, ok := a.programsEngine(w)
		if !ok {
			return
		}
		result, err := e.Programs(r.Context(), input)
		if err != nil {
			programFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}
func (a *API) programPlan(w http.ResponseWriter, r *http.Request) {
	var input core.ProgramChangeRequest
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	e, ok := a.programsEngine(w)
	if !ok {
		return
	}
	result, err := e.ProgramPlan(r.Context(), input)
	if err != nil {
		programFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) programExecute(w http.ResponseWriter, r *http.Request) {
	var input core.ProgramExecuteRequest
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	if input.Confirm.Action == "" || input.ExpectedPlanSHA256 == "" {
		fail(w, 400, "confirmation_mismatch", "Bestätigung und erwarteter Plan-Hash sind erforderlich.")
		return
	}
	e, ok := a.programsEngine(w)
	if !ok {
		return
	}
	result, err := e.ProgramExecute(r.Context(), input)
	if err != nil {
		programFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
