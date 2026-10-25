// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
)

func TestSQLBatchHTTP(t *testing.T) {
	engine, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	api := New(engine, "token")
	send := func(method, path, body, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	for _, route := range []string{"capabilities", "sql/batch", "sql/explain", "sql/simulate"} {
		method := "POST"
		if route == "capabilities" {
			method = "GET"
		}
		if response := send(method, "/api/v12/"+route, "{}", ""); response.Code != 401 {
			t.Fatal(route, response.Code)
		}
	}
	response := send("GET", "/api/v12/capabilities", "", "token")
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if dir := os.Getenv("DBM_SQLBATCH_DUMP"); dir != "" {
		if err = os.WriteFile(filepath.Join(dir, "http.SQLBatchCapabilities.json"), response.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{
		`{"connection":{"driver":"sqlite","database":"probe"},"sql":"CREATE TABLE nope(x)","confirm":false}`,
		`{"connection":{"driver":"sqlite","database":"probe"},"sql":"SELECT 1","confirm":true,"analyze":true}`,
		`{"connection":{"driver":"sqlite","database":"probe"},"sql":"SELECT 1","confirm":true,"max_rows":10001}`,
		`{"connection":{"driver":"sqlite","database":"probe"},"sql":"SELECT 1","confirm":true,"max_rows":null}`,
	} {
		response = send("POST", "/api/v12/sql/batch", body, "token")
		if response.Code != 400 {
			t.Fatalf("request %s returned %d", body, response.Code)
		}
	}
	for _, action := range []string{"batch", "explain", "simulate"} {
		query := "SELECT 1;SELECT 2"
		if action == "explain" {
			query = "SELECT 1"
		}
		if action == "simulate" {
			query = "INSERT INTO nonexistent VALUES (1)"
		}
		payload, _ := json.Marshal(map[string]any{"connection": map[string]string{"driver": "sqlite", "database": "probe"}, "sql": query, "confirm": true})
		response = send("POST", "/api/v12/sql/"+action, string(payload), "token")
		if response.Code != 200 {
			t.Fatalf("%s %d %s", action, response.Code, response.Body.String())
		}
	}
	response = send("POST", "/api/v12/sql/batch", `{"connection":{"driver":"sqlite","database":"probe"},"sql":"SELECT secret_value FROM nonexistent","confirm":true}`, "token")
	if response.Code != 200 || strings.Contains(response.Body.String(), "secret_value") {
		t.Fatal(response.Body.String())
	}
	response = send("POST", "/api/v1/query", `{"connection":{"driver":"sqlite","database":"probe"},"sql":"SELECT 1;SELECT 2"}`, "token")
	if response.Code != 422 {
		t.Fatal("v1 batch behavior changed", response.Code)
	}
}
