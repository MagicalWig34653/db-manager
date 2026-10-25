// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestSearchHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 32)
	api := httpapi.New(e, token)
	send := func(path string, body any, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		method := "POST"
		if path == "capabilities" {
			method = "GET"
		}
		r := httptest.NewRequest(method, "/api/v15/"+path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if dir := os.Getenv("DBM_SEARCH_DUMP"); dir != "" && w.Code == 200 {
			kind := map[string]string{"capabilities": "capabilities", "replace/plan": "plan", "replace": "result", "search": "search", "tables/browse": "browse"}[path]
			if err := os.WriteFile(filepath.Join(dir, t.Name()+"."+kind+".json"), w.Body.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return w
	}
	for _, path := range []string{"capabilities", "search", "tables/browse", "replace/plan", "replace"} {
		if w := send(path, map[string]any{}, false); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	if w := send("capabilities", nil, true); w.Code != 200 || !strings.Contains(w.Body.String(), "0.15.0") {
		t.Fatal(w.Code, w.Body.String())
	}
	conn := core.Connection{Driver: "sqlite", Database: "search_http"}
	for _, sql := range []string{"CREATE TABLE sample(id INTEGER PRIMARY KEY,value TEXT)", "INSERT INTO sample VALUES(1,'fox')"} {
		data, _ := json.Marshal(map[string]any{"connection": conn, "sql": sql})
		r := httptest.NewRequest("POST", "/api/v1/query", bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	base := func() map[string]any {
		return map[string]any{"connection": conn, "table": map[string]any{"name": "sample"}, "column": "value", "term": "fox", "replacement": "cat", "mode": "substring"}
	}
	for _, field := range []string{"connection", "table", "column", "term", "replacement", "mode"} {
		body := base()
		delete(body, field)
		if w := send("replace/plan", body, true); w.Code != 400 {
			t.Fatalf("missing %s %d %s", field, w.Code, w.Body.String())
		}
		body = base()
		body[field] = nil
		if w := send("replace/plan", body, true); w.Code != 400 {
			t.Fatalf("null %s %d %s", field, w.Code, w.Body.String())
		}
	}
	body := base()
	body["sql"] = "private SQL"
	if w := send("replace/plan", body, true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	w := send("replace/plan", base(), true)
	var p core.ReplacePlan
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	body = base()
	body["expected_plan_sha256"] = p.SHA256
	body["confirm"] = map[string]any{"action": "replace", "target_name": "sample"}
	if w := send("replace", body, true); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	body["confirm"] = core.ReplaceConfirm{Action: "replace", TargetName: "sample", AffectedRows: 1}
	body["expected_plan_sha256"] = strings.Repeat("0", 64)
	if w := send("replace", body, true); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	body["expected_plan_sha256"] = p.SHA256
	if w := send("replace", body, true); w.Code != 200 || !strings.Contains(w.Body.String(), "completed") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, f := range []any{map[string]any{"operator": "eq", "value": map[string]any{"kind": "text", "value": ""}}, map[string]any{"column": "value", "operator": "eq", "value": map[string]any{"kind": "text"}}} {
		if w := send("tables/browse", map[string]any{"connection": conn, "table": map[string]any{"name": "sample"}, "filters": []any{f}}, true); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	// No raw SQLite diagnostics or submitted SQL text in errors.
	body = base()
	body["table"] = map[string]any{"name": "private_missing_table"}
	w = send("replace/plan", body, true)
	if w.Code != 404 || strings.Contains(w.Body.String(), "private_missing_table") || strings.Contains(w.Body.String(), "SELECT") {
		t.Fatal(w.Code, w.Body.String())
	}
}
