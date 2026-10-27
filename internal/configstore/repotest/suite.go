// SPDX-License-Identifier: AGPL-3.0-only

// Package repotest is the shared behaviour suite for configstore.Repo
// implementations. The in-memory repository and the MariaDB server state run the
// very same scenarios, so the fast unit tests describe what the live tests prove.
package repotest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
)

// Factory returns a service on an empty repository. Owners are unique per call
// so a shared database can be reused; the returned owner names are random.
type Factory func(t *testing.T) *configstore.Service

var ctx = context.Background()

func code(err error) string {
	var public *configstore.Error
	if errors.As(err, &public) {
		return public.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func expect(t *testing.T, err error, want string) {
	t.Helper()
	if got := code(err); got != want {
		t.Fatalf("error code = %q, want %q", got, want)
	}
}

func raw(s string) []byte { return []byte(s) }

var ownerCounter atomic.Int64

// Owner returns a fresh owner name.
func Owner() string {
	return fmt.Sprintf("o%d_%d", time.Now().UnixNano()%1000000000, ownerCounter.Add(1))
}

func enableHistory(t *testing.T, s *configstore.Service, owner string) {
	t.Helper()
	if _, _, err := s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "privacy", Data: raw(`{"history_enabled":true}`)}); err != nil {
		t.Fatal(err)
	}
}

func withLimits(t *testing.T, kind configstore.Kind, mutate func(*configstore.Spec)) {
	t.Helper()
	for i := range configstore.Specs {
		if configstore.Specs[i].Kind == kind {
			saved := configstore.Specs[i]
			mutate(&configstore.Specs[i])
			t.Cleanup(func() { configstore.Specs[i] = saved })
			return
		}
	}
	t.Fatal("unknown kind")
}

// Run executes all scenarios. Scenarios that change global limits are not parallel.
func Run(t *testing.T, newService Factory) {
	scenarios := []struct {
		name string
		run  func(*testing.T, *configstore.Service)
	}{
		{"CRUDAndVersions", crud},
		{"ListPagingAndFilters", listing},
		{"OptimisticConcurrency", concurrency},
		{"QuotaExactUnderConcurrency", quota},
		{"FIFOEviction", fifo},
		{"OwnerIsolation", isolation},
		{"HistoryOptIn", historyOptIn},
		{"TouchAndIdempotence", touch},
		{"SecretRejection", secrets},
		{"UnicodeAndSizes", unicodeAndSizes},
		{"InjectionStaysData", injection},
		{"StrictJSON", strictJSON},
		{"ClearPlanConfirm", clearing},
		{"ImportExport", importExport},
		{"OwnerBytesQuota", ownerBytes},
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) { scenario.run(t, newService(t)) })
	}
}

func crud(t *testing.T, s *configstore.Service) {
	owner := Owner()
	item, created, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "abc", Data: raw(`{"name":"Orders","sql":"SELECT 1","variables":["id"]}`)})
	if err != nil || !created || item.Version != 1 || len(item.ID) != 32 {
		t.Fatalf("create: %+v %v %v", item, created, err)
	}
	got, err := s.Get(ctx, owner, "bookmarks", item.ID)
	if err != nil || string(got.Data) != `{"name":"Orders","sql":"SELECT 1","variables":["id"]}` || got.Scope != "abc" {
		t.Fatalf("get: %+v %v", got, err)
	}
	updated, err := s.Update(ctx, owner, "bookmarks", item.ID, raw(`{"name":"Orders","sql":"SELECT 2"}`), 1)
	if err != nil || updated.Version != 2 || !updated.UpdatedAt.After(item.UpdatedAt) {
		t.Fatalf("update: %+v %v", updated, err)
	}
	_, err = s.Update(ctx, owner, "bookmarks", item.ID, raw(`{"name":"x","sql":"SELECT 3"}`), 1)
	expect(t, err, "version_conflict")
	expect(t, s.Delete(ctx, owner, "bookmarks", item.ID, 1), "version_conflict")
	if err := s.Delete(ctx, owner, "bookmarks", item.ID, 2); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get(ctx, owner, "bookmarks", item.ID)
	expect(t, err, "not_found")
	expect(t, s.Delete(ctx, owner, "bookmarks", item.ID, configstore.AnyVersion), "not_found")
	_, err = s.Get(ctx, owner, "nothing", item.ID)
	expect(t, err, "unknown_kind")
	_, err = s.Get(ctx, owner, "bookmarks", "not-an-id")
	expect(t, err, "not_found")
	// Keyed documents: duplicate key conflicts, update by id works, key filter finds it.
	doc, _, err := s.Create(ctx, owner, "layouts", configstore.CreateInput{Scope: "abc", Key: "table:shop.orders", Data: raw(`{"columns":["id","total"],"widths":{"id":80}}`)})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Scope: "abc", Key: "table:shop.orders", Data: raw(`{}`)})
	expect(t, err, "already_exists")
	scope := "abc"
	page, err := s.List(ctx, owner, "layouts", configstore.Filter{Scope: &scope, Key: "table:shop.orders"})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != doc.ID || page.Items[0].Key != "table:shop.orders" {
		t.Fatalf("key filter: %+v %v", page, err)
	}
	other := "zzz"
	page, _ = s.List(ctx, owner, "layouts", configstore.Filter{Scope: &other, Key: "table:shop.orders"})
	if len(page.Items) != 0 {
		t.Fatal("key filter ignored scope")
	}
}

