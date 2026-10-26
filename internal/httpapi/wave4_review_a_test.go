// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

// Independent adversarial HTTP tests for W-P12 (API 0.18) and W-P13 (API 0.19).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/database"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func reviewAPI(t *testing.T) (*API, core.Connection) {
	t.Helper()
	engine, err := database.New(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := core.Connection{Driver: "sqlite", Database: "probe", TLSMode: "verify-full"}
	if _, err = engine.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE rv(id INTEGER PRIMARY KEY, value TEXT)"}); err != nil {
		t.Fatal(err)
	}
	return New(engine, "token"), c
}

func rawMultipart(t *testing.T, api *API, path string, build func(w *multipart.Writer)) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	build(w)
	_ = w.Close()
	req := httptest.NewRequest("POST", path, &body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func TestReviewA_ImportMultipartAbuse(t *testing.T) {
	api, c := reviewAPI(t)
	meta, _ := json.Marshal(core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "rv"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}})
	field := func(w *multipart.Writer, name string, data []byte) {
		p, _ := w.CreateFormField(name)
		_, _ = p.Write(data)
	}
	cases := map[string]func(w *multipart.Writer){
		"missing-file":     func(w *multipart.Writer) { field(w, "metadata", meta) },
		"missing-metadata": func(w *multipart.Writer) { field(w, "file", []byte("1,a")) },
		"dup-metadata": func(w *multipart.Writer) {
			field(w, "metadata", meta)
			field(w, "metadata", meta)
			field(w, "file", []byte("1,a"))
		},
		"dup-file": func(w *multipart.Writer) {
			field(w, "metadata", meta)
			field(w, "file", []byte("1,a"))
			field(w, "file", []byte("2,b"))
		},
		"extra-part": func(w *multipart.Writer) {
			field(w, "metadata", meta)
			field(w, "file", []byte("1,a"))
			field(w, "extra", []byte("x"))
		},
		"empty-file": func(w *multipart.Writer) { field(w, "metadata", meta); field(w, "file", nil) },
		"metadata-1MiB+": func(w *multipart.Writer) {
			field(w, "metadata", append(append([]byte(`{"x":"`), bytes.Repeat([]byte("a"), 1<<20)...), []byte(`"}`)...))
			field(w, "file", []byte("1,a"))
		},
		"metadata-garbage": func(w *multipart.Writer) { field(w, "metadata", []byte("{")); field(w, "file", []byte("1,a")) },
		"metadata-unknown-field": func(w *multipart.Writer) {
			field(w, "metadata", []byte(`{"connection":{"driver":"sqlite","database":"probe"},"format":"csv","evil":1}`))
			field(w, "file", []byte("1,a"))
		},
	}
	for name, build := range cases {
		rec := rawMultipart(t, api, "/api/v18/imports/plan", build)
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s: status %d %s", name, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "goroutine") || strings.Contains(rec.Body.String(), "/Users/") {
			t.Errorf("%s leaks: %s", name, rec.Body.String())
		}
	}
	// Non-multipart and wrong content types.
	for _, ct := range []string{"application/json", "text/plain", "multipart/mixed; boundary=x", "multipart/form-data"} {
		req := httptest.NewRequest("POST", "/api/v18/imports/plan", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Errorf("content-type %q -> %d", ct, rec.Code)
		}
	}
}

// An unbounded chunked body is cut at the documented limit and does not exhaust memory.
func TestReviewA_ImportUnboundedBody(t *testing.T) {
	api, _ := reviewAPI(t)
	srv := httptest.NewServer(api)
	defer srv.Close()
	var sent atomic.Int64
	pr, pw := io.Pipe()
	boundary := "BOUND"
	go func() {
		defer pw.Close()
		_, _ = io.WriteString(pw, "--"+boundary+"\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\n")
		chunk := bytes.Repeat([]byte("a"), 1<<20)
		for i := 0; i < 200; i++ { // would be 200 MiB
			n, err := pw.Write(chunk)
			sent.Add(int64(n))
			if err != nil {
				return
			}
		}
	}()
	req, _ := http.NewRequest("POST", srv.URL+"/api/v18/imports/plan", pr)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("client error (connection cut): %v", err)
	} else {
		defer resp.Body.Close()
		t.Logf("status %d after %d MiB", resp.StatusCode, sent.Load()>>20)
		if resp.StatusCode != 413 && resp.StatusCode != 400 {
			t.Errorf("status %d", resp.StatusCode)
		}
	}
	if sent.Load() > 70<<20 {
		t.Errorf("server consumed %d MiB of an unbounded body", sent.Load()>>20)
	}
}

