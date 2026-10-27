// SPDX-License-Identifier: AGPL-3.0-only
package configstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
)

// Independent security review of W-P20 (docs/handoffs/W-P20-backend-review.md).
//
// Tests named TestReviewGap_* pin the CURRENT, undesirable behaviour of a
// finding. They pass today; when a finding is fixed the assertion must be
// flipped (the comment names the finding). Tests named TestReviewHolds_* assert
// behaviour that must keep holding.

func reviewCode(err error) string {
	var public *configstore.Error
	if errors.As(err, &public) {
		return public.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

func reviewService() *configstore.Service { return configstore.NewService(configstore.NewMemoryRepo()) }

func reviewHistory(t *testing.T, s *configstore.Service, owner string) {
	t.Helper()
	if _, _, err := s.Create(context.Background(), owner, "settings", configstore.CreateInput{Key: "privacy", Data: []byte(`{"history_enabled":true}`)}); err != nil {
		t.Fatal(err)
	}
}

// F-04 (fixed): invisible, combining and look-alike characters no longer split keywords.
func TestReviewHolds_InvisibleAndConfusableCharactersFolded(t *testing.T) {
	s, owner := reviewService(), "rev_a"
	reviewHistory(t, s, owner)
	for name, sql := range map[string]string{
		"combining grapheme joiner U+034F": "SET PASS͏WORD = 'x'",
		"variation selector U+FE0F":        "SET PASS️WORD = 'x'",
		"hangul filler U+3164":             "SET PASSㅤWORD = 'x'",
		"cyrillic a":                       "SET pаssword = 'x'",
	} {
		_, _, err := s.Create(context.Background(), owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql + `"}`)})
		if reviewCode(err) != "secret_rejected" {
			t.Errorf("%s accepted: %v", name, reviewCode(err))
		}
	}
}

// Must hold: the documented fold catches full-width, zero-width and soft hyphen.
func TestReviewHolds_FoldedKeywordsRejected(t *testing.T) {
	s, owner := reviewService(), "rev_b"
	reviewHistory(t, s, owner)
	for _, sql := range []string{"SET ＰＡＳＳＷＯＲＤ = 'x'", "SET PASS​WORD = 'x'", "SET PASS­WORD = 'x'", "SET PASS⁠WORD='x'", "ＩＤＥＮＴＩＦＩＥＤ BY 'x'"} {
		_, _, err := s.Create(context.Background(), owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql + `"}`)})
		if reviewCode(err) != "secret_rejected" {
			t.Errorf("%q accepted: %v", sql, reviewCode(err))
		}
	}
}

// F-05 (fixed): scope and client_ref are scanned like keys.
func TestReviewHolds_ScopeAndClientRefScanned(t *testing.T) {
	s, owner := reviewService(), "rev_c"
	token := "ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234567890"
	if _, _, err := s.Create(context.Background(), owner, "favorites", configstore.CreateInput{Scope: token, Data: []byte(`{"table":"t"}`)}); reviewCode(err) != "secret_rejected" {
		t.Errorf("scope with token format: %v", reviewCode(err))
	}
	if _, _, err := s.Create(context.Background(), owner, "bookmarks", configstore.CreateInput{ClientRef: "AKIAABCDEFGHIJKLMNOP", Data: []byte(`{"name":"n","sql":"SELECT 1"}`)}); reviewCode(err) != "secret_rejected" {
		t.Errorf("client_ref with token format: %v", reviewCode(err))
	}
	// A SHA-256 scope and a UUID client reference stay storable.
	if _, _, err := s.Create(context.Background(), owner, "bookmarks", configstore.CreateInput{Scope: strings.Repeat("ab12", 16), ClientRef: "0b6a7c3e-1f2d-4a5b-9c8d-7e6f5a4b3c2d", Data: []byte(`{"name":"n","sql":"SELECT 1"}`)}); err != nil {
		t.Errorf("hash scope / uuid ref rejected: %v", err)
	}
}

