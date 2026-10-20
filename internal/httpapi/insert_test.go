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

func TestInsertHTTPContractsPresenceAndOutcomes(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("i", 32)
	api := httpapi.New(e, token)
	send := func(path string, body any, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		method := http.MethodPost
		if strings.HasSuffix(path, "capabilities") {
			method = http.MethodGet
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	connection := map[string]any{"driver": "sqlite", "database": "insert_http"}
	table := map[string]any{"name": "sample"}
	for _, path := range []string{"/api/v4/capabilities", "/api/v4/tables/insert-eligibility", "/api/v4/rows/insert"} {
		if got := send(path, map[string]any{}, false); got.Code != 401 {
			t.Fatalf("auth %s %d", path, got.Code)
		}
	}
	for version := 1; version <= 4; version++ {
		path := "/api/v" + string(rune('0'+version)) + "/capabilities"
		got := send(path, map[string]any{}, true)
		var data map[string]any
		json.Unmarshal(got.Body.Bytes(), &data)
		want := "0." + string(rune('0'+version)) + ".0"
		if got.Code != 200 || data["contract_version"] != want {
			t.Fatalf("capability %s %#v", path, data)
		}
	}
	got := send("/api/v1/query", map[string]any{"connection": connection, "sql": `CREATE TABLE sample(id INTEGER PRIMARY KEY,"" BLOB,value TEXT DEFAULT 'omitted')`}, true)
	if got.Code != 200 {
		t.Fatal(got.Body.String())
	}
	value := func(column, kind, content string) map[string]any {
		return map[string]any{"column": column, "value": map[string]any{"kind": kind, "value": content}}
	}
	base := func() map[string]any {
		return map[string]any{"connection": connection, "table": table, "confirm": true, "values": []any{value("id", "integer", "1"), value("", "binary", "")}}
	}
	for _, field := range []string{"confirm", "values"} {
		body := base()
		delete(body, field)
		if got := send("/api/v4/rows/insert", body, true); got.Code != 400 {
			t.Fatalf("missing %s %d", field, got.Code)
		}
	}
	for _, change := range []any{
		map[string]any{"value": map[string]any{"kind": "text", "value": ""}},
		map[string]any{"column": "value"},
		map[string]any{"column": "value", "value": nil},
		map[string]any{"column": "value", "value": map[string]any{"kind": "text"}},
		map[string]any{"column": "value", "value": map[string]any{"value": ""}},
		map[string]any{"column": "value", "value": map[string]any{"kind": "null", "value": ""}},
	} {
		body := base()
		body["values"] = []any{change}
		if got := send("/api/v4/rows/insert", body, true); got.Code != 400 {
			t.Fatalf("bad presence %#v %d %s", change, got.Code, got.Body.String())
		}
	}
	body := base()
	body["confirm"] = false
	if got := send("/api/v4/rows/insert", body, true); got.Code != 400 {
		t.Fatal("unconfirmed insert")
	}
	good := send("/api/v4/rows/insert", base(), true)
	var result core.InsertResult
	json.Unmarshal(good.Body.Bytes(), &result)
	if good.Code != 200 || good.Header().Get("Cache-Control") != "no-store" || result.Row.Values[1] != (core.Cell{Kind: "binary", Value: ""}) || result.Row.Values[2].Value != "omitted" {
		t.Fatalf("good insert %d %s", good.Code, good.Body.String())
	}
	if got := send("/api/v4/rows/insert", base(), true); got.Code != 409 {
		t.Fatalf("duplicate %d %s", got.Code, got.Body.String())
	}
	for i, cell := range []map[string]any{{"kind": "null"}, {"kind": "text", "value": ""}} {
		body = base()
		body["values"] = []any{value("id", "integer", string(rune('2'+i))), map[string]any{"column": "value", "value": cell}}
		if got := send("/api/v4/rows/insert", body, true); got.Code != 200 {
			t.Fatalf("explicit empty/null %d %s", got.Code, got.Body.String())
		}
	}
	body = base()
	body["values"] = []any{value("value", "text", strings.Repeat("x", httpapi.MaxRequestBytes))}
	if got := send("/api/v4/rows/insert", body, true); got.Code != 413 {
		t.Fatalf("request cap %d", got.Code)
	}
	api = httpapi.New(unknownInsertCommitEngine{e}, token)
	if got := send("/api/v4/rows/insert", base(), true); got.Code != 503 || !strings.Contains(got.Body.String(), "outcome_unknown") {
		t.Fatalf("unknown commit %d %s", got.Code, got.Body.String())
	}
}

type unknownInsertCommitEngine struct{ *database.Engine }

func (e unknownInsertCommitEngine) Insert(context.Context, core.InsertRequest) (core.InsertResult, error) {
	return core.InsertResult{}, &core.Error{Code: "outcome_unknown", Message: "Commit-Ergebnis ist unbekannt."}
}