// Two slots only; the third concurrent upload is refused, and failed uploads free their slot.
func TestReviewA_ImportSlotsAndRelease(t *testing.T) {
	api, c := reviewAPI(t)
	srv := httptest.NewServer(api)
	defer srv.Close()
	meta, _ := json.Marshal(core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "rv"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}})
	type held struct{ pw *io.PipeWriter }
	hold := func() *io.PipeWriter {
		pr, pw := io.Pipe()
		go func() {
			req, _ := http.NewRequest("POST", srv.URL+"/api/v18/imports/plan", pr)
			req.Header.Set("Authorization", "Bearer token")
			req.Header.Set("Content-Type", "multipart/form-data; boundary=B")
			if resp, err := http.DefaultClient.Do(req); err == nil {
				resp.Body.Close()
			}
		}()
		_, _ = io.WriteString(pw, "--B\r\nContent-Disposition: form-data; name=\"metadata\"\r\n\r\n"+string(meta)+"\r\n")
		return pw
	}
	a, b := hold(), hold()
	time.Sleep(300 * time.Millisecond)
	rec := rawMultipart(t, api, "/api/v18/imports/plan", func(w *multipart.Writer) {
		p, _ := w.CreateFormField("metadata")
		_, _ = p.Write(meta)
		p, _ = w.CreateFormField("file")
		_, _ = p.Write([]byte("1,a"))
	})
	t.Logf("third concurrent: %d %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	if rec.Code != 503 {
		t.Errorf("third upload not refused: %d", rec.Code)
	}
	a.Close()
	b.Close()
	time.Sleep(300 * time.Millisecond)
	// Many failing requests must not leak slots.
	for i := 0; i < 8; i++ {
		rawMultipart(t, api, "/api/v18/imports/plan", func(w *multipart.Writer) {})
		rawMultipart(t, api, "/api/v18/imports/execute", func(w *multipart.Writer) {
			p, _ := w.CreateFormField("metadata")
			_, _ = p.Write([]byte("{"))
		})
	}
	rec = rawMultipart(t, api, "/api/v18/imports/plan", func(w *multipart.Writer) {
		p, _ := w.CreateFormField("metadata")
		_, _ = p.Write(meta)
		p, _ = w.CreateFormField("file")
		_, _ = p.Write([]byte("1,a"))
	})
	if rec.Code != 200 {
		t.Errorf("slot leaked after failures: %d %s", rec.Code, rec.Body.String())
	}
}

// Peak heap of one maximum-size plan request (documents memory amplification).
func TestReviewA_ImportPeakMemory(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	api, c := reviewAPI(t)
	meta := core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "rv"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
	var file bytes.Buffer
	cell := strings.Repeat("x", (1<<20)-100)
	for file.Len() < 64<<20-(2<<20) {
		fmt.Fprintf(&file, "1,\"%s\"\n", cell)
	}
	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)
	var peak atomic.Uint64
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > peak.Load() {
				peak.Store(m.HeapAlloc)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	rec := importHTTP(t, api, "plan", meta, file.Bytes())
	close(stop)
	t.Logf("file=%d MiB status=%d baseHeap=%d MiB peakHeap=%d MiB", file.Len()>>20, rec.Code, base.HeapAlloc>>20, peak.Load()>>20)
}

