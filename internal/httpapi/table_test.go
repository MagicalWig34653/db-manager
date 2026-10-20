// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestTableRoutesPreserveV1AndRequireAuthentication(t *testing.T) {
	engine, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("x", 32)
	api := httpapi.New(engine, token)
	request := func(method, path, body string, authorized bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if authorized {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	for _, path := range []string{"/api/v2/capabilities", "/api/v2/tables/describe", "/api/v2/tables/browse"} {
		method := http.MethodPost
		if path == "/api/v2/capabilities" {
			method = http.MethodGet
		}
		if got := request(method, path, "{}", false); got.Code != 401 {
			t.Fatalf("unauthenticated %s => %d", path, got.Code)
		}
	}
	old := request(http.MethodGet, "/api/v1/capabilities", "", true)
	var oldBody map[string]any
	if err := json.Unmarshal(old.Body.Bytes(), &oldBody); err != nil {
		t.Fatal(err)
	}
	if oldBody["contract_version"] != "0.1.0" || len(oldBody) != 5 {
		t.Fatalf("v1 capabilities changed %#v", oldBody)
	}
	current := request(http.MethodGet, "/api/v2/capabilities", "", true)
	var currentBody map[string]any
	json.Unmarshal(current.Body.Bytes(), &currentBody)
	if current.Code != 200 || currentBody["contract_version"] != "0.2.0" || currentBody["mutations"] != false {
		t.Fatalf("v2 capabilities %s", current.Body.String())
	}
	for _, sql := range []string{"CREATE TABLE sample(id INTEGER PRIMARY KEY,value TEXT)", "INSERT INTO sample VALUES(1,'kept')", `CREATE TABLE emptyfields("" BLOB,value TEXT)`, `INSERT INTO emptyfields VALUES(X'','')`} {
		payload, _ := json.Marshal(map[string]any{"connection": map[string]string{"driver": "sqlite", "database": "test"}, "sql": sql})
		if got := request(http.MethodPost, "/api/v1/query", string(payload), true); got.Code != 200 {
			t.Fatalf("setup: %s", got.Body.String())
		}
	}
	good := `{"connection":{"driver":"sqlite","database":"test"},"table":{"name":"sample"}}`
	for _, path := range []string{"/api/v2/tables/describe", "/api/v2/tables/browse"} {
		response := request(http.MethodPost, path, good, true)
		if response.Code != 200 {
			t.Fatalf("%s: %s", path, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("API caching enabled")
		}
	}
	for _, body := range []string{
		`{"connection":{"driver":"sqlite","database":"test"},"table":{"name":"sample"},"sql":"DROP TABLE sample"}`,
		`{"connection":{"driver":"sqlite","database":"test"},"table":null}`,
		`{"connection":{"driver":"sqlite","database":"test"},"table":{"name":"sample"},"page_size":1001}`,
	} {
		if got := request(http.MethodPost, "/api/v2/tables/browse", body, true); got.Code == 200 {
			t.Fatalf("bad request accepted %s", body)
		}
	}
	prefix := `{"connection":{"driver":"sqlite","database":"test"},"table":{"name":"emptyfields"},`
	for _, kind := range []string{"text", "binary", "timestamp", "decimal", "integer", "float", "boolean"} {
		for _, value := range []string{`{"kind":"` + kind + `"}`, `{"kind":"` + kind + `","value":null}`} {
			body := prefix + `"filters":[{"column":"value","operator":"eq","value":` + value + `}]}`
			if got := request(http.MethodPost, "/api/v2/tables/browse", body, true); got.Code != 400 {
				t.Fatalf("missing/null filter value accepted %s: %d", body, got.Code)
			}
		}
	}
	for _, suffix := range []string{
		`"filters":[{"column":"value","operator":"eq","value":{"value":""}}]}`,
		`"filters":[{"operator":"eq","value":{"kind":"binary","value":""}}]}`,
		`"sort":[{"direction":"asc"}]}`,
	} {
		if got := request(http.MethodPost, "/api/v2/tables/browse", prefix+suffix, true); got.Code != 400 {
			t.Fatalf("missing required field accepted %s: %d", suffix, got.Code)
		}
	}
	for _, filter := range []string{
		`{"column":"value","operator":"eq","value":{"kind":"text","value":""}}`,
		`{"column":"","operator":"eq","value":{"kind":"binary","value":""}}`,
	} {
		got := request(http.MethodPost, "/api/v2/tables/browse", prefix+`"sort":[{"column":"","direction":"asc"}],"filters":[`+filter+`]}`, true)
		var body struct {
			Result struct {
				Rows [][]any `json:"rows"`
			} `json:"result"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil || got.Code != 200 || len(body.Result.Rows) != 1 {
			t.Fatalf("explicit empty value/column rejected: %d %s", got.Code, got.Body.String())
		}
	}

}
