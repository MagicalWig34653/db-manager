// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"
)

// Independent re-review of the W-P20 follow-ups (docs/handoffs/W-P20-backend-rereview.md).
// Holds_ tests must stay green. Gap_ tests pin current, undesirable behaviour of a finding
// and must be flipped when the finding is fixed.

func rrCode(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return "other:" + err.Error()
}

func rrService() *Service { return NewService(NewMemoryRepo()) }

// Folding only affects scanning: the stored value is exactly what the client sent.
func TestRereviewHolds_FoldingDoesNotChangeStoredValue(t *testing.T) {
	s, owner := rrService(), "rr_a"
	ctx := context.Background()
	for _, name := range []string{"Zürich\u200d", "e\u0301clair", "Заказы", "Αθήνα", "пользователи", "Σύνολο", "tok\u00ades", "ﬁle"} {
		data := []byte(`{"name":"` + name + `","sql":"SELECT 1"}`)
		item, _, err := s.Create(ctx, owner, "bookmarks", CreateInput{Data: data})
		if err != nil {
			t.Errorf("legitimate name %q rejected: %v", name, rrCode(err))
			continue
		}
		got, err := s.Get(ctx, owner, "bookmarks", item.ID)
		if err != nil || !strings.Contains(string(got.Data), name) {
			t.Errorf("stored value of %q changed: %s %v", name, got.Data, err)
		}
	}
}

// Confusables: every table entry maps to the ASCII letter it is documented for, and
// no entry maps an ASCII letter to another one (that would create false positives).
func TestRereviewHolds_ConfusableTableIsSane(t *testing.T) {
	for from, to := range confusables {
		if from < 0x80 {
			t.Errorf("ASCII %q mapped", from)
		}
		if !((to >= 'a' && to <= 'z') || (to >= 'A' && to <= 'Z')) {
			t.Errorf("%q maps to non-letter %q", from, to)
		}
	}
	// Every substitution of one letter by a table entry must still be rejected.
	for _, word := range []string{"password", "secret", "token", "bearer"} {
		for from, to := range confusables {
			if fold(string(from)) != string(to) {
				continue // dead entry, see TestRereviewGap_DeadConfusableEntry
			}
			for i, letter := range word {
				if letter != to {
					continue
				}
				mutated := word[:i] + string(from) + word[i+1:]
				if !LooksSecret("-- "+mutated, ScanSQL) {
					t.Errorf("%q (%U for %q) not rejected", mutated, from, letter)
				}
			}
		}
	}
}

// Fixed (L-02): the table also runs before NFKC, so U+03F2 works.
func TestRereviewHolds_FormerlyDeadConfusableEntry(t *testing.T) {
	if fold("ϲ") != "c" || !LooksSecret("-- seϲret", ScanSQL) {
		t.Fatal("U+03F2 is not folded")
	}
}

// Fixed (L-02): small caps and Armenian look-alikes are folded.
func TestRereviewHolds_ConfusablesOutsideTheOldTable(t *testing.T) {
	for _, text := range []string{"SET ᴘᴀꜱꜱᴡᴏʀᴅ = 'x'", "-- tokeո abc"} {
		if !LooksSecret(text, ScanSQL) {
			t.Errorf("%q accepted", text)
		}
	}
}

// Fixed (L-03): redis URLs with an empty user and Go DSNs are rejected.
func TestRereviewHolds_StructuralFormsRejected(t *testing.T) {
	for _, text := range []string{"redis://:hunter2hunter2@cache:6379/0", "user:hunter2@tcp(db:3306)/app"} {
		if !LooksSecret(text, ScanStructural) {
			t.Errorf("%q accepted", text)
		}
	}
}

