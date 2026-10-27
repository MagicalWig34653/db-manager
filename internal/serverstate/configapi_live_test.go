// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

const liveToken = "synthetic-live-token-with-32-chars!"

type liveAPI struct {
	t       *testing.T
	handler http.Handler
	dump    string
}

func (a liveAPI) call(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+liveToken)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	a.handler.ServeHTTP(response, request)
	return response
}

// record stores the unmodified response body as a real fixture of the component.
func (a liveAPI) record(component, name string, response *httptest.ResponseRecorder) {
	a.t.Helper()
	if a.dump == "" {
		return
	}
	var pretty json.RawMessage = response.Body.Bytes()
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.dump, component+".real-"+name+".json"), append(out, '\n'), 0o600); err != nil {
		a.t.Fatal(err)
	}
}

func field(t *testing.T, response *httptest.ResponseRecorder, name string) any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %s", response.Body.String())
	}
	return body[name]
}

func TestConfigAPIOnRealStateStore(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	p := newProxy(t, net.JoinHostPort(cfg.Host, strconv.Itoa(f.port)))
	live := cfg
	live.Host, live.Port = "localhost", p.port
	store := openStore(t, live)
	if err := store.WaitReady(context.Background(), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	store.Monitor(100 * time.Millisecond)
	api := liveAPI{t: t, dump: os.Getenv("DBM_CONFIG_DUMP")}
	api.handler = httpapi.New(nil, liveToken, httpapi.WithStateStore(store), httpapi.WithConfigStore(configstore.NewService(store.ConfigRepo())))

	// Honest capability report, available.
	caps := api.call("GET", "/api/v24/capabilities", "", nil)
	if caps.Code != 200 || field(t, caps, "available") != true || field(t, caps, "mode") != "server" {
		t.Fatalf("capabilities: %s", caps.Body.String())
	}
	api.record("ConfigCapabilities", "mariadb", caps)

	// History needs an explicit opt-in (settings.privacy).
	if r := api.call("POST", "/api/v24/config/history", `{"scope":"conn1","data":{"sql":"SELECT 1"}}`, nil); r.Code != 409 || !strings.Contains(r.Body.String(), "history_disabled") {
		t.Fatalf("history without opt-in: %d %s", r.Code, r.Body.String())
	}
	privacy := api.call("POST", "/api/v24/config/settings", `{"key":"privacy","data":{"history_enabled":true}}`, nil)
	if privacy.Code != 201 {
		t.Fatalf("privacy: %s", privacy.Body.String())
	}
	if r := api.call("POST", "/api/v24/config/history", `{"scope":"conn1","client_ref":"local-1","data":{"sql":"SELECT * FROM orders WHERE id = 42"}}`, nil); r.Code != 201 {
		t.Fatalf("history: %s", r.Body.String())
	}
	created := api.call("POST", "/api/v24/config/bookmarks", `{"scope":"conn1","client_ref":"bm-1","data":{"name":"Offene Bestellungen","sql":"SELECT * FROM orders WHERE status = :status","variables":["status"]}}`, nil)
	if created.Code != 201 || created.Header().Get("ETag") != `"1"` {
		t.Fatalf("bookmark: %d %s", created.Code, created.Body.String())
	}
	api.record("ConfigItem", "mariadb", created)
	id := field(t, created, "id").(string)

	// Optimistic concurrency over HTTP: eight clients hold version 1, exactly one wins.
	var wg sync.WaitGroup
	var wins, conflicts atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"data":{"name":"Offene Bestellungen","sql":"SELECT ` + strconv.Itoa(i) + `"}}`
			switch api.call("PUT", "/api/v24/config/bookmarks/"+id, body, map[string]string{"If-Match": `"1"`}).Code {
			case 200:
				wins.Add(1)
			case 412:
				conflicts.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != 7 {
		t.Fatalf("lost update over HTTP: %d wins, %d conflicts", wins.Load(), conflicts.Load())
	}
	conflict := api.call("PUT", "/api/v24/config/bookmarks/"+id, `{"data":{"name":"x","sql":"SELECT 9"}}`, map[string]string{"If-Match": `"1"`})
	api.record("Error", "version-conflict", conflict)

	for _, table := range []string{"orders", "customers", "items"} {
		api.call("POST", "/api/v24/config/favorites", `{"scope":"conn1","data":{"database":"shop","table":"`+table+`"}}`, nil)
	}
	page := api.call("GET", "/api/v24/config/favorites?scope=conn1&limit=2", "", nil)
	if page.Code != 200 || len(field(t, page, "items").([]any)) != 2 || field(t, page, "next_cursor") == "" {
		t.Fatalf("page: %s", page.Body.String())
	}
	api.record("ConfigList", "mariadb", page)
	api.call("POST", "/api/v24/config/layouts", `{"scope":"conn1","key":"table:shop.orders","data":{"columns":["id","total","password_hash"],"widths":{"id":80}}}`, nil)
	status := api.call("GET", "/api/v24/config/status", "", nil)
	if status.Code != 200 {
		t.Fatal(status.Body.String())
	}
	api.record("ConfigStatus", "mariadb", status)

	imported := api.call("POST", "/api/v24/config/import", `{"items":[
		{"kind":"history","scope":"conn2","client_ref":"local-1","time":"2026-03-01T10:00:00Z","data":{"sql":"SELECT 1"}},
		{"kind":"bookmarks","scope":"conn2","client_ref":"bm-9","data":{"name":"Import","sql":"SELECT 2"}},
		{"kind":"bookmarks","scope":"conn2","data":{"name":"Fremd","sql":"SET PASSWORD = 'geheim'"}}]}`, nil)
	if imported.Code != 200 || field(t, imported, "created") != float64(2) || field(t, imported, "rejected") != float64(1) {
		t.Fatalf("import: %s", imported.Body.String())
	}
	api.record("ConfigImportResult", "mariadb", imported)
	exported := api.call("GET", "/api/v24/config/export", "", nil)
	if exported.Code != 200 || strings.Contains(exported.Body.String(), "geheim") {
		t.Fatalf("export: %d", exported.Code)
	}
	api.record("ConfigExport", "mariadb", exported)

	plan := api.call("POST", "/api/v24/config/clear/plan", `{"kinds":["history","bookmarks"],"scope":"conn2"}`, nil)
	if plan.Code != 200 || field(t, plan, "total_items") != float64(2) {
		t.Fatalf("plan: %s", plan.Body.String())
	}
	api.record("ConfigClearPlan", "mariadb", plan)
	hash := field(t, plan, "plan_hash").(string)
	execBody := `{"kinds":["history","bookmarks"],"scope":"conn2","plan_hash":"` + hash + `","confirmation":"%s"}`
	if r := api.call("POST", "/api/v24/config/clear/execute", strings.Replace(execBody, "%s", "LÖSCHEN 1", 1), nil); r.Code != 422 {
		t.Fatalf("wrong confirmation accepted: %d", r.Code)
	}
	cleared := api.call("POST", "/api/v24/config/clear/execute", strings.Replace(execBody, "%s", "LÖSCHEN 2", 1), nil)
	if cleared.Code != 200 || field(t, cleared, "total") != float64(2) {
		t.Fatalf("clear: %s", cleared.Body.String())
	}
	api.record("ConfigClearResult", "mariadb", cleared)
	if r := api.call("GET", "/api/v24/config/bookmarks?scope=conn1", "", nil); field(t, r, "items") == nil || len(field(t, r, "items").([]any)) != 1 {
		t.Fatalf("scoped clear removed other scopes: %s", r.Body.String())
	}

	// Outage: fail closed with 503 and an honest capability report; recovery without restart.
	port := p.port
	p.stop()
	eventually(t, 5*time.Second, "unhealthy", func() bool { return !store.Healthy() })
	down := api.call("GET", "/api/v24/config/bookmarks", "", nil)
	if down.Code != 503 || down.Header().Get("Retry-After") == "" || strings.Contains(down.Body.String(), "localhost") {
		t.Fatalf("outage: %d %s", down.Code, down.Body.String())
	}
	api.record("Error", "state-unavailable", down)
	capsDown := api.call("GET", "/api/v24/capabilities", "", nil)
	if field(t, capsDown, "available") != false || field(t, capsDown, "mode") != "local_only" || field(t, capsDown, "reason") != "unreachable" {
		t.Fatalf("outage capabilities: %s", capsDown.Body.String())
	}
	api.record("ConfigCapabilities", "mariadb-unreachable", capsDown)
	p.start(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	eventually(t, 15*time.Second, "recovery", func() bool { return store.Healthy() })
	if r := api.call("GET", "/api/v24/config/bookmarks/"+id, "", nil); r.Code != 200 {
		t.Fatalf("state lost across outage: %d", r.Code)
	}

	// Without a configured state store the same handler reports local mode.
	local := liveAPI{t: t, dump: api.dump, handler: httpapi.New(nil, liveToken)}
	lc := local.call("GET", "/api/v24/capabilities", "", nil)
	if field(t, lc, "available") != false || field(t, lc, "reason") != "not_configured" {
		t.Fatalf("local mode: %s", lc.Body.String())
	}
	local.record("ConfigCapabilities", "local-only", lc)
}
