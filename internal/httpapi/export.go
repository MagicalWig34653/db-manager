// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func (a *API) registerExport(mux *http.ServeMux) {
	mux.Handle("GET /api/v11/capabilities", a.auth(http.HandlerFunc(a.exportCapabilities)))
	mux.Handle("POST /api/v11/export", a.auth(WithRouteClass(ClassStandard, http.HandlerFunc(a.exportDownload))))
}
func (a *API) exportCapabilities(w http.ResponseWriter, r *http.Request) {
	_, available := a.engine.(core.ExportEngine)
	writeJSON(w, 200, map[string]any{
		"contract_version": core.ExportContractVersion, "available": available,
		"formats": []string{"sql", "csv", "excel-csv", "json"}, "charsets": []string{"utf-8", "utf-8-bom"},
		"drivers": []map[string]any{
			{"driver": "sqlite", "snapshot": "exclusive_local_transaction", "sql_server": false, "state": "limited", "reason": "ADR-0004: exklusiv, lokal, eine Verbindung; keine WAL-/HA-Zusage."},
			{"driver": "mysql", "snapshot": "repeatable_read_innodb_only", "sql_server": true, "state": "limited", "reason": "Nur InnoDB; kein Snapshot über mehrere Datenbanken."},
			{"driver": "postgres", "snapshot": "repeatable_read", "sql_server": false, "state": "limited", "reason": "DDL aus Katalog rekonstruiert; keine vollständige Sicherung oder SQL-Serverexport."},
		},
		"default_max_bytes": core.ExportDefaultBytes, "max_bytes": core.ExportMaxBytes,
		"default_max_rows": core.ExportDefaultRows, "max_rows": core.ExportMaxRows,
		"default_timeout_ms": core.ExportDefaultTimeoutMS, "max_timeout_ms": core.ExportMaxTimeoutMS,
		"max_tables": core.ExportMaxTables, "max_request_bytes": MaxRequestBytes,
	})
}

// downloadWriter delays HTTP 200 until the engine finishes metadata validation.
type downloadWriter struct {
	response  http.ResponseWriter
	request   core.ExportRequest
	started   bool
	gzip      *gzip.Writer
	pending   int
	lastFlush time.Time
}

func (w *downloadWriter) Write(p []byte) (int, error) {
	if !w.started {
		w.started = true
		media, extension := "application/sql; charset=utf-8", "sql"
		switch w.request.Format {
		case "json":
			media, extension = "application/json; charset=utf-8", "json"
		case "csv", "excel-csv":
			media, extension = "text/csv; charset=utf-8", "csv"
		}
		filename := w.request.Filename
		if filename == "" {
			filename = "export." + extension
		}
		if w.request.Gzip {
			filename += ".gz"
			media = "application/gzip"
			w.gzip = gzip.NewWriter(w.response)
		}
		w.response.Header().Set("Content-Type", media)
		w.response.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		snapshot := "per-database"
		if w.request.Scope == "query" {
			snapshot = "read-only-best-effort"
		}
		w.response.Header().Set("X-Export-Snapshot", snapshot)
		w.response.Header().Set("Trailer", "X-Export-Status, X-Export-Error")
		w.response.WriteHeader(http.StatusOK)
	}
	var out io.Writer = w.response
	if w.gzip != nil {
		out = w.gzip
	}
	n, err := out.Write(p)
	w.pending += n
	// Bounded periodic flushing propagates disconnects without forcing a GZip
	// sync block for every JSON punctuation token.
	if err == nil && (w.lastFlush.IsZero() || w.pending >= 64<<10 || time.Since(w.lastFlush) >= 100*time.Millisecond) {
		if w.gzip != nil {
			err = w.gzip.Flush()
		}
		if err == nil {
			err = http.NewResponseController(w.response).Flush()
			if errors.Is(err, http.ErrNotSupported) {
				err = nil
			}
		}
		w.pending = 0
		w.lastFlush = time.Now()
	}
	return n, err
}
func (a *API) exportDownload(w http.ResponseWriter, r *http.Request) {
	request := core.ExportRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &request) || !validConnection(w, request.Connection) {
		return
	}
	request, err := core.NormalizeExport(request)
	if err != nil {
		fail(w, 400, "invalid_export", "Exportoptionen sind ungültig oder nicht unterstützt.")
		return
	}
	engine, ok := a.engine.(core.ExportEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Export ist nicht verfügbar.")
		return
	}
	controller := http.NewResponseController(w)
	// Export has a separate duration budget, beyond the standard server timeout.
	_ = controller.SetWriteDeadline(time.Now().Add(time.Duration(request.TimeoutMS) * time.Millisecond))
	defer controller.SetWriteDeadline(time.Time{})
	stream := &downloadWriter{response: w, request: request}
	err = engine.Export(r.Context(), request, stream)
	if stream.gzip != nil {
		closeErr := stream.gzip.Close()
		if err == nil {
			err = closeErr
		}
	}
	if !stream.started {
		if err != nil {
			var p *core.Error
			if errors.As(err, &p) && p.Code == "invalid_export" {
				fail(w, 400, p.Code, p.Message)
			} else {
				engineError(w, err)
			}
		} else {
			fail(w, 500, "export_failed", "Export lieferte keine Daten.")
		}
		return
	}
	status := "complete"
	code := ""
	if err != nil {
		status = "incomplete"
		code = "export_failed"
		var p *core.Error
		if errors.As(err, &p) {
			code = p.Code
		}
	}
	w.Header().Set("X-Export-Status", status)
	if code != "" {
		w.Header().Set("X-Export-Error", code)
	}
}
