// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerPartitions(mux *http.ServeMux) {
	mux.Handle("GET /api/v21/capabilities", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		features := []string{}
		if _, ok := a.engine.(core.PartitionsEngine); ok {
			features = []string{"partitions", "prefix_batch"}
		}
		writeJSON(w, 200, map[string]any{"contract_version": core.PartitionsContractVersion, "features": features, "drivers": core.PartitionsCapabilities(), "max_tables": core.PrefixMaxTables, "max_partitions": 100, "max_timeout_ms": 60000, "default_timeout_ms": 30000, "max_request_bytes": MaxRequestBytes})
	})))
	for _, kind := range []string{"partitions", "prefix"} {
		for _, mode := range []string{"list", "plan", "execute"} {
			mux.Handle("POST /api/v21/tables/"+kind+"/"+mode, a.auth(a.partitionHandler(kind, mode)))
		}
	}
}
func (a *API) partitionHandler(kind, mode string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engine, ok := a.engine.(core.PartitionsEngine)
		// Decode before capability checks, as on the existing structural routes.
		var out any
		var err error
		if kind == "partitions" {
			if mode == "execute" {
				var in core.StructureExecuteRequest
				in.Connection.TLSMode = "verify-full"
				if !decode(w, r, &in) || !validConnection(w, in.Connection) {
					return
				}
				if !ok {
					fail(w, 422, "unsupported_feature", "Partitionsoperationen sind nicht verfügbar.")
					return
				}
				out, err = engine.ExecutePartitions(r.Context(), in)
			} else {
				var in core.PartitionRequest
				in.Connection.TLSMode = "verify-full"
				if !decode(w, r, &in) || !validConnection(w, in.Connection) {
					return
				}
				if !ok {
					fail(w, 422, "unsupported_feature", "Partitionsoperationen sind nicht verfügbar.")
					return
				}
				if mode == "list" {
					out, err = engine.Partitions(r.Context(), in)
				} else {
					out, err = engine.PlanPartitions(r.Context(), in)
				}
			}
		} else {
			if mode == "execute" {
				var in core.PrefixExecuteRequest
				in.Connection.TLSMode = "verify-full"
				if !decode(w, r, &in) || !validConnection(w, in.Connection) {
					return
				}
				if !ok {
					fail(w, 422, "unsupported_feature", "Präfixoperationen sind nicht verfügbar.")
					return
				}
				out, err = engine.ExecutePrefix(r.Context(), in)
			} else {
				var in core.PrefixRequest
				in.Connection.TLSMode = "verify-full"
				if !decode(w, r, &in) || !validConnection(w, in.Connection) {
					return
				}
				if !ok {
					fail(w, 422, "unsupported_feature", "Präfixoperationen sind nicht verfügbar.")
					return
				}
				if mode == "list" {
					out, err = engine.PrefixTables(r.Context(), in)
				} else {
					out, err = engine.PlanPrefix(r.Context(), in)
				}
			}
		}
		if err != nil {
			objectOpFail(w, err)
			return
		}
		writeJSON(w, 200, out)
	})
}
