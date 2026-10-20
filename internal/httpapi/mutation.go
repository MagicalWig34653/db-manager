// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

type inputMutationCell struct {
	Kind  *string `json:"kind"`
	Value *string `json:"value,omitempty"`
}

func (v inputMutationCell) cell() (core.Cell, bool) {
	if v.Kind == nil {
		return core.Cell{}, false
	}
	if *v.Kind == "null" {
		return core.Cell{Kind: "null"}, v.Value == nil
	}
	if v.Value == nil {
		return core.Cell{}, false
	}
	return core.Cell{Kind: *v.Kind, Value: *v.Value}, true
}

type mutationInput struct {
	Connection core.Connection `json:"connection"`
	Table      core.TableRef   `json:"table"`
	Expected   *struct {
		Columns *[]string            `json:"columns"`
		Values  *[]inputMutationCell `json:"values"`
	} `json:"expected"`
	Changes *[]struct {
		Column *string            `json:"column"`
		Value  *inputMutationCell `json:"value"`
	} `json:"changes,omitempty"`
	Confirm   *bool `json:"confirm"`
	TimeoutMS int   `json:"timeout_ms,omitempty"`
}

func (a *API) registerMutations(mux *http.ServeMux) {
	mux.Handle("GET /api/v3/capabilities", a.auth(http.HandlerFunc(a.mutationCapabilities)))
	mux.Handle("POST /api/v3/tables/eligibility", a.auth(http.HandlerFunc(a.mutationEligibility)))
	mux.Handle("POST /api/v3/rows/update", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.mutateRow(w, r, false) })))
	mux.Handle("POST /api/v3/rows/delete", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.mutateRow(w, r, true) })))
}
func (a *API) mutationEngine(w http.ResponseWriter) (core.MutationEngine, bool) {
	e, ok := a.engine.(core.MutationEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Zeilenänderung ist nicht verfügbar.")
	}
	return e, ok
}
func (a *API) mutationCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.MutationEngine); ok {
		features = []string{"row_eligibility", "row_update", "row_delete"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.MutationContractVersion, "features": features, "max_rows_per_mutation": 1, "max_request_bytes": MaxRequestBytes, "max_timeout_ms": 60000})
}
func (a *API) mutationEligibility(w http.ResponseWriter, r *http.Request) {
	request := core.DescribeRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.mutationEngine(w)
	if !ok {
		return
	}
	result, err := engine.Eligibility(r.Context(), request)
	if err != nil {
		mutationError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) mutateRow(w http.ResponseWriter, r *http.Request, deleting bool) {
	input := mutationInput{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	invalid := func() {
		fail(w, 400, "invalid_request", "Vollständiges Zeilenabbild und ausdrückliche Schreibbestätigung erforderlich.")
	}
	if input.Confirm == nil || !*input.Confirm || input.Expected == nil || input.Expected.Columns == nil || input.Expected.Values == nil || (deleting && input.Changes != nil) || (!deleting && input.Changes == nil) {
		invalid()
		return
	}
	request := core.MutationRequest{Connection: input.Connection, Table: input.Table, Confirm: true, TimeoutMS: input.TimeoutMS, Expected: core.RowSnapshot{Columns: *input.Expected.Columns, Values: []core.Cell{}}}
	for _, value := range *input.Expected.Values {
		cell, ok := value.cell()
		if !ok {
			invalid()
			return
		}
		request.Expected.Values = append(request.Expected.Values, cell)
	}
	if input.Changes != nil {
		for _, change := range *input.Changes {
			if change.Column == nil || change.Value == nil {
				invalid()
				return
			}
			cell, ok := change.Value.cell()
			if !ok {
				invalid()
				return
			}
			request.Changes = append(request.Changes, core.RowChange{Column: *change.Column, Value: cell})
		}
	}
	engine, ok := a.mutationEngine(w)
	if !ok {
		return
	}
	var result core.MutationResult
	var err error
	if deleting {
		result, err = engine.Delete(r.Context(), request)
	} else {
		result, err = engine.Update(r.Context(), request)
	}
	if err != nil {
		mutationError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func mutationError(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		if p.Code == "row_conflict" {
			fail(w, 409, p.Code, p.Message)
			return
		}
		if p.Code == "outcome_unknown" {
			fail(w, 503, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}