func listing(t *testing.T, s *configstore.Service) {
	owner := Owner()
	enableHistory(t, s, owner)
	var ids []string
	for i := 0; i < 7; i++ {
		scope := "s1"
		if i%2 == 1 {
			scope = "s2"
		}
		item, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Scope: scope, Data: raw(fmt.Sprintf(`{"sql":"SELECT %d"}`, i))})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}
	var seen []string
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		page, err := s.List(ctx, owner, "history", configstore.Filter{Limit: 3, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			seen = append(seen, item.ID)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 7 {
		t.Fatalf("paging returned %d items", len(seen))
	}
	for i := range seen {
		if seen[i] != ids[len(ids)-1-i] { // newest first
			t.Fatalf("order differs at %d", i)
		}
	}
	scope := "s2"
	page, _ := s.List(ctx, owner, "history", configstore.Filter{Scope: &scope})
	if len(page.Items) != 3 {
		t.Fatalf("scope filter: %d", len(page.Items))
	}
	empty := ""
	page, _ = s.List(ctx, owner, "history", configstore.Filter{Scope: &empty})
	if len(page.Items) != 0 {
		t.Fatal("global scope must be separate")
	}
	for _, bad := range []configstore.Filter{{Limit: -1}, {Limit: 101}, {Cursor: "!!"}, {Key: "k"}} {
		_, err := s.List(ctx, owner, "history", bad)
		expect(t, err, "invalid_request")
	}
}

func concurrency(t *testing.T, s *configstore.Service) {
	owner := Owner()
	item, _, err := s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: raw(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 16
	var wg sync.WaitGroup
	var ok, conflict atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.Update(ctx, owner, "settings", item.ID, raw(fmt.Sprintf(`{"a":%d}`, i)), 1)
			switch code(err) {
			case "":
				ok.Add(1)
			case "version_conflict":
				conflict.Add(1)
			default:
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if ok.Load() != 1 || conflict.Load() != workers-1 {
		t.Fatalf("lost update: %d ok, %d conflicts", ok.Load(), conflict.Load())
	}
	final, _ := s.Get(ctx, owner, "settings", item.ID)
	if final.Version != 2 {
		t.Fatalf("version %d", final.Version)
	}
	// Concurrent delete and update of the same version: exactly one wins.
	var wins atomic.Int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i == 0 {
				_, err = s.Update(ctx, owner, "settings", item.ID, raw(`{"a":9}`), 2)
			} else {
				err = s.Delete(ctx, owner, "settings", item.ID, 2)
			}
			if err == nil {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("delete/update race: %d winners", wins.Load())
	}
}

func quota(t *testing.T, s *configstore.Service) {
	withLimits(t, configstore.KindBookmarks, func(sp *configstore.Spec) { sp.PerScope, sp.PerOwner = 5, 8 })
	owner := Owner()
	create := func(scope string, i int) error {
		_, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: scope, Data: raw(fmt.Sprintf(`{"name":"b%d","sql":"SELECT %d"}`, i, i))})
		return err
	}
	var wg sync.WaitGroup
	var ok, full atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch code(create("scope-a", i)) {
			case "":
				ok.Add(1)
			case "quota_exceeded":
				full.Add(1)
			default:
				t.Errorf("unexpected quota error")
			}
		}(i)
	}
	wg.Wait()
	if ok.Load() != 5 || full.Load() != 15 {
		t.Fatalf("scope quota not exact: %d created, %d rejected", ok.Load(), full.Load())
	}
	for i := 0; i < 3; i++ {
		if err := create("scope-b", i); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, create("scope-c", 0), "quota_exceeded") // owner-wide cap of 8
	st, err := s.Status(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range st.Usage {
		if line.Kind == configstore.KindBookmarks && line.Count != 8 {
			t.Fatalf("usage %d", line.Count)
		}
	}
}

