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
	// Pointer fields preserve required JSON value presence without changing Cell.
	var payload struct {
		Connection core.Connection `json:"connection"`
		Table      core.TableRef   `json:"table"`
		PageSize   int             `json:"page_size,omitempty"`
		Offset     int             `json:"offset,omitempty"`
		Sort       []struct {
			Column    *string `json:"column"`
			Direction string  `json:"direction"`
		} `json:"sort,omitempty"`
		Filters []struct {
			Column   *string `json:"column"`
			Operator string  `json:"operator"`
			Value    *struct {
				Kind  *string `json:"kind"`
				Value *string `json:"value"`
			} `json:"value,omitempty"`
		} `json:"filters,omitempty"`
		TimeoutMS int `json:"timeout_ms,omitempty"`
	}
	payload.Connection.TLSMode = "verify-full"
	if !decode(w, r, &payload) || !validConnection(w, payload.Connection) {
		return
	}
	request := core.BrowseRequest{Connection: payload.Connection, Table: payload.Table, PageSize: payload.PageSize, Offset: payload.Offset, TimeoutMS: payload.TimeoutMS}
	for _, item := range payload.Sort {
		if item.Column == nil {
			fail(w, 400, "invalid_request", "Sortierung benötigt eine Spalte.")
			return
		}
		request.Sort = append(request.Sort, core.TableSort{Column: *item.Column, Direction: item.Direction})
	}
	for _, filter := range payload.Filters {
		if filter.Column == nil {
			fail(w, 400, "invalid_request", "Filter benötigt eine Spalte.")
			return
		}
		converted := core.TableFilter{Column: *filter.Column, Operator: filter.Operator}
		if filter.Value != nil {
			if filter.Value.Kind == nil || filter.Value.Value == nil {
				fail(w, 400, "invalid_request", "Filterwert benötigt Art und Wert.")
				return
			}
			converted.Value = &core.Cell{Kind: *filter.Value.Kind, Value: *filter.Value.Value}
		}
		request.Filters = append(request.Filters, converted)
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
