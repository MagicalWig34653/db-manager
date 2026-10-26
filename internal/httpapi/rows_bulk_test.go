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

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
)

func TestRowsBulkHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	api := New(e, "token")
	ctx := context.Background()
	c := core.Connection{Driver: "sqlite", Database: "p13"}
	send := func(path string, value any, auth bool) *httptest.ResponseRecorder {
		b, _ := json.Marshal(value)
		r := httptest.NewRequest("POST", "/api/v19/"+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.Header.Set("Authorization", "Bearer token")
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	for _, route := range []string{"rows/plan", "rows/execute", "values/upload", "values/download", "foreign-keys/lookup"} {
		if w := send(route, map[string]any{}, false); w.Code != 401 {
			t.Fatal(route, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/v19/capabilities", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if dir := os.Getenv("DBM_ROWS_BULK_DUMP"); dir != "" {
		if err = os.WriteFile(filepath.Join(dir, "RowsBulkCapabilities.real-http.json"), w.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.Query(ctx, core.QueryRequest{Connection: c, SQL: "CREATE TABLE p13_rows(id INTEGER PRIMARY KEY,raw BLOB,value TEXT)"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Query(ctx, core.QueryRequest{Connection: c, SQL: "INSERT INTO p13_rows VALUES(1,X'00','one')"})
	if err != nil {
		t.Fatal(err)
	}
	snap := core.RowSnapshot{Columns: []string{"id", "raw", "value"}, Values: []core.Cell{{Kind: "integer", Value: "1"}, {Kind: "binary", Value: "AA=="}, {Kind: "text", Value: "one"}}}
	request := core.RowsBulkRequest{Connection: c, Table: core.TableRef{Name: "p13_rows"}, Action: "update", Rows: []core.BulkRow{{Expected: snap}}, Changes: []core.RowChange{{Column: "value", Value: core.Cell{Kind: "text", Value: "two"}}}}
	w = send("rows/plan", request, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var plan core.RowsBulkPlan
	if err = json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	request.Confirm = true
	request.ExpectedPlanSHA256 = plan.PlanSHA256
	request.Rows[0].StabilitySHA256 = plan.Rows[0].StabilitySHA256
	w = send("rows/execute", request, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result core.RowsBulkResult
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result.Outcome != "completed" {
		t.Fatal(result)
	}
	w = send("rows/execute", request, true)
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"outcome":"rejected"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	// Missing cell value must not silently become a real empty string.
	b, _ := json.Marshal(request)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["changes"] = []any{map[string]any{"column": "value", "value": map[string]any{"kind": "text"}}}
	w = send("rows/plan", raw, true)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, body := range []string{`{"connection":{"driver":"sqlite"},"table":{"name":"x"},"action":"delete","rows":null}`, `{"connection":{"driver":"sqlite"},"table":{"name":"x"},"action":"delete","rows":[],"sql":"DROP TABLE x"}`} {
		r := httptest.NewRequest("POST", "/api/v19/rows/plan", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	fileRequest := core.FileValueRequest{Connection: c, Table: request.Table, Expected: *result.Rows[0].Row, Column: "raw", Confirm: true}
	upload := func(extra bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		metadata, _ := writer.CreateFormField("metadata")
		_ = json.NewEncoder(metadata).Encode(fileRequest)
		file, _ := writer.CreateFormFile("file", "ignored.html")
		_, _ = file.Write([]byte{0, 255, 1})
		if extra {
			field, _ := writer.CreateFormField("extra")
			_, _ = io.WriteString(field, "bad")
		}
		_ = writer.Close()
		r := httptest.NewRequest("POST", "/api/v19/values/upload", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	w = upload(true)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = upload(false)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var mutation core.MutationResult
	_ = json.Unmarshal(w.Body.Bytes(), &mutation)
	fileRequest.Expected = *mutation.Row
	w = send("values/download", fileRequest, true)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), []byte{0, 255, 1}) || w.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	fileRequest.Preview = true
	w = send("values/download", fileRequest, true)
	if w.Code != 422 || strings.Contains(w.Body.String(), "ignored.html") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestRowsBulkHTTPUnknownCommit(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := core.Connection{Driver: "sqlite", Database: "unknown", TLSMode: "verify-full"}
	ctx := context.Background()
	for _, sql := range []string{"CREATE TABLE parent(id INTEGER PRIMARY KEY)", "INSERT INTO parent VALUES(1)", "CREATE TABLE child(id INTEGER PRIMARY KEY,parent INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED)", "INSERT INTO child VALUES(1,1)"} {
		if _, err = e.Query(ctx, core.QueryRequest{Connection: c, SQL: sql}); err != nil {
			t.Fatal(err)
		}
	}
	request := core.RowsBulkRequest{Connection: c, Table: core.TableRef{Name: "child"}, Action: "update", Rows: []core.BulkRow{{Expected: core.RowSnapshot{Columns: []string{"id", "parent"}, Values: []core.Cell{{Kind: "integer", Value: "1"}, {Kind: "integer", Value: "1"}}}}}, Changes: []core.RowChange{{Column: "parent", Value: core.Cell{Kind: "integer", Value: "999"}}}}
	plan, err := e.RowsBulkPlan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Confirm = true
	request.ExpectedPlanSHA256 = plan.PlanSHA256
	request.Rows[0].StabilitySHA256 = plan.Rows[0].StabilitySHA256
	body, _ := json.Marshal(request)
	r := httptest.NewRequest("POST", "/api/v19/rows/execute", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	New(e, "token").ServeHTTP(w, r)
	if w.Code != 503 || !strings.Contains(w.Body.String(), `"outcome":"outcome_unknown"`) || strings.Contains(w.Body.String(), `"row":`) {
		t.Fatal(w.Code, w.Body.String())
	}
}
