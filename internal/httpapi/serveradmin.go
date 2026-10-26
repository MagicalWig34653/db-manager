// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerServerAdmin(mux *http.ServeMux) {
	mux.Handle("GET /api/v15/capabilities", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		features := []string{}
		if _, ok := a.engine.(core.ServerAdminEngine); ok {
			features = []string{"status", "variables", "processes"}
		}
		writeJSON(w, 200, map[string]any{"contract_version": core.ServerAdminContractVersion, "features": features, "drivers": core.ServerAdminCapabilities(), "max_limit": 200, "default_limit": 100, "max_offset": 10000, "max_timeout_ms": 10000, "default_timeout_ms": 5000, "max_request_bytes": MaxRequestBytes})
	})))
	for _, kind := range []string{"status", "variables", "processes"} {
		mux.Handle("POST /api/v15/"+kind, a.auth(a.serverAdminRead(kind)))
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
		case "invalid_server_admin":
			status = 400
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
