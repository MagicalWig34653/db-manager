// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerSearch(mux *http.ServeMux) {
	mux.Handle("GET /api/v14/capabilities", a.auth(http.HandlerFunc(a.searchCapabilities)))
	for _, path := range []string{"search", "tables/browse", "replace/plan", "replace"} {
		mux.Handle("POST /api/v14/"+path, a.auth(a.searchHandler(path)))
	}
}
func (a *API) searchCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.SearchEngine); ok {
		features = []string{"search", "browse", "replace"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.SearchContractVersion, "features": features, "drivers": core.SearchCapabilities(), "max_tables": core.SearchMaxTables, "max_rows": core.SearchMaxRows, "max_preview_rows": core.SearchMaxPreview, "max_value_bytes": core.SearchMaxValue, "default_timeout_ms": 10000, "max_timeout_ms": 60000})
}
func searchFail(w http.ResponseWriter, err error) {
	if p, ok := err.(*core.Error); ok {
		switch p.Code {
		case "invalid_search", "confirmation_mismatch":
			fail(w, 400, p.Code, p.Message)
			return
		case "plan_mismatch", "row_conflict":
			fail(w, 409, p.Code, p.Message)
			return
		case "replace_limit", "unsupported_search":
			fail(w, 422, p.Code, p.Message)
			return
		case "table_not_found":
			fail(w, 404, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}
func (a *API) searchHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engine, ok := a.engine.(core.SearchEngine)
		if !ok {
			fail(w, 422, "unsupported_feature", "Suchfunktionen sind nicht verfügbar.")
			return
		}
		var result any
		var err error
		switch path {
		case "search":
			input := core.SearchRequest{Connection: core.Connection{TLSMode: "verify-full"}}
			if !decode(w, r, &input) || !validConnection(w, input.Connection) {
				return
			}
			result, err = engine.Search(r.Context(), input)
		case "tables/browse":
			var input struct {
				Connection core.Connection `json:"connection"`
				Table      *core.TableRef  `json:"table"`
				PageSize   int             `json:"page_size,omitempty"`
				Offset     int             `json:"offset,omitempty"`
				Sort       []struct {
					Column    *string `json:"column"`
					Direction string  `json:"direction"`
				} `json:"sort,omitempty"`
				Filters   []filterInput `json:"filters,omitempty"`
				TimeoutMS int           `json:"timeout_ms,omitempty"`
			}
			input.Connection.TLSMode = "verify-full"
			if !decode(w, r, &input) || !validConnection(w, input.Connection) {
				return
			}
			if input.Table == nil {
				fail(w, 400, "invalid_request", "Tabelle ist erforderlich.")
				return
			}
			sorts := []core.TableSort{}
			for _, item := range input.Sort {
				if item.Column == nil {
					fail(w, 400, "invalid_request", "Sortierspalte ist erforderlich.")
					return
				}
				sorts = append(sorts, core.TableSort{Column: *item.Column, Direction: item.Direction})
			}
			filters, ok := tableFilters(w, input.Filters)
			if !ok {
				return
			}
			result, err = engine.SearchBrowse(r.Context(), core.BrowseRequest{Connection: input.Connection, Table: *input.Table, PageSize: input.PageSize, Offset: input.Offset, Sort: sorts, Filters: filters, TimeoutMS: input.TimeoutMS})
		case "replace/plan":
			input := core.ReplaceRequest{Connection: core.Connection{TLSMode: "verify-full"}}
			if !decode(w, r, &input) || !validConnection(w, input.Connection) {
				return
			}
			result, err = engine.ReplacePlan(r.Context(), input)
		case "replace":
			var input struct {
				core.ReplaceRequest
				Confirm *struct {
					Action       string `json:"action"`
					TargetName   string `json:"target_name"`
					AffectedRows *int   `json:"affected_rows"`
				} `json:"confirm"`
				ExpectedPlanSHA256 *string `json:"expected_plan_sha256"`
			}
			input.Connection.TLSMode = "verify-full"
			if !decode(w, r, &input) || !validConnection(w, input.Connection) {
				return
			}
			if input.Confirm == nil || input.Confirm.AffectedRows == nil || input.ExpectedPlanSHA256 == nil {
				fail(w, 400, "invalid_request", "Bestätigung und Plan-Hash sind erforderlich.")
				return
			}
			result, err = engine.ReplaceExecute(r.Context(), core.ReplaceExecuteRequest{ReplaceRequest: input.ReplaceRequest, Confirm: core.ReplaceConfirm{Action: input.Confirm.Action, TargetName: input.Confirm.TargetName, AffectedRows: *input.Confirm.AffectedRows}, ExpectedPlanSHA256: *input.ExpectedPlanSHA256})
		}
		if err != nil {
			searchFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}

type filterInput struct {
	Column   *string `json:"column"`
	Operator string  `json:"operator"`
	Value    *struct {
		Kind  *string `json:"kind"`
		Value *string `json:"value"`
	} `json:"value,omitempty"`
}

func tableFilters(w http.ResponseWriter, input []filterInput) ([]core.TableFilter, bool) {
	result := []core.TableFilter{}
	for _, f := range input {
		if f.Column == nil {
			fail(w, 400, "invalid_request", "Filterspalte ist erforderlich.")
			return nil, false
		}
		out := core.TableFilter{Column: *f.Column, Operator: f.Operator}
		if f.Value != nil {
			if f.Value.Kind == nil || f.Value.Value == nil {
				fail(w, 400, "invalid_request", "Filterwert benötigt Art und Wert.")
				return nil, false
			}
			out.Value = &core.Cell{Kind: *f.Value.Kind, Value: *f.Value.Value}
		}
		result = append(result, out)
	}
	return result, true
}