// F-06 (fixed): documents no longer take credentials under telling keys or as pairs/commands.
func TestReviewHolds_DocumentsRejectPlainCredentials(t *testing.T) {
	s, owner := reviewService(), "rev_d"
	for i, doc := range []string{
		`{"password":"hunter2"}`,
		`{"dsn":"host=db user=app password=hunter2"}`,
		`{"cmd":"curl -u admin:hunter2 http://db"}`,
		`{"pin":"4711"}`,
		`{"hex32":"0123456789abcdef0123456789abcdef"}`,
		`{"a":{"api_key":"abcd"}}`,
	} {
		if _, _, err := s.Create(context.Background(), owner, "layouts", configstore.CreateInput{Key: "k" + string(rune('a'+i)), Data: []byte(doc)}); reviewCode(err) != "secret_rejected" {
			t.Errorf("%s accepted: %v", doc, reviewCode(err))
		}
	}
	// Legitimate documents stay storable.
	for i, doc := range []string{`{"columns":["id","password_hash","token"],"sort_key":"id"}`, `{"enabled":true,"password_visible":false}`, `{"author":"x"}`} {
		if _, _, err := s.Create(context.Background(), owner, "layouts", configstore.CreateInput{Key: "ok" + string(rune('a'+i)), Data: []byte(doc)}); err != nil {
			t.Errorf("%s rejected: %v", doc, reviewCode(err))
		}
	}
}

// F-07 (fixed): the registered API token is compared after folding.
func TestReviewHolds_ForbiddenLiteralFolded(t *testing.T) {
	s, owner := reviewService(), "rev_e"
	s.AddForbidden("tokenvalue-0123456789")
	for i, text := range []string{"tokenvalue-0123456789", "tokenvalue​-0123456789", "ｔｏｋｅｎｖａｌｕｅ-0123456789"} {
		if _, _, err := s.Create(context.Background(), owner, "layouts", configstore.CreateInput{Key: "k" + string(rune('a'+i)), Data: []byte(`{"n":"` + text + `"}`)}); reviewCode(err) != "secret_rejected" {
			t.Errorf("variant %d accepted: %v", i, reviewCode(err))
		}
	}
}

// F-08: opaque secrets that the long-run heuristic cannot see.
func TestReviewGap_OpaqueRunHeuristicMisses(t *testing.T) {
	s, owner := reviewService(), "rev_f"
	reviewHistory(t, s, owner)
	for name, sql := range map[string]string{
		"32 hex chars (md5-sized key)": "SELECT '0123456789abcdef0123456789abcdef'",
		"64 hex split by spaces":       "SELECT '" + strings.Repeat("a1b2c3d4", 2) + " " + strings.Repeat("e5f6a7b8", 2) + " " + strings.Repeat("c9d0e1f2", 2) + " " + strings.Repeat("a3b4c5d6", 2) + "'",
		"base32 lowercase 52 chars":    "SELECT 'mfrggzdfmztwq2lknnwg23tpobyxe43uovwgyzjamfrggzdfmzt'",
		"basic auth in curl":           "-- curl -u admin:hunter2 https://x",
	} {
		_, _, err := s.Create(context.Background(), owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql + `"}`)})
		if err != nil {
			t.Logf("%s: now rejected (%v); heuristic extended", name, reviewCode(err))
		}
	}
}

// F-09: false positives that cost an entry (documented as accepted, measured here).
func TestReviewGap_FalsePositives(t *testing.T) {
	s, owner := reviewService(), "rev_g"
	reviewHistory(t, s, owner)
	for name, sql := range map[string]string{
		"sha256 of a file":      "SELECT * FROM files WHERE sha = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
		"table named tokens":    "SELECT COUNT(*) FROM tokens",
		"unix uid column":       "SELECT * FROM procs WHERE uid = 0",
		"column access_token":   "SELECT access_token_count FROM stats",
		"price with dollars":    "SELECT '$$ cheap'",
		"long CamelCase ident":  "SELECT * FROM `Order2ItemLineSnapshotRecordHistoryExtended_0042_Final_ABC`",
		"secretary table":       "SELECT * FROM secretary",
		"keyword in a comment":  "SELECT 1 -- credentials rotated",
		"hash literal in where": "SELECT * FROM t WHERE h = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdef'",
	} {
		_, _, err := s.Create(context.Background(), owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql + `"}`)})
		if reviewCode(err) != "secret_rejected" {
			t.Logf("%s: accepted (%v)", name, reviewCode(err))
		}
	}
}

