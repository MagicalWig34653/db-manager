// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestStructureHTTPAuthenticationContractsAndValidation(t *testing.T) {
	engine, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("s", 32)
	api := httpapi.New(engine, token)
	send := func(path string, body any, auth bool, canceled bool) *httptest.ResponseRecorder {
		t.Helper()
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		method := http.MethodPost
		if strings.HasSuffix(path, "capabilities") {
			method = http.MethodGet
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		request.Header.Set("Content-Type", "application/json")
		if auth {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if canceled {
			ctx, cancel := context.WithCancel(request.Context())
			cancel()
			request = request.WithContext(ctx)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	for _, path := range []string{"/api/v5/capabilities", "/api/v5/tables/structure"} {
		if got := send(path, map[string]any{}, false, false); got.Code != 401 {
			t.Fatalf("auth %s %d", path, got.Code)
		}
	}
	for version := 1; version <= 5; version++ {
		got := send("/api/v"+strconv.Itoa(version)+"/capabilities", map[string]any{}, true, false)
		var body map[string]any
		json.Unmarshal(got.Body.Bytes(), &body)
		if got.Code != 200 || body["contract_version"] != "0."+strconv.Itoa(version)+".0" {
			t.Fatalf("capabilities v%d %#v", version, body)
		}
		if version == 5 && (body["mutations"] != false || body["max_terms"] != float64(database.MaxStructureTerms) || body["max_objects_per_section"] != float64(database.MaxStructureObjects)) {
			t.Fatalf("structure limits %#v", body)
		}
	}
	connection := map[string]any{"driver": "sqlite", "database": "structure_http"}
	for _, sql := range []string{`CREATE TABLE sample(id INTEGER PRIMARY KEY,value TEXT UNIQUE)`, `INSERT INTO sample VALUES(1,'retained')`} {
		got := send("/api/v1/query", map[string]any{"connection": connection, "sql": sql}, true, false)
		if got.Code != 200 {
			t.Fatalf("fixture %s", got.Body.String())
		}
	}
	good := map[string]any{"connection": connection, "table": map[string]any{"name": "sample"}}
	got := send("/api/v5/tables/structure", good, true, false)
	var structure core.TableStructure
	json.Unmarshal(got.Body.Bytes(), &structure)
	if got.Code != 200 || got.Header().Get("Cache-Control") != "no-store" || structure.Coverage.Checks != "unavailable" || len(structure.Constraints) != 2 {
		t.Fatalf("structure %d %s", got.Code, got.Body.String())
	}
	for _, body := range []any{map[string]any{"connection": connection, "table": nil}, map[string]any{"connection": connection, "table": map[string]any{"name": "sample"}, "sql": "DROP TABLE sample"}, map[string]any{"connection": connection, "table": map[string]any{"name": "sample"}, "timeout_ms": 100000}} {
		if got := send("/api/v5/tables/structure", body, true, false); got.Code != 400 {
			t.Fatalf("invalid body accepted %d %s", got.Code, got.Body.String())
		}
	}
	if got := send("/api/v5/tables/structure", map[string]any{"connection": connection, "table": map[string]any{"name": strings.Repeat("x", httpapi.MaxRequestBytes)}}, true, false); got.Code != 413 {
		t.Fatalf("request cap %d", got.Code)
	}
	if got := send("/api/v5/tables/structure", good, true, true); got.Code != 408 {
		t.Fatalf("cancellation %d %s", got.Code, got.Body.String())
	}
	read := send("/api/v1/query", map[string]any{"connection": connection, "sql": "SELECT value FROM sample"}, true, false)
	if read.Code != 200 || !strings.Contains(read.Body.String(), "retained") {
		t.Fatal("metadata route changed data or leaked connection")
	}
}
