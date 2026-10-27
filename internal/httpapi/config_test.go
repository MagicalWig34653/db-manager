// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
)

type healthy struct{ up bool }

func (h *healthy) Healthy() bool { return h.up }

func configAPI(t *testing.T, options ...Option) (http.Handler, *healthy, *configstore.MemoryRepo) {
	t.Helper()
	repo := configstore.NewMemoryRepo()
	state := &healthy{up: true}
	all := append([]Option{WithStateStore(state), WithConfigStore(configstore.NewService(repo))}, options...)
	return New(&testEngine{}, testToken, all...), state, repo
}

func do(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testToken)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeBody(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %q", response.Body.String())
	}
	return out
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	body := decodeBody(t, response)
	errBody, _ := body["error"].(map[string]any)
	code, _ := errBody["code"].(string)
	return code
}

func TestConfigRoutesRequireAuthentication(t *testing.T) {
	handler, _, _ := configAPI(t)
	id := strings.Repeat("a", 32)
	for _, route := range []string{
		"GET /api/v24/capabilities", "GET /api/v24/config/status", "GET /api/v24/config/export", "POST /api/v24/config/import",
		"POST /api/v24/config/clear/plan", "POST /api/v24/config/clear/execute", "GET /api/v24/config/bookmarks", "POST /api/v24/config/bookmarks",
		"GET /api/v24/config/bookmarks/" + id, "PUT /api/v24/config/bookmarks/" + id, "DELETE /api/v24/config/bookmarks/" + id,
	} {
		parts := strings.SplitN(route, " ", 2)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(parts[0], parts[1], nil))
		if response.Code != 401 {
			t.Errorf("%s: want 401, got %d", route, response.Code)
		}
	}
}

func TestConfigCapabilitiesAreHonestAboutTheState(t *testing.T) {
	// No state store configured: local mode, config routes fail closed.
	plain := New(&testEngine{}, testToken)
	capabilities := decodeBody(t, do(plain, "GET", "/api/v24/capabilities", "", nil))
	if capabilities["available"] != false || capabilities["mode"] != "local_only" || capabilities["reason"] != "not_configured" || capabilities["contract_version"] != "0.24.0" {
		t.Fatalf("unconfigured capabilities: %v", capabilities)
	}
	response := do(plain, "GET", "/api/v24/config/bookmarks", "", nil)
	if response.Code != 503 || errorCode(t, response) != "state_unavailable" || response.Header().Get("Retry-After") == "" {
		t.Fatalf("unconfigured list: %d %s", response.Code, response.Body.String())
	}
	if response := do(plain, "POST", "/api/v24/config/bookmarks", `{"data":{"name":"n","sql":"SELECT 1"}}`, nil); response.Code != 503 {
		t.Fatalf("create without state: %d", response.Code)
	}
	handler, state, _ := configAPI(t)
	if c := decodeBody(t, do(handler, "GET", "/api/v24/capabilities", "", nil)); c["available"] != true || c["mode"] != "server" || c["reason"] != "ok" || c["owner_model"] != "single" {
		t.Fatalf("available capabilities: %v", c)
	}
	state.up = false
	if c := decodeBody(t, do(handler, "GET", "/api/v24/capabilities", "", nil)); c["available"] != false || c["mode"] != "local_only" || c["reason"] != "unreachable" {
		t.Fatalf("outage capabilities: %v", c)
	}
	if response := do(handler, "GET", "/api/v24/config/bookmarks", "", nil); response.Code != 503 {
		t.Fatalf("list during outage: %d", response.Code)
	}
	state.up = true
	if response := do(handler, "GET", "/api/v24/config/bookmarks", "", nil); response.Code != 200 {
		t.Fatalf("no recovery: %d", response.Code)
	}
	// Bearer-only features are unaffected by the outage.
	state.up = false
	engine := &testEngine{}
	if response := perform(New(engine, testToken, WithStateStore(state)), "POST", "/api/v1/query", queryBody, testToken); response.Code != 200 {
		t.Fatalf("bearer mode affected: %d", response.Code)
	}
}

