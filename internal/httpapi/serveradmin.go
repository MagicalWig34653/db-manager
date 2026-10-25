// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"net/http"
)

func (a *API) registerServerAdmin(mux *http.ServeMux) {
	mux.Handle("GET /api/v17/capabilities", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		features := []string{}
		if _, ok := a.engine.(core.ServerAdminEngine); ok {
			features = []string{"status", "variables", "processes", "kill", "set_variable"}
		}
		writeJSON(w, 200, map[string]any{"contract_version": core.ServerAdminContractVersion, "features": features, "drivers": core.ServerAdminCapabilities(), "max_limit": 200, "default_limit": 100, "max_offset": 10000, "max_timeout_ms": 10000, "default_timeout_ms": 5000, "max_request_bytes": MaxRequestBytes})
	})))
	for _, kind := range []string{"status", "variables", "processes"} {
		mux.Handle("POST /api/v17/"+kind, a.auth(a.serverAdminRead(kind)))
	}
	for _, route := range []struct{ action, path string }{{"kill", "processes/kill"}, {"set_variable", "variables/set"}} {
		mux.Handle("POST /api/v17/"+route.path+"/plan", a.auth(a.serverAdminWrite(route.action, false)))
		mux.Handle("POST /api/v17/"+route.path, a.auth(a.serverAdminWrite(route.action, true)))
	}
}
func (a *API) serverAdminEngine(w http.ResponseWriter) (core.ServerAdminEngine, bool) {
	engine, ok := a.engine.(core.ServerAdminEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Serververwaltung ist nicht verfügbar.")
	}
	return engine, ok
}
func serverAdminFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		status := 0
		switch p.Code {
		case "invalid_server_admin", "confirmation_mismatch":
			status = 400
		case "object_not_found":
			status = 404
		case "plan_mismatch":
			status = 409
		case "unsupported_action":
			status = 422
		}
		if status != 0 {
			fail(w, status, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}
func (a *API) serverAdminRead(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input := core.ServerAdminReadRequest{Connection: core.Connection{TLSMode: "verify-full"}}
		if !decode(w, r, &input) || !validConnection(w, input.Connection) {
			return
		}
		engine, ok := a.serverAdminEngine(w)
		if !ok {
			return
		}
		result, err := engine.ServerAdminRead(r.Context(), kind, input)
		if err != nil {
			serverAdminFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}

type adminKillInput struct {
	Connection core.Connection `json:"connection"`
	TimeoutMS  int             `json:"timeout_ms,omitempty"`
	ID         string          `json:"id"`
	Identity   string          `json:"identity"`
	Mode       string          `json:"mode"`
}

func (i adminKillInput) request() core.ServerAdminRequest {
	return core.ServerAdminRequest{Connection: i.Connection, TimeoutMS: i.TimeoutMS, ID: i.ID, Identity: i.Identity, Mode: i.Mode}
}

type adminSetInput struct {
	Connection core.Connection `json:"connection"`
	TimeoutMS  int             `json:"timeout_ms,omitempty"`
	Name       string          `json:"name"`
	Scope      string          `json:"scope"`
	Value      string          `json:"value"`
}

func (i adminSetInput) request() core.ServerAdminRequest {
	return core.ServerAdminRequest{Connection: i.Connection, TimeoutMS: i.TimeoutMS, Name: i.Name, Scope: i.Scope, Value: i.Value}
}

type adminConfirmationInput struct {
	Confirm            *dbplan.Confirmation `json:"confirm"`
	ExpectedPlanSHA256 *string              `json:"expected_plan_sha256"`
}

func (a *API) serverAdminWrite(action string, execute bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request core.ServerAdminRequest
		var confirmation adminConfirmationInput
		if action == "kill" {
			if execute {
				var input struct {
					adminKillInput
					adminConfirmationInput
				}
				input.Connection.TLSMode = "verify-full"
				if !decode(w, r, &input) {
					return
				}
				request = input.adminKillInput.request()
				confirmation = input.adminConfirmationInput
			} else {
				var input adminKillInput
				input.Connection.TLSMode = "verify-full"
				if !decode(w, r, &input) {
					return
				}
				request = input.request()
			}
		} else {
			if execute {
				var input struct {
					adminSetInput
					adminConfirmationInput
				}
				input.Connection.TLSMode = "verify-full"
				if !decode(w, r, &input) {
					return
				}
				request = input.adminSetInput.request()
				confirmation = input.adminConfirmationInput
			} else {
				var input adminSetInput
				input.Connection.TLSMode = "verify-full"
				if !decode(w, r, &input) {
					return
				}
				request = input.request()
			}
		}
		if !validConnection(w, request.Connection) {
			return
		}
		if execute && (confirmation.Confirm == nil || confirmation.ExpectedPlanSHA256 == nil) {
			fail(w, 400, "invalid_request", "Bestätigung und erwarteter Plan-Hash sind erforderlich.")
			return
		}
		engine, ok := a.serverAdminEngine(w)
		if !ok {
			return
		}
		if !execute {
			result, err := engine.ServerAdminPlan(r.Context(), action, request)
			if err != nil {
				serverAdminFail(w, err)
				return
			}
			writeJSON(w, 200, result)
			return
		}
		result, err := engine.ServerAdminExecute(r.Context(), action, core.ServerAdminExecuteRequest{ServerAdminRequest: request, Confirm: *confirmation.Confirm, ExpectedPlanSHA256: *confirmation.ExpectedPlanSHA256})
		if err != nil {
			serverAdminFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}
