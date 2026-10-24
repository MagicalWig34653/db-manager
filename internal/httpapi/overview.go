// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerOverview(mux *http.ServeMux) {
	mux.Handle("GET /api/v7/capabilities", a.auth(http.HandlerFunc(a.overviewCapabilities)))
	mux.Handle("POST /api/v7/databases", a.auth(http.HandlerFunc(a.overviewDatabases)))
	mux.Handle("POST /api/v7/objects", a.auth(http.HandlerFunc(a.overviewObjects)))
	mux.Handle("POST /api/v7/definition", a.auth(http.HandlerFunc(a.overviewDefinition)))
	mux.Handle("POST /api/v7/server-info", a.auth(http.HandlerFunc(a.overviewServerInfo)))
}

func (a *API) overviewCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.OverviewEngine); ok {
		features = []string{"databases", "objects", "definition", "server_info"}
	}
	writeJSON(w, 200, map[string]any{
		"contract_version": core.OverviewContractVersion, "features": features, "mutations": false,
		"drivers": core.OverviewCapabilities(), "default_limit": core.OverviewDefaultLimit, "max_limit": core.OverviewMaxLimit,
		"max_offset": core.OverviewMaxOffset, "max_exact_objects": core.OverviewMaxExactObjects, "default_timeout_ms": core.OverviewDefaultTimeoutMS,
		"max_timeout_ms": core.OverviewMaxTimeoutMS, "max_list_items": core.OverviewMaxListItems, "max_definition_bytes": core.OverviewMaxDefinition,
		"max_request_bytes": MaxRequestBytes,
	})
}

func (a *API) overviewEngine(w http.ResponseWriter) (core.OverviewEngine, bool) {
	engine, ok := a.engine.(core.OverviewEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Übersichtsfunktionen sind nicht verfügbar.")
	}
	return engine, ok
}

// overviewFail maps invalid_overview and missing objects to precise statuses.
func overviewFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "invalid_overview":
			fail(w, 400, p.Code, p.Message)
			return
		case "object_not_found":
			fail(w, 404, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}

func (a *API) overviewDatabases(w http.ResponseWriter, r *http.Request) {
	request := core.DatabasesRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.overviewEngine(w)
	if !ok {
		return
	}
	result, err := engine.Databases(r.Context(), request)
	if err != nil {
		overviewFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *API) overviewObjects(w http.ResponseWriter, r *http.Request) {
	request := core.ObjectsRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.overviewEngine(w)
	if !ok {
		return
	}
	result, err := engine.Objects(r.Context(), request)
	if err != nil {
		overviewFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *API) overviewDefinition(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Connection core.Connection `json:"connection"`
		Object     *core.TableRef  `json:"object"`
		TimeoutMS  int             `json:"timeout_ms,omitempty"`
	}
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	if input.Object == nil {
		fail(w, 400, "invalid_request", "Objektbezug ist erforderlich.")
		return
	}
	engine, ok := a.overviewEngine(w)
	if !ok {
		return
	}
	result, err := engine.Definition(r.Context(), core.DefinitionRequest{Connection: input.Connection, Object: *input.Object, TimeoutMS: input.TimeoutMS})
	if err != nil {
		overviewFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (a *API) overviewServerInfo(w http.ResponseWriter, r *http.Request) {
	request := core.ServerInfoRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.overviewEngine(w)
	if !ok {
		return
	}
	result, err := engine.ServerInfo(r.Context(), request)
	if err != nil {
		overviewFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
