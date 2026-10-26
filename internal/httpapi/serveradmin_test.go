// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestServerAdminHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 32)
	api := httpapi.New(e, token)
	send := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"status", "variables", "processes"} {
		if w := send("POST", "/api/v15/"+path, `{}`, false); w.Code != 401 {
			t.Fatalf("unauth %s: %d", path, w.Code)
		}
	}
	if w := send("GET", "/api/v15/capabilities", "", false); w.Code != 401 {
		t.Fatal("unauth capabilities")
	}
	if w := send("GET", "/api/v15/capabilities", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.15.0"`) || strings.Contains(w.Body.String(), "kill") || strings.Contains(w.Body.String(), "set_variable") {
		t.Fatalf("capabilities: %d %s", w.Code, w.Body.String())
	}
	body := `{"connection":{"driver":"sqlite","database":"placeholder","password":"never-echo-placeholder"}}`
	if w := send("POST", "/api/v15/status", body, true); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "never-echo") || strings.Contains(w.Body.String(), ".db") {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	for _, tail := range []string{`,"sql":"secret"}`, `,"value":"123"}`, `,"id":"123"}`, `,"limit":201}`, `,"timeout_ms":10001}`, `,"scope":"INVALID"}`} {
		if w := send("POST", "/api/v15/status", strings.TrimSuffix(body, "}")+tail, true); w.Code != 400 {
			t.Fatalf("invalid: %d %s", w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"variables", "processes"} {
		if w := send("POST", "/api/v15/"+path, body, true); w.Code != 422 || strings.Contains(w.Body.String(), "never-echo") {
			t.Fatalf("unsupported %s: %d %s", path, w.Code, w.Body.String())
		}
	}

	for _, path := range []string{"processes/kill", "processes/kill/plan", "variables/set", "variables/set/plan"} {
		for _, auth := range []bool{false, true} {
			if w := send("POST", "/api/v15/"+path, body, auth); w.Code != 404 {
				t.Fatalf("write route exposed %s: %d", path, w.Code)
			}
		}
	}
	if w := send("GET", "/api/v17/capabilities", "", true); w.Code != 404 {
		t.Fatal("obsolete API exposed")
	}
}
