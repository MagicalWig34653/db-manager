// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccountsRoutesAuthenticated(t *testing.T) {
	api := New(nil, "token")
	routes := []string{"list", "inspect", "access"}
	for _, action := range core.AccountActions {
		routes = append(routes, action, action+"/plan")
	}
	for _, route := range routes {
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest("POST", "/api/v15/accounts/"+route, nil))
		if response.Code != 401 {
			t.Errorf("%s: %d", route, response.Code)
		}
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest("GET", "/api/v15/capabilities", nil))
	if response.Code != 401 {
		t.Fatal(response.Code)
	}
}
func TestAccountsStrictRequests(t *testing.T) {
	api := New(nil, "token")
	for _, body := range []string{
		`{"connection":{"driver":"sqlite","database":"fixture"},"secret":"fixture-only"}`,
		`{"connection":{"driver":"sqlite","database":"fixture"},"account":null}`,
		`{"connection":{"driver":"sqlite","database":"fixture"},"sql":"DROP USER innocent"}`,
	} {
		r := httptest.NewRequest("POST", "/api/v15/accounts/password/plan", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer token")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != 400 || strings.Contains(w.Body.String(), "fixture-only") || strings.Contains(w.Body.String(), "DROP USER") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestAccountsActionSpecificSchemas(t *testing.T) {
	api := New(nil, "token")
	for _, tc := range []struct{ route, body string }{
		{"inspect", `{"connection":{"driver":"mysql"},"account":{"name":"fixture","host":"%"},"limit":0}`},
		{"list", `{"connection":{"driver":"mysql"},"limit":null}`},
		{"list", `{"connection":{"driver":"mysql"},"offset":null}`},
		{"list", `{"connection":{"driver":"mysql"},"timeout_ms":null}`},
		{"grant/plan", `{"connection":{"driver":"mysql"},"account":{"name":"fixture","host":"%"},"grant":{"scope":"global","privilege":"SELECT","grant_option":null}}`},
		{"list", `{"connection":{"driver":"mysql"},"new_account":{"name":"fixture","host":"%"}}`},
		{"grant/plan", `{"connection":{"driver":"mysql"},"account":{"name":"fixture","host":"%"},"grant":{"scope":"global","privilege":"SELECT"}}`},
		{"create", `{"connection":{"driver":"mysql"},"new_account":{"name":"fixture","host":"%"},"secret":"FIXTURE_SECRET","expected_plan_sha256":"0000000000000000000000000000000000000000000000000000000000000000"}`},
	} {
		r := httptest.NewRequest("POST", "/api/v15/accounts/"+tc.route, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != 400 || strings.Contains(w.Body.String(), "FIXTURE_SECRET") {
			t.Fatal(tc.route, w.Code, w.Body.String())
		}
	}
}

func TestAccountsCapabilitiesVersion(t *testing.T) {
	api := New(nil, "token")
	r := httptest.NewRequest("GET", "/api/v15/capabilities", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.15.0"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, version := range []string{"18"} {
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest("GET", "/api/v"+version+"/capabilities", nil))
		if w.Code != 404 {
			t.Fatalf("unexpected accounts version alias %s: %d", version, w.Code)
		}
	}
}
