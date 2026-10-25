// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"context"
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

func TestProgramsHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	api := httpapi.New(e, strings.Repeat("p", 32))
	send := func(method, path string, input any, auth bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(input)
		r := httptest.NewRequest(method, "/api/v19/"+path, strings.NewReader(string(raw)))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("p", 32))
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	dump := func(component string, w *httptest.ResponseRecorder) {
		if dir := os.Getenv("DBM_PROGRAMS_DUMP"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, t.Name()+"."+component+".json"), w.Body.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, route := range []string{"programs/list", "programs/describe", "programs/dependencies", "programs/plan", "programs/execute"} {
		if w := send("POST", route, map[string]any{}, false); w.Code != 401 {
			t.Fatalf("auth %s: %d", route, w.Code)
		}
		if w := send("GET", route, nil, true); w.Code != 405 {
			t.Fatalf("method %s: %d", route, w.Code)
		}
	}
	w := send("GET", "capabilities", nil, true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	dump("ProgramsCapabilities", w)
	c := map[string]any{"driver": "sqlite", "database": "programs_http"}
	for _, route := range []string{"programs/describe", "programs/dependencies"} {
		if w = send("POST", route, map[string]any{"connection": c}, true); w.Code != 400 {
			t.Fatalf("missing object %d", w.Code)
		}
	}
	obj := map[string]any{"kind": "view", "name": "v\";DROP TABLE neighbor--"}
	request := map[string]any{"connection": c, "object": obj, "action": "create", "input": map[string]any{"body": "SELECT 42 AS value"}}
	w = send("POST", "programs/plan", request, true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	dump("ProgramPlan", w)
	var plan core.ProgramPlan
	if err = json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if w = send("POST", "programs/execute", request, true); w.Code != 400 {
		t.Fatal("missing confirmation")
	}
	request["confirm"] = map[string]any{"action": "create"}
	request["expected_plan_sha256"] = strings.Repeat("0", 64)
	if w = send("POST", "programs/execute", request, true); w.Code != 409 {
		t.Fatalf("hash %d %s", w.Code, w.Body.String())
	}
	request["expected_plan_sha256"] = plan.SHA256
	w = send("POST", "programs/execute", request, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"completed"`) {
		t.Fatal(w.Body.String())
	}
	dump("ProgramResult", w)
	w = send("POST", "programs/describe", map[string]any{"connection": c, "object": obj}, true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	dump("ProgramsResult", w)
	dropRequest := map[string]any{"connection": c, "object": obj, "action": "drop"}
	w = send("POST", "programs/plan", dropRequest, true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil {
		t.Fatal(w.Body.String())
	}
	dropRequest["expected_plan_sha256"] = plan.SHA256
	dropRequest["confirm"] = map[string]any{"action": "drop", "target_name": "wrong"}
	if w = send("POST", "programs/execute", dropRequest, true); w.Code != 400 {
		t.Fatal("typed target accepted")
	}
	dropRequest["confirm"] = map[string]any{"action": "drop", "target_name": plan.Object.Name}
	if w = send("POST", "programs/execute", dropRequest, true); w.Code != 200 || !strings.Contains(w.Body.String(), `"completed"`) {
		t.Fatal(w.Body.String())
	}
	if w = send("POST", "programs/describe", map[string]any{"connection": c, "object": obj}, true); w.Code != 404 {
		t.Fatal("dropped target remains")
	}
	nested := map[string]any{"connection": c, "object": obj, "action": "create", "input": map[string]any{"body": "SELECT 1", "unexpected": "private_marker"}}
	if w = send("POST", "programs/plan", nested, true); w.Code != 400 || strings.Contains(w.Body.String(), "private_marker") {
		t.Fatal("nested field accepted or echoed")
	}
	for _, field := range []string{"sql", "password", "unexpected"} {
		w = send("POST", "programs/list", map[string]any{"connection": c, field: "secret"}, true)
		if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("unknown field %d", w.Code)
		}
	}
	// A driver error must never echo body text, identifiers or host paths.
	bad := map[string]any{"connection": c, "object": map[string]any{"kind": "view", "name": "bad"}, "action": "create", "input": map[string]any{"body": "SELECT secret_unpublished FROM absent_private"}}
	w = send("POST", "programs/plan", bad, true)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	bad["confirm"] = map[string]any{"action": "create"}
	bad["expected_plan_sha256"] = plan.SHA256
	w = send("POST", "programs/execute", bad, true)
	if strings.Contains(w.Body.String(), "secret_unpublished") || strings.Contains(w.Body.String(), "absent_private") {
		t.Fatal("raw SQL error")
	}
	if _, err = e.Query(context.Background(), core.QueryRequest{Connection: core.Connection{Driver: "sqlite", Database: "programs_http"}, SQL: "SELECT 1"}); err != nil {
		t.Fatal(err)
	}
	if w = send("POST", "programs/call", nil, true); w.Code != 404 {
		t.Fatal("routine execution route exists")
	}
}
