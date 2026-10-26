// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"context"
	"encoding/json"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPartitionsHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	api := httpapi.New(e, strings.Repeat("p", 32))
	send := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+strings.Repeat("p", 32))
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	dump := func(component string, w *httptest.ResponseRecorder) {
		t.Helper()
		if dir := os.Getenv("DBM_PARTITIONS_DUMP"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, t.Name()+"."+component+".json"), w.Body.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, group := range []string{"partitions", "prefix"} {
		for _, mode := range []string{"list", "plan", "execute"} {
			path := "/api/v21/tables/" + group + "/" + mode
			if w := send("POST", path, `{}`, false); w.Code != 401 {
				t.Fatal(path, w.Code)
			}
		}
	}
	w := send("GET", "/api/v21/capabilities", "", true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	dump("PartitionsCapabilities", w)
	conn := `{"driver":"sqlite","database":"partition_http"}`
	for _, body := range []string{`{"connection":` + conn + `,"table":{"name":"x"},"sql":"DROP TABLE x"}`, `{"connection":` + conn + `,"table":null}`, `{"connection":` + conn + `,"table":{"name":"x"},"action":null}`} {
		if w = send("POST", "/api/v21/tables/partitions/plan", body, true); w.Code != 400 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	w = send("POST", "/api/v21/tables/partitions/list", `{"connection":`+conn+`,"table":{"name":"x"}}`, true)
	if w.Code != 422 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	c := core.Connection{Driver: "sqlite", Database: "partition_http"}
	_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE http_one(id INTEGER)"})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"connection":` + conn + `,"prefix":"http_","action":"drop"}`
	w = send("POST", "/api/v21/tables/prefix/plan", body, true)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	dump("StructurePlan", w)
	var p core.StructurePlan
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	bad := strings.TrimSuffix(body, "}") + `,"confirm":{"action":"prefix_drop","target_name":"wrong"},"expected_plan_sha256":"` + p.SHA256 + `"}`
	w = send("POST", "/api/v21/tables/prefix/execute", bad, true)
	if w.Code != 400 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	good := strings.Replace(bad, `"wrong"`, `"http_"`, 1)
	w = send("POST", "/api/v21/tables/prefix/execute", good, true)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	dump("StructureResult", w)
}
