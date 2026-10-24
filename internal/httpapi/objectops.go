// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// objectOpRoutes maps each action to its route prefix (plan route: prefix + "/plan").
var objectOpRoutes = []struct{ action, path string }{
	{core.OpDatabaseCreate, "/api/v8/databases/create"},
	{core.OpDatabaseDrop, "/api/v8/databases/drop"},
	{core.OpDatabaseCollation, "/api/v8/databases/collation"},
	{core.OpTableRename, "/api/v8/tables/rename"},
	{core.OpTableCopy, "/api/v8/tables/copy"},
	{core.OpTableTruncate, "/api/v8/tables/truncate"},
	{core.OpTableDrop, "/api/v8/tables/drop"},
	{core.OpTableOptions, "/api/v8/tables/options"},
}

func (a *API) registerObjectOps(mux *http.ServeMux) {
	mux.Handle("GET /api/v8/capabilities", a.auth(http.HandlerFunc(a.objectOpCapabilities)))
	mux.Handle("POST /api/v8/tables/dependencies", a.auth(http.HandlerFunc(a.objectOpDependencies)))
	for _, route := range objectOpRoutes {
		mux.Handle("POST "+route.path+"/plan", a.auth(a.objectOpPlan(route.action)))
		mux.Handle("POST "+route.path, a.auth(a.objectOpExecute(route.action)))
	}
}

func (a *API) objectOpCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.ObjectOpsEngine); ok {
		features = append(append(features, core.ObjectOpActions...), core.OpDependencies)
	}
	writeJSON(w, 200, map[string]any{
		"contract_version": core.ObjectOpsContractVersion, "features": features, "mutations": true,
		"drivers": core.ObjectOpCapabilities(), "table_option_fields": core.ObjectOpTableOptionFields(),
		"default_timeout_ms": core.ObjectOpDefaultTimeoutMS, "max_timeout_ms": core.ObjectOpMaxTimeoutMS,
		"max_comment_length": core.ObjectOpMaxComment, "max_dependencies": core.ObjectOpMaxDependencies,
		"max_request_bytes": MaxRequestBytes,
	})
}

func (a *API) objectOpsEngine(w http.ResponseWriter) (core.ObjectOpsEngine, bool) {
	engine, ok := a.engine.(core.ObjectOpsEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Datenbank- und Tabellenoperationen sind nicht verfügbar.")
	}
	return engine, ok
}

// objectOpFail maps the public codes of the object operations to precise statuses.
func objectOpFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "invalid_object_op", "confirmation_mismatch":
			fail(w, 400, p.Code, p.Message)
			return
		case "object_not_found":
			fail(w, 404, p.Code, p.Message)
			return
		case "object_exists", "plan_mismatch":
			fail(w, 409, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}

func (a *API) objectOpPlan(action string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input core.ObjectOpRequest
		input.Connection.TLSMode = "verify-full"
		if !decode(w, r, &input) || !validConnection(w, input.Connection) {
			return
		}
		engine, ok := a.objectOpsEngine(w)
		if !ok {
			return
		}
		plan, err := engine.ObjectOpPlan(r.Context(), action, input)
		if err != nil {
			objectOpFail(w, err)
			return
		}
		writeJSON(w, 200, plan)
	})
}

func (a *API) objectOpExecute(action string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			core.ObjectOpRequest
			Confirm            *core.ObjectOpConfirm `json:"confirm"`
			ExpectedPlanSHA256 *string               `json:"expected_plan_sha256"`
		}
		input.Connection.TLSMode = "verify-full"
		if !decode(w, r, &input) || !validConnection(w, input.Connection) {
			return
		}
		if input.Confirm == nil || input.ExpectedPlanSHA256 == nil {
			fail(w, 400, "invalid_request", "Bestätigung und erwarteter Plan-Hash sind erforderlich.")
			return
		}
		engine, ok := a.objectOpsEngine(w)
		if !ok {
			return
		}
		result, err := engine.ObjectOpExecute(r.Context(), action, core.ObjectOpExecuteRequest{ObjectOpRequest: input.ObjectOpRequest, Confirm: *input.Confirm, ExpectedPlanSHA256: *input.ExpectedPlanSHA256})
		if err != nil {
			objectOpFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}

func (a *API) objectOpDependencies(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Connection core.Connection `json:"connection"`
		Table      *core.TableRef  `json:"table"`
		TimeoutMS  int             `json:"timeout_ms,omitempty"`
	}
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	if input.Table == nil {
		fail(w, 400, "invalid_request", "Tabellenbezug ist erforderlich.")
		return
	}
	engine, ok := a.objectOpsEngine(w)
	if !ok {
		return
	}
	result, err := engine.Dependencies(r.Context(), core.DependenciesRequest{Connection: input.Connection, Table: *input.Table, TimeoutMS: input.TimeoutMS})
	if err != nil {
		objectOpFail(w, err)
		return
	}
	writeJSON(w, 200, result)
}