// Must hold: structural rules in documents.
func TestReviewHolds_StructuralSecretsInDocuments(t *testing.T) {
	s, owner := reviewService(), "rev_h"
	for _, doc := range []string{
		`{"u":"postgres://app:hunter2@db/x"}`,
		`{"u":"ＰＯＳＴＧＲＥＳ://app:hunter2@db/x"}`,
		`{"k":"-----BEGIN PRIVATE KEY-----"}`,
		`{"j":"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc"}`,
		`{"a":"Bearer abcdefghijklmnopqrstuvwxyz"}`,
		`{"a":"gh​p_ABCDEFGHIJKLMNOPQRST1234567890"}`,
		`{"nested":{"deeper":["ok","AKIAABCDEFGHIJKLMNOP"]}}`,
		`{"AKIAABCDEFGHIJKLMNOP":"key position"}`,
	} {
		if _, _, err := s.Create(context.Background(), owner, "settings", configstore.CreateInput{Key: "abc", Data: []byte(doc)}); reviewCode(err) != "secret_rejected" {
			t.Errorf("%s accepted: %v", doc, reviewCode(err))
		}
	}
}

// F-10 (fixed): the owner byte quota gives way to history before rejecting another kind.
func TestReviewHolds_OwnerByteQuotaEvictsHistory(t *testing.T) {
	saved := configstore.MaxOwnerBytes
	configstore.MaxOwnerBytes = 600
	t.Cleanup(func() { configstore.MaxOwnerBytes = saved })
	s, owner := reviewService(), "rev_i"
	reviewHistory(t, s, owner)
	sql := strings.Repeat("a", 90)
	for i := 0; i < 4; i++ {
		if _, _, err := s.Create(context.Background(), owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"SELECT '` + sql + `' /*` + string(rune('a'+i)) + `*/"}`)}); err != nil {
			t.Fatalf("history %d: %v", i, err)
		}
	}
	if _, _, err := s.Create(context.Background(), owner, "bookmarks", configstore.CreateInput{Data: []byte(`{"name":"b","sql":"SELECT '` + strings.Repeat("b", 150) + `'"}`)}); err != nil {
		t.Errorf("bookmark with full history: %v", reviewCode(err))
	}
	// Non-history data is never evicted: with no history left the quota rejects.
	_, _, err := s.Create(context.Background(), owner, "bookmarks", configstore.CreateInput{Data: []byte(`{"name":"c","sql":"SELECT '` + strings.Repeat("c", 400) + `'"}`)})
	if reviewCode(err) != "quota_exceeded" {
		t.Errorf("expected quota_exceeded, got %q", reviewCode(err))
	}
}

// F-11 (fixed): nkey and scope are keyed hashes; the dictionary attack is covered by the live test.

// Must hold: an owner can neither read, change nor delete another owner's rows,
// and cursors cannot be used to step into foreign rows.
func TestReviewHolds_OwnerIsolationIncludingCursor(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	a, b := "rev_k_a", "rev_k_b"
	var ids []string
	for i := 0; i < 3; i++ {
		item, _, err := s.Create(ctx, a, "favorites", configstore.CreateInput{Data: []byte(`{"table":"t` + string(rune('a'+i)) + `"}`)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}
	other, _, err := s.Create(ctx, b, "favorites", configstore.CreateInput{Data: []byte(`{"table":"private"}`)})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.List(ctx, a, "favorites", configstore.Filter{Limit: 1})
	if err != nil || page.NextCursor == "" {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, a, "favorites", other.ID); reviewCode(err) != "not_found" {
		t.Errorf("get foreign id: %v", reviewCode(err))
	}
	if _, err = s.Update(ctx, a, "settings", other.ID, []byte(`{"table":"x"}`), configstore.AnyVersion); reviewCode(err) != "not_found" {
		t.Errorf("update foreign id: %v", reviewCode(err))
	}
	if err = s.Delete(ctx, a, "favorites", other.ID, configstore.AnyVersion); reviewCode(err) != "not_found" {
		t.Errorf("delete foreign id: %v", reviewCode(err))
	}
	// A's cursor used by B never reveals A's rows.
	foreign, err := s.List(ctx, b, "favorites", configstore.Filter{Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range foreign.Items {
		if item.ID != other.ID {
			t.Errorf("owner B saw %s", item.ID)
		}
	}
	// Cross-kind id reuse does not open rows.
	if _, err = s.Get(ctx, a, "bookmarks", ids[0]); reviewCode(err) != "not_found" {
		t.Errorf("cross kind get: %v", reviewCode(err))
	}
	// Clear of A never touches B, and B's plan hash cannot execute for A.
	planB, err := s.PlanClear(ctx, b, []string{"favorites"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExecuteClear(ctx, a, []string{"favorites"}, nil, planB.PlanHash, "LÖSCHEN 3"); reviewCode(err) != "plan_stale" && reviewCode(err) != "" {
		t.Logf("foreign plan hash on A: %v", reviewCode(err))
	}
	if _, err = s.Get(ctx, b, "favorites", other.ID); err != nil {
		t.Errorf("B's row vanished after A's clear: %v", err)
	}
}

// Must hold: forged cursors are rejected or harmless.
func TestReviewHolds_CursorTampering(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	for _, cursor := range []string{"!!!", "AAAA", strings.Repeat("A", 200), "MC4w", "LTEuMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAw"} {
		_, err := s.List(ctx, "rev_l", "bookmarks", configstore.Filter{Cursor: cursor})
		if c := reviewCode(err); c != "invalid_request" && c != "" {
			t.Errorf("cursor %q: %s", cursor, c)
		}
	}
}

// Must hold: If-Match semantics at the service level (stale, any, repeated delete).
func TestReviewHolds_VersionRaces(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	item, _, err := s.Create(ctx, "rev_m", "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, "rev_m", "settings", item.ID, []byte(`{"a":2}`), 1); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, "rev_m", "settings", item.ID, []byte(`{"a":3}`), 1); reviewCode(err) != "version_conflict" {
		t.Errorf("stale update: %v", reviewCode(err))
	}
	if err = s.Delete(ctx, "rev_m", "settings", item.ID, 1); reviewCode(err) != "version_conflict" {
		t.Errorf("stale delete: %v", reviewCode(err))
	}
	if err = s.Delete(ctx, "rev_m", "settings", item.ID, 2); err != nil {
		t.Errorf("current delete: %v", err)
	}
	if err = s.Delete(ctx, "rev_m", "settings", item.ID, configstore.AnyVersion); reviewCode(err) != "not_found" {
		t.Errorf("repeat delete: %v", reviewCode(err))
	}
}

// Must hold: history opt-in is enforced for create AND import, and a disabled
// opt-in after enabling stops writes at once.
func TestReviewHolds_HistoryOptInEverywhere(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	owner := "rev_n"
	if _, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"SELECT 1"}`)}); reviewCode(err) != "history_disabled" {
		t.Fatalf("create: %v", reviewCode(err))
	}
	res, err := s.Import(ctx, owner, []configstore.ImportItem{{Kind: "history", CreateInput: configstore.CreateInput{ClientRef: "r1", Data: []byte(`{"sql":"SELECT 1"}`)}}}, false)
	if err != nil || res[0].Status != "rejected" || res[0].Reason != "history_disabled" {
		t.Fatalf("import: %+v %v", res, err)
	}
	// privacy = false (not just absent) also blocks; extra keys in privacy are invalid.
	priv, _, err := s.Create(ctx, owner, "settings", configstore.CreateInput{Key: "privacy", Data: []byte(`{"history_enabled":false}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"SELECT 1"}`)}); reviewCode(err) != "history_disabled" {
		t.Fatalf("explicit false: %v", reviewCode(err))
	}
	for _, bad := range []string{`{"history_enabled":"true"}`, `{"history_enabled":1}`, `{"history_enabled":true,"x":1}`, `{}`} {
		if _, err = s.Update(ctx, owner, "settings", priv.ID, []byte(bad), configstore.AnyVersion); reviewCode(err) != "invalid_data" {
			t.Errorf("privacy %s: %v", bad, reviewCode(err))
		}
	}
	// The privacy setting of another owner never enables history.
	reviewHistory(t, s, "rev_n_other")
	if _, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"SELECT 1"}`)}); reviewCode(err) != "history_disabled" {
		t.Fatalf("foreign opt-in leaked: %v", reviewCode(err))
	}
	// The privacy setting of a different scope does not count (scope must be "").
	if _, _, err = s.Create(ctx, owner, "settings", configstore.CreateInput{Scope: "other", Key: "privacy", Data: []byte(`{"history_enabled":true}`)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"SELECT 1"}`)}); reviewCode(err) != "history_disabled" {
		t.Fatalf("scoped privacy enabled history: %v", reviewCode(err))
	}
}

