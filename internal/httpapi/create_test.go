// SPDX-License-Identifier: AGPL-3.0-only
package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/httpapi"
)

func TestCreateHTTPPresenceAuthAndOldContracts(t *testing.T) {
	e, _ := database.New(t.TempDir(), nil)
	token := strings.Repeat("c", 32)
	api := httpapi.New(e, token)
	send := func(path, body string, auth bool) *httptest.ResponseRecorder {
		method := http.MethodPost
		if strings.HasSuffix(path, "capabilities") {
			method = http.MethodGet
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v6/capabilities", "/api/v6/tables/create"} {
		if w := send(path, "{}", false); w.Code != 401 {
			t.Fatalf("auth %d", w.Code)
		}
	}
	for v := 1; v <= 6; v++ {
		w := send("/api/v"+strconv.Itoa(v)+"/capabilities", "{}", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.`+strconv.Itoa(v)+`.0"`) {
			t.Fatalf("capabilities %d %s", v, w.Body.String())
		}
	}
	valid := `{"connection":{"driver":"sqlite","database":"create_http"},"table":{"name":"created"},"columns":[{"name":"id","type":{"kind":"int64"},"nullable":false,"default":{"mode":"none"}},{"name":"value","type":{"kind":"text","length":20},"nullable":true,"default":{"mode":"value","value":{"kind":"text","value":""}}}],"primary_key":["id"],"confirm":true}`
	for _, bad := range []string{
		strings.Replace(valid, `"confirm":true`, `"confirm":false`, 1), strings.Replace(valid, `"nullable":false,`, "", 1), strings.Replace(valid, `"name":"id",`, "", 1),
		strings.Replace(valid, `"primary_key":["id"],`, "", 1), strings.Replace(valid, `"default":{"mode":"none"}`, `"default":{"mode":"none","value":{"kind":"text","value":""}}`, 1),
		strings.Replace(valid, `"value":""`, `"value":null`, 1), strings.Replace(valid, `"kind":"text","value":""`, `"kind":"text"`, 1),
		strings.Replace(valid, `"kind":"int64"`, `"kind":"int64","scale":0`, 1), strings.Replace(valid, `"nullable":false`, `"nullable":null`, 1),
		strings.Replace(valid, `"kind":"text","length":20`, `"kind":"decimal","precision":5`, 1), strings.Replace(valid, `"kind":"int64"`, `"kind":"BIGINT);DROP TABLE x"`, 1),
	} {
		w := send("/api/v6/tables/create", bad, true)
		if w.Code != 400 {
			t.Fatalf("invalid definition accepted %d %s", w.Code, w.Body.String())
		}
	}
	if w := send("/api/v6/tables/create", `{"padding":"`+strings.Repeat("x", 1<<20)+`"}`, true); w.Code != 413 {
		t.Fatalf("body limit %d", w.Code)
	}
	w := send("/api/v6/tables/create", valid, true)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&result); err != nil || result["structure_check"] != "transactional" || result["snapshot_atomic"] != true {
		t.Fatalf("result %#v %v", result, err)
	}
	if w := send("/api/v6/tables/create", valid, true); w.Code != 409 {
		t.Fatalf("duplicate %d %s", w.Code, w.Body.String())
	}
}
