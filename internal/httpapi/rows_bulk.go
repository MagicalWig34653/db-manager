// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerRowsBulk(mux *http.ServeMux) {
	mux.Handle("GET /api/v19/capabilities", a.auth(http.HandlerFunc(a.rowsBulkCapabilities)))
	mux.Handle("POST /api/v19/rows/plan", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.rowsBulk(w, r, false) })))
	mux.Handle("POST /api/v19/rows/execute", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.rowsBulk(w, r, true) })))
	mux.Handle("POST /api/v19/values/upload", a.auth(http.HandlerFunc(a.fileValueUpload)))
	mux.Handle("POST /api/v19/values/download", a.auth(http.HandlerFunc(a.fileValueDownload)))
	mux.Handle("POST /api/v19/foreign-keys/lookup", a.auth(http.HandlerFunc(a.foreignKeyLookup)))
}
func (a *API) rowsBulkCapabilities(w http.ResponseWriter, r *http.Request) {
	_, available := a.engine.(core.RowsBulkEngine)
	writeJSON(w, 200, map[string]any{"contract_version": core.RowsBulkContractVersion, "available": available, "max_rows": core.MaxBulkRows, "max_file_bytes": core.MaxFileValueBytes, "max_request_bytes": MaxRequestBytes, "max_timeout_ms": 60000, "max_lookup_page_size": 100, "drivers": []map[string]string{
		{"driver": "sqlite", "bulk": "limited", "file_values": "limited", "foreign_key_lookup": "limited", "reason": "Nur geeignete Basistabellen mit stabilem Primärschlüssel; Binärspalten; einfache Fremdschlüssel; SQLite-Sandbox."},
		{"driver": "mysql", "bulk": "limited", "file_values": "limited", "foreign_key_lookup": "limited", "reason": "Auch MariaDB; schreibend nur transaktionale geeignete Tabellen; Binärspalten; einfache Fremdschlüssel."},
		{"driver": "postgres", "bulk": "limited", "file_values": "limited", "foreign_key_lookup": "limited", "reason": "Keine Vererbung, Partitionen oder Rewrite-Regeln im Schreibpfad; BYTEA; einfache Fremdschlüssel."},
	}})
}
func (a *API) rowsBulkEngine(w http.ResponseWriter) (core.RowsBulkEngine, bool) {
	e, ok := a.engine.(core.RowsBulkEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Mehrzeilen und Dateiwerte sind nicht verfügbar.")
	}
	return e, ok
}

