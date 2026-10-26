// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func importHTTP(t *testing.T, api *API, action string, input any, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	meta, err := form.CreateFormField("metadata")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.NewEncoder(meta).Encode(input); err != nil {
		t.Fatal(err)
	}
	part, err := form.CreateFormFile("file", "../../ignored.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(file)
	_ = form.Close()
	request := httptest.NewRequest("POST", "/api/v18/imports/"+action, &body)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}
func importStatusHTTP(api *API, method, path string, auth bool) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	if auth {
		request.Header.Set("Authorization", "Bearer token")
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	return response
}
func awaitImportHTTP(t *testing.T, api *API, id string) importJobStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response := importStatusHTTP(api, "GET", "/api/v18/imports/"+id, true)
		var status importJobStatus
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &status) != nil {
			t.Fatal(response.Code, response.Body.String())
		}
		if status.State == "finished" {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("import did not finish")
	return importJobStatus{}
}
func TestImportHTTP(t *testing.T) {
	engine, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	api := New(engine, "token")
	for _, route := range []string{"GET /api/v18/capabilities", "POST /api/v18/imports/plan", "POST /api/v18/imports/execute", "GET /api/v18/imports/unknown", "POST /api/v18/imports/unknown/cancel"} {
		parts := strings.SplitN(route, " ", 2)
		if r := importStatusHTTP(api, parts[0], parts[1], false); r.Code != 401 {
			t.Fatal(route, r.Code)
		}
	}
	caps := importStatusHTTP(api, "GET", "/api/v18/capabilities", true)
	if caps.Code != 200 || !strings.Contains(caps.Body.String(), `"max_file_bytes":67108864`) {
		t.Fatal(caps.Body.String())
	}
	if dir := os.Getenv("DBM_IMPORT_DUMP"); dir != "" {
		if err = os.WriteFile(filepath.Join(dir, "ImportCapabilities.real-http.json"), caps.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := core.Connection{Driver: "sqlite", Database: "probe", TLSMode: "verify-full"}
	if _, err = engine.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE p12_rows(id INTEGER PRIMARY KEY,value TEXT)"}); err != nil {
		t.Fatal(err)
	}
	input := core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "p12_rows"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
	file := []byte("1,hello\n2,world")
	response := importHTTP(t, api, "plan", input, file)
	var p core.ImportPlan
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &p) != nil || p.Rows != 2 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = importHTTP(t, api, "execute", input, file)
	if response.Code != 400 {
		t.Fatal("missing confirmation accepted", response.Code)
	}
	input.Confirm = &dbplan.Confirmation{Action: "import_csv"}
	input.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	response = importHTTP(t, api, "execute", input, file)
	if response.Code != 409 {
		t.Fatal(response.Code, response.Body.String())
	}
	input.ExpectedPlanSHA256 = p.SHA256
	response = importHTTP(t, api, "execute", input, file)
	var accepted importJobStatus
	if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &accepted) != nil || len(accepted.ID) != 32 {
		t.Fatal(response.Code, response.Body.String())
	}
	status := awaitImportHTTP(t, api, accepted.ID)
	if status.Result.Outcome != "completed" || status.Result.Inserted != 2 || status.Progress.Processed != 2 {
		t.Fatalf("status %#v", status)
	}
	if response = importStatusHTTP(api, "POST", "/api/v18/imports/"+accepted.ID+"/cancel", true); response.Code != 200 {
		t.Fatal(response.Code)
	}
	if response = importStatusHTTP(api, "GET", "/api/v18/imports/unknown", true); response.Code != 404 {
		t.Fatal(response.Code)
	}
	// Standard JSON routes retain their 1 MiB body limit.
	r := httptest.NewRequest("POST", "/api/v1/query", strings.NewReader(`{"sql":"`+strings.Repeat("x", MaxRequestBytes)+`"}`))
	r.Header.Set("Authorization", "Bearer token")
	r.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	api.ServeHTTP(response, r)
	if response.Code != 413 {
		t.Fatal(response.Code)
	}
	for _, meta := range []any{map[string]any{"format": "csv", "server_path": "secret"}, map[string]any{"connection": c, "format": "csv", "max_rows": nil}, map[string]any{"connection": c, "format": "csv", "confirm": map[string]any{"action": "import_csv"}}} {
		response = importHTTP(t, api, "plan", meta, file)
		if response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	// Uploads larger than standard JSON limits are accepted in their own class.
	sqlInput := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	large := []byte("-- " + strings.Repeat("x", MaxRequestBytes) + "\nCREATE TABLE p12_large(id INTEGER)")
	response = importHTTP(t, api, "execute", sqlInput, large)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	_ = json.Unmarshal(response.Body.Bytes(), &accepted)
	status = awaitImportHTTP(t, api, accepted.ID)
	if status.Result.Reason != "import_too_large" {
		t.Fatalf("statement limit %#v", status)
	}
}

