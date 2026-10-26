// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// Independent re-review (wave 4): sweep code points through the YAML exporter,
// as cell values, column names and table names. Dumps for scripts/review-wave4-rereview-yaml.py
// when DBM_REREVIEW_DUMP is set.
func rereviewProbes() []rune {
	var out []rune
	for r := rune(1); r < 0x3100; r++ {
		out = append(out, r)
	}
	for r := rune(0xd7f0); r <= 0xd7ff; r++ {
		out = append(out, r)
	}
	for r := rune(0xe000); r <= 0xe010; r++ {
		out = append(out, r)
	}
	for r := rune(0xfdd0); r <= 0xfdef; r++ {
		out = append(out, r)
	}
	for r := rune(0xfff0); r <= 0xffff; r++ {
		out = append(out, r)
	}
	out = append(out, 0x10000, 0x1f600, 0x1fffe, 0x1ffff, 0x2fffe, 0xeffff, 0x10fffe, 0x10ffff)
	return out
}

func TestReReviewYAMLSweep(t *testing.T) {
	e := testEngine(t)
	c := sqliteConnection()
	probes := rereviewProbes()
	dir := os.Getenv("DBM_REREVIEW_DUMP")
	// one table; columns named by probe in chunks of 500
	type exp struct {
		Table string   `json:"table"`
		Cols  []string `json:"cols"`
		Vals  []string `json:"vals"`
	}
	var expected []exp
	const chunk = 500
	for i := 0; i < len(probes); i += chunk {
		end := min(i+chunk, len(probes))
		table := fmt.Sprintf("t%d_%c_%c", i, probes[i], probes[end-1])
		q := quoteIdentifier(c.Driver, table)
		var cols, vals string
		x := exp{Table: table}
		for j, r := range probes[i:end] {
			name := fmt.Sprintf("c%d_%cz", j, r)
			val := "a" + string(r) + "b"
			if j > 0 {
				cols += ","
				vals += ","
			}
			cols += quoteIdentifier(c.Driver, name) + " TEXT"
			vals += reviewHexText(val)
			x.Cols = append(x.Cols, name)
			x.Vals = append(x.Vals, val)
		}
		execute(t, e, c, "CREATE TABLE "+q+"("+cols+")")
		execute(t, e, c, "INSERT INTO "+q+" VALUES("+vals+")")
		expected = append(expected, x)
	}
	// table names carrying the risky characters
	for _, r := range []rune{0x7f, 0x80, 0x85, 0x9f, 0xa0, 0xfffe, 0xffff, 0xfeff, 0x2028, 0x1fffe} {
		table := "n" + string(r) + "n"
		execute(t, e, c, "CREATE TABLE "+quoteIdentifier(c.Driver, table)+"(v TEXT)")
		execute(t, e, c, "INSERT INTO "+quoteIdentifier(c.Driver, table)+" VALUES('x')")
		expected = append(expected, exp{Table: table, Cols: []string{"v"}, Vals: []string{"x"}})
	}
	var refs []core.TableRef
	for _, x := range expected {
		refs = append(refs, core.TableRef{Schema: "main", Name: x.Table})
	}
	for _, format := range []string{"yaml", "json"} {
		var out bytes.Buffer
		if err := e.Export(context.Background(), core.ExportRequest{Connection: c, Tables: refs, Format: format}, &out); err != nil {
			t.Fatal(err)
		}
		if format == "yaml" {
			for _, r := range out.String() {
				if r == 0x7f || r >= 0x80 && r <= 0x9f || r == 0xfffe || r == 0xffff {
					t.Fatalf("yaml export holds raw U+%04X", r)
				}
			}
		}
		if dir != "" {
			if err := os.WriteFile(filepath.Join(dir, "probe."+format), out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if dir != "" {
		b, _ := json.Marshal(expected)
		_ = os.WriteFile(filepath.Join(dir, "probe.expected.json"), b, 0600)
	}
}

// F4: a 512 KiB value replaces a 512 KiB value; 512 KiB + 1 byte is refused as file_too_large.
func TestReReview_FileReplaceAtLimit(t *testing.T) {
	eachReviewSystem(t, func(t *testing.T, s reviewSystem) {
		dropIf(t, s, "rrv_fr")
		execute(t, s.e, s.c, fileTableDDL(s, "rrv_fr"))
		defer dropIf(t, s, "rrv_fr")
		execute(t, s.e, s.c, "INSERT INTO rrv_fr (id,note) VALUES (1,'a')")
		ref := core.TableRef{Name: "rrv_fr"}
		up := func(n int) error {
			snap := bulkSnapshots(t, s.e, s.c, ref)[0].Expected
			_, err := s.e.FileValueUpload(context.Background(), core.FileValueRequest{Connection: s.c, Table: ref, Expected: snap, Column: "data", Confirm: true}, bytes.NewReader(bytes.Repeat([]byte{byte(n)}, core.MaxFileValueBytes)))
			return err
		}
		if err := up(1); err != nil {
			t.Fatal(err)
		}
		if err := up(2); err != nil {
			t.Errorf("replacing a %d byte file by another one: %v", core.MaxFileValueBytes, err)
		}
		snap := bulkSnapshots(t, s.e, s.c, ref)[0].Expected
		_, err := s.e.FileValueUpload(context.Background(), core.FileValueRequest{Connection: s.c, Table: ref, Expected: snap, Column: "data", Confirm: true}, bytes.NewReader(bytes.Repeat([]byte{3}, core.MaxFileValueBytes+1)))
		var pe *core.Error
		if !errors.As(err, &pe) || pe.Code != "file_too_large" {
			t.Errorf("oversize value: want file_too_large, got %v", err)
		}
	})
}

func TestReReview_PlanLedgerRaceSingleWinner(t *testing.T) {
	var l planLedger
	for round := 0; round < 200; round++ {
		key := fmt.Sprintf("k%d", round)
		var wins atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if l.consume(key) {
					wins.Add(1)
				}
			}()
		}
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("round %d: %d winners for one digest", round, wins.Load())
		}
	}
}

