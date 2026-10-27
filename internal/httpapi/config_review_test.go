// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Independent W-P20 review (docs/handoffs/W-P20-backend-review.md). TestReviewGap_*
// pin current undesirable behaviour of a finding; flip them when fixed.

func reviewJSON(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	return do(handler, method, path, body, headers)
}

func TestReviewHolds_PreconditionAndEtagForms(t *testing.T) {
	handler, _, _ := configAPI(t)
	created := reviewJSON(handler, "POST", "/api/v24/config/settings", `{"key":"navigation","data":{"a":1}}`, nil)
	if created.Code != 201 || created.Header().Get("ETag") != `"1"` {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	id := decodeBody(t, created)["id"].(string)
	path := "/api/v24/config/settings/" + id
	for name, tc := range map[string]struct {
		header map[string]string
		want   int
	}{
		"missing":         {nil, 428},
		"empty":           {map[string]string{"If-Match": ""}, 428},
		"stale":           {map[string]string{"If-Match": `"7"`}, 412},
		"weak":            {map[string]string{"If-Match": `W/"1"`}, 400},
		"unquoted":        {map[string]string{"If-Match": `1`}, 400},
		"list":            {map[string]string{"If-Match": `"1", "2"`}, 400},
		"negative":        {map[string]string{"If-Match": `"-1"`}, 400},
		"zero":            {map[string]string{"If-Match": `"0"`}, 412},
		"overflow digits": {map[string]string{"If-Match": `"99999999999999999999"`}, 400},
		"star spaced":     {map[string]string{"If-Match": ` * `}, 0},
	} {
		got := reviewJSON(handler, "PUT", path, `{"data":{"a":2}}`, tc.header).Code
		if tc.want != 0 && got != tc.want {
			t.Errorf("%s: PUT %d want %d", name, got, tc.want)
		}
		if got2 := reviewJSON(handler, "DELETE", path, "", tc.header).Code; tc.want != 0 && got2 != tc.want {
			t.Errorf("%s: DELETE %d want %d", name, got2, tc.want)
		}
	}
	// Both PUT and DELETE with the correct version succeed once; the repeat gets 412/404, never silently succeeds.
	ok := reviewJSON(handler, "PUT", path, `{"data":{"a":2}}`, map[string]string{"If-Match": `"1"`})
	again := reviewJSON(handler, "PUT", path, `{"data":{"a":3}}`, map[string]string{"If-Match": `"1"`})
	if ok.Code != 200 || ok.Header().Get("ETag") != `"2"` || again.Code != 412 {
		t.Errorf("sequence: %d %s %d", ok.Code, ok.Header().Get("ETag"), again.Code)
	}
}

func TestReviewHolds_ErrorsNeverEchoInput(t *testing.T) {
	handler, _, _ := configAPI(t)
	canary := "CANARY_8f3a_zq"
	for name, tc := range map[string]struct{ method, path, body string }{
		"bad json":       {"POST", "/api/v24/config/bookmarks", `{"data":{"name":"` + canary + `"`},
		"unknown field":  {"POST", "/api/v24/config/bookmarks", `{"` + canary + `":1}`},
		"unknown kind":   {"GET", "/api/v24/config/" + canary, ``},
		"bad id":         {"GET", "/api/v24/config/bookmarks/" + canary, ``},
		"secret":         {"POST", "/api/v24/config/bookmarks", `{"data":{"name":"` + canary + `","sql":"SET PASSWORD='` + canary + `'"}}`},
		"bad scope":      {"POST", "/api/v24/config/bookmarks", `{"scope":"` + canary + `!","data":{"name":"n","sql":"SELECT 1"}}`},
		"bad param":      {"GET", "/api/v24/config/bookmarks?" + canary + "=1", ``},
		"bad cursor":     {"GET", "/api/v24/config/bookmarks?cursor=" + canary, ``},
		"bad kinds plan": {"POST", "/api/v24/config/clear/plan", `{"kinds":["` + canary + `"]}`},
		"bad time":       {"POST", "/api/v24/config/import", `{"items":[{"kind":"favorites","time":"` + canary + `","data":{"table":"t"}}]}`},
		"invalid data":   {"POST", "/api/v24/config/layouts", `{"key":"k","data":{"a":null,"` + canary + `":1}}`},
	} {
		response := reviewJSON(handler, tc.method, tc.path, tc.body, nil)
		if strings.Contains(response.Body.String(), canary) || strings.Contains(response.Header().Get("Location"), canary) {
			t.Errorf("%s echoes input: %s", name, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control %q", name, response.Header().Get("Cache-Control"))
		}
	}
}

// F-13 (fixed): duplicate keys anywhere in a request are rejected.
func TestReviewHolds_DuplicateEnvelopeKeys(t *testing.T) {
	handler, _, _ := configAPI(t)
	for _, body := range []string{`{"scope":"a","scope":"b","data":{"table":"x"}}`, `{"data":{"table":"x"},"data":{"table":"y"}}`} {
		if got := reviewJSON(handler, "POST", "/api/v24/config/favorites", body, nil).Code; got != 400 {
			t.Errorf("%s: %d, want 400", body, got)
		}
	}
}

// Must hold: a duplicate key inside the document is rejected even when the
// envelope decoder would accept it.
func TestReviewHolds_DuplicateKeyInsideData(t *testing.T) {
	handler, _, _ := configAPI(t)
	response := reviewJSON(handler, "POST", "/api/v24/config/layouts", `{"key":"k","data":{"a":1,"a":2}}`, nil)
	if response.Code != 400 && response.Code != 422 {
		t.Errorf("duplicate key in data: %d", response.Code)
	}
	response = reviewJSON(handler, "POST", "/api/v24/config/layouts", `{"key":"k","data":{"a":1},"data":{"a":null}}`, nil)
	if response.Code != 400 {
		t.Errorf("null hidden behind duplicate envelope key: %d", response.Code)
	}
}

func TestReviewHolds_BodyAndItemLimits(t *testing.T) {
	handler, _, _ := configAPI(t)
	// Exactly at the request limit is processed (and rejected for content), above is 413.
	big := `{"data":{"name":"n","sql":"` + strings.Repeat("x", MaxRequestBytes) + `"}}`
	if got := reviewJSON(handler, "POST", "/api/v24/config/bookmarks", big, nil).Code; got != 413 {
		t.Errorf("oversized body: %d", got)
	}
	// 101 import items.
	items := strings.TrimSuffix(strings.Repeat(`{"kind":"favorites","data":{"table":"t"}},`, 101), ",")
	if got := reviewJSON(handler, "POST", "/api/v24/config/import", `{"items":[`+items+`]}`, nil).Code; got != 400 {
		t.Errorf("101 items: %d", got)
	}
	// Wrong content type and empty body.
	request := httptest.NewRequest("POST", "/api/v24/config/bookmarks", strings.NewReader(`{"data":{}}`))
	request.Header.Set("Authorization", "Bearer "+testToken)
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 415 {
		t.Errorf("text/plain: %d", response.Code)
	}
	if got := reviewJSON(handler, "POST", "/api/v24/config/bookmarks", ``, nil).Code; got != 400 {
		t.Errorf("empty body: %d", got)
	}
	// Query abuse.
	for _, query := range []string{"limit=101", "limit=0", "limit=-1", "limit=1&limit=2", "limit=1e2", "limit=%20", "scope=a&scope=b", "key=x", "scope=" + strings.Repeat("a", 129)} {
		if got := reviewJSON(handler, "GET", "/api/v24/config/bookmarks?"+query, "", nil).Code; got != 400 {
			t.Errorf("query %q: %d", query, got)
		}
	}
}

func TestReviewHolds_DeletePlanProtocolOverHTTP(t *testing.T) {
	handler, _, _ := configAPI(t)
	for i := 0; i < 2; i++ {
		reviewJSON(handler, "POST", "/api/v24/config/favorites", `{"data":{"table":"t`+string(rune('a'+i))+`"}}`, nil)
	}
	plan := decodeBody(t, reviewJSON(handler, "POST", "/api/v24/config/clear/plan", `{"kinds":["favorites"]}`, nil))
	hash, confirmation := plan["plan_hash"].(string), plan["confirmation"].(string)
	if confirmation != "LÖSCHEN 2" {
		t.Fatalf("confirmation %q", confirmation)
	}
	for name, body := range map[string]string{
		"no confirmation":    `{"kinds":["favorites"],"plan_hash":"` + hash + `"}`,
		"lowercase":          `{"kinds":["favorites"],"plan_hash":"` + hash + `","confirmation":"löschen 2"}`,
		"no hash":            `{"kinds":["favorites"],"confirmation":"LÖSCHEN 2"}`,
		"kinds omitted":      `{"plan_hash":"` + hash + `","confirmation":"LÖSCHEN 2"}`,
		"duplicate kinds":    `{"kinds":["favorites","favorites"],"plan_hash":"` + hash + `","confirmation":"LÖSCHEN 2"}`,
		"unknown kind":       `{"kinds":["nope"],"plan_hash":"` + hash + `","confirmation":"LÖSCHEN 2"}`,
		"null kinds":         `{"kinds":null,"plan_hash":"` + hash + `","confirmation":"LÖSCHEN 2"}`,
		"hash of other kind": `{"kinds":["recents"],"plan_hash":"` + hash + `","confirmation":"LÖSCHEN 0"}`,
	} {
		if got := reviewJSON(handler, "POST", "/api/v24/config/clear/execute", body, nil).Code; got < 400 && name != "hash of other kind" {
			t.Errorf("%s executed: %d", name, got)
		}
	}
	if list := decodeBody(t, reviewJSON(handler, "GET", "/api/v24/config/favorites", "", nil)); len(list["items"].([]any)) != 2 {
		t.Errorf("rows deleted by rejected requests: %v", list["items"])
	}
	ok := reviewJSON(handler, "POST", "/api/v24/config/clear/execute", `{"kinds":["favorites"],"plan_hash":"`+hash+`","confirmation":"LÖSCHEN 2"}`, nil)
	if ok.Code != 200 {
		t.Errorf("valid execute: %d %s", ok.Code, ok.Body.String())
	}
	// Replay of the same plan after execution: stale (hash of empty set differs), nothing happens twice.
	if replay := reviewJSON(handler, "POST", "/api/v24/config/clear/execute", `{"kinds":["favorites"],"plan_hash":"`+hash+`","confirmation":"LÖSCHEN 0"}`, nil); replay.Code != 409 {
		t.Errorf("replay: %d", replay.Code)
	}
}

// F-03 (fixed): a resolver can never reach the reserved legacy owner "single",
// and only receives the request context. Invalid owners never reach the store.
func TestReviewHolds_ResolverCannotCollideWithLegacyOwner(t *testing.T) {
	handler := ownerResolverAPI(t, "X-Test-User")
	// The legacy owner's data (written without a resolver) is unreachable through "single".
	if got := reviewJSON(handler, "GET", "/api/v24/config/favorites", "", map[string]string{"X-Test-User": "single"}).Code; got != 401 {
		t.Errorf("resolver value 'single': %d, want 401", got)
	}
	reviewJSON(handler, "POST", "/api/v24/config/favorites", `{"data":{"table":"t"}}`, map[string]string{"X-Test-User": "alice"})
	if other := decodeBody(t, reviewJSON(handler, "GET", "/api/v24/config/favorites", "", map[string]string{"X-Test-User": "bob"})); len(other["items"].([]any)) != 0 {
		t.Error("bob sees alice's rows")
	}
	for _, bad := range []string{"a b", "a|b", "../x", strings.Repeat("a", 65), "aé"} {
		if got := reviewJSON(handler, "GET", "/api/v24/config/favorites", "", map[string]string{"X-Test-User": bad}).Code; got != 400 {
			t.Errorf("owner %q: %d", bad, got)
		}
	}
	if got := reviewJSON(handler, "GET", "/api/v24/config/favorites", "", nil).Code; got != 401 {
		t.Errorf("resolver without owner: %d", got)
	}
}

// Must hold: capabilities needs the Bearer token and reveals only coarse state;
// data routes fail closed with 503 after authentication, never before.
func TestReviewHolds_CapabilitiesAndFailClosed(t *testing.T) {
	handler, state, _ := configAPI(t)
	request := httptest.NewRequest("GET", "/api/v24/capabilities", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 || strings.Contains(response.Body.String(), "owner_model") {
		t.Errorf("unauthenticated capabilities: %d", response.Code)
	}
	state.up = false
	caps := decodeBody(t, reviewJSON(handler, "GET", "/api/v24/capabilities", "", nil))
	if caps["available"] != false || caps["reason"] != "unreachable" {
		t.Errorf("capabilities during outage: %v", caps)
	}
	for _, route := range []string{"GET /api/v24/config/status", "GET /api/v24/config/export", "GET /api/v24/config/bookmarks"} {
		parts := strings.SplitN(route, " ", 2)
		r := reviewJSON(handler, parts[0], parts[1], "", nil)
		if r.Code != 503 || r.Header().Get("Retry-After") == "" {
			t.Errorf("%s during outage: %d", route, r.Code)
		}
		anon := httptest.NewRecorder()
		handler.ServeHTTP(anon, httptest.NewRequest(parts[0], parts[1], nil))
		if anon.Code != 401 {
			t.Errorf("%s anonymous during outage: %d", route, anon.Code)
		}
	}
	// A wrong token with valid shape is 401 as well.
	wrong := httptest.NewRequest("GET", "/api/v24/config/status", nil)
	wrong.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, wrong)
	if rec.Code != 401 {
		t.Errorf("wrong token: %d", rec.Code)
	}
}

// Must hold: the API token cannot be stored (even in documents) and the check
// is registered by New() regardless of option order.
func TestReviewHolds_BearerTokenLiteralRejected(t *testing.T) {
	handler, _, _ := configAPI(t)
	for _, kind := range []string{"layouts", "settings"} {
		r := reviewJSON(handler, "POST", "/api/v24/config/"+kind, `{"key":"abc","data":{"note":"x `+testToken+` y"}}`, nil)
		if r.Code != 422 {
			t.Errorf("%s: %d", kind, r.Code)
		}
	}
}
