// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestMutationHTTPPresenceAuthenticationAndOutcomes(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("m", 32)
	api := httpapi.New(e, token)
	send := func(path string, body any, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		method := http.MethodPost
		if path == "/api/v3/capabilities" {
			method = http.MethodGet
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		request.Header.Set("Content-Type", "application/json")
		if auth {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	connection := map[string]any{"driver": "sqlite", "database": "http_edit"}
	table := map[string]any{"name": "sample"}
	for _, path := range []string{"/api/v3/capabilities", "/api/v3/tables/eligibility", "/api/v3/rows/update", "/api/v3/rows/delete"} {
		if got := send(path, map[string]any{}, false); got.Code != 401 {
			t.Fatalf("auth %s %d", path, got.Code)
		}
	}
	for _, sql := range []string{`CREATE TABLE sample(id INTEGER PRIMARY KEY,"" BLOB,value TEXT)`, `INSERT INTO sample VALUES(1,X'','')`} {
		if got := send("/api/v1/query", map[string]any{"connection": connection, "sql": sql}, true); got.Code != 200 {
			t.Fatalf("setup %s", got.Body.String())
		}
	}
	snapshot := core.RowSnapshot{Columns: []string{"id", "", "value"}, Values: []core.Cell{{Kind: "integer", Value: "1"}, {Kind: "binary", Value: ""}, {Kind: "text", Value: ""}}}
	base := func() map[string]any {
		return map[string]any{"connection": connection, "table": table, "expected": snapshot, "confirm": true, "changes": []any{map[string]any{"column": "value", "value": map[string]any{"kind": "text", "value": "changed"}}}}
	}
	for _, field := range []string{"confirm", "expected", "changes"} {
		body := base()
		delete(body, field)
		if got := send("/api/v3/rows/update", body, true); got.Code != 400 {
			t.Fatalf("missing %s %d %s", field, got.Code, got.Body.String())
		}
	}
	badChanges := []any{
		map[string]any{"value": map[string]any{"kind": "text", "value": ""}},
		map[string]any{"column": "value"},
		map[string]any{"column": "value", "value": map[string]any{"kind": "text"}},
		map[string]any{"column": "value", "value": map[string]any{"value": ""}},
		map[string]any{"column": "value", "value": map[string]any{"kind": "null", "value": ""}},
		map[string]any{"column": "value", "value": nil},
	}
	for _, change := range badChanges {
		body := base()
		body["changes"] = []any{change}
		if got := send("/api/v3/rows/update", body, true); got.Code != 400 {
			t.Fatalf("bad change accepted %#v %d %s", change, got.Code, got.Body.String())
		}
	}
	for _, expected := range []any{map[string]any{"columns": snapshot.Columns}, map[string]any{"values": snapshot.Values}, map[string]any{"columns": snapshot.Columns, "values": []any{map[string]any{"kind": "integer"}}}} {
		body := base()
		body["expected"] = expected
		if got := send("/api/v3/rows/update", body, true); got.Code != 400 {
			t.Fatalf("bad expected accepted %d %s", got.Code, got.Body.String())
		}
	}
	body := base()
	body["confirm"] = false
	if got := send("/api/v3/rows/update", body, true); got.Code != 400 {
		t.Fatal("unconfirmed write")
	}
	good := send("/api/v3/rows/update", base(), true)
	if good.Code != 200 || good.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("update %d %s", good.Code, good.Body.String())
	}
	if got := send("/api/v3/rows/update", base(), true); got.Code != 409 {
		t.Fatalf("stale snapshot %d %s", got.Code, got.Body.String())
	}
	var result core.MutationResult
	if err := json.Unmarshal(good.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	body = base()
	body["expected"] = result.Row
	body["changes"] = []any{map[string]any{"column": "", "value": map[string]any{"kind": "binary", "value": ""}}, map[string]any{"column": "value", "value": map[string]any{"kind": "null"}}}
	good = send("/api/v3/rows/update", body, true)
	if good.Code != 200 {
		t.Fatalf("empty/null update %d %s", good.Code, good.Body.String())
	}
	json.Unmarshal(good.Body.Bytes(), &result)
	body = base()
	body["expected"] = result.Row
	if got := send("/api/v3/rows/delete", body, true); got.Code != 400 {
		t.Fatal("delete accepted changes")
	}
	delete(body, "changes")
	if got := send("/api/v3/rows/delete", body, true); got.Code != 200 {
		t.Fatalf("delete %d %s", got.Code, got.Body.String())
	}
	body = base()
	body["changes"] = []any{map[string]any{"column": "value", "value": map[string]any{"kind": "text", "value": strings.Repeat("x", httpapi.MaxRequestBytes)}}}
	if got := send("/api/v3/rows/update", body, true); got.Code != 413 {
		t.Fatalf("oversize %d", got.Code)
	}
	api = httpapi.New(unknownCommitEngine{e}, token)
	if got := send("/api/v3/rows/update", base(), true); got.Code != 503 || !strings.Contains(got.Body.String(), "outcome_unknown") {
		t.Fatalf("unknown commit HTTP mapping %d %s", got.Code, got.Body.String())
	}

}

type unknownCommitEngine struct{ *database.Engine }

func (e unknownCommitEngine) Update(context.Context, core.MutationRequest) (core.MutationResult, error) {
	return core.MutationResult{}, &core.Error{Code: "outcome_unknown", Message: "Commit-Ergebnis unbekannt."}
}
