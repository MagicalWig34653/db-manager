// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"errors"
	"net/http"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerCreate(mux *http.ServeMux) {
	mux.Handle("GET /api/v6/capabilities", a.auth(http.HandlerFunc(a.createCapabilities)))
	mux.Handle("POST /api/v6/tables/create", a.auth(http.HandlerFunc(a.createTable)))
}
func (a *API) createCapabilities(w http.ResponseWriter, r *http.Request) {
	features := []string{}
	if _, ok := a.engine.(core.CreateEngine); ok {
		features = []string{"table_create"}
	}
	writeJSON(w, 200, map[string]any{"contract_version": core.CreateContractVersion, "features": features, "types": []string{"int64", "text", "binary", "decimal"}, "max_columns": 32, "max_primary_key_columns": 8, "max_text_length": 1024, "max_decimal_precision": 38, "max_timeout_ms": 60000, "max_request_bytes": MaxRequestBytes})
}
func (a *API) createTable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Connection core.Connection `json:"connection"`
		Table      *core.TableRef  `json:"table"`
		Columns    *[]struct {
			Name *string `json:"name"`
			Type *struct {
				Kind      *string `json:"kind"`
				Length    *int    `json:"length,omitempty"`
				Precision *int    `json:"precision,omitempty"`
				Scale     *int    `json:"scale,omitempty"`
			} `json:"type"`
			Nullable *bool `json:"nullable"`
			Default  *struct {
				Mode  *string            `json:"mode"`
				Value *inputMutationCell `json:"value,omitempty"`
			} `json:"default"`
		} `json:"columns"`
		PrimaryKey *[]string `json:"primary_key"`
		Confirm    *bool     `json:"confirm"`
		TimeoutMS  int       `json:"timeout_ms,omitempty"`
	}
	input.Connection.TLSMode = "verify-full"
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	invalid := func() {
		fail(w, 400, "invalid_request", "Vollständige strukturierte Definition und ausdrückliche Bestätigung erforderlich.")
	}
	if input.Table == nil || input.Columns == nil || input.PrimaryKey == nil || input.Confirm == nil || !*input.Confirm {
		invalid()
		return
	}
	request := core.CreateRequest{Connection: input.Connection, Table: *input.Table, PrimaryKey: *input.PrimaryKey, Confirm: true, TimeoutMS: input.TimeoutMS, Columns: []core.CreateColumn{}}
	for _, column := range *input.Columns {
		if column.Name == nil || column.Type == nil || column.Type.Kind == nil || column.Nullable == nil || column.Default == nil || column.Default.Mode == nil {
			invalid()
			return
		}
		typ := core.CreateType{Kind: *column.Type.Kind}
		switch typ.Kind {
		case "int64", "binary":
			if column.Type.Length != nil || column.Type.Precision != nil || column.Type.Scale != nil {
				invalid()
				return
			}
		case "text":
			if column.Type.Length == nil || column.Type.Precision != nil || column.Type.Scale != nil {
				invalid()
				return
			}
			typ.Length = *column.Type.Length
		case "decimal":
			if column.Type.Length != nil || column.Type.Precision == nil || column.Type.Scale == nil {
				invalid()
				return
			}
			typ.Precision = *column.Type.Precision
			typ.Scale = *column.Type.Scale
		default:
			invalid()
			return
		}
		def := core.CreateDefault{Mode: *column.Default.Mode}
		if def.Mode == "value" {
			if column.Default.Value == nil {
				invalid()
				return
			}
			cell, ok := column.Default.Value.cell()
			if !ok {
				invalid()
				return
			}
			def.Value = &cell
		} else if column.Default.Value != nil {
			invalid()
			return
		}
		request.Columns = append(request.Columns, core.CreateColumn{Name: *column.Name, Type: typ, Nullable: *column.Nullable, Default: def})
	}
	engine, ok := a.engine.(core.CreateEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Tabellenanlage ist nicht verfügbar.")
		return
	}
	result, err := engine.CreateTable(r.Context(), request)
	if err != nil {
		var p *core.Error
		if errors.As(err, &p) {
			if p.Code == "invalid_create" {
				fail(w, 400, p.Code, p.Message)
				return
			}
			if p.Code == "create_conflict" {
				fail(w, 409, p.Code, p.Message)
				return
			}
		}
		mutationError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