func TestReReview_PlanLedgerBoundedAndEvictsOldest(t *testing.T) {
	var l planLedger
	for i := 0; i < planLedgerMax+500; i++ {
		if !l.consume(fmt.Sprintf("k%d", i)) {
			t.Fatalf("fresh digest %d refused", i)
		}
		l.mu.Lock()
		n := len(l.consumed)
		l.mu.Unlock()
		if n > planLedgerMax {
			t.Fatalf("ledger grew to %d entries", n)
		}
	}
	// the newest entries stay protected, the oldest were evicted (documented limit: replay then possible)
	if l.consume(fmt.Sprintf("k%d", planLedgerMax+499)) {
		t.Error("newest digest can be replayed")
	}
	if !l.consume("k0") {
		t.Error("oldest digest should have been evicted")
	}
}

func TestReReview_PlanLedgerExpiryAndRearm(t *testing.T) {
	var l planLedger
	l.consume("a")
	if l.consume("a") {
		t.Fatal("replay accepted")
	}
	l.arm("a")
	if !l.consume("a") {
		t.Fatal("arm did not release")
	}
	l.mu.Lock()
	l.consumed["a"] = time.Now().Add(-planLedgerTTL - time.Second)
	l.mu.Unlock()
	if !l.consume("a") {
		t.Fatal("expired entry still blocks")
	}
	// expired entries are purged when the ledger is full
	l.mu.Lock()
	for i := 0; i < planLedgerMax; i++ {
		l.consumed[fmt.Sprintf("old%d", i)] = time.Now().Add(-2 * planLedgerTTL)
	}
	l.mu.Unlock()
	l.consume("fresh")
	l.mu.Lock()
	n := len(l.consumed)
	l.mu.Unlock()
	if n > 3 {
		t.Errorf("expired entries not purged at capacity: %d left", n)
	}
}

func TestReReview_MonitorUnavailableClassification(t *testing.T) {
	for number, want := range map[uint16]bool{1044: false, 1045: false, 1049: false, 2002: false, 1142: true, 1143: true, 1227: true, 1064: true, 1146: true} {
		if got := monitorUnavailable(&mysql.MySQLError{Number: number}); got != want {
			t.Errorf("mysql %d: unavailable=%v want %v", number, got, want)
		}
	}
	for code, want := range map[string]bool{"28P01": false, "28000": false, "3D000": false, "57014": false, "42501": true, "42P01": true} {
		if got := monitorUnavailable(&pgconn.PgError{Code: code}); got != want {
			t.Errorf("pg %s: unavailable=%v want %v", code, got, want)
		}
	}
}