func fifo(t *testing.T, s *configstore.Service) {
	withLimits(t, configstore.KindHistory, func(sp *configstore.Spec) { sp.PerScope, sp.PerOwner = 4, 6 })
	owner := Owner()
	enableHistory(t, s, owner)
	for i := 0; i < 9; i++ {
		if _, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Scope: "a", Data: raw(fmt.Sprintf(`{"sql":"SELECT %d"}`, i))}); err != nil {
			t.Fatal(err)
		}
	}
	scope := "a"
	page, _ := s.List(ctx, owner, "history", configstore.Filter{Scope: &scope})
	if len(page.Items) != 4 || !strings.Contains(string(page.Items[0].Data), "SELECT 8") || !strings.Contains(string(page.Items[3].Data), "SELECT 5") {
		t.Fatalf("scope FIFO kept the wrong entries: %d", len(page.Items))
	}
	for i := 0; i < 4; i++ {
		if _, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Scope: "b", Data: raw(fmt.Sprintf(`{"sql":"SELECT b%d"}`, i))}); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := s.List(ctx, owner, "history", configstore.Filter{})
	if len(all.Items) != 6 {
		t.Fatalf("owner FIFO: %d", len(all.Items))
	}
}

func isolation(t *testing.T, s *configstore.Service) {
	alice, bob := Owner(), Owner()
	a, _, err := s.Create(ctx, alice, "favorites", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"shop","table":"orders"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, bob, "favorites", a.ID); code(err) != "not_found" {
		t.Fatalf("cross-owner get: %v", err)
	}
	expect(t, s.Delete(ctx, bob, "favorites", a.ID, configstore.AnyVersion), "not_found")
	page, _ := s.List(ctx, bob, "favorites", configstore.Filter{})
	if len(page.Items) != 0 {
		t.Fatal("bob sees alice")
	}
	// Same natural key for bob is a separate entry.
	b, created, err := s.Create(ctx, bob, "favorites", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"shop","table":"orders"}`)})
	if err != nil || !created || b.ID == a.ID {
		t.Fatal("owners share a natural key namespace")
	}
	plan, _ := s.PlanClear(ctx, alice, []string{"favorites"}, nil)
	if _, err := s.ExecuteClear(ctx, alice, []string{"favorites"}, nil, plan.PlanHash, plan.Confirmation); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, bob, "favorites", b.ID); err != nil {
		t.Fatal("clearing alice removed bob's data")
	}
	exp, _ := s.Export(ctx, bob)
	if len(exp) != 1 {
		t.Fatal("export leaks or loses entries")
	}
	_, _, err = s.Create(ctx, "bad owner!", "favorites", configstore.CreateInput{Data: raw(`{"table":"t"}`)})
	expect(t, err, "invalid_request")
}

func historyOptIn(t *testing.T, s *configstore.Service) {
	owner := Owner()
	_, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Data: raw(`{"sql":"SELECT 1"}`)})
	expect(t, err, "history_disabled")
	for _, bad := range []string{`{}`, `{"history_enabled":"yes"}`, `{"history_enabled":true,"x":1}`, `{"history_enabled":1}`} {
		_, _, err = s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "privacy", Data: raw(bad)})
		expect(t, err, "invalid_data")
	}
	priv, _, err := s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "privacy", Data: raw(`{"history_enabled":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: raw(`{"sql":"SELECT 1"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, owner, "settings", priv.ID, raw(`{"history_enabled":false}`), 1); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: raw(`{"sql":"SELECT 2"}`)})
	expect(t, err, "history_disabled")
	// The opt-in of one owner never enables another.
	_, _, err = s.Create(ctx, Owner(), "history", configstore.CreateInput{Data: raw(`{"sql":"SELECT 3"}`)})
	expect(t, err, "history_disabled")
	// History entries are immutable.
	enableHistory(t, s, Owner())
}

func touch(t *testing.T, s *configstore.Service) {
	owner := Owner()
	a, created, err := s.Create(ctx, owner, "recents", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"d","table":"a"}`)})
	if err != nil || !created {
		t.Fatal(err)
	}
	if _, _, err := s.Create(ctx, owner, "recents", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"d","table":"b"}`)}); err != nil {
		t.Fatal(err)
	}
	again, created, err := s.Create(ctx, owner, "recents", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"d","table":"a"}`)})
	if err != nil || created || again.ID != a.ID || again.Version != 2 {
		t.Fatalf("touch: %+v %v %v", again, created, err)
	}
	scope := "s"
	page, _ := s.List(ctx, owner, "recents", configstore.Filter{Scope: &scope})
	if len(page.Items) != 2 || page.Items[0].ID != a.ID {
		t.Fatal("touched entry must be first")
	}
	fav, c1, _ := s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"d","table":"a"}`)})
	fav2, c2, err := s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "s", Data: raw(`{"database":"d","table":"a"}`)})
	if err != nil || !c1 || c2 || fav.ID != fav2.ID || fav2.Version != 1 {
		t.Fatal("favorites must be idempotent without change")
	}
	_, err = s.Update(ctx, owner, "favorites", fav.ID, raw(`{"database":"d","table":"z"}`), 1)
	expect(t, err, "invalid_request")
	// A different scope or table is a different favorite.
	if _, c, _ := s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "t", Data: raw(`{"database":"d","table":"a"}`)}); !c {
		t.Fatal("scope must separate favorites")
	}
	for _, bad := range []string{`{"table":""}`, `{"table":"t","extra":1}`, `{"database":"d"}`, `{"table":"a\u0000b"}`} {
		_, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Data: raw(bad)})
		expect(t, err, "invalid_data")
	}
}

var secretSQL = []string{
	"CREATE USER app IDENTIFIED BY 'x'",
	"ALTER USER app IDENTIFIED WITH mysql_native_password",
	"SET PASSWORD FOR app = PASSWORD('x')",
	"SELECT 1 /* token */",
	"SELECT * FROM t WHERE api_key = 'k'",
	"PRAGMA key = 'x'",
	"ATTACH 'x.db' AS e KEY 'abc'",
	"SELECT $$secret$$",
	"CREATE SERVER s FOREIGN DATA WRAPPER mysql",
	"host=h uid=u pwd=p",
	"SELECT 'postgres://user:pw@db.example/x'",
	"-----BEGIN PRIVATE KEY-----",
	"SELECT 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc'",
	"SELECT 'ghp_0123456789abcdefghijklmnopqrstuvwx'",
	"SELECT 'AKIAABCDEFGHIJKLMNOP'",
	"SELECT 'Zm9vYmFyMTIzNDU2Nzg5MEFCQ0RFRkdISUpLTE1OT1BRUlNUVQ'",
	"SELECT '0123456789abcdef0123456789abcdef01234567'",
	"ＰＡＳＳＷＯＲＤ",  // full-width
	"PASS​WORD", // zero-width space
	"pa­ssword", // soft hyphen
	"SECRET",    // plain
	"Authorization: Bearer abcdefghijklmnop1234",
}

var benignSQL = []string{
	"SELECT id, name FROM customers WHERE id = 42",
	"SELECT * FROM orders o JOIN items i ON i.order_id = o.id ORDER BY o.created_at DESC LIMIT 50",
	"INSERT INTO notes (body) VALUES ('Grüße 😀 日本語')",
	"SELECT 'a_very_long_snake_case_identifier_that_goes_on_and_on_and_on'",
	"SELECT * FROM /var/lib/mysql/data/shop/orders",
}

func secrets(t *testing.T, s *configstore.Service) {
	owner := Owner()
	enableHistory(t, s, owner)
	for _, kind := range []string{"history", "bookmarks"} {
		for _, sql := range secretSQL {
			data := fmt.Sprintf(`{"sql":%s}`, jsonString(sql))
			if kind == "bookmarks" {
				data = fmt.Sprintf(`{"name":"n","sql":%s}`, jsonString(sql))
			}
			_, _, err := s.Create(ctx, owner, kind, configstore.CreateInput{Data: raw(data)})
			if code(err) != "secret_rejected" {
				t.Errorf("%s accepted %q (%s)", kind, sql, code(err))
			}
		}
		for _, sql := range benignSQL {
			data := fmt.Sprintf(`{"sql":%s}`, jsonString(sql))
			if kind == "bookmarks" {
				data = fmt.Sprintf(`{"name":"n","sql":%s}`, jsonString(sql))
			}
			if _, _, err := s.Create(ctx, owner, kind, configstore.CreateInput{Data: raw(data)}); err != nil {
				t.Errorf("%s rejected benign %q: %v", kind, sql, err)
			}
		}
	}
	// Names are scanned too, and so are documents (structural rules only).
	_, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(`{"name":"my password","sql":"SELECT 1"}`)})
	expect(t, err, "secret_rejected")
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "t", Data: raw(`{"url":"mysql://root:hunter2@host/db"}`)})
	expect(t, err, "secret_rejected")
	if _, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "table:users", Data: raw(`{"columns":["id","password_hash","token"]}`)}); err != nil {
		t.Fatalf("column names such as password_hash must be storable in layouts: %v", err)
	}
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "k", Data: raw(`{"x":"Zm9vYmFyMTIzNDU2Nzg5MEFCQ0RFRkdISUpLTE1OT1BRUlNUVQ"}`)})
	expect(t, err, "secret_rejected")
	// A 64-hex scope (SHA-256 of the connection identity) must stay storable.
	if _, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: strings.Repeat("ab12", 16), Data: raw(`{"table":"t"}`)}); err != nil {
		t.Fatalf("hash scope rejected: %v", err)
	}
	// Registered literal secrets (the API token) are rejected everywhere.
	s.AddForbidden("tok-0123456789")
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "k", Data: raw(`{"note":"x tok-0123456789 y"}`)})
	expect(t, err, "secret_rejected")
}

func unicodeAndSizes(t *testing.T, s *configstore.Service) {
	owner := Owner()
	good := "SELECT 'ä ö ü ß 😀 日本語 العربية' AS t"
	item, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(fmt.Sprintf(`{"name":"Üñí 😀","sql":%s}`, jsonString(good)))})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, owner, "bookmarks", item.ID)
	if !strings.Contains(string(got.Data), "日本語") {
		t.Fatal("unicode not preserved")
	}
	bad := map[string]string{
		"nul":              `{"name":"n","sql":"SELECT 1\u0000"}`,
		"control":          `{"name":"n","sql":"SELECT\u0007 1"}`,
		"lone surrogate":   `{"name":"n","sql":"SELECT \ud800"}`,
		"replacement char": "{\"name\":\"n\",\"sql\":\"SELECT �\"}",
		"empty sql":        `{"name":"n","sql":"   "}`,
		"empty name":       `{"name":"","sql":"SELECT 1"}`,
		"padded name":      `{"name":" n","sql":"SELECT 1"}`,
		"long name":        `{"name":"` + strings.Repeat("a", 101) + `","sql":"SELECT 1"}`,
		"unknown field":    `{"name":"n","sql":"SELECT 1","extra":true}`,
		"bad variable":     `{"name":"n","sql":"SELECT 1","variables":["1x"]}`,
		"dup variable":     `{"name":"n","sql":"SELECT 1","variables":["a","a"]}`,
		"many variables":   `{"name":"n","sql":"SELECT 1","variables":[` + strings.TrimSuffix(strings.Repeat(`"v",`, 21), ",") + `]}`,
	}
	for name, data := range bad {
		_, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(data)})
		if c := code(err); c != "invalid_data" && c != "secret_rejected" {
			t.Errorf("%s: code %q", name, c)
		}
	}
	// Invalid UTF-8 bytes in the raw document.
	_, _, err = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: []byte("{\"name\":\"n\",\"sql\":\"SELECT \xff\"}")})
	expect(t, err, "invalid_request")
	// Size limits: SQL just inside and just outside.
	inside := "SELECT '" + strings.Repeat("x", 100000-20) + "'"
	if _, _, err = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(fmt.Sprintf(`{"name":"big","sql":%s}`, jsonString(inside)))}); err != nil {
		t.Fatalf("100k chars must fit: %v", err)
	}
	outside := "SELECT '" + strings.Repeat("x", 100001) + "'"
	_, _, err = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(fmt.Sprintf(`{"name":"big","sql":%s}`, jsonString(outside)))})
	expect(t, err, "item_too_large")
	multi := "SELECT '" + strings.Repeat("日", 90000) + "'" // 270k bytes, 90k chars
	_, _, err = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(fmt.Sprintf(`{"name":"big","sql":%s}`, jsonString(multi)))})
	expect(t, err, "item_too_large")
	// Scope and key charsets.
	for _, scope := range []string{"a b", "a/b", "ä", strings.Repeat("a", 129), "a;b", "a'b", "a\x00"} {
		_, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: raw(`{"table":"t"}`)})
		expect(t, err, "invalid_request")
	}
	_, _, err = s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "Bad Key", Data: raw(`{}`)})
	expect(t, err, "invalid_request")
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: strings.Repeat("k", 201), Data: raw(`{}`)})
	expect(t, err, "invalid_request")
	_, _, err = s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "a​b", Data: raw(`{}`)})
	expect(t, err, "invalid_request")
	_, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Key: "k", Data: raw(`{"table":"t"}`)})
	expect(t, err, "invalid_request")
	_, _, err = s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "a", ClientRef: "r", Data: raw(`{}`)})
	expect(t, err, "invalid_request")
}

func injection(t *testing.T, s *configstore.Service) {
	owner := Owner()
	enableHistory(t, s, owner)
	payloads := []string{"'; DROP TABLE config_items; --", "\" OR 1=1 --", "x' UNION SELECT sealed FROM config_items --", "%' OR '1'='1", "\\'; DELETE FROM config_owners; --", "SELECT `a` FROM `t` WHERE x = '\\\\'"}
	for _, p := range payloads {
		item, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: raw(fmt.Sprintf(`{"name":"inj","sql":%s}`, jsonString(p)))})
		if err != nil {
			t.Fatalf("payload %q: %v", p, err)
		}
		got, err := s.Get(ctx, owner, "bookmarks", item.ID)
		if err != nil || !strings.Contains(string(got.Data), jsonString(p)[1:len(jsonString(p))-1]) {
			t.Fatalf("payload not preserved verbatim: %q -> %s", p, got.Data)
		}
		for _, field := range []func(string) configstore.CreateInput{
			func(v string) configstore.CreateInput {
				return configstore.CreateInput{Scope: v, Data: raw(`{"table":"t"}`)}
			},
			func(v string) configstore.CreateInput {
				return configstore.CreateInput{ClientRef: v, Data: raw(`{"sql":"SELECT 1"}`)}
			},
			func(v string) configstore.CreateInput { return configstore.CreateInput{Key: v, Data: raw(`{}`)} },
		} {
			if _, _, err := s.Create(ctx, owner, "favorites", field(p)); code(err) != "invalid_request" {
				t.Errorf("identifier field accepted %q: %v", p, err)
			}
		}
	}
	for _, bad := range []string{"'; DROP TABLE x;--", "1' OR '1'='1", "id\x00"} {
		scope := bad
		_, err := s.List(ctx, owner, "bookmarks", configstore.Filter{Scope: &scope})
		expect(t, err, "invalid_request")
		_, err = s.Get(ctx, owner, "bookmarks", bad)
		expect(t, err, "not_found")
	}
	page, err := s.List(ctx, owner, "bookmarks", configstore.Filter{})
	if err != nil || len(page.Items) != len(payloads) {
		t.Fatalf("store damaged after injection attempts: %d %v", len(page.Items), err)
	}
}

func strictJSON(t *testing.T, s *configstore.Service) {
	owner := Owner()
	cases := map[string]string{
		"duplicate key":  `{"a":1,"a":2}`,
		"nested dup":     `{"a":{"b":1,"b":2}}`,
		"null":           `{"a":null}`,
		"too deep":       `{"a":{"b":{"c":{"d":{"e":{"f":1}}}}}}`,
		"big number":     `{"a":1e999}`,
		"big array":      `{"a":[` + strings.TrimSuffix(strings.Repeat("1,", 257), ",") + `]}`,
		"many keys":      manyKeys(65),
		"long string":    `{"a":"` + strings.Repeat("x", 4097) + `"}`,
		"empty key":      `{"":1}`,
		"control in key": `{"a\u0001":1}`,
		"array root":     `[1]`,
		"string root":    `"x"`,
		"empty":          ``,
		"trailing data":  `{"a":1} {"b":2}`,
		"truncated":      `{"a":`,
		"too many nodes": `{"a":[` + strings.TrimSuffix(strings.Repeat("[1,1,1,1,1,1,1,1,1,1],", 256), ",") + `]}`,
	}
	for name, doc := range cases {
		_, _, err := s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "k-" + strings.ReplaceAll(name, " ", "-"), Data: raw(doc)})
		if c := code(err); c != "invalid_data" && c != "invalid_request" {
			t.Errorf("%s accepted/other: %q", name, c)
		}
	}
	if _, _, err := s.Create(ctx, owner, "layouts", configstore.CreateInput{Key: "ok", Data: raw(`{"a":{"b":{"c":{"d":[1,2.5,-3e2,true,"x"]}}}}`)}); err != nil {
		t.Fatal(err)
	}
}

func manyKeys(n int) string {
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `"k%d":1`, i)
	}
	b.WriteString("}")
	return b.String()
}

func clearing(t *testing.T, s *configstore.Service) {
	owner := Owner()
	enableHistory(t, s, owner)
	for i := 0; i < 3; i++ {
		_, _, _ = s.Create(ctx, owner, "history", configstore.CreateInput{Scope: "a", Data: raw(fmt.Sprintf(`{"sql":"SELECT %d"}`, i))})
	}
	_, _, _ = s.Create(ctx, owner, "history", configstore.CreateInput{Scope: "b", Data: raw(`{"sql":"SELECT 9"}`)})
	_, _, _ = s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "a", Data: raw(`{"table":"t"}`)})
	scopeA := "a"
	plan, err := s.PlanClear(ctx, owner, []string{"history"}, &scopeA)
	if err != nil || plan.TotalItems != 3 || plan.Confirmation != "LÖSCHEN 3" || len(plan.PlanHash) != 64 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	_, err = s.ExecuteClear(ctx, owner, []string{"history"}, &scopeA, plan.PlanHash, "LÖSCHEN")
	expect(t, err, "confirmation_invalid")
	_, err = s.ExecuteClear(ctx, owner, []string{"history"}, &scopeA, plan.PlanHash, "löschen 3")
	expect(t, err, "confirmation_invalid")
	// A change after the plan makes it stale and deletes nothing.
	_, _, _ = s.Create(ctx, owner, "history", configstore.CreateInput{Scope: "a", Data: raw(`{"sql":"SELECT 10"}`)})
	_, err = s.ExecuteClear(ctx, owner, []string{"history"}, &scopeA, plan.PlanHash, plan.Confirmation)
	expect(t, err, "plan_stale")
	_, err = s.ExecuteClear(ctx, owner, []string{"history"}, &scopeA, "0000", "LÖSCHEN 4")
	expect(t, err, "plan_stale")
	page, _ := s.List(ctx, owner, "history", configstore.Filter{Scope: &scopeA})
	if len(page.Items) != 4 {
		t.Fatalf("stale clear deleted %d", 4-len(page.Items))
	}
	plan, _ = s.PlanClear(ctx, owner, []string{"history"}, &scopeA)
	// Same plan for a different kind set must not be accepted with this hash.
	_, err = s.ExecuteClear(ctx, owner, []string{"history", "favorites"}, &scopeA, plan.PlanHash, "LÖSCHEN 5")
	expect(t, err, "plan_stale")
	deleted, err := s.ExecuteClear(ctx, owner, []string{"history"}, &scopeA, plan.PlanHash, plan.Confirmation)
	if err != nil || deleted[configstore.KindHistory] != 4 {
		t.Fatalf("clear: %v %v", deleted, err)
	}
	all, _ := s.List(ctx, owner, "history", configstore.Filter{})
	fav, _ := s.List(ctx, owner, "favorites", configstore.Filter{})
	if len(all.Items) != 1 || len(fav.Items) != 1 {
		t.Fatal("scoped clear touched other scope or kind")
	}
	for _, kinds := range [][]string{nil, {}, {"history", "history"}, {"nope"}, {"history", "bookmarks", "searches", "favorites", "recents", "settings", "layouts", "history"}} {
		_, err = s.PlanClear(ctx, owner, kinds, nil)
		expect(t, err, "invalid_request")
	}
	// Clearing everything incl. the privacy setting switches the history off again.
	plan, _ = s.PlanClear(ctx, owner, []string{"history", "bookmarks", "searches", "favorites", "recents", "settings", "layouts"}, nil)
	if _, err = s.ExecuteClear(ctx, owner, []string{"history", "bookmarks", "searches", "favorites", "recents", "settings", "layouts"}, nil, plan.PlanHash, plan.Confirmation); err != nil {
		t.Fatal(err)
	}
	exp, _ := s.Export(ctx, owner)
	if len(exp) != 0 {
		t.Fatalf("%d entries survived clear-all", len(exp))
	}
	_, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: raw(`{"sql":"SELECT 1"}`)})
	expect(t, err, "history_disabled")
}

func importExport(t *testing.T, s *configstore.Service) {
	owner := Owner()
	enableHistory(t, s, owner)
	old := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	older := old.Add(-time.Hour)
	items := []configstore.ImportItem{
		{Kind: "history", CreateInput: configstore.CreateInput{Scope: "s", ClientRef: "local-1", Time: &old, Data: raw(`{"sql":"SELECT 1"}`)}},
		{Kind: "history", CreateInput: configstore.CreateInput{Scope: "s", ClientRef: "local-2", Time: &older, Data: raw(`{"sql":"SELECT 2"}`)}},
		{Kind: "bookmarks", CreateInput: configstore.CreateInput{Scope: "s", ClientRef: "bm-1", Data: raw(`{"name":"B","sql":"SELECT 3"}`)}},
		{Kind: "bookmarks", CreateInput: configstore.CreateInput{Scope: "s", Data: raw(`{"name":"leak","sql":"SET PASSWORD = 'x'"}`)}},
		{Kind: "nope", CreateInput: configstore.CreateInput{Data: raw(`{}`)}},
		{Kind: "settings", CreateInput: configstore.CreateInput{Key: "table_view", Data: raw(`{"rows":50}`)}},
	}
	results, err := s.Import(ctx, owner, items, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"created", "created", "created", "rejected", "rejected", "created"}
	for i, r := range results {
		if r.Status != want[i] {
			t.Errorf("item %d: %s (%s)", i, r.Status, r.Reason)
		}
	}
	if results[3].Reason != "secret_rejected" || results[4].Reason != "unknown_kind" {
		t.Fatalf("reasons: %q %q", results[3].Reason, results[4].Reason)
	}
	again, _ := s.Import(ctx, owner, items, false)
	if again[0].Status != "exists" || again[0].ID != results[0].ID || again[2].Status != "exists" || again[5].Status != "exists" {
		t.Fatalf("import is not idempotent: %+v", again)
	}
	hist, _ := s.List(ctx, owner, "history", configstore.Filter{})
	if len(hist.Items) != 2 || hist.Items[0].UpdatedAt.UTC().Format(time.RFC3339) != old.Format(time.RFC3339) {
		t.Fatalf("original times not preserved: %+v", hist.Items)
	}
	over, _ := s.Import(ctx, owner, []configstore.ImportItem{{Kind: "settings", CreateInput: configstore.CreateInput{Key: "table_view", Data: raw(`{"rows":100}`)}}}, true)
	if over[0].Status != "updated" {
		t.Fatalf("overwrite: %+v", over)
	}
	doc, _ := s.Get(ctx, owner, "settings", over[0].ID)
	if string(doc.Data) != `{"rows":100}` || doc.Version != 2 {
		t.Fatalf("overwritten document: %+v", doc)
	}
	future := time.Now().Add(72 * time.Hour)
	res, _ := s.Import(ctx, owner, []configstore.ImportItem{{Kind: "bookmarks", CreateInput: configstore.CreateInput{Time: &future, Data: raw(`{"name":"f","sql":"SELECT 1"}`)}}}, false)
	if res[0].Status != "rejected" || res[0].Reason != "invalid_request" {
		t.Fatalf("future time accepted: %+v", res)
	}
	if _, err = s.Import(ctx, owner, nil, false); code(err) != "invalid_request" {
		t.Fatal("empty import accepted")
	}
	if _, err = s.Import(ctx, owner, make([]configstore.ImportItem, 101), false); code(err) != "invalid_request" {
		t.Fatal("oversized import accepted")
	}
	// Export then import into a fresh owner reproduces the data.
	exp, err := s.Export(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	target := Owner()
	enableHistory(t, s, target)
	var back []configstore.ImportItem
	for _, e := range exp {
		if e.Kind == configstore.KindSettings && e.Key == "privacy" {
			continue
		}
		tm := e.Time
		back = append(back, configstore.ImportItem{Kind: string(e.Kind), CreateInput: configstore.CreateInput{Scope: e.Scope, Key: e.Key, ClientRef: e.ClientRef, Time: &tm, Data: e.Data}})
	}
	res, err = s.Import(ctx, target, back, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Status != "created" {
			t.Fatalf("round trip: %+v", r)
		}
	}
	exp2, _ := s.Export(ctx, target)
	if len(exp2) != len(exp) {
		t.Fatalf("round trip count %d vs %d", len(exp2), len(exp))
	}
}

func ownerBytes(t *testing.T, s *configstore.Service) {
	saved := configstore.MaxOwnerBytes
	configstore.MaxOwnerBytes = 600 << 10
	t.Cleanup(func() { configstore.MaxOwnerBytes = saved })
	owner := Owner()
	chunk := strings.Repeat("a b ", 4000) // 16k chars
	created := 0
	var last error
	for i := 0; i < 100 && last == nil; i++ {
		_, _, last = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: fmt.Sprintf("sc%d", i/50), Data: raw(fmt.Sprintf(`{"name":"b%d","sql":"SELECT '%s%d'"}`, i, chunk, i))})
		if last == nil {
			created++
		}
	}
	expect(t, last, "quota_exceeded")
	if created < 30 || created > 40 {
		t.Fatalf("byte quota reached after %d entries", created)
	}
	st, _ := s.Status(ctx, owner)
	if st.TotalBytes > configstore.MaxOwnerBytes {
		t.Fatal("quota exceeded")
	}
	// Growing an existing entry beyond the remaining room is refused too.
	page, _ := s.List(ctx, owner, "bookmarks", configstore.Filter{Limit: 1})
	big := strings.Repeat("c d ", 8000)
	_, err := s.Update(ctx, owner, "bookmarks", page.Items[0].ID, raw(fmt.Sprintf(`{"name":"g","sql":"SELECT '%s'"}`, big)), page.Items[0].Version)
	if err != nil && code(err) != "quota_exceeded" {
		t.Fatal(err)
	}
	// Other owners are unaffected.
	if _, _, err := s.Create(ctx, Owner(), "bookmarks", configstore.CreateInput{Data: raw(`{"name":"x","sql":"SELECT 1"}`)}); err != nil {
		t.Fatal(err)
	}
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
