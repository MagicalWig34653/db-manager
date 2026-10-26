// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"encoding/json"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMonitorHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("m", 32)
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
	w := send("GET", "/api/v20/capabilities", "", true)
	var cap map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &cap) != nil || cap["contract_version"] != "0.20.0" || cap["read_only"] != true {
		t.Fatalf("capabilities %d %s", w.Code, w.Body.String())
	}
	body := `{"connection":{"driver":"sqlite","database":"monitor_http"}}`
	for _, action := range []string{"sample", "analysis", "advisor", "replication", "binary-logs", "binary-events", "profiling", "engines"} {
		w := send("POST", "/api/v20/monitor/"+action, body, false)
		if w.Code != 401 {
			t.Fatalf("auth %s %d", action, w.Code)
		}
	}
	for _, body := range []string{`{"connection":{"driver":"sqlite","database":"monitor_http"},"limit":201}`, `{"connection":{"driver":"sqlite","database":"monitor_http"},"unknown":true}`, `{"connection":{"driver":"sqlite","database":"monitor_http"},"sql":"STOP REPLICA"}`} {
		w := send("POST", "/api/v20/monitor/sample", body, true)
		if w.Code != 400 {
			t.Fatalf("invalid %d %s", w.Code, w.Body.String())
		}
	}
	w = send("POST", "/api/v20/monitor/sample", body, true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"source":"sqlite_version"`) {
		t.Fatalf("sample %d %s", w.Code, w.Body.String())
	}
	w = send("POST", "/api/v20/monitor/replication", body, true)
	if w.Code != 422 {
		t.Fatalf("unsupported %d", w.Code)
	}
	for _, action := range []string{"start", "stop", "purge", "skip-error", "change-source"} {
		w := send("POST", "/api/v20/monitor/"+action, body, true)
		if w.Code != 404 {
			t.Fatalf("write route exists %s", action)
		}
	}
}