func TestConfigCRUDWithETagAndIfMatch(t *testing.T) {
	handler, _, _ := configAPI(t)
	created := do(handler, "POST", "/api/v24/config/bookmarks", `{"scope":"abc","data":{"name":"Orders","sql":"SELECT 1"}}`, nil)
	if created.Code != 201 || created.Header().Get("ETag") != `"1"` || !strings.HasPrefix(created.Header().Get("Location"), "/api/v24/config/bookmarks/") {
		t.Fatalf("create: %d %v %s", created.Code, created.Header(), created.Body.String())
	}
	item := decodeBody(t, created)
	path := "/api/v24/config/bookmarks/" + item["id"].(string)
	if got := do(handler, "GET", path, "", nil); got.Code != 200 || got.Header().Get("ETag") != `"1"` || got.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("get: %d %v", got.Code, got.Header())
	}
	update := `{"data":{"name":"Orders","sql":"SELECT 2"}}`
	if response := do(handler, "PUT", path, update, nil); response.Code != 428 || errorCode(t, response) != "precondition_required" {
		t.Fatalf("missing If-Match: %d", response.Code)
	}
	for _, bad := range []string{"1", `W/"1"`, `"abc"`, `"1", "2"`, `""`, `"99999999999999999999"`} {
		if response := do(handler, "PUT", path, update, map[string]string{"If-Match": bad}); response.Code != 400 {
			t.Errorf("If-Match %q: %d", bad, response.Code)
		}
	}
	if response := do(handler, "PUT", path, update, map[string]string{"If-Match": `"7"`}); response.Code != 412 || errorCode(t, response) != "version_conflict" {
		t.Fatalf("stale version: %d", response.Code)
	}
	updated := do(handler, "PUT", path, update, map[string]string{"If-Match": `"1"`})
	if updated.Code != 200 || updated.Header().Get("ETag") != `"2"` {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	if response := do(handler, "DELETE", path, "", nil); response.Code != 428 {
		t.Fatalf("delete without If-Match: %d", response.Code)
	}
	if response := do(handler, "DELETE", path, "", map[string]string{"If-Match": `"1"`}); response.Code != 412 {
		t.Fatalf("delete stale: %d", response.Code)
	}
	if response := do(handler, "DELETE", path, "", map[string]string{"If-Match": "*"}); response.Code != 204 || response.Body.Len() != 0 {
		t.Fatalf("delete: %d", response.Code)
	}
	if response := do(handler, "GET", path, "", nil); response.Code != 404 || errorCode(t, response) != "not_found" {
		t.Fatalf("get after delete: %d", response.Code)
	}
	// Idempotent create answers 200 with the existing entry.
	first := do(handler, "POST", "/api/v24/config/favorites", `{"data":{"database":"d","table":"t"}}`, nil)
	second := do(handler, "POST", "/api/v24/config/favorites", `{"data":{"database":"d","table":"t"}}`, nil)
	if first.Code != 201 || second.Code != 200 || decodeBody(t, first)["id"] != decodeBody(t, second)["id"] {
		t.Fatalf("idempotent create: %d %d", first.Code, second.Code)
	}
}