// Cancel/status semantics for jobs.
func TestReviewA_ImportJobStatusAndCancel(t *testing.T) {
	api, c := reviewAPI(t)
	for _, p := range []string{"/api/v18/imports/", "/api/v18/imports/../../etc", "/api/v18/imports/%00", "/api/v18/imports/" + strings.Repeat("a", 5000)} {
		for _, m := range []string{"GET", "POST"} {
			path := p
			if m == "POST" {
				path += "/cancel"
			}
			rec := importStatusHTTP(api, m, path, true)
			if rec.Code == 200 {
				t.Errorf("%s %s -> 200", m, path)
			}
		}
	}
	// SQL import with a confirm for a different action and with a plan hash.
	meta := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_csv"}}
	if rec := importHTTP(t, api, "execute", meta, []byte("INSERT INTO rv VALUES(1,'a')")); rec.Code != 400 {
		t.Errorf("sql with csv confirmation: %d", rec.Code)
	}
	meta.Confirm.Action = "import_sql"
	meta.ExpectedPlanSHA256 = strings.Repeat("0", 64)
	rec := importHTTP(t, api, "execute", meta, []byte("INSERT INTO rv VALUES(1,'a')"))
	t.Logf("sql with expected hash: %d %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	// SQL plan is not offered.
	meta.ExpectedPlanSHA256 = ""
	if rec := importHTTP(t, api, "plan", meta, []byte("SELECT 1")); rec.Code == 200 {
		t.Errorf("sql plan returned 200")
	}
	// Long-running SQL import is cancellable and reports cancel + no completed claim.
	meta.TimeoutMS = 300000
	slow := "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<2000000000) SELECT COUNT(*) FROM c"
	rec = importHTTP(t, api, "execute", meta, []byte(slow+";INSERT INTO rv VALUES(9,'after')"))
	var acc importJobStatus
	if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &acc) != nil {
		t.Fatal(rec.Code, rec.Body.String())
	}
	time.Sleep(200 * time.Millisecond)
	cancelRec := importStatusHTTP(api, "POST", "/api/v18/imports/"+acc.ID+"/cancel", true)
	if cancelRec.Code != 200 {
		t.Fatal(cancelRec.Code)
	}
	fin := awaitImportHTTP(t, api, acc.ID)
	b, _ := json.Marshal(fin)
	t.Logf("after cancel: %s", b)
	if fin.Result == nil || fin.Result.Outcome == "completed" {
		t.Errorf("cancelled job claims completion: %s", b)
	}
	if !fin.CancelRequested {
		t.Error("cancel flag missing")
	}
}