// Live (MySQL/MariaDB/PostgreSQL when the fixture variables are set): credentials and user
// statement errors are real errors on every engine, and no raw error text leaks.
func TestReReview_MonitorRealErrorsLive(t *testing.T) {
	eachReviewSystem(t, func(t *testing.T, s reviewSystem) {
		if s.c.Driver == "sqlite" {
			t.Skip("credentials and profiling grammar are server features")
		}
		bad := s.c
		bad.Password = "WRONG_pw_zz"
		for _, action := range []string{"sample", "advisor", "analysis", "engines"} {
			r, err := s.e.Monitor(context.Background(), action, core.MonitorRequest{Connection: bad})
			if err == nil {
				t.Errorf("%s with a wrong password: no error, state=%q", action, r.State)
				continue
			}
			for _, leak := range []string{"WRONG_pw_zz", "Access denied", "password authentication", "SQLSTATE"} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("%s error leaks %q: %v", action, leak, err)
				}
			}
		}
		for _, q := range []string{"SELECT * FROM no_such_table_zz", "SELECT nosuchcol FROM information_schema.tables"} {
			r, err := s.e.Monitor(context.Background(), "profiling", core.MonitorRequest{Connection: s.c, SQL: q})
			if err == nil {
				t.Errorf("profiling %q: want an error, got state=%q", q, r.State)
			}
		}
	})
}

// F6: a CSV import cancelled before COMMIT reports the same on every engine.
func TestReReview_CSVCancelUniform(t *testing.T) {
	eachReviewSystem(t, func(t *testing.T, s reviewSystem) {
		dropIf(t, s, "rrv_c")
		execute(t, s.e, s.c, "CREATE TABLE rrv_c(id INTEGER, v VARCHAR(10))")
		defer dropIf(t, s, "rrv_c")
		var b strings.Builder
		for i := 0; i < 20000; i++ {
			fmt.Fprintf(&b, "%d,v\n", i)
		}
		r := core.ImportRequest{Connection: s.c, Format: "csv", Table: core.TableRef{Name: "rrv_c"}, CSV: core.CSVImportOptions{Columns: []string{"id", "v"}}}
		file := []byte(b.String())
		p, err := s.e.PlanImport(context.Background(), r, file)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		got, err := s.e.ExecuteImport(ctx, importConfirm(r, p), file, func(pr core.ImportProgress) {
			if pr.Processed >= 500 {
				cancel()
			}
		})
		rows := scalar(t, s, "SELECT COUNT(*) FROM rrv_c")
		t.Logf("%s: err=%v outcome=%s reason=%s inserted=%d rows=%s", s.name, err, got.Outcome, got.Reason, got.Inserted, rows)
		if got.Outcome != "rejected" || got.Inserted != 0 || got.Reason != "query_canceled" || rows != "0" {
			t.Errorf("not uniform: outcome=%s reason=%s inserted=%d rows=%s", got.Outcome, got.Reason, got.Inserted, rows)
		}
	})
}

// A failed execute (nothing written) still consumes the plan; documents what the user sees.
func TestReReview_FailedBulkExecuteConsumesPlan(t *testing.T) {
	eachReviewSystem(t, func(t *testing.T, s reviewSystem) {
		dropIf(t, s, "rrv_uq")
		ddl := strings.Replace(autoPKDDL(s, "rrv_uq"), "label VARCHAR(50)", "label VARCHAR(50) UNIQUE", 1)
		execute(t, s.e, s.c, ddl)
		defer dropIf(t, s, "rrv_uq")
		execute(t, s.e, s.c, "INSERT INTO rrv_uq (label) VALUES ('orig')")
		ref := core.TableRef{Name: "rrv_uq"}
		req := core.RowsBulkRequest{Connection: s.c, Table: ref, Action: "copy", Rows: bulkSnapshots(t, s.e, s.c, ref)}
		req = confirmBulk(t, s.e, req)
		first, err1 := s.e.RowsBulkExecute(context.Background(), req)
		_, err2 := s.e.RowsBulkExecute(context.Background(), req)
		t.Logf("%s: first outcome=%s reason=%s err=%v; retry err=%v", s.name, first.Outcome, first.Reason, err1, err2)
		if n := scalar(t, s, "SELECT COUNT(*) FROM rrv_uq"); n != "1" {
			t.Errorf("rows=%s", n)
		}
		// a new preview releases the digest again
		if _, err := s.e.RowsBulkPlan(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		_, err3 := s.e.RowsBulkExecute(context.Background(), req)
		t.Logf("%s: after new preview err=%v", s.name, err3)
	})
}