func TestConfigRequestValidationAndErrorsWithoutRawText(t *testing.T) {
	handler, _, _ := configAPI(t)
	cases := []struct {
		name, method, path, body string
		headers                  map[string]string
		status                   int
		code                     string
	}{
		{"unknown kind", "GET", "/api/v24/config/nothing", "", nil, 404, "unknown_kind"},
		{"unknown kind create", "POST", "/api/v24/config/nothing", `{"data":{}}`, nil, 404, "unknown_kind"},
		{"bad content type", "POST", "/api/v24/config/bookmarks", `x`, map[string]string{"Content-Type": "text/plain"}, 415, "unsupported_media_type"},
		{"unknown field", "POST", "/api/v24/config/bookmarks", `{"data":{"name":"n","sql":"SELECT 1"},"extra":1}`, nil, 400, "invalid_request"},
		{"null", "POST", "/api/v24/config/bookmarks", `{"data":{"name":"n","sql":null}}`, nil, 400, "invalid_request"},
		{"two objects", "POST", "/api/v24/config/bookmarks", `{"data":{}}{"data":{}}`, nil, 400, "invalid_request"},
		{"missing data", "POST", "/api/v24/config/bookmarks", `{}`, nil, 422, "invalid_data"},
		{"too large", "POST", "/api/v24/config/bookmarks", `{"data":{"name":"n","sql":"` + strings.Repeat("x", MaxRequestBytes) + `"}}`, nil, 413, "request_too_large"},
		{"secret", "POST", "/api/v24/config/bookmarks", `{"data":{"name":"n","sql":"CREATE USER a IDENTIFIED BY 'pw-1234'"}}`, nil, 422, "secret_rejected"},
		{"history off", "POST", "/api/v24/config/history", `{"data":{"sql":"SELECT 1"}}`, nil, 409, "history_disabled"},
		{"bad scope", "POST", "/api/v24/config/bookmarks", `{"scope":"a b","data":{"name":"n","sql":"SELECT 1"}}`, nil, 400, "invalid_request"},
		{"bad id", "GET", "/api/v24/config/bookmarks/not-an-id", "", nil, 404, "not_found"},
		{"unknown query", "GET", "/api/v24/config/bookmarks?foo=1", "", nil, 400, "invalid_request"},
		{"repeated query", "GET", "/api/v24/config/bookmarks?limit=1&limit=2", "", nil, 400, "invalid_request"},
		{"bad limit", "GET", "/api/v24/config/bookmarks?limit=abc", "", nil, 400, "invalid_request"},
		{"limit zero", "GET", "/api/v24/config/bookmarks?limit=0", "", nil, 400, "invalid_request"},
		{"limit big", "GET", "/api/v24/config/bookmarks?limit=101", "", nil, 400, "invalid_request"},
		{"bad cursor", "GET", "/api/v24/config/bookmarks?cursor=!!", "", nil, 400, "invalid_request"},
		{"empty import", "POST", "/api/v24/config/import", `{"items":[]}`, nil, 400, "invalid_request"},
		{"clear no kinds", "POST", "/api/v24/config/clear/plan", `{"kinds":[]}`, nil, 400, "invalid_request"},
		{"clear unknown kind", "POST", "/api/v24/config/clear/plan", `{"kinds":["x"]}`, nil, 400, "invalid_request"},
		{"clear wrong confirm", "POST", "/api/v24/config/clear/execute", `{"kinds":["bookmarks"],"plan_hash":"x","confirmation":"nein"}`, nil, 409, "plan_stale"},
	}
	for _, c := range cases {
		response := do(handler, c.method, c.path, c.body, c.headers)
		if response.Code != c.status || errorCode(t, response) != c.code {
			t.Errorf("%s: got %d %q, want %d %q", c.name, response.Code, errorCode(t, response), c.status, c.code)
		}
		for _, leak := range []string{"pw-1234", "IDENTIFIED", "SELECT", "config_items", "goroutine", "/Users/"} {
			if strings.Contains(response.Body.String(), leak) {
				t.Errorf("%s: response leaks %q: %s", c.name, leak, response.Body.String())
			}
		}
	}
}

func TestConfigListPagingAndFilters(t *testing.T) {
	handler, _, _ := configAPI(t)
	for i := 0; i < 5; i++ {
		body := `{"scope":"s","data":{"database":"d","table":"t` + string(rune('a'+i)) + `"}}`
		if response := do(handler, "POST", "/api/v24/config/favorites", body, nil); response.Code != 201 {
			t.Fatal(response.Body.String())
		}
	}
	page := decodeBody(t, do(handler, "GET", "/api/v24/config/favorites?scope=s&limit=2", "", nil))
	items := page["items"].([]any)
	if len(items) != 2 || page["next_cursor"] == "" {
		t.Fatalf("first page: %v", page)
	}
	next := decodeBody(t, do(handler, "GET", "/api/v24/config/favorites?scope=s&limit=2&cursor="+page["next_cursor"].(string), "", nil))
	if len(next["items"].([]any)) != 2 || next["items"].([]any)[0].(map[string]any)["id"] == items[0].(map[string]any)["id"] {
		t.Fatalf("second page: %v", next)
	}
	if empty := decodeBody(t, do(handler, "GET", "/api/v24/config/favorites?scope=other", "", nil)); len(empty["items"].([]any)) != 0 || empty["next_cursor"] != "" {
		t.Fatalf("scope filter: %v", empty)
	}
}

type testOwnerKey struct{}

// ownerResolverAPI simulates an authentication middleware that puts a verified
// owner (here taken from a test header) into the request context.
func ownerResolverAPI(t *testing.T, header string) http.Handler {
	t.Helper()
	resolve := func(ctx context.Context) (string, bool) {
		owner, ok := ctx.Value(testOwnerKey{}).(string)
		return owner, ok && owner != ""
	}
	inner, _, _ := configAPI(t, WithOwnerResolver(resolve, "account"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if owner := r.Header.Get(header); owner != "" {
			r = r.WithContext(context.WithValue(r.Context(), testOwnerKey{}, owner))
		}
		inner.ServeHTTP(w, r)
	})
}

