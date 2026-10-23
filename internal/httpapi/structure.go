// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerStructure(mux *http.ServeMux) {
	mux.Handle("GET /api/v5/capabilities", a.auth(http.HandlerFunc(a.structureCapabilities)))
	mux.Handle("POST /api/v5/tables/structure", a.auth(http.HandlerFunc(a.tableStructure)))
}
func (a *API) structureCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.StructureEngine); ok {
		features = []string{"table_structure"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.StructureContractVersion, "features": features, "mutations": false, "max_objects_per_section": databaseStructureObjects, "max_terms": databaseStructureTerms, "timeout_ms": 10000})
}

// These public API limits mirror the bounded engine contract, without an HTTP-to-DB dependency.
const databaseStructureObjects = 1024
const databaseStructureTerms = 8192

func (a *API) tableStructure(w http.ResponseWriter, r *http.Request) {
	request := core.DescribeRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.engine.(core.StructureEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Strukturmetadaten sind nicht verfügbar.")
		return
	}
	result, err := engine.Structure(r.Context(), request)
	if err != nil {
		engineError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
