// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

// Reviewer B (wave 4): the read-only surfaces must expose exactly GET capabilities and POST reads.
func TestReviewB17B27NoWriteSurface(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("r", 32)
	api := httpapi.New(e, token)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	body := `{"connection":{"driver":"sqlite","database":"x"}}`
	paths := []string{"/api/v17/status", "/api/v17/variables", "/api/v17/processes", "/api/v17/capabilities"}
	for _, a := range []string{"sample", "analysis", "advisor", "replication", "binary-logs", "binary-events", "profiling", "engines"} {
		paths = append(paths, "/api/v20/monitor/"+a)
	}
	paths = append(paths, "/api/v20/capabilities")
	for _, p := range paths {
		for _, m := range []string{"PUT", "PATCH", "DELETE"} {
			if w := send(m, p, body); w.Code == 200 || w.Code >= 500 {
				t.Errorf("%s %s -> %d", m, p, w.Code)
			}
		}
		if strings.HasSuffix(p, "capabilities") {
			if w := send("POST", p, body); w.Code == 200 {
				t.Errorf("POST %s -> 200", p)
			}
		} else if w := send("GET", p, ""); w.Code == 200 {
			t.Errorf("GET %s -> 200", p)
		}
	}
	for _, p := range []string{"kill", "processes/kill", "variables/set", "variables", "flush", "monitor/kill", "monitor/replication/start", "monitor/replication/stop", "monitor/set", "monitor/profiling/execute", "monitor/explain-analyze"} {
		for _, v := range []string{"/api/v17/", "/api/v20/"} {
			if v == "/api/v17/" && p == "variables" {
				continue
			}
			if w := send("POST", v+p, body); w.Code != 404 {
				t.Errorf("unexpected route %s%s -> %d", v, p, w.Code)
			}
		}
	}
	// profiling cannot be abused through other actions or write statements
	for _, sql := range []string{"DELETE FROM t", "SELECT 1; DROP TABLE t", "SELECT SLEEP(5)", "EXPLAIN ANALYZE SELECT 1"} {
		w := send("POST", "/api/v20/monitor/profiling", `{"connection":{"driver":"mysql","host":"127.0.0.1","port":1,"database":"x"},"sql":"`+sql+`"}`)
		if w.Code != 400 {
			t.Errorf("profiling %q -> %d %s", sql, w.Code, w.Body.String())
		}
	}
}
