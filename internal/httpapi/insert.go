// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerInserts(mux *http.ServeMux) {
	mux.Handle("GET /api/v4/capabilities", a.auth(http.HandlerFunc(a.insertCapabilities)))
	mux.Handle("POST /api/v4/tables/insert-eligibility", a.auth(http.HandlerFunc(a.insertEligibility)))
	mux.Handle("POST /api/v4/rows/insert", a.auth(http.HandlerFunc(a.insertRow)))
}
func (a *API) insertEngine(w http.ResponseWriter) (core.InsertEngine, bool) {
	e, ok := a.engine.(core.InsertEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Einfügen ist nicht verfügbar.")
	}
	return e, ok
}
func (a *API) insertCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.InsertEngine); ok {
		features = []string{"row_insert_eligibility", "row_insert"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.InsertContractVersion, "features": features, "max_rows_per_insert": 1, "max_request_bytes": MaxRequestBytes, "max_timeout_ms": 60000})
}
func (a *API) insertEligibility(w http.ResponseWriter, r *http.Request) {
	request := core.DescribeRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	engine, ok := a.insertEngine(w)
	if !ok {
		return
	}
	result, err := engine.InsertEligibility(r.Context(), request)
	if err != nil {
		insertHTTPError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) insertRow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Connection core.Connection `json:"connection"`
		Table      core.TableRef   `json:"table"`
		Values     *[]struct {
			Column *string            `json:"column"`
			Value  *inputMutationCell `json:"value"`
		} `json:"values"`
		Confirm   *bool `json:"confirm"`
		TimeoutMS int   `json:"timeout_ms,omitempty"`
	}
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	invalid := func() {
		fail(w, 400, "invalid_request", "Vollständige Einfügewerte und ausdrückliche Schreibbestätigung erforderlich.")
	}
	if input.Values == nil || input.Confirm == nil || !*input.Confirm {
		invalid()
		return
	}
	request := core.InsertRequest{Connection: input.Connection, Table: input.Table, Values: []core.RowChange{}, Confirm: true, TimeoutMS: input.TimeoutMS}
	for _, value := range *input.Values {
		if value.Column == nil || value.Value == nil {
			invalid()
			return
		}
		cell, ok := value.Value.cell()
		if !ok {
			invalid()
			return
		}
		request.Values = append(request.Values, core.RowChange{Column: *value.Column, Value: cell})
	}
	engine, ok := a.insertEngine(w)
	if !ok {
		return
	}
	result, err := engine.Insert(r.Context(), request)
	if err != nil {
		insertHTTPError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func insertHTTPError(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) && p.Code == "insert_conflict" {
		fail(w, 409, p.Code, p.Message)
		return
	}
	mutationError(w, err)
}
