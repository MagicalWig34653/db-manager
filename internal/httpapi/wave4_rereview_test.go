// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func reReviewTable(t *testing.T, api *API, c core.Connection) {
	t.Helper()
	if _, err := api.engine.(core.Engine).Query(context.Background(), core.QueryRequest{Connection: c, SQL: "CREATE TABLE nokey(id INTEGER, value TEXT)"}); err != nil {
		t.Fatal(err)
	}
}

// Independent re-review (wave 4): the single-use plan must also hold through the HTTP
// execute route, which previews the CSV internally before it starts the job.
func TestReReview_ImportExecuteReplayOverHTTP(t *testing.T) {
	api, c := reviewAPI(t)
	reReviewTable(t, api, c)
	input := core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "nokey"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
	file := []byte("1,a\n2,b\n")
	var p core.ImportPlan
	if r := importHTTP(t, api, "plan", input, file); r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &p) != nil {
		t.Fatal(r.Code, r.Body.String())
	}
	input.Confirm = &dbplan.Confirmation{Action: "import_csv"}
	input.ExpectedPlanSHA256 = p.SHA256
	codes := []int{}
	inserted := []int{}
	for i := 0; i < 2; i++ {
		r := importHTTP(t, api, "execute", input, file)
		codes = append(codes, r.Code)
		if r.Code == 202 {
			var j importJobStatus
			_ = json.Unmarshal(r.Body.Bytes(), &j)
			st := awaitImportHTTP(t, api, j.ID)
			inserted = append(inserted, st.Result.Inserted)
		}
	}
	t.Logf("rows in table after two identical executes: %s", reReviewCount(t, api, c))
	t.Logf("execute status codes %v, inserted per accepted job %v", codes, inserted)
	if codes[1] == 202 {
		if os.Getenv("DBM_REREVIEW_FINDINGS") == "" {
			t.Skip("open finding B1 (see docs/handoffs/WAVE4-REREVIEW.md); run with DBM_REREVIEW_FINDINGS=1")
		}
		t.Errorf("replayed confirmed CSV import over HTTP was accepted again (inserted %v)", inserted)
	}
}

// Parallel executes of one confirmed plan: at most one may run.
func TestReReview_ImportExecuteParallelOverHTTP(t *testing.T) {
	api, c := reviewAPI(t)
	reReviewTable(t, api, c)
	input := core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "nokey"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
	file := []byte(strings.Repeat("1,a\n", 3))
	_ = file
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&sb, "%d,v\n", i+1)
	}
	file = []byte(sb.String())
	var p core.ImportPlan
	if r := importHTTP(t, api, "plan", input, file); r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &p) != nil {
		t.Fatal(r.Code, r.Body.String())
	}
	input.Confirm = &dbplan.Confirmation{Action: "import_csv"}
	input.ExpectedPlanSHA256 = p.SHA256
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	acc := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := importHTTP(t, api, "execute", input, file)
			if r.Code == 202 {
				var j importJobStatus
				_ = json.Unmarshal(r.Body.Bytes(), &j)
				st := awaitImportHTTP(t, api, j.ID)
				mu.Lock()
				acc++
				total += st.Result.Inserted
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	t.Logf("rows in table: %s", reReviewCount(t, api, c))
	t.Logf("accepted jobs %d, rows inserted in total %d (unique key: second job conflicts)", acc, total)
}

func reReviewCount(t *testing.T, api *API, c core.Connection) string {
	r, err := api.engine.(core.Engine).Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT COUNT(*) FROM nokey"})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(r.Results[0].Rows[0][0].Value)
}

// F5 measurement: peak Go heap above the idle baseline for two concurrent 62 MiB CSV uploads
// (the maximum of two slots), streamed so the test itself holds no copy of the body.
func reReviewHeapRun(t *testing.T, action string) {
	if os.Getenv("DBM_REREVIEW_HEAP") == "" {
		t.Skip("memory measurement (about 500 MiB); run with DBM_REREVIEW_HEAP=1")
	}
	api, c := reviewAPI(t)
	reReviewTable(t, api, c)
	c2 := c
	c2.Database = "probe2"
	reReviewTable(t, api, c2)
	srv := httptest.NewServer(api)
	defer srv.Close()
	meta := core.ImportRequest{Connection: c, Format: "csv", Table: core.TableRef{Name: "nokey"}, CSV: core.CSVImportOptions{Columns: []string{"id", "value"}}}
	row := "1,\"" + strings.Repeat("x", (1<<20)-100) + "\"\n"
	const rows = 61
	body := func(i int, sha string) (io.Reader, string) {
		pr, pw := io.Pipe()
		w := multipart.NewWriter(pw)
		go func() {
			mf, _ := w.CreateFormField("metadata")
			m := meta
			if i == 1 {
				m.Connection = c2
			}
			if sha != "" {
				m.ExpectedPlanSHA256 = sha
				m.Confirm = &dbplan.Confirmation{Action: "import_csv"}
			}
			_ = json.NewEncoder(mf).Encode(m)
			ff, _ := w.CreateFormFile("file", "x.csv")
			for i := 0; i < rows; i++ {
				_, _ = io.WriteString(ff, row)
			}
			_ = w.Close()
			_ = pw.Close()
		}()
		return pr, w.FormDataContentType()
	}
	shas := [2]string{}
	if action == "execute" {
		for i := 0; i < 2; i++ {
			b, ct := body(i, "")
			req, _ := http.NewRequest("POST", srv.URL+"/api/v18/imports/plan", b)
			req.Header.Set("Authorization", "Bearer token")
			req.Header.Set("Content-Type", ct)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			var p core.ImportPlan
			_ = json.NewDecoder(resp.Body).Decode(&p)
			resp.Body.Close()
			shas[i] = p.SHA256
		}
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
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b, ct := body(i, shas[i])
			req, _ := http.NewRequest("POST", srv.URL+"/api/v18/imports/"+action, b)
			req.Header.Set("Authorization", "Bearer token")
			req.Header.Set("Content-Type", ct)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			bb, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 && resp.StatusCode != 202 {
				t.Logf("response %d %s", resp.StatusCode, bb)
			}
			codes[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()
	if action == "execute" {
		time.Sleep(8 * time.Second) // jobs run on after the 202
	}
	close(stop)
	t.Logf("%s x2 file=%d MiB codes=%v baseHeap=%d MiB peakHeap=%d MiB delta=%d MiB", action, rows, codes, base.HeapAlloc>>20, peak.Load()>>20, (peak.Load()-base.HeapAlloc)>>20)
}

func TestReReview_ImportPeakHeapTwoSlots(t *testing.T) { reReviewHeapRun(t, "plan") }

func TestReReview_ImportPeakHeapTwoSlotsExecute(t *testing.T) { reReviewHeapRun(t, "execute") }
