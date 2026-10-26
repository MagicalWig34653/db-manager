// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// ImportUploadLimit includes 1 MiB for multipart framing and metadata. File and
// expanded contents independently stay at or below 64 MiB.
const ImportUploadLimit = core.MaxImportBytes + MaxRequestBytes

type importJobStatus struct {
	ID              string              `json:"id"`
	State           string              `json:"state"`
	CancelRequested bool                `json:"cancel_requested"`
	Progress        core.ImportProgress `json:"progress"`
	Result          *core.ImportResult  `json:"result,omitempty"`
}
type importJob struct {
	status   importJobStatus
	cancel   context.CancelFunc
	finished time.Time
}
type importJobs struct {
	mu    sync.Mutex
	jobs  map[string]*importJob
	slots chan struct{}
}

func (a *API) registerImport(mux *http.ServeMux) {
	jobs := &importJobs{jobs: map[string]*importJob{}, slots: make(chan struct{}, 2)}
	mux.Handle("GET /api/v18/capabilities", a.auth(http.HandlerFunc(a.importCapabilities)))
	for _, action := range []string{"plan", "execute"} {
		mux.Handle("POST /api/v18/imports/"+action, a.auth(WithRouteClass(ClassUpload, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { a.importUpload(w, r, action, jobs) }))))
	}
	mux.Handle("GET /api/v18/imports/{id}", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jobs.status(w, r, false) })))
	mux.Handle("POST /api/v18/imports/{id}/cancel", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jobs.status(w, r, true) })))
}
func (a *API) importCapabilities(w http.ResponseWriter, r *http.Request) {
	_, ok := a.engine.(core.ImportEngine)
	writeJSON(w, 200, map[string]any{"contract_version": core.ImportContractVersion, "available": ok, "max_file_bytes": core.MaxImportBytes, "max_expanded_bytes": core.MaxImportBytes, "max_request_bytes": ImportUploadLimit, "max_rows": core.MaxImportRows, "max_statements": core.MaxImportStatements, "max_statement_bytes": 1 << 20, "max_timeout_ms": 300000, "max_jobs": 2, "retention_seconds": 300, "formats": []string{"sql", "csv"}, "compression": []string{"none", "gzip"}, "charsets": []string{"utf-8", "iso-8859-1", "windows-1252"}, "confirmation_required": true, "drivers": []map[string]string{
		{"driver": "sqlite", "sql": "limited", "csv": "limited", "reason": "Verwaltete SQLite-Sandbox; CSV nur vorhandene Basistabellen und portable Typteilmenge."},
		{"driver": "mysql", "sql": "limited", "csv": "limited", "reason": "Auch MariaDB: DDL mit implizitem Commit; CSV nur InnoDB, keine universelle Rücknahme von Triggern/externen Effekten."},
		{"driver": "postgres", "sql": "limited", "csv": "limited", "reason": "CSV ohne Partitionen, Vererbung, Regeln oder Fremdtabellen; Sequenzen/externe Effekte nicht rücknehmbar."},
	}})
}
func readImportUpload(w http.ResponseWriter, r *http.Request) (core.ImportRequest, []byte, bool) {
	input := core.ImportRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	r.Body = http.MaxBytesReader(w, r.Body, requestBodyLimit(r))
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" {
		fail(w, 415, "unsupported_media_type", "Multipart-Upload ist erforderlich.")
		return input, nil, false
	}
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 400, "invalid_import", "Ungültiger Upload.")
		return input, nil, false
	}
	var metadata, data []byte
	seen := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			importUploadError(w, err)
			return input, nil, false
		}
		name := part.FormName()
		if (name != "metadata" && name != "file") || seen[name] {
			part.Close()
			fail(w, 400, "invalid_import", "Genau Metadaten und eine Datei sind erforderlich.")
			return input, nil, false
		}
		seen[name] = true
		limit := int64(core.MaxImportBytes)
		if name == "metadata" {
			limit = MaxRequestBytes
		}
		// Pre-size from Content-Length so a large file is read once without doubling garbage.
		var buf bytes.Buffer
		if name == "file" && r.ContentLength > 0 {
			buf.Grow(int(min(r.ContentLength, limit+1)))
		}
		_, err = buf.ReadFrom(io.LimitReader(part, limit+1))
		b := buf.Bytes()
		if err != nil {
			part.Close()
			importUploadError(w, err)
			return input, nil, false
		}
		if int64(len(b)) > limit {
			part.Close()
			fail(w, 413, "import_too_large", "Uploadteil ist zu groß.")
			return input, nil, false
		}
		part.Close()
		if name == "metadata" {
			metadata = b
		} else {
			data = b
		}
	}
	// Drain the epilogue through MaxBytesReader too; no unbounded extra body.
	if _, err = io.Copy(io.Discard, r.Body); err != nil {
		importUploadError(w, err)
		return input, nil, false
	}
	if !seen["metadata"] || !seen["file"] || len(data) == 0 {
		fail(w, 400, "invalid_import", "Metadaten und nichtleere Datei sind erforderlich.")
		return input, nil, false
	}
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(metadata))
	clone.Header = r.Header.Clone()
	clone.Header.Set("Content-Type", "application/json")
	clone = clone.WithContext(context.WithValue(clone.Context(), bodyLimitKey{}, int64(MaxRequestBytes)))
	if !decode(w, clone, &input) || !validConnection(w, input.Connection) {
		return input, nil, false
	}
	return input, data, true
}
func importUploadError(w http.ResponseWriter, err error) {
	var size *http.MaxBytesError
	if errors.As(err, &size) {
		fail(w, 413, "request_too_large", "Upload ist zu groß.")
	} else {
		fail(w, 400, "invalid_import", "Upload konnte nicht vollständig gelesen werden.")
	}
}
func importHTTPError(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "plan_mismatch":
			fail(w, 409, p.Code, p.Message)
			return
		case "invalid_import", "confirmation_mismatch":
			fail(w, 400, p.Code, p.Message)
			return
		case "import_too_large":
			fail(w, 413, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}
func (a *API) importUpload(w http.ResponseWriter, r *http.Request, action string, jobs *importJobs) {
	engine, ok := a.engine.(core.ImportEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Import ist nicht verfügbar.")
		return
	}
	// Bound memory before reading a multipart body, including plan requests.
	select {
	case jobs.slots <- struct{}{}:
	default:
		fail(w, 503, "import_busy", "Bereits zwei Importe oder Vorschauen aktiv.")
		return
	}
	release := true
	defer func() {
		if release {
			<-jobs.slots
		}
	}()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(60 * time.Second))
	_ = controller.SetWriteDeadline(time.Now().Add(305 * time.Second))
	input, data, ok := readImportUpload(w, r)
	if !ok {
		return
	}
	_ = controller.SetWriteDeadline(time.Now().Add(305 * time.Second))
	if action == "plan" {
		p, err := engine.PlanImport(r.Context(), input, data)
		if err != nil {
			importHTTPError(w, err)
			return
		}
		writeJSON(w, 200, p)
		return
	}
	if input.Confirm == nil || input.Confirm.Action != "import_"+input.Format {
		fail(w, 400, "confirmation_mismatch", "Ausdrückliche Schreibzustimmung ist erforderlich.")
		return
	}
	if input.Format == "csv" {
		preview := input
		preview.Confirm = nil
		preview.ExpectedPlanSHA256 = ""
		p, err := engine.PlanImport(core.WithPlanPreflight(r.Context()), preview, data)
		if err != nil {
			importHTTPError(w, err)
			return
		}
		if err = dbplan.Check(p.Plan, input.ExpectedPlanSHA256, *input.Confirm, "import_csv", "", false); err != nil {
			fail(w, 409, "plan_mismatch", "Importplan hat sich geändert.")
			return
		}
	} else if input.Format != "sql" {
		fail(w, 400, "invalid_import", "Importformat ist ungültig.")
		return
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		fail(w, 503, "import_busy", "Import kann nicht gestartet werden.")
		return
	}
	id := hex.EncodeToString(random[:])
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	job := &importJob{status: importJobStatus{ID: id, State: "running"}, cancel: cancel}
	jobs.mu.Lock()
	jobs.expire()
	if len(jobs.jobs) >= 16 {
		jobs.mu.Unlock()
		cancel()
		fail(w, 503, "import_busy", "Ergebnisablage ist belegt; Ergebnisse laufen nach fünf Minuten ab.")
		return
	}
	jobs.jobs[id] = job
	jobs.mu.Unlock()
	release = false
	go func() {
		defer func() { cancel(); <-jobs.slots }()
		result, err := engine.ExecuteImport(ctx, input, data, func(p core.ImportProgress) { jobs.mu.Lock(); job.status.Progress = p; jobs.mu.Unlock() })
		if err != nil {
			result = core.ImportResult{Outcome: "rejected", Reason: "database_error", Statements: []core.SQLStatementResult{}, Errors: []core.ImportIssue{}, Warnings: []string{}}
			var p *core.Error
			if errors.As(err, &p) {
				result.Reason = p.Code
			} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				result.Reason = "query_canceled"
			}
		}
		jobs.mu.Lock()
		job.status.State = "finished"
		job.status.Result = &result
		job.finished = time.Now()
		jobs.mu.Unlock()
	}()
	w.Header().Set("Location", "/api/v18/imports/"+id)
	writeJSON(w, 202, importJobStatus{ID: id, State: "running", Progress: core.ImportProgress{}})
}

// Expiry is lazy; only bounded, credential-free results survive execution. Jobs
// cannot be resumed or automatically retried after expiry or process restart.
func (jobs *importJobs) expire() {
	for id, j := range jobs.jobs {
		if !j.finished.IsZero() && time.Since(j.finished) > 5*time.Minute {
			delete(jobs.jobs, id)
		}
	}
}
func (jobs *importJobs) status(w http.ResponseWriter, r *http.Request, cancel bool) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	jobs.expire()
	j := jobs.jobs[r.PathValue("id")]
	if j == nil {
		fail(w, 404, "import_not_found", "Import ist unbekannt oder abgelaufen.")
		return
	}
	if cancel && j.status.State == "running" {
		j.status.CancelRequested = true
		j.cancel()
	}
	// Marshal under the mutex: progress may change concurrently.
	data, err := json.Marshal(j.status)
	if err != nil {
		fail(w, 500, "database_error", "Status konnte nicht gelesen werden.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write(append(data, '\n'))
}
