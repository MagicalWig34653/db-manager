// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerSQLBatch(mux *http.ServeMux) {
	mux.Handle("GET /api/v12/capabilities", a.auth(http.HandlerFunc(a.sqlBatchCapabilities)))
	for _, action := range []string{"batch", "explain", "simulate"} {
		mux.Handle("POST /api/v12/sql/"+action, a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.sqlBatchAction(w, r, action) })))
	}
}
func (a *API) sqlBatchCapabilities(w http.ResponseWriter, r *http.Request) {
	_, available := a.engine.(core.SQLBatchEngine)
	writeJSON(w, 200, map[string]any{"contract_version": core.SQLBatchContractVersion, "available": available, "max_statements": 100, "max_sql_bytes": 1048576, "max_rows": 10000, "max_timeout_ms": 60000, "max_response_bytes": 8388608, "confirmation_required": true, "explain_analyze": false, "drivers": []map[string]string{
		{"driver": "sqlite", "batch": "limited", "explain": "limited", "simulation": "limited", "reason": "SQLite-Sandbox; Simulation einfacher DML ohne RETURNING und virtuelle Tabellen, Rollback erforderlich."},
		{"driver": "mysql", "batch": "limited", "explain": "limited", "simulation": "unsupported", "reason": "Auch MariaDB: keine allgemeine Rücknahme von Engine-, Trigger- und externen Effekten; maximal 100 Warncodes ohne Rohtext."},
		{"driver": "postgres", "batch": "limited", "explain": "limited", "simulation": "unsupported", "reason": "Sequenzen und externe Effekte sind nicht allgemein rücknehmbar; Serverhinweise nicht erfasst."},
	}})
}
func (a *API) sqlBatchAction(w http.ResponseWriter, r *http.Request, action string) {
	input := core.SQLBatchRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	engine, ok := a.engine.(core.SQLBatchEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "SQL-Werkzeuge sind nicht verfügbar.")
		return
	}
	var result any
	var err error
	switch action {
	case "batch":
		result, err = engine.SQLBatch(r.Context(), input)
	case "explain":
		result, err = engine.SQLExplain(r.Context(), input)
	case "simulate":
		result, err = engine.SQLSimulate(r.Context(), input)
	}
	if err != nil {
		var p *core.Error
		if errors.As(err, &p) && p.Code == "invalid_sqlbatch" {
			fail(w, 400, p.Code, p.Message)
		} else {
			engineError(w, err)
		}
		return
	}
	writeJSON(w, 200, result)
}