func TestConfigOwnerResolverSeparatesData(t *testing.T) {
	handler := ownerResolverAPI(t, "X-Test-Owner")
	created := do(handler, "POST", "/api/v24/config/bookmarks", `{"data":{"name":"mine","sql":"SELECT 1"}}`, map[string]string{"X-Test-Owner": "alice"})
	id := decodeBody(t, created)["id"].(string)
	if response := do(handler, "GET", "/api/v24/config/bookmarks/"+id, "", map[string]string{"X-Test-Owner": "bob"}); response.Code != 404 {
		t.Fatalf("bob reads alice: %d", response.Code)
	}
	if response := do(handler, "GET", "/api/v24/config/bookmarks/"+id, "", nil); response.Code != 401 {
		t.Fatalf("no owner: %d", response.Code)
	}
	if c := decodeBody(t, do(handler, "GET", "/api/v24/capabilities", "", nil)); c["owner_model"] != "account" {
		t.Fatalf("owner model: %v", c["owner_model"])
	}
}

func TestConfigRejectsTheAPITokenAsContent(t *testing.T) {
	handler, _, _ := configAPI(t)
	body, _ := json.Marshal(map[string]any{"key": "k", "data": map[string]any{"note": "x " + testToken + " y"}})
	if response := do(handler, "POST", "/api/v24/config/layouts", string(body), nil); response.Code != 422 || errorCode(t, response) != "secret_rejected" {
		t.Fatalf("token accepted: %d %s", response.Code, response.Body.String())
	}
}

func TestConfigClearFlowAndExportOverHTTP(t *testing.T) {
	handler, _, _ := configAPI(t)
	for _, table := range []string{"a", "b"} {
		do(handler, "POST", "/api/v24/config/favorites", `{"data":{"table":"`+table+`"}}`, nil)
	}
	plan := decodeBody(t, do(handler, "POST", "/api/v24/config/clear/plan", `{"kinds":["favorites"]}`, nil))
	if plan["total_items"] != float64(2) || plan["confirmation"] != "LÖSCHEN 2" {
		t.Fatalf("plan: %v", plan)
	}
	wrong := `{"kinds":["favorites"],"plan_hash":"` + plan["plan_hash"].(string) + `","confirmation":"LÖSCHEN 1"}`
	if response := do(handler, "POST", "/api/v24/config/clear/execute", wrong, nil); response.Code != 422 || errorCode(t, response) != "confirmation_invalid" {
		t.Fatalf("wrong confirmation: %d", response.Code)
	}
	exported := do(handler, "GET", "/api/v24/config/export", "", nil)
	if exported.Code != 200 || !strings.Contains(exported.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %d %v", exported.Code, exported.Header())
	}
	if body := decodeBody(t, exported); body["format"] != "dbm-config-export" || len(body["items"].([]any)) != 2 {
		t.Fatalf("export body: %v", body)
	}
	right := `{"kinds":["favorites"],"plan_hash":"` + plan["plan_hash"].(string) + `","confirmation":"LÖSCHEN 2"}`
	if response := do(handler, "POST", "/api/v24/config/clear/execute", right, nil); response.Code != 200 || decodeBody(t, response)["total"] != float64(2) {
		t.Fatalf("execute: %d %s", response.Code, response.Body.String())
	}
	status := decodeBody(t, do(handler, "GET", "/api/v24/config/status", "", nil))
	if status["total_bytes"] != float64(0) {
		t.Fatalf("status after clear: %v", status)
	}
}

func TestConfigImportOverHTTP(t *testing.T) {
	handler, _, _ := configAPI(t)
	do(handler, "POST", "/api/v24/config/settings", `{"key":"privacy","data":{"history_enabled":true}}`, nil)
	body := `{"items":[
		{"kind":"history","scope":"s","client_ref":"h1","time":"2026-03-01T10:00:00Z","data":{"sql":"SELECT 1"}},
		{"kind":"bookmarks","scope":"s","client_ref":"b1","data":{"name":"n","sql":"SELECT 2"}},
		{"kind":"bookmarks","scope":"s","data":{"name":"n","sql":"ALTER USER x IDENTIFIED BY 'y'"}},
		{"kind":"bookmarks","scope":"s","time":"yesterday","data":{"name":"n","sql":"SELECT 3"}}]}`
	response := do(handler, "POST", "/api/v24/config/import", body, nil)
	result := decodeBody(t, response)
	if response.Code != 200 || result["created"] != float64(2) || result["rejected"] != float64(2) {
		t.Fatalf("import: %d %s", response.Code, response.Body.String())
	}
	results := result["results"].([]any)
	if results[2].(map[string]any)["reason"] != "secret_rejected" || results[3].(map[string]any)["reason"] != "invalid_request" {
		t.Fatalf("reasons: %v", results)
	}
	if strings.Contains(response.Body.String(), "IDENTIFIED") {
		t.Fatal("import result echoes input")
	}
}