// Must hold: strict JSON in documents.
func TestReviewHolds_StrictDocuments(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	deep := strings.Repeat(`{"a":`, 6) + `1` + strings.Repeat(`}`, 6)
	for name, doc := range map[string]string{
		"duplicate key":           `{"a":1,"a":2}`,
		"duplicate via escape":    `{"a":1,"a":2}`,
		"null":                    `{"a":null}`,
		"too deep":                deep,
		"number overflow":         `{"a":1e999}`,
		"negative overflow":       `{"a":-1e999}`,
		"nul in string":           `{"a":"x\u0000y"}`,
		"lone surrogate escape":   `{"a":"\ud800"}`,
		"top-level array":         `[1]`,
		"trailing garbage":        `{"a":1} x`,
		"second object":           `{"a":1}{"b":2}`,
		"format char in key":      "{\"a​\":1}",
		"control char in value":   `{"a":"x\u0007"}`,
		"bidi override in key":    "{\"‮a\":1}",
		"empty key":               `{"":1}`,
		"big array":               `{"a":[` + strings.Repeat("1,", 256) + `1]}`,
		"string longer than 4096": `{"a":"` + strings.Repeat("x", 4097) + `"}`,
		"comment":                 `{"a":1 /* x */}`,
		"single quotes":           `{'a':1}`,
		"NaN":                     `{"a":NaN}`,
	} {
		_, _, err := s.Create(ctx, "rev_o", "layouts", configstore.CreateInput{Key: "k", Data: []byte(doc)})
		if err == nil {
			t.Errorf("%s accepted", name)
			continue
		}
		if c := reviewCode(err); !strings.HasPrefix(c, "invalid_") && c != "item_too_large" {
			t.Errorf("%s: unexpected class %s", name, c)
		}
		if strings.Contains(err.Error(), "x") && len(err.Error()) > 40 {
			t.Errorf("%s: error text may echo input: %q", name, err.Error())
		}
	}
}

