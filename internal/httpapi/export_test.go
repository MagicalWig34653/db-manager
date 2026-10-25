// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
)

func exportAPI(t *testing.T) (*API, *database.Engine) {
	t.Helper()
	e, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return New(e, "token"), e
}
func exportHTTP(t *testing.T, a *API, input any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v11/export", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer token")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestExportHTTPDownload(t *testing.T) {
	a, e := exportAPI(t)
	c := core.Connection{Driver: "sqlite", Database: "download"}
	_, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE data(value TEXT)"})
	if err != nil {
		t.Fatal(err)
	}
	var plain []byte
	for _, compress := range []bool{false, true} {
		w := exportHTTP(t, a, core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, Gzip: compress})
		response := w.Result()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || response.Trailer.Get("X-Export-Status") != "complete" || !strings.Contains(response.Header.Get("Content-Disposition"), "export.sql") {
			t.Fatalf("%d %#v %#v %s", w.Code, response.Header, response.Trailer, body)
		}
		if compress {
			z, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			body, err = io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			z.Close()
		}
		if !compress {
			plain = append([]byte(nil), body...)
		} else if !bytes.Equal(plain, body) {
			t.Fatal("GZip changed export bytes")
		}
		if !strings.HasSuffix(string(body), "-- DBMANAGER EXPORT COMPLETE\n") {
			t.Fatal(string(body))
		}
	}
}
func TestExportHTTPValidationAndLimits(t *testing.T) {
	a, e := exportAPI(t)
	c := core.Connection{Driver: "sqlite", Database: "download"}
	_, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE data(value TEXT)"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []any{
		map[string]any{"connection": c, "tables": []any{map[string]any{"name": "data"}}, "unknown": true},
		core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, Filename: "../bad\r\nX-Leak: true"},
		core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, MaxBytes: -1},
	} {
		if w := exportHTTP(t, a, input); w.Code != 400 {
			t.Fatalf("invalid input accepted: %d %s", w.Code, w.Body.String())
		}
	}
	w := exportHTTP(t, a, core.ExportRequest{Connection: c, Tables: []core.TableRef{{Name: "data"}}, MaxBytes: 150})
	if w.Result().Trailer.Get("X-Export-Status") != "incomplete" || strings.Contains(w.Body.String(), "EXPORT COMPLETE") {
		t.Fatalf("limit not marked %#v %s", w.Result().Trailer, w.Body.String())
	}
	for _, method := range []string{"GET", "POST"} {
		path := "/api/v11/export"
		if method == "GET" {
			path = "/api/v11/capabilities"
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatal(w.Code)
		}
	}
}
func TestExportCapabilities(t *testing.T) {
	a, _ := exportAPI(t)
	r := httptest.NewRequest("GET", "/api/v11/capabilities", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if dir := os.Getenv("DBM_EXPORT_DUMP"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "capabilities.json"), w.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"contract_version":"0.11.0"`) {
		t.Fatal(w.Body.String())
	}
}
