// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

// dumpHTTP writes real responses for the contract checker when DBM_OBJECTOPS_DUMP names a directory.
func dumpHTTP(t *testing.T, kind string, body []byte) {
	t.Helper()
	if dir := os.Getenv("DBM_OBJECTOPS_DUMP"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, t.Name()+"."+kind+".json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestObjectOpsHTTP(t *testing.T) {
	e, _ := database.New(t.TempDir(), nil)
	token := strings.Repeat("p", 32)
	api := httpapi.New(e, token)
	send := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	conn := `{"driver":"sqlite","database":"objectops_http"}`
	routes := []string{"/api/v8/tables/dependencies"}
	for _, group := range []string{"databases/create", "databases/drop", "databases/collation", "tables/rename", "tables/copy", "tables/truncate", "tables/drop", "tables/options"} {
		routes = append(routes, "/api/v8/"+group, "/api/v8/"+group+"/plan")
	}
	for _, path := range routes {
		if w := send(http.MethodPost, path, `{"connection":`+conn+`}`, false); w.Code != 401 {
			t.Fatalf("%s without token: %d", path, w.Code)
		}
		if w := send(http.MethodGet, path, "", true); w.Code != 405 && w.Code != 404 {
			t.Fatalf("%s GET: %d", path, w.Code)
		}
	}
	w := send(http.MethodGet, "/api/v8/capabilities", "", true)
	var capabilities struct {
		Version  string                                                 `json:"contract_version"`
		Features []string                                               `json:"features"`
		Drivers  map[string]map[string]struct{ Support, Reason string } `json:"drivers"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &capabilities) != nil || capabilities.Version != "0.8.0" || len(capabilities.Features) != 9 {
		t.Fatalf("capabilities %d %s", w.Code, w.Body.String())
	}
	dumpHTTP(t, "capabilities", w.Body.Bytes())
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, action := range []string{"database_create", "database_drop", "database_collation", "table_rename", "table_copy", "table_truncate", "table_drop", "table_options", "dependencies"} {
			c := capabilities.Drivers[driver][action]
			if c.Support == "" || (c.Support != "supported" && c.Reason == "") {
				t.Fatalf("capability %s/%s %#v", driver, action, c)
			}
		}
	}
	if capabilities.Drivers["sqlite"]["database_create"].Support != "unsupported" || capabilities.Drivers["postgres"]["database_collation"].Support != "unsupported" {
		t.Fatal("capabilities must be honest")
	}
	for path, body := range map[string]string{
		"/api/v8/tables/drop/plan":     `{"connection":` + conn + `,"table":{"name":"a"},"unknown":1}`,
		"/api/v8/tables/drop":          `{"connection":` + conn + `,"table":{"name":"a"}}`,
		"/api/v8/tables/drop/plan ":    `{"connection":{"driver":"oracle"}}`,
		"/api/v8/tables/dependencies":  `{"connection":` + conn + `}`,
		"/api/v8/tables/truncate/plan": `{"connection":` + conn + `}`,
		"/api/v8/tables/rename/plan":   `{"connection":` + conn + `,"table":{"name":"a"},"new_name":null}`,
	} {
		if w := send(http.MethodPost, strings.TrimSpace(path), body, true); w.Code != 400 {
			t.Fatalf("%s invalid accepted: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := send(http.MethodPost, "/api/v8/tables/drop/plan", `{"connection":`+conn+`,"table":{"name":"absent"}}`, true); w.Code != 404 || !strings.Contains(w.Body.String(), "object_not_found") {
		t.Fatalf("absent table: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/api/v8/tables/drop/plan", `{"connection":`+conn+`,"table":{"name":"a"},"database":"x"}`, true); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid_object_op") {
		t.Fatalf("foreign field: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/api/v8/databases/create/plan", `{"connection":`+conn+`,"database":"x"}`, true); w.Code != 422 || !strings.Contains(w.Body.String(), "unsupported_action") {
		t.Fatalf("sqlite database create: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/api/v1/query", `{"connection":`+conn+`,"sql":"CREATE TABLE a (id INTEGER PRIMARY KEY)"}`, true); w.Code != 200 {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	w = send(http.MethodPost, "/api/v8/tables/drop/plan", `{"connection":`+conn+`,"table":{"name":"a"}}`, true)
	var plan struct {
		SHA256 string `json:"plan_sha256"`
		Target struct{ Name string }
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil || len(plan.SHA256) != 64 || strings.Contains(w.Body.String(), "password") {
		t.Fatalf("plan: %d %s", w.Code, w.Body.String())
	}
	dumpHTTP(t, "http-plan", w.Body.Bytes())
	execute := func(confirm string, sha string) *httptest.ResponseRecorder {
		return send(http.MethodPost, "/api/v8/tables/drop", `{"connection":`+conn+`,"table":{"name":"a"},"confirm":`+confirm+`,"expected_plan_sha256":"`+sha+`"}`, true)
	}
	if w := execute(`{"action":"table_drop","target_name":"b"}`, plan.SHA256); w.Code != 400 || !strings.Contains(w.Body.String(), "confirmation_mismatch") {
		t.Fatalf("wrong target: %d %s", w.Code, w.Body.String())
	}
	if w := execute(`{"action":"table_drop","target_name":"a"}`, strings.Repeat("a", 64)); w.Code != 409 || !strings.Contains(w.Body.String(), "plan_mismatch") {
		t.Fatalf("stale plan: %d %s", w.Code, w.Body.String())
	}
	if w := execute(`{"action":"table_drop","target_name":"a"}`, "nothex"); w.Code != 400 {
		t.Fatalf("bad digest: %d %s", w.Code, w.Body.String())
	}
	w = execute(`{"action":"table_drop","target_name":"a"}`, plan.SHA256)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"outcome":"completed"`) {
		t.Fatalf("execute: %d %s", w.Code, w.Body.String())
	}
	dumpHTTP(t, "http-result", w.Body.Bytes())
	if w := send(http.MethodPost, "/api/v1/query", `{"connection":`+conn+`,"sql":"SELECT 1 FROM a"}`, true); w.Code == 200 {
		t.Fatal("table still exists")
	}
}