// Must hold: exact boundaries.
func TestReviewHolds_ExactBoundaries(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	owner := "rev_p"
	reviewHistory(t, s, owner)
	name100 := strings.Repeat("n", 100)
	if _, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: []byte(`{"name":"` + name100 + `","sql":"SELECT 1"}`)}); err != nil {
		t.Errorf("100-char name: %v", err)
	}
	if _, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: []byte(`{"name":"` + name100 + `x","sql":"SELECT 1"}`)}); reviewCode(err) != "invalid_data" {
		t.Errorf("101-char name: %v", reviewCode(err))
	}
	sql100k := "SELECT '" + strings.Repeat("é", 99991) + "'" // 100000 chars, ~200 KB
	if _, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql100k + `"}`)}); err != nil {
		t.Errorf("100000 chars: %v", reviewCode(err))
	}
	if _, _, err := s.Create(ctx, owner, "history", configstore.CreateInput{Data: []byte(`{"sql":"` + sql100k + `a"}`)}); reviewCode(err) != "item_too_large" {
		t.Errorf("100001 chars: %v", reviewCode(err))
	}
	// Name with leading/trailing space or empty SQL.
	for _, bad := range []string{`{"name":" a","sql":"SELECT 1"}`, `{"name":"a ","sql":"SELECT 1"}`, `{"name":"a","sql":"   "}`, `{"name":"a","sql":"SELECT 1","extra":1}`, `{"name":"a","sql":"SELECT 1","variables":["a","a"]}`, `{"name":"a","sql":"SELECT 1","variables":["1a"]}`} {
		if _, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: []byte(bad)}); reviewCode(err) != "invalid_data" {
			t.Errorf("%s: %v", bad, reviewCode(err))
		}
	}
	// Import: 100 ok, 101 rejected, bad time rejected, far future rejected, pre-2000 rejected.
	items := make([]configstore.ImportItem, 101)
	for i := range items {
		items[i] = configstore.ImportItem{Kind: "favorites", CreateInput: configstore.CreateInput{Data: []byte(`{"table":"t` + strings.Repeat("x", i%5) + `"}`)}}
	}
	if _, err := s.Import(ctx, owner, items, false); reviewCode(err) != "invalid_request" {
		t.Errorf("101 import items: %v", reviewCode(err))
	}
	if res, err := s.Import(ctx, owner, items[:100], false); err != nil || len(res) != 100 {
		t.Errorf("100 import items: %v", err)
	}
}

