// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestMaintenanceHTTP(t *testing.T) {
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := core.Connection{Driver: "sqlite", Database: "maint_http"}
	if _, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: `CREATE TABLE "sample"(id INTEGER)`}); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("m", 32)
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
	for _, path := range []string{"/api/v10/capabilities", "/api/v10/tables/maintenance/plan", "/api/v10/tables/maintenance"} {
		method := "POST"
		if strings.HasSuffix(path, "capabilities") {
			method = "GET"
		}
		if w := send(method, path, "{}", false); w.Code != 401 {
			t.Fatalf("auth %d", w.Code)
		}
	}
	w := send("GET", "/api/v10/capabilities", "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.10.0"`) {
		t.Fatalf("caps %d %s", w.Code, w.Body.String())
	}
	conn := `"connection":{"driver":"sqlite","database":"maint_http"}`
	item := `{"table":{"schema":"main","name":"sample"},"action":"analyze"}`
	for _, body := range []string{
		`{` + conn + `}`, `{` + conn + `,"items":[]}`, `{` + conn + `,"items":[` + item + `],"sql":"secret"}`,
		`{` + conn + `,"items":[{"table":{"name":"sample"},"action":"raw"}]}`,
		`{` + conn + `,"items":[{"action":"analyze"}]}`,
		`{` + conn + `,"items":[{"table":{"name":"sample"},"action":"analyze","full":null}]}`,
		`{` + conn + `,"items":[` + item + `],"timeout_ms":60001}`,
	} {
		if w := send("POST", "/api/v10/tables/maintenance/plan", body, true); w.Code != 400 {
			t.Fatalf("invalid %d %s", w.Code, w.Body.String())
		}
	}
	w = send("POST", "/api/v10/tables/maintenance/plan", `{`+conn+`,"items":[`+item+`]}`, true)
	var preview core.MaintenancePlanResult
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &preview) != nil || preview.Items[0].Plan == nil {
		t.Fatalf("preview %d %s", w.Code, w.Body.String())
	}
	request := core.MaintenanceExecuteRequest{Connection: c, Items: []core.MaintenanceExecuteItem{{MaintenanceItem: core.MaintenanceItem{Table: core.TableRef{Schema: "main", Name: "sample"}, Action: "analyze"}, ExpectedPlanSHA256: preview.Items[0].Plan.SHA256, Confirm: dbplan.Confirmation{Action: "analyze"}}}}
	for _, hash := range []string{strings.Repeat("0", 64), preview.Items[0].Plan.SHA256} {
		request.Items[0].ExpectedPlanSHA256 = hash
		data, _ := json.Marshal(request)
		w = send("POST", "/api/v10/tables/maintenance", string(data), true)
		var result core.MaintenanceResult
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatalf("execute %d %s", w.Code, w.Body.String())
		}
		want := dbplan.StatusCompleted
		if hash == strings.Repeat("0", 64) {
			want = dbplan.StatusRejected
		}
		if result.Items[0].Outcome != want {
			t.Fatalf("execute %#v", result)
		}
	}
	if w := send("POST", "/api/v10/tables/maintenance", `{`+conn+`,"items":[`+item+`]}`, true); w.Code != 400 {
		t.Fatal("confirmation missing accepted")
	}
}