// Gap: a string under a nested object below a credential-named key is not rejected, nor are
// short credential key names such as pw/auth, nor numeric values.
func TestRereviewHolds_DocumentKeyRules(t *testing.T) {
	s, owner := rrService(), "rr_b"
	for i, doc := range []string{
		`{"password":{"value":"hunter2hunter"}}`,
		`{"pw":"hunter2hunter"}`,
		`{"auth":"hunter2hunter"}`,
		`{"creds":{"token":{"a":["hunter2hunter"]}}}`,
	} {
		_, _, err := s.Create(context.Background(), owner, "layouts", CreateInput{Key: "k" + string(rune('a'+i)), Data: []byte(doc)})
		if rrCode(err) != "secret_rejected" {
			t.Errorf("%s: %s", doc, rrCode(err))
		}
	}
	// Direct string, array and unicode-escaped key forms are rejected (must hold).
	for _, doc := range []string{`{"password":"hunter2hunter"}`, `{"list":[{"password":"hunter2"}]}`, `{"p\u0061ssword":"hunter2hunter"}`, `{"Passwоrd":"hunter2hunter"}`} {
		if _, _, err := s.Create(context.Background(), owner, "layouts", CreateInput{Key: "z" + doc[2:4], Data: []byte(doc)}); rrCode(err) != "secret_rejected" {
			t.Errorf("%s: %s", doc, rrCode(err))
		}
	}
}

// No ReDoS: RE2 is linear; pathological inputs scan in well under the budget. Default size is 128 KiB (SQL limit 256 KiB);
// DBM_REVIEW_FULL=1 uses 1 MiB (request body limit), about 0.1-0.8 s per mode without -race.
func TestRereviewHolds_PathologicalInputsStayLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	size := 128 << 10
	if os.Getenv("DBM_REVIEW_FULL") != "" {
		size = 1 << 20
	}
	rep := func(unit string) string { return strings.Repeat(unit, size/len(unit)) }
	cases := map[string]string{
		"a":       rep("a"),
		"colon":   "http://" + rep("a:"),
		"curl -u": "curl " + rep(" -u"),
		"basic":   rep("basic "),
		"pass":    rep("pass "),
		"scheme":  rep("a://b:"),
		"run":     rep("Ab1"),
		"cf":      rep("p\u200b"),
		"mn":      rep("a\u0301"),
		"nfkc":    rep("ﬃ"),
		"nl curl": rep("curl x\n"),
	}
	for name, text := range cases {
		for _, mode := range []ScanMode{ScanSQL, ScanStructural} {
			start := time.Now()
			LooksSecret(text, mode)
			if took := time.Since(start); took > 20*time.Second {
				t.Errorf("%s mode %d: %v for %d bytes", name, mode, took, len(text))
			}
		}
	}
}

// The index hashes never look like the raw or plain-SHA values, are salted per owner and
// differ between installations (different index keys).
func TestRereviewHolds_KeyedScopeAndNKey(t *testing.T) {
	ctx := context.Background()
	repoA, repoB := NewMemoryRepo(), NewMemoryRepo()
	a, b := NewService(repoA), NewService(repoB)
	identity := sha256.Sum256([]byte("mariadb://root@localhost:3306/shop"))
	scope := hex.EncodeToString(identity[:])
	plain := sha256.Sum256([]byte(scope))
	hashes := map[string]string{}
	for label, tc := range map[string]struct {
		s     *Service
		repo  *MemoryRepo
		owner string
	}{"A/alice": {a, repoA, "alice"}, "A/bob": {a, repoA, "bob"}, "B/alice": {b, repoB, "alice"}} {
		if _, _, err := tc.s.Create(ctx, tc.owner, "favorites", CreateInput{Scope: scope, Data: []byte(`{"database":"shop","table":"customers"}`)}); err != nil {
			t.Fatal(err)
		}
		rows, err := tc.repo.Export(ctx, tc.owner)
		if err != nil || len(rows) != 1 {
			t.Fatalf("export: %v %d", err, len(rows))
		}
		if rows[0].Scope == scope || rows[0].Scope == hex.EncodeToString(plain[:]) || len(rows[0].Scope) != 64 || len(rows[0].NKey) != 64 {
			t.Errorf("%s: scope/nkey not keyed: %q %q", label, rows[0].Scope, rows[0].NKey)
		}
		hashes[label+"/scope"], hashes[label+"/nkey"] = rows[0].Scope, rows[0].NKey
	}
	for _, field := range []string{"scope", "nkey"} {
		if hashes["A/alice/"+field] == hashes["A/bob/"+field] {
			t.Errorf("%s hash equal across owners (scope must be salted per owner)", field)
		}
		if hashes["A/alice/"+field] == hashes["B/alice/"+field] {
			t.Errorf("%s hash equal across installations", field)
		}
	}
}