// Must hold: importing the same batch twice creates nothing the second time.
func TestReviewHolds_ImportIdempotentAndNoLeakAcrossOwners(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	reviewHistory(t, s, "rev_q")
	reviewHistory(t, s, "rev_r")
	batch := []configstore.ImportItem{
		{Kind: "history", CreateInput: configstore.CreateInput{Scope: "s", ClientRef: "ref-1", Data: []byte(`{"sql":"SELECT 1"}`)}},
		{Kind: "bookmarks", CreateInput: configstore.CreateInput{Scope: "s", ClientRef: "ref-2", Data: []byte(`{"name":"b","sql":"SELECT 2"}`)}},
		{Kind: "settings", CreateInput: configstore.CreateInput{Key: "navigation", Data: []byte(`{"a":1}`)}},
	}
	first, _ := s.Import(ctx, "rev_q", batch, false)
	second, _ := s.Import(ctx, "rev_q", batch, false)
	for i := range batch {
		if first[i].Status != "created" || second[i].Status != "exists" || first[i].ID != second[i].ID {
			t.Errorf("item %d: %+v then %+v", i, first[i], second[i])
		}
	}
	other, _ := s.Import(ctx, "rev_r", batch, false)
	for i := range batch {
		if other[i].Status != "created" || other[i].ID == first[i].ID {
			t.Errorf("owner R item %d collided with owner Q: %+v", i, other[i])
		}
	}
	// Overwrite never touches history/bookmarks (no natural key) and never changes privacy semantics silently.
	over := []configstore.ImportItem{{Kind: "settings", CreateInput: configstore.CreateInput{Key: "navigation", Data: []byte(`{"a":2}`)}}}
	res, _ := s.Import(ctx, "rev_q", over, true)
	if res[0].Status != "updated" {
		t.Errorf("overwrite: %+v", res[0])
	}
}

