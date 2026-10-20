// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerTables(mux *http.ServeMux) {
	mux.Handle("GET /api/v2/capabilities", a.auth(http.HandlerFunc(a.tableCapabilities)))
	mux.Handle("POST /api/v2/tables/describe", a.auth(http.HandlerFunc(a.describeTable)))
	mux.Handle("POST /api/v2/tables/browse", a.auth(http.HandlerFunc(a.browseTable)))
}
func (a *API) tableCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.TableEngine); ok {
		features = []string{"table_describe", "table_browse"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.TableContractVersion, "features": features, "mutations": false, "max_page_size": 1000, "max_offset": 1000000, "max_filters": 16, "max_sort": 16})
}
func (a *API) tableEngine(w http.ResponseWriter) (core.TableEngine, bool) {
	engine, ok := a.engine.(core.TableEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Tabellenzugriff ist nicht verfügbar.")
	}
	return engine, ok
}
func (a *API) describeTable(w http.ResponseWriter, r *http.Request) {
	request := core.DescribeRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.tableEngine(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := engine.Describe(ctx, request)
	if err != nil {
		engineError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) browseTable(w http.ResponseWriter, r *http.Request) {
	request := core.BrowseRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.tableEngine(w)
	if !ok {
		return
	}
	result, err := engine.Browse(r.Context(), request)
	if err != nil {
		engineError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
