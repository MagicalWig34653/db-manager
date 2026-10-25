// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestColumnsHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("c", 32)
	api := httpapi.New(e, token)
	send := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"columns/plan", "columns/alter"} {
		if w := send("POST", "/api/v9/"+path, `{}`, false); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	w := send("GET", "/api/v9/capabilities", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var cap map[string]any
	if err = json.Unmarshal(w.Body.Bytes(), &cap); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("DBM_COLUMNS_DUMP"); dir != "" {
		if err = os.WriteFile(filepath.Join(dir, "http.ColumnsCapabilities.json"), w.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	conn := `{"driver":"sqlite","database":"columns_http"}`
	if _, err = e.Query(context.Background(), core.QueryRequest{Connection: core.Connection{Driver: "sqlite", Database: "columns_http"}, SQL: `CREATE TABLE fixture(id BIGINT, value TEXT)`}); err != nil {
		t.Fatal(err)
	}
	base := `"connection":` + conn + `,"table":{"name":"fixture"}`
	change := `"changes":[{"action":"drop","column":"value"}]`
	for _, body := range []string{`{}`, `{` + base + `,"changes":[]}`, `{` + base + `,"changes":null}`, `{` + base + `,"changes":[{"action":"drop","column":"value","sql":"DROP"}]}`, `{` + base + `,` + change + `,"confirm":{"action":"columns_alter"}}`} {
		if w := send("POST", "/api/v9/columns/plan", body, true); w.Code != 400 {
			t.Fatal(body, w.Code, w.Body.String())
		}
	}
	if w := send("POST", "/api/v9/columns/plan", `{`+base+`,"changes":[{"action":"generated"}]}`, true); w.Code != 422 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = send("POST", "/api/v9/columns/plan", `{`+base+`,`+change+`}`, true)
	var plan core.ColumnsPlan
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	execute := func(digest, target string) *httptest.ResponseRecorder {
		body := `{` + base + `,` + change + `,"expected_plan_sha256":"` + digest + `","confirm":{"action":"columns_alter","target_name":"` + target + `"}}`
		return send("POST", "/api/v9/columns/alter", body, true)
	}
	if w = execute(strings.Repeat("0", 64), "fixture"); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = execute(plan.SHA256, "wrong"); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = execute(plan.SHA256, "fixture"); w.Code != 200 || !strings.Contains(w.Body.String(), `"outcome":"completed"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = execute(plan.SHA256, "fixture"); w.Code != 404 {
		t.Fatal("replay", w.Code, w.Body.String())
	}
	if w = send(http.MethodPost, "/api/v9/columns/plan", `{`+base+`,"changes":[{"action":"drop","column":"id"}],"padding":"`+strings.Repeat("x", httpapi.MaxRequestBytes)+`"}`, true); w.Code != 413 && w.Code != 400 {
		t.Fatal(w.Code)
	}
}