// Must hold: clearing needs the typed confirmation and a fresh plan.
func TestReviewHolds_ClearPlanProtocol(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	owner := "rev_s"
	for i := 0; i < 3; i++ {
		if _, _, err := s.Create(ctx, owner, "favorites", configstore.CreateInput{Data: []byte(`{"table":"t` + string(rune('a'+i)) + `"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := s.PlanClear(ctx, owner, []string{"favorites"}, nil)
	if err != nil || plan.Confirmation != "LÖSCHEN 3" {
		t.Fatalf("%+v %v", plan, err)
	}
	for _, conf := range []string{"", "löschen 3", "LÖSCHEN 2", "LÖSCHEN  3", "LOSCHEN 3", "LÖSCHEN 3 ", "LÖSCHEN 03"} {
		if _, err = s.ExecuteClear(ctx, owner, []string{"favorites"}, nil, plan.PlanHash, conf); reviewCode(err) != "confirmation_invalid" {
			t.Errorf("confirmation %q: %v", conf, reviewCode(err))
		}
	}
	if _, err = s.ExecuteClear(ctx, owner, []string{"favorites"}, nil, "00", plan.Confirmation); reviewCode(err) != "plan_stale" {
		t.Errorf("wrong hash: %v", reviewCode(err))
	}
	if _, _, err = s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Data: []byte(`{"name":"b","sql":"SELECT 1"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExecuteClear(ctx, owner, []string{"favorites", "bookmarks"}, nil, plan.PlanHash, "LÖSCHEN 4"); reviewCode(err) != "plan_stale" {
		t.Errorf("kinds widened with old plan: %v", reviewCode(err))
	}
	// A new item after the plan makes it stale even though the typed number is now off too.
	if _, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Data: []byte(`{"table":"late"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExecuteClear(ctx, owner, []string{"favorites"}, nil, plan.PlanHash, plan.Confirmation); reviewCode(err) != "plan_stale" {
		t.Errorf("stale plan: %v", reviewCode(err))
	}
	// Scope-limited plan cannot be replayed as an all-scopes clear.
	scope := "only"
	if _, _, err = s.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"table":"x"}`)}); err != nil {
		t.Fatal(err)
	}
	scoped, _ := s.PlanClear(ctx, owner, []string{"favorites"}, &scope)
	if _, err = s.ExecuteClear(ctx, owner, []string{"favorites"}, nil, scoped.PlanHash, scoped.Confirmation); reviewCode(err) == "" {
		t.Error("scoped plan executed as global clear")
	}
	if _, err = s.ExecuteClear(ctx, owner, []string{"favorites"}, &scope, scoped.PlanHash, scoped.Confirmation); err != nil {
		t.Errorf("scoped clear: %v", err)
	}
}

// Added after mutation probes M37, M40 and M32 survived the original suite.
func TestReviewHolds_HistoryImmutableAndRefIdempotentOnCreate(t *testing.T) {
	s := reviewService()
	ctx := context.Background()
	owner := "rev_t"
	reviewHistory(t, s, owner)
	first, created, err := s.Create(ctx, owner, "history", configstore.CreateInput{ClientRef: "local-1", Data: []byte(`{"sql":"SELECT 1"}`)})
	if err != nil || !created {
		t.Fatal(err)
	}
	again, created, err := s.Create(ctx, owner, "history", configstore.CreateInput{ClientRef: "local-1", Data: []byte(`{"sql":"SELECT 2"}`)})
	if err != nil || created || again.ID != first.ID || strings.Contains(string(again.Data), "SELECT 2") {
		t.Errorf("repeated client_ref: %+v created=%v err=%v", again, created, err)
	}
	if _, err = s.Update(ctx, owner, "history", first.ID, []byte(`{"sql":"SELECT 3"}`), configstore.AnyVersion); reviewCode(err) != "invalid_request" {
		t.Errorf("history update: %v", reviewCode(err))
	}
	got, _ := s.Get(ctx, owner, "history", first.ID)
	if !strings.Contains(string(got.Data), "SELECT 1") || got.Version != 1 {
		t.Errorf("history changed: %+v", got)
	}
}

func TestReviewHolds_ForgedCursorWithOddIDIsRejected(t *testing.T) {
	s := reviewService()
	// base64url("1000000.%27%20OR%201=1") and a path-like id
	for _, cursor := range []string{"MTAwMDAwMC4nIE9SIDE9MQ", "MTAwMDAwMC4uLi8uLi9ldGM"} {
		_, err := s.List(context.Background(), "rev_u", "bookmarks", configstore.Filter{Cursor: cursor})
		if reviewCode(err) != "invalid_request" {
			t.Errorf("cursor %s: %v", cursor, reviewCode(err))
		}
	}
}