// Presence checks preserve the distinction between missing fields and real empty values.
func decodeRowsBulk(w http.ResponseWriter, r *http.Request, target any) bool {
	var captured bytes.Buffer
	r.Body = io.NopCloser(io.TeeReader(r.Body, &captured))
	if !decode(w, r, target) {
		return false
	}
	var raw any
	if json.Unmarshal(captured.Bytes(), &raw) != nil || !rowsBulkPresence(reflect.TypeOf(target).Elem(), raw) {
		fail(w, 400, "invalid_request", "Pflichtfelder fehlen.")
		return false
	}
	return true
}
func rowsBulkPresence(t reflect.Type, v any) bool {
	if t.Kind() == reflect.Struct {
		obj, ok := v.(map[string]any)
		if !ok {
			return false
		}
		if t == reflect.TypeOf(core.Cell{}) {
			kind, ok := obj["kind"].(string)
			if !ok {
				return false
			}
			_, value := obj["value"]
			return (kind == "null" && !value) || (kind != "null" && value)
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "" || tag[0] == "-" {
				continue
			}
			val, exists := obj[tag[0]]
			if !exists {
				if len(tag) == 1 {
					return false
				}
				continue
			}
			if !rowsBulkPresence(f.Type, val) {
				return false
			}
		}
	} else if t.Kind() == reflect.Slice {
		list, ok := v.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if !rowsBulkPresence(t.Elem(), item) {
				return false
			}
		}
	}
	return true
}
func rowsBulkError(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "invalid_bulk", "invalid_mutation":
			fail(w, 400, p.Code, p.Message)
			return
		case "row_conflict", "plan_conflict":
			fail(w, 409, p.Code, p.Message)
			return
		case "file_too_large":
			fail(w, 413, p.Code, p.Message)
			return
		}
	}
	mutationError(w, err)
}
func (a *API) rowsBulk(w http.ResponseWriter, r *http.Request, execute bool) {
	input := core.RowsBulkRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decodeRowsBulk(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	e, ok := a.rowsBulkEngine(w)
	if !ok {
		return
	}
	if !execute {
		if input.Confirm || input.ExpectedPlanSHA256 != "" {
			fail(w, 400, "invalid_request", "Plan enthält keine Ausführungsbestätigung.")
			return
		}
		result, err := e.RowsBulkPlan(r.Context(), input)
		if err != nil {
			rowsBulkError(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	result, err := e.RowsBulkExecute(r.Context(), input)
	if err != nil {
		status := 422
		var p *core.Error
		if errors.As(err, &p) {
			if p.Code == "row_conflict" || p.Code == "plan_conflict" {
				status = 409
			}
			if p.Code == "invalid_bulk" || p.Code == "invalid_mutation" {
				status = 400
			}
		}
		writeJSON(w, status, result)
		return
	}
	status := 200
	if result.Outcome == "outcome_unknown" {
		status = 503
	}
	writeJSON(w, status, result)
}
func (a *API) fileValueUpload(w http.ResponseWriter, r *http.Request) {
	e, ok := a.rowsBulkEngine(w)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 415, "unsupported_media_type", "Multipart mit metadata und file ist erforderlich.")
		return
	}
	metadata, err := reader.NextPart()
	if err != nil || metadata.FormName() != "metadata" {
		fail(w, 400, "invalid_request", "Metadaten müssen zuerst gesendet werden.")
		return
	}
	input := core.FileValueRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	mr := r.Clone(r.Context())
	mr.Header = r.Header.Clone()
	mr.Header.Set("Content-Type", "application/json")
	mr.Body = io.NopCloser(metadata)
	if !decodeRowsBulk(w, mr, &input) || !validConnection(w, input.Connection) {
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" {
		fail(w, 400, "invalid_request", "Dateiteil fehlt.")
		return
	}
	// Finish and validate the multipart envelope before any database write.
	data, err := io.ReadAll(io.LimitReader(part, core.MaxFileValueBytes+1))
	if err != nil {
		fail(w, 400, "invalid_file", "Datei konnte nicht gelesen werden.")
		return
	}
	if len(data) > core.MaxFileValueBytes {
		fail(w, 413, "file_too_large", "Dateiwert überschreitet das Größenlimit.")
		return
	}
	if _, err = reader.NextPart(); err != io.EOF {
		fail(w, 400, "invalid_request", "Genau metadata und file erforderlich.")
		return
	}
	result, err := e.FileValueUpload(r.Context(), input, bytes.NewReader(data))
	if err != nil {
		rowsBulkError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *API) fileValueDownload(w http.ResponseWriter, r *http.Request) {
	input := core.FileValueRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decodeRowsBulk(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	e, ok := a.rowsBulkEngine(w)
	if !ok {
		return
	}
	started := false
	err := e.FileValueDownload(r.Context(), input, func(media string, size int64) (io.Writer, error) {
		w.Header().Set("Content-Type", media)
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Content-Disposition", `attachment; filename="wert.bin"`)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cache-Control", "no-store")
		started = true
		return w, nil
	})
	if err != nil && !started {
		rowsBulkError(w, err)
	}
}
func (a *API) foreignKeyLookup(w http.ResponseWriter, r *http.Request) {
	input := core.ForeignKeyLookupRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decodeRowsBulk(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	e, ok := a.rowsBulkEngine(w)
	if !ok {
		return
	}
	result, err := e.ForeignKeyLookup(r.Context(), input)
	if err != nil {
		rowsBulkError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