func TestReviewA_RowsBulkAuthAndUploadAbuse(t *testing.T) {
	api, c := reviewAPI(t)
	for _, route := range []string{"GET /api/v19/capabilities", "POST /api/v19/rows/plan", "POST /api/v19/rows/execute", "POST /api/v19/values/upload", "POST /api/v19/values/download", "POST /api/v19/foreign-keys/lookup"} {
		parts := strings.SplitN(route, " ", 2)
		if r := importStatusHTTP(api, parts[0], parts[1], false); r.Code != 401 {
			t.Errorf("%s unauthenticated -> %d", route, r.Code)
		}
	}
	// JSON routes reject other content types.
	for _, path := range []string{"/api/v19/rows/plan", "/api/v19/foreign-keys/lookup", "/api/v19/values/download"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s text/plain -> %d", path, rec.Code)
		}
	}
	engine := api.engine.(*database.Engine)
	_, _ = engine.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE rvf(id INTEGER PRIMARY KEY, note TEXT, data BLOB)"})
	_, _ = engine.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "INSERT INTO rvf(id,note) VALUES (1,'a')"})
	page, err := engine.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "rvf"}, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	cols := []string{}
	for _, col := range page.Description.Columns {
		cols = append(cols, col.Name)
	}
	meta := core.FileValueRequest{Connection: c, Table: core.TableRef{Name: "rvf"}, Expected: core.RowSnapshot{Columns: cols, Values: page.Result.Rows[0]}, Column: "data", Confirm: true}
	mb, _ := json.Marshal(meta)
	field := func(w *multipart.Writer, name string, data []byte) {
		p, _ := w.CreateFormField(name)
		_, _ = p.Write(data)
	}
	post := func(build func(w *multipart.Writer)) *httptest.ResponseRecorder {
		return rawMultipart(t, api, "/api/v19/values/upload", build)
	}
	cases := map[string]func(w *multipart.Writer){
		"file-first":   func(w *multipart.Writer) { field(w, "file", []byte("x")); field(w, "metadata", mb) },
		"missing-file": func(w *multipart.Writer) { field(w, "metadata", mb) },
		"extra-part": func(w *multipart.Writer) {
			field(w, "metadata", mb)
			field(w, "file", []byte("x"))
			field(w, "more", []byte("y"))
		},
		"dup-file": func(w *multipart.Writer) {
			field(w, "metadata", mb)
			field(w, "file", []byte("x"))
			field(w, "file", []byte("y"))
		},
		"oversize-513K": func(w *multipart.Writer) {
			field(w, "metadata", mb)
			field(w, "file", bytes.Repeat([]byte{1}, 512<<10+1))
		},
		"oversize-2MiB": func(w *multipart.Writer) { field(w, "metadata", mb); field(w, "file", bytes.Repeat([]byte{1}, 2<<20)) },
		"no-confirm": func(w *multipart.Writer) {
			m := meta
			m.Confirm = false
			b, _ := json.Marshal(m)
			field(w, "metadata", b)
			field(w, "file", []byte("x"))
		},
		"missing-fields": func(w *multipart.Writer) {
			field(w, "metadata", []byte(`{"connection":{"driver":"sqlite","database":"probe"}}`))
			field(w, "file", []byte("x"))
		},
	}
	for name, build := range cases {
		if rec := post(build); rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s -> %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if n := scalarHTTP(t, engine, c, "SELECT COUNT(*) FROM rvf WHERE data IS NOT NULL"); n != "0" {
		t.Errorf("rejected uploads wrote %s rows", n)
	}
	ok := post(func(w *multipart.Writer) { field(w, "metadata", mb); field(w, "file", []byte("hello")) })
	if ok.Code != 200 {
		t.Fatalf("valid upload: %d %s", ok.Code, ok.Body.String())
	}
	// Download headers are inert; non-binary column cannot be downloaded.
	page, _ = engine.Browse(context.Background(), core.BrowseRequest{Connection: c, Table: core.TableRef{Name: "rvf"}, PageSize: 10})
	meta.Expected.Values = page.Result.Rows[0]
	body, _ := json.Marshal(meta)
	req := httptest.NewRequest("POST", "/api/v19/values/download", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	h := rec.Header()
	if rec.Code != 200 || rec.Body.String() != "hello" || h.Get("Content-Type") != "application/octet-stream" || h.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") || h.Get("Cache-Control") != "no-store" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("download: %d %v %q", rec.Code, h, rec.Body.String())
	}
	meta.Column = "note"
	body, _ = json.Marshal(meta)
	req = httptest.NewRequest("POST", "/api/v19/values/download", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if rec.Code == 200 {
		t.Error("text column downloaded")
	}
}

func scalarHTTP(t *testing.T, e *database.Engine, c core.Connection, q string) string {
	t.Helper()
	r, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: q})
	if err != nil {
		t.Fatal(err)
	}
	return r.Results[0].Rows[0][0].Value
}

// At most 16 job results are kept; the 17th request is refused until they expire.
func TestReviewA_ImportJobStoreBounded(t *testing.T) {
	api, c := reviewAPI(t)
	meta := core.ImportRequest{Connection: c, Format: "sql", Confirm: &dbplan.Confirmation{Action: "import_sql"}}
	refused := 0
	for i := 0; i < 20; i++ {
		rec := importHTTP(t, api, "execute", meta, []byte("SELECT 1"))
		if rec.Code == 503 {
			refused++
			continue
		}
		var acc importJobStatus
		if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &acc) != nil {
			t.Fatalf("%d: %d %s", i, rec.Code, rec.Body.String())
		}
		awaitImportHTTP(t, api, acc.ID)
	}
	if refused != 4 {
		t.Errorf("expected 4 refusals after 16 stored results, got %d", refused)
	}
}