// The MAC joins its parts with NUL: this is only injective because NUL is rejected in
// scopes, keys and client references (validated here), so the rule must stay.
func TestRereviewHolds_NULNeverReachesTheMAC(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	if macHex(key, "a", "b") != macHex(key, "a\x00b") {
		t.Fatal("macHex became injective by itself: simplify the NUL rule")
	}
	s, owner := rrService(), "rr_c"
	ctx := context.Background()
	for name, in := range map[string]CreateInput{
		"scope":      {Scope: "a\x00b", Data: []byte(`{"table":"t"}`)},
		"key":        {Key: "a\x00b", Data: []byte(`{"v":1}`)},
		"client_ref": {ClientRef: "a\x00b", Data: []byte(`{"table":"t"}`)},
	} {
		kind := "layouts"
		if name != "key" {
			kind = "favorites"
		}
		if _, _, err := s.Create(ctx, owner, kind, in); rrCode(err) == "" {
			t.Errorf("NUL in %s accepted", name)
		}
	}
}

// Owner identifiers outside [A-Za-z0-9_-]{1,64} (in particular '|', the AEAD context separator) never reach a Repo.
func TestRereviewHolds_OwnerGrammar(t *testing.T) {
	s := rrService()
	for _, owner := range []string{"", "a|b", "a b", "a\x00", "a\n", strings.Repeat("a", 65), "é", "a/../b", "cfg2|meta"} {
		if _, _, err := s.Create(context.Background(), owner, "favorites", CreateInput{Data: []byte(`{"table":"t"}`)}); rrCode(err) != "invalid_request" {
			t.Errorf("owner %q: %s", owner, rrCode(err))
		}
	}
}

// Gap (memory repo only): a failed insert evicts rows before it reports the quota error,
// whereas the MariaDB repository rolls the evictions back. Compared live in
// internal/serverstate (TestRereviewLive_QuotaFailureIsAtomicOnlyOnMariaDB).
func TestRereviewHolds_MemoryRepoQuotaFailureIsAtomic(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepo()
	lim := Limits{PerScope: 2, PerOwner: 10, FIFO: true, OwnerBytes: 100}
	big := Limits{PerScope: 5, PerOwner: 10, OwnerBytes: 100}
	row := func(kind Kind, id string, nkey string, size int, at int) Row {
		stamp := time.Date(2026, 1, 1, 0, 0, at, 0, time.UTC)
		return Row{Kind: kind, ID: strings.Repeat(id, 32), Scope: "s", NKey: strings.Repeat(nkey, 64), Data: []byte(strings.Repeat("x", size)), CreatedAt: stamp, UpdatedAt: stamp}
	}
	for _, r := range []struct {
		row Row
		lim Limits
	}{{row(KindBookmarks, "b", "b", 60, 1), big}, {row(KindHistory, "1", "1", 10, 2), lim}, {row(KindHistory, "2", "2", 10, 3), lim}} {
		if _, err := repo.Insert(ctx, "o", r.row, r.lim); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Insert(ctx, "o", row(KindHistory, "3", "3", 50, 4), lim); err == nil {
		t.Fatal("expected a quota error")
	}
	rows, _ := repo.Export(ctx, "o")
	if len(rows) != 3 {
		t.Fatalf("memory repo kept %d of 3 rows after the failed insert", len(rows))
	}
}

// Must hold (kills the "basic auth rule off" mutation): short Basic credentials and the curl/wget
// forms are rejected in documents as well as in SQL.
func TestRereviewHolds_BasicAuthAndCommandLineForms(t *testing.T) {
	for _, text := range []string{
		"Authorization: Basic YWRtaW46aHVudGVyMg==",
		"authorization:basic dXNlcjpwdw==",
		"curl -u admin:hunter2 https://x",
		"wget --user=admin --password=hunter2 https://x",
		"curl --user admin:hunter2 https://x",
		"Server=h;Uid=app;Pwd=hunter2;",
		"jdbc:mysql://h/db?user=a&password=hunter2",
	} {
		for _, mode := range []ScanMode{ScanSQL, ScanStructural} {
			if !LooksSecret(text, mode) {
				t.Errorf("mode %d accepts %q", mode, text)
			}
		}
	}
}
