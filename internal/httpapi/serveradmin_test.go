// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
	"net/http/httptest"
	"strings"
	"testing"
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
	for _, path := range []string{"status", "variables", "processes", "processes/kill/plan", "processes/kill", "variables/set/plan", "variables/set"} {
		if w := send("POST", "/api/v17/"+path, `{}`, false); w.Code != 401 {
			t.Fatalf("unauth %s: %d", path, w.Code)
		}
	}
	if w := send("GET", "/api/v17/capabilities", "", false); w.Code != 401 {
		t.Fatal("unauth capabilities")
	}
	if w := send("GET", "/api/v17/capabilities", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.17.0"`) {
		t.Fatalf("capabilities: %d %s", w.Code, w.Body.String())
	}
	body := `{"connection":{"driver":"sqlite","database":"placeholder","password":"never-echo-placeholder"}}`
	if w := send("POST", "/api/v17/status", body, true); w.Code != 200 || strings.Contains(w.Body.String(), "never-echo") || strings.Contains(w.Body.String(), ".db") {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	for _, tail := range []string{`,"sql":"secret"}`, `,"limit":201}`, `,"timeout_ms":10001}`, `,"scope":"INVALID"}`} {
		if w := send("POST", "/api/v17/status", strings.TrimSuffix(body, "}")+tail, true); w.Code != 400 {
			t.Fatalf("invalid: %d %s", w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"variables", "processes", "processes/kill/plan", "variables/set/plan"} {
		if w := send("POST", "/api/v17/"+path, body, true); w.Code != 422 || strings.Contains(w.Body.String(), "never-echo") {
			t.Fatalf("unsupported %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, test := range []struct{ path, extra string }{{"processes/kill/plan", `,"name":""}`}, {"variables/set/plan", `,"id":""}`}, {"processes/kill/plan", `,"confirm":null}`}} {
		if w := send("POST", "/api/v17/"+test.path, strings.TrimSuffix(body, "}")+test.extra, true); w.Code != 400 {
			t.Fatalf("strict action schema: %d %s", w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"processes/kill", "variables/set"} {
		if w := send("POST", "/api/v17/"+path, body, true); w.Code != 400 {
			t.Fatalf("missing confirmation: %d %s", w.Code, w.Body.String())
		}
	}

}
