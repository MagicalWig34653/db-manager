// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestExtendedExportHTTP(t *testing.T) {
	a, e := exportAPI(t)
	send := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		a.ServeHTTP(out, req)
		return out
	}
	for _, path := range []string{"/api/v21/capabilities", "/api/v21/export"} {
		method := "GET"
		if strings.HasSuffix(path, "export") {
			method = "POST"
		}
		if got := send(method, path, "{}", "").Code; got != 401 {
			t.Fatal(path, got)
		}
	}
	cap := send("GET", "/api/v21/capabilities", "", "token")
	var result struct {
		Version string   `json:"contract_version"`
		Formats []string `json:"formats"`
	}
	if cap.Code != 200 || json.Unmarshal(cap.Body.Bytes(), &result) != nil || result.Version != "0.21.0" || len(result.Formats) != 12 {
		t.Fatal(cap.Code, cap.Body.String())
	}
	if dir := os.Getenv("DBM_EXTENDED_EXPORT_DUMP"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "ExtendedExportCapabilities.actual.json"), cap.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := core.Connection{Driver: "sqlite", Database: "extended"}
	for _, sql := range []string{"CREATE TABLE data(value TEXT)", "INSERT INTO data VALUES('<>&'),('two')"} {
		if _, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: sql}); err != nil {
			t.Fatal(err)
		}
	}
	for _, format := range result.Formats[4:] {
		body, _ := json.Marshal(core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, Format: format})
		plain := send("POST", "/api/v21/export", string(body), "token")
		response := plain.Result()
		if plain.Code != 200 || response.Trailer.Get("X-Export-Status") != "complete" || plain.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(format, plain.Code, plain.Body.String())
		}
		body, _ = json.Marshal(core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, Format: format, Gzip: true})
		compressed := send("POST", "/api/v21/export", string(body), "token")
		z, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(z)
		z.Close()
		if err != nil || !bytes.Equal(data, plain.Body.Bytes()) {
			t.Fatal("gzip parity", format, err)
		}
		body, _ = json.Marshal(core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, Format: format, MaxRows: 1})
		limited := send("POST", "/api/v21/export", string(body), "token")
		if limited.Code != 200 || limited.Result().Trailer.Get("X-Export-Status") != "incomplete" || limited.Result().Trailer.Get("X-Export-Error") != "export_row_limit" {
			t.Fatal("limit trailer", format, limited.Code)
		}
	}
	for _, extra := range []string{`"format":"xml"`, `"comments":false`, `"CoMmEnTs":false`, `"aliases":[]`, `"views_as_tables":false`, `"definer":"omit"`} {
		body := `{"connection":{"driver":"sqlite","database":"extended"},"tables":[{"name":"data"}],` + extra + `}`
		if got := send("POST", "/api/v11/export", body, "token").Code; got != 400 {
			t.Fatal("legacy contract widened", extra, got)
		}
	}
	for _, extra := range []string{`"format":"pdf"`, `"filename":"../private"`, `"unknown":false`, `"data_statement":"replace","connection_extra":true`} {
		body := `{"connection":{"driver":"sqlite","database":"extended"},"tables":[{"name":"data"}],` + extra + `}`
		if got := send("POST", "/api/v21/export", body, "token").Code; got != 400 {
			t.Fatal("invalid options accepted", extra, got)
		}
	}
	if got := send("GET", "/api/v21/export", "", "token").Code; got != 405 {
		t.Fatal(got)
	}
}
