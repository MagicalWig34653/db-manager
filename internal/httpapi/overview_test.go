// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestOverviewHTTP(t *testing.T) {
	e, _ := database.New(t.TempDir(), nil)
	token := strings.Repeat("o", 32)
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
	conn := `{"driver":"sqlite","database":"overview_http"}`
	for _, path := range []string{"/api/v7/databases", "/api/v7/objects", "/api/v7/definition", "/api/v7/server-info"} {
		if w := send(http.MethodPost, path, `{"connection":`+conn+`}`, false); w.Code != 401 {
			t.Fatalf("%s without token: %d", path, w.Code)
		}
	}
	w := send(http.MethodGet, "/api/v7/capabilities", "", true)
	var capabilities struct {
		Version  string                                                 `json:"contract_version"`
		Features []string                                               `json:"features"`
		Drivers  map[string]map[string]struct{ Support, Reason string } `json:"drivers"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &capabilities) != nil || capabilities.Version != "0.7.0" || len(capabilities.Features) != 4 {
		t.Fatalf("capabilities %d %s", w.Code, w.Body.String())
	}
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, action := range []string{"databases", "objects", "definition", "server_info", "exact_rows"} {
			c := capabilities.Drivers[driver][action]
			if c.Support == "" || (c.Support != "supported" && c.Reason == "") {
				t.Fatalf("capability %s/%s %#v", driver, action, c)
			}
		}
	}
	for path, body := range map[string]string{
		"/api/v7/objects":     `{"connection":` + conn + `,"limit":1001}`,
		"/api/v7/definition":  `{"connection":` + conn + `,"object":{"name":""}}`,
		"/api/v7/databases":   `{"connection":` + conn + `,"unknown":1}`,
		"/api/v7/server-info": `{"connection":{"driver":"oracle"}}`,
	} {
		if w := send(http.MethodPost, path, body, true); w.Code != 400 {
			t.Fatalf("%s invalid accepted: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := send(http.MethodPost, "/api/v7/definition", `{"connection":`+conn+`}`, true); w.Code != 400 {
		t.Fatalf("missing object: %d", w.Code)
	}
	if w := send(http.MethodPost, "/api/v7/definition", `{"connection":`+conn+`,"object":{"name":"absent"}}`, true); w.Code != 404 || !strings.Contains(w.Body.String(), "object_not_found") {
		t.Fatalf("absent object: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/api/v7/objects", `{"connection":`+conn+`}`, true); w.Code != 200 || !strings.Contains(w.Body.String(), `"objects":[]`) {
		t.Fatalf("objects: %d %s", w.Code, w.Body.String())
	}
	if w := send(http.MethodPost, "/api/v7/server-info", `{"connection":`+conn+`}`, true); w.Code != 200 || strings.Contains(w.Body.String(), "password") {
		t.Fatalf("server-info: %d %s", w.Code, w.Body.String())
	}
}
