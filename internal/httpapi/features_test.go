// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFeatureListKeepsRegistrationOrder(t *testing.T) {
	if len(featureList) != 9 {
		t.Fatalf("feature count changed: %d", len(featureList))
	}
}

func TestAllKnownRoutesRegisteredAndAuthenticated(t *testing.T) {
	api := New(nil, "token")
	routes := []string{
		"GET /api/v1/capabilities", "POST /api/v1/connections/test", "POST /api/v1/catalog", "POST /api/v1/query",
		"GET /api/v2/capabilities", "POST /api/v2/tables/describe", "POST /api/v2/tables/browse",
		"GET /api/v3/capabilities", "POST /api/v3/tables/eligibility", "POST /api/v3/rows/update", "POST /api/v3/rows/delete",
		"GET /api/v4/capabilities", "POST /api/v4/tables/insert-eligibility", "POST /api/v4/rows/insert",
		"GET /api/v5/capabilities", "POST /api/v5/tables/structure",
		"GET /api/v6/capabilities", "POST /api/v6/tables/create",
		"GET /api/v7/capabilities", "POST /api/v7/databases", "POST /api/v7/objects", "POST /api/v7/definition", "POST /api/v7/server-info",
	}
	for _, route := range routes {
		parts := strings.SplitN(route, " ", 2)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest(parts[0], parts[1], nil))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: want 401 without token, got %d", route, response.Code)
		}
	}
	for _, route := range []string{"GET /api/v9/capabilities", "POST /api/v1/other"} {
		parts := strings.SplitN(route, " ", 2)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest(parts[0], parts[1], nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", route, response.Code)
		}
	}
}

func TestBodyLimitClasses(t *testing.T) {
	if BodyLimit(ClassStandard) != MaxRequestBytes || BodyLimit(ClassUpload) != MaxRequestBytes || BodyLimit("unknown") != MaxRequestBytes {
		t.Fatal("default limits must stay at 1 MiB")
	}
	saved := classBodyLimits[ClassUpload]
	classBodyLimits[ClassUpload] = 4 * MaxRequestBytes
	defer func() { classBodyLimits[ClassUpload] = saved }()
	var target struct {
		V string `json:"v"`
	}
	body := `{"v":"` + strings.Repeat("x", 2*MaxRequestBytes) + `"}`
	send := func(handler http.Handler) int {
		request := httptest.NewRequest("POST", "/x", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	handle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if decode(w, r, &target) {
			w.WriteHeader(204)
		}
	})
	if got := send(handle); got != 413 {
		t.Errorf("standard route: want 413, got %d", got)
	}
	if got := send(WithRouteClass(ClassUpload, handle)); got != 204 {
		t.Errorf("upload class: want 204, got %d", got)
	}
}