// blockedImportEngine allows deterministic cancellation and memory-slot tests.
type blockedImportEngine struct{ core.Engine }

func (blockedImportEngine) PlanImport(context.Context, core.ImportRequest, []byte) (core.ImportPlan, error) {
	return core.ImportPlan{}, nil
}
func (blockedImportEngine) ExecuteImport(ctx context.Context, _ core.ImportRequest, _ []byte, report func(core.ImportProgress)) (core.ImportResult, error) {
	report(core.ImportProgress{Total: 1})
	<-ctx.Done()
	return core.ImportResult{Outcome: "outcome_unknown", Reason: "query_canceled", Statements: []core.SQLStatementResult{}, Errors: []core.ImportIssue{}, Warnings: []string{}}, nil
}
func TestImportHTTPCancellationAndBusy(t *testing.T) {
	api := New(blockedImportEngine{}, "token")
	input := core.ImportRequest{Connection: core.Connection{Driver: "sqlite", Database: "probe"}, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	ids := []string{}
	for range 2 {
		response := importHTTP(t, api, "execute", input, []byte("SELECT 1"))
		var status importJobStatus
		if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &status) != nil {
			t.Fatal(response.Code, response.Body.String())
		}
		ids = append(ids, status.ID)
	}
	if response := importHTTP(t, api, "execute", input, []byte("SELECT 1")); response.Code != 503 {
		t.Fatal("unbounded uploads", response.Code)
	}
	for _, id := range ids {
		response := importStatusHTTP(api, "POST", "/api/v18/imports/"+id+"/cancel", true)
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
		status := awaitImportHTTP(t, api, id)
		if !status.CancelRequested || status.Result.Outcome != "outcome_unknown" {
			t.Fatalf("cancel %#v", status)
		}
	}
}
func TestImportMultipartLimitsAndExpiry(t *testing.T) {
	// A tiny class limit exercises the same bounded multipart reader without a
	// 64 MiB test allocation. File and decompression bounds have engine tests.
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	field, _ := form.CreateFormField("metadata")
	_, _ = io.WriteString(field, strings.Repeat("x", 2048))
	_ = form.Close()
	r := httptest.NewRequest("POST", "/", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), bodyLimitKey{}, int64(1024)))
	w := httptest.NewRecorder()
	if _, _, ok := readImportUpload(w, r); ok || w.Code != 413 {
		t.Fatal(w.Code, w.Body.String())
	}
	jobs := &importJobs{jobs: map[string]*importJob{"old": {finished: time.Now().Add(-6 * time.Minute)}, "running": {}}}
	jobs.expire()
	if len(jobs.jobs) != 1 || jobs.jobs["running"] == nil {
		t.Fatal("expiry")
	}
}

var _ http.Handler = (*API)(nil)

func TestImportHTTPMaxUpload(t *testing.T) {
	if os.Getenv("DBM_IMPORT_STRESS") != "1" {
		t.Skip("set DBM_IMPORT_STRESS=1 for 64 MiB upload boundary checks")
	}
	api := New(blockedImportEngine{}, "token")
	input := core.ImportRequest{Connection: core.Connection{Driver: "sqlite", Database: "probe"}, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	for _, size := range []int{core.MaxImportBytes, core.MaxImportBytes + 1} {
		file := bytes.Repeat([]byte("x"), size)
		response := importHTTP(t, api, "execute", input, file)
		if size > core.MaxImportBytes {
			if response.Code != 413 {
				t.Fatal("oversize", response.Code, response.Body.String())
			}
			continue
		}
		var status importJobStatus
		if response.Code != 202 || json.Unmarshal(response.Body.Bytes(), &status) != nil {
			t.Fatal("boundary", response.Code, response.Body.String())
		}
		_ = importStatusHTTP(api, "POST", "/api/v18/imports/"+status.ID+"/cancel", true)
		_ = awaitImportHTTP(t, api, status.ID)
	}
}
