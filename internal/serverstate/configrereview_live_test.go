// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mrand "math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
	"github.com/MagicalWig34653/db-manager/internal/configstore/repotest"
)

// Independent re-review of the W-P20 follow-ups against a real MariaDB (docs/handoffs/W-P20-backend-rereview.md).
// Run via scripts/review-w-p20-live.py. Gap_ tests pin current, undesirable behaviour and must be flipped on a fix.

func rrRow(kind configstore.Kind, n int, size int, at time.Time) configstore.Row {
	return configstore.Row{
		Kind: kind, ID: fmt.Sprintf("%032x", n), Scope: strings.Repeat("a", 64), NKey: fmt.Sprintf("%064x", n),
		Data: []byte(`{"p":"` + strings.Repeat("x", size) + `"}`), CreatedAt: at, UpdatedAt: at,
	}
}

func rrIDs(t *testing.T, repo configstore.Repo, owner string) []string {
	t.Helper()
	rows, err := repo.Export(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, row := range rows {
		ids = append(ids, string(row.Kind)+":"+row.ID[24:])
	}
	sort.Strings(ids)
	return ids
}

func rrDetail(repo configstore.Repo, owner string) string {
	rows, _ := repo.Export(context.Background(), owner)
	var out []string
	for _, row := range rows {
		out = append(out, fmt.Sprintf("%s:%s(%d,%s,%s)", row.Kind, row.ID[28:], len(row.Data), row.Scope[62:], row.UpdatedAt.Format("05")))
	}
	return strings.Join(out, " ")
}

// (2) First start of many replicas at once: exactly one installation key wins and every replica uses it.
func TestRereviewLiveHolds_IndexKeyRaceHasOneWinner(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	const replicas = 16
	keys := make([][]byte, replicas)
	errs := make([]error, replicas)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < replicas; i++ {
		store := openStore(t, cfg)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			keys[i], errs[i] = store.ConfigRepo().IndexKey(context.Background())
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range keys {
		if errs[i] != nil || len(keys[i]) != 32 || !bytes.Equal(keys[i], keys[0]) {
			t.Fatalf("replica %d: %v len=%d equal=%v", i, errs[i], len(keys[i]), bytes.Equal(keys[i], keys[0]))
		}
	}
	var rows int
	var sealed []byte
	if err := f.root.QueryRow("SELECT COUNT(*) FROM " + cfg.Database + ".config_meta").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("config_meta rows: %d %v", rows, err)
	}
	if err := f.root.QueryRow("SELECT sealed FROM " + cfg.Database + ".config_meta WHERE name = 'index_key'").Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, keys[0]) || bytes.Equal(keys[0], make([]byte, 32)) {
		t.Fatal("index key in clear or all zero")
	}
}

// (2) Key material comes from the CSPRNG: independent installations get different keys.
func TestRereviewLiveHolds_IndexKeysDifferPerInstallation(t *testing.T) {
	a, b := newFixture(t), newFixture(t)
	ka, err := openStore(t, a.migrated(t)).ConfigRepo().IndexKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kb, err := openStore(t, b.migrated(t)).ConfigRepo().IndexKey(context.Background())
	if err != nil || bytes.Equal(ka, kb) {
		t.Fatalf("keys equal or failed: %v", err)
	}
}

// (2) A tampered or foreign index_key row fails closed. A deleted row is regenerated (Gap below).
func TestRereviewLiveHolds_IndexKeyTamperFailsClosed(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	scope := strings.Repeat("c", 64)
	item, _, err := service.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"table":"t"}`)})
	if err != nil {
		t.Fatal(err)
	}
	meta, items := cfg.Database+".config_meta", cfg.Database+".config_items"
	var original, itemSealed []byte
	if err := f.root.QueryRow("SELECT sealed FROM " + meta).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err := f.root.QueryRow("SELECT sealed FROM "+items+" WHERE item_id = ?", item.ID).Scan(&itemSealed); err != nil {
		t.Fatal(err)
	}
	garbage := make([]byte, 80)
	_, _ = rand.Read(garbage)
	for name, blob := range map[string][]byte{"random": garbage, "item blob as key": itemSealed, "truncated": original[:20], "empty": {}} {
		f.exec(t, "UPDATE "+meta+" SET sealed = X'"+fmt.Sprintf("%x", blob)+"'")
		fresh := configstore.NewService(openStore(t, cfg).ConfigRepo())
		if _, err := fresh.List(ctx, owner, "favorites", configstore.Filter{Scope: &scope}); reviewPublic(err) != "state_unreadable" {
			t.Errorf("%s: %v", name, reviewPublic(err))
		}
		var after []byte
		_ = f.root.QueryRow("SELECT sealed FROM " + meta).Scan(&after)
		if !bytes.Equal(after, blob) {
			t.Errorf("%s: a bad key row was silently replaced", name)
		}
	}
	// The index key blob placed into an item row never opens (context binding in the other direction).
	f.exec(t, "UPDATE "+meta+" SET sealed = X'"+fmt.Sprintf("%x", original)+"'")
	f.exec(t, "UPDATE "+items+" SET sealed = X'"+fmt.Sprintf("%x", original)+"' WHERE item_id = '"+item.ID+"'")
	if _, err := service.Get(ctx, owner, "favorites", item.ID); reviewPublic(err) != "state_unreadable" {
		t.Errorf("index key blob served as item: %v", reviewPublic(err))
	}
}

// (2) Fixed (L-05): deleting config_meta while rows exist never mints a new key; the store answers
// state_unreadable instead of silently orphaning the rows. With no rows a fresh key is created.
func TestRereviewLiveHolds_DeletedIndexKeyFailsClosed(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	owner := repotest.Owner()
	scope := strings.Repeat("d", 64)
	first := configstore.NewService(openStore(t, cfg).ConfigRepo())
	if _, _, err := first.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"table":"t"}`)}); err != nil {
		t.Fatal(err)
	}
	f.exec(t, "DELETE FROM "+cfg.Database+".config_meta")
	second := configstore.NewService(openStore(t, cfg).ConfigRepo())
	if _, err := second.List(ctx, owner, "favorites", configstore.Filter{Scope: &scope}); reviewPublic(err) != "state_unreadable" {
		t.Fatalf("list after key loss: %v", reviewPublic(err))
	}
	if _, _, err := second.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"table":"t"}`)}); reviewPublic(err) != "state_unreadable" {
		t.Fatalf("create after key loss: %v", reviewPublic(err))
	}
	var metaRows int
	_ = f.root.QueryRow("SELECT COUNT(*) FROM " + cfg.Database + ".config_meta").Scan(&metaRows)
	if metaRows != 0 {
		t.Fatal("a new installation key was minted next to existing rows")
	}
	// An empty installation (no rows) just starts again with a new key.
	f.exec(t, "DELETE FROM "+cfg.Database+".config_items")
	third := configstore.NewService(openStore(t, cfg).ConfigRepo())
	if _, _, err := third.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"table":"t"}`)}); err != nil {
		t.Fatalf("empty store after meta loss: %v", err)
	}
}

// (2) Gap: the index key row is never re-sealed. After a rotation it keeps the old key id, so the
// old key must stay configured for good even when every item has been re-sealed (documented in
// BETRIEB.md, but there is no re-wrap path).
func TestRereviewLiveGap_RotationNeedsTheOldKeyForever(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	owner := repotest.Owner()
	old := openStore(t, cfg)
	service := configstore.NewService(old.ConfigRepo())
	item, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	rotated := cfg
	rotated.ServerKey = bytes.Repeat([]byte{9}, KeySize)
	rs := openStore(t, rotated)
	var err2 error
	if rs.cipher, err2 = NewCipher(rotated.ServerKey, cfg.ServerKey); err2 != nil {
		t.Fatal(err2)
	}
	rsvc := configstore.NewService(rs.ConfigRepo())
	if _, err := rsvc.Update(ctx, owner, "settings", item.ID, []byte(`{"v":2}`), 1); err != nil {
		t.Fatal(err)
	}
	var itemKey, metaKey string
	_ = f.root.QueryRow("SELECT key_id FROM " + cfg.Database + ".config_items").Scan(&itemKey)
	_ = f.root.QueryRow("SELECT key_id FROM " + cfg.Database + ".config_meta").Scan(&metaKey)
	if itemKey == metaKey {
		t.Fatalf("index key re-sealed under the new key: the rotation gap is fixed, flip this test")
	}
	// Old key dropped after "all rows are re-sealed": everything is unreadable.
	only := openStore(t, rotated)
	scope := strings.Repeat("e", 64)
	if _, err := configstore.NewService(only.ConfigRepo()).List(ctx, owner, "settings", configstore.Filter{Scope: &scope}); reviewPublic(err) != "state_unreadable" {
		t.Errorf("rotation without the old key: %v", reviewPublic(err))
	}
}

// (1) Versions: tampering with the unauthenticated version column is detected (DoS only), the
// seal and version change atomically under concurrency, and the version cannot wrap or go negative.
func TestRereviewLiveHolds_VersionColumnTamperAndConcurrency(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	items := cfg.Database + ".config_items"
	item, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":0}`)})
	if err != nil {
		t.Fatal(err)
	}
	// 16 concurrent unconditional updates: every update lands, the row stays readable, version == 1 + 16.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := service.Update(ctx, owner, "settings", item.ID, []byte(fmt.Sprintf(`{"v":%d}`, i+1)), configstore.AnyVersion); err != nil {
				t.Errorf("update %d: %v", i, reviewPublic(err))
			}
		}(i)
	}
	wg.Wait()
	got, err := service.Get(ctx, owner, "settings", item.ID)
	if err != nil || got.Version != 17 {
		t.Fatalf("after concurrent updates: %+v %v", got, reviewPublic(err))
	}
	for name, value := range map[string]string{"plus one": "version + 1", "minus one": "version - 1", "zero": "0", "2^63": "9223372036854775808", "max uint64": "18446744073709551615"} {
		f.exec(t, "UPDATE "+items+" SET version = 17 WHERE item_id = '"+item.ID+"'")
		f.exec(t, "UPDATE "+items+" SET version = "+value+" WHERE item_id = '"+item.ID+"'")
		if _, err := service.Get(ctx, owner, "settings", item.ID); reviewPublic(err) != "state_unreadable" {
			t.Errorf("version %s: %v", name, reviewPublic(err))
		}
		if _, err := service.Update(ctx, owner, "settings", item.ID, []byte(`{"v":99}`), configstore.AnyVersion); err == nil {
			t.Errorf("update on a tampered version (%s) succeeded", name)
		}
	}
}

// (1) Gap (documented residual risk, ADR 9): the seal and version column rolled back together, or a
// deleted row re-inserted from a copy, read as a consistent old state. The version in the context
// only stops rolling back the seal alone.
func TestRereviewLiveGap_PairwiseRollbackAndResurrection(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	items := cfg.Database + ".config_items"
	item, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":"old"}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, "CREATE TABLE "+cfg.Database+".copy_items AS SELECT * FROM "+items)
	var oldSealed []byte
	_ = f.root.QueryRow("SELECT sealed FROM "+items+" WHERE item_id = ?", item.ID).Scan(&oldSealed)
	if _, err := service.Update(ctx, owner, "settings", item.ID, []byte(`{"v":"new"}`), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.root.Exec("UPDATE "+items+" SET sealed = ?, version = 1 WHERE item_id = ?", oldSealed, item.ID); err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, owner, "settings", item.ID)
	if err != nil || !strings.Contains(string(got.Data), "old") || got.Version != 1 {
		t.Fatalf("pairwise rollback is now detected (%v): flip this test", reviewPublic(err))
	}
	if err := service.Delete(ctx, owner, "settings", item.ID, 1); err != nil {
		t.Fatal(err)
	}
	f.exec(t, "INSERT INTO "+items+" SELECT * FROM "+cfg.Database+".copy_items")
	if _, err := service.Get(ctx, owner, "settings", item.ID); err != nil {
		t.Fatalf("resurrection is now detected (%v): flip this test", reviewPublic(err))
	}
	// An older version for a new write is never reused: the next update continues from the restored version.
	if again, err := service.Update(ctx, owner, "settings", item.ID, []byte(`{"v":"again"}`), 1); err != nil || again.Version != 2 {
		t.Fatalf("update after resurrection: %v", reviewPublic(err))
	}
}

// (1) AEAD context: injective by construction (length-prefixed fields), also for fields that
// contain the old separator.
func TestRereviewLive_ItemContextInjectivity(t *testing.T) {
	rng := mrand.New(mrand.NewSource(7))
	kinds := []configstore.Kind{"history", "bookmarks", "searches", "favorites", "recents", "settings", "layouts"}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-|"
	str := func(n int) string {
		b := make([]byte, 1+rng.Intn(n))
		for i := range b {
			b[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(b)
	}
	seen := map[string]string{}
	for i := 0; i < 50000; i++ {
		o, k, id, sc, nk, v := str(64), kinds[rng.Intn(len(kinds))], str(32), str(64), str(64), int64(1+rng.Intn(5))
		ctx := string(itemContext(o, k, id, sc, nk, v))
		tuple := fmt.Sprintf("%q", []any{o, k, id, sc, nk, v})
		if prev, ok := seen[ctx]; ok && prev != tuple {
			t.Fatalf("context collision: %q vs %q", prev, tuple)
		}
		seen[ctx] = tuple
	}
	a := itemContext("x|settings", "layouts", strings.Repeat("1", 32), "s", "n", 1)
	b := itemContext("x", "settings", "layouts|"+strings.Repeat("1", 32), "s", "n", 1)
	if bytes.Equal(a, b) {
		t.Fatal("separator ambiguity")
	}
	if bytes.Equal(itemContext("o", "settings", "i", "s", "n", 1), []byte("cfg2|meta|index_key")) {
		t.Fatal("item context equals the index key context")
	}
}

type rrOp struct {
	kind      configstore.Kind
	size      int
	scope     string
	fifo      bool
	perScope  int
	perOwner  int
	ownerSize int64
}

// (5) Differential test: memory and MariaDB repositories run the same random insert sequences
// (FIFO, per-scope and per-owner counts, byte quota with history/recents eviction) and must keep
// exactly the same rows and return the same error classes.
func TestRereviewLiveHolds_QuotaDifferentialMemoryVsMariaDB(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	maria := openStore(t, cfg).ConfigRepo()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for seed := int64(1); seed <= 6; seed++ {
		rng := mrand.New(mrand.NewSource(seed))
		mem := configstore.NewMemoryRepo()
		owner := repotest.Owner()
		kinds := []configstore.Kind{"history", "recents", "bookmarks", "favorites"}
		for step := 0; step < 120; step++ {
			kind := kinds[rng.Intn(len(kinds))]
			fifo := kind == "history" || kind == "recents"
			lim := configstore.Limits{PerScope: 3, PerOwner: 5, FIFO: fifo, OwnerBytes: 300}
			row := rrRow(kind, int(seed)*10000+step, 5+rng.Intn(70), base.Add(time.Duration(step)*time.Second))
			row.Scope = fmt.Sprintf("%064x", rng.Intn(2))
			before := rrDetail(maria, owner)
			_, errMem := mem.Insert(ctx, owner, row, lim)
			_, errMaria := maria.Insert(ctx, owner, row, lim)
			if fmt.Sprint(errMem) != fmt.Sprint(errMaria) {
				t.Fatalf("seed %d step %d: memory %v, mariadb %v", seed, step, errMem, errMaria)
			}
			if errMaria != nil && !errors.Is(errMaria, configstore.RepoQuota) {
				t.Fatalf("seed %d step %d: %v", seed, step, errMaria)
			}
			if errMaria != nil {
				// The memory repo evicts before failing (see the gap test); resynchronise it from MariaDB.
				mem = configstore.NewMemoryRepo()
				rows, err := maria.Export(ctx, owner)
				if err != nil {
					t.Fatal(err)
				}
				for _, kept := range rows {
					if _, err := mem.Insert(ctx, owner, kept, configstore.Limits{PerScope: 1000, PerOwner: 1000, OwnerBytes: 1 << 30}); err != nil {
						t.Fatal(err)
					}
				}
				continue
			}
			if a, b := rrIDs(t, mem, owner), rrIDs(t, maria, owner); fmt.Sprint(a) != fmt.Sprint(b) {
				t.Fatalf("seed %d step %d (insert %s id %s size %d scope %s): survivors differ\nmemory  %v\nmariadb %v\nbefore  %v\nmem detail %s\nmaria detail %s", seed, step, kind, row.ID[24:], len(row.Data), row.Scope[62:], a, b, before, rrDetail(mem, owner), rrDetail(maria, owner))
			}
		}
	}
}

// (5) Fixed: after a quota failure both repositories keep all rows.
func TestRereviewLive_QuotaFailureIsAtomicOnBothRepos(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	maria := openStore(t, cfg).ConfigRepo()
	mem := configstore.NewMemoryRepo()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	big := configstore.Limits{PerScope: 5, PerOwner: 10, OwnerBytes: 100}
	fifo := configstore.Limits{PerScope: 2, PerOwner: 10, FIFO: true, OwnerBytes: 100}
	for name, repo := range map[string]configstore.Repo{"mariadb": maria, "memory": mem} {
		owner := repotest.Owner()
		for i, step := range []struct {
			row configstore.Row
			lim configstore.Limits
		}{{rrRow("bookmarks", 1, 40, base), big}, {rrRow("history", 2, 5, base.Add(time.Second)), fifo}, {rrRow("history", 3, 5, base.Add(2*time.Second)), fifo}} {
			if _, err := repo.Insert(ctx, owner, step.row, step.lim); err != nil {
				t.Fatalf("%s step %d: %v", name, i, err)
			}
		}
		if _, err := repo.Insert(ctx, owner, rrRow("history", 4, 50, base.Add(3*time.Second)), fifo); !errors.Is(err, configstore.RepoQuota) {
			t.Fatalf("%s: want quota error, got %v", name, err)
		}
		if left := len(rrIDs(t, repo, owner)); left != 3 {
			t.Errorf("%s lost rows on a failed insert: %d of 3", name, left)
		}
	}
}

func TestRereviewLiveHolds_ByteQuotaConcurrentWriters(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	ctx := context.Background()
	a, b := openStore(t, cfg).ConfigRepo(), openStore(t, cfg).ConfigRepo()
	owner := repotest.Owner()
	lim := configstore.Limits{PerScope: 1000, PerOwner: 1000, OwnerBytes: 1000}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo := a
			if i%2 == 1 {
				repo = b
			}
			_, err := repo.Insert(ctx, owner, rrRow("bookmarks", 500+i, 80, base.Add(time.Duration(i)*time.Second)), lim)
			if err != nil && !errors.Is(err, configstore.RepoQuota) {
				t.Errorf("insert %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	usage, err := a.Usage(ctx, owner)
	if err != nil || usage.Bytes["bookmarks"] > 1000 || usage.Counts["bookmarks"] == 0 {
		t.Fatalf("usage %+v %v", usage, err)
	}
	t.Logf("kept %d rows, %d bytes", usage.Counts["bookmarks"], usage.Bytes["bookmarks"])
}

// (5, F-12) Fixed (L-04): pre-existing tables with the right keys but different columns, types or
// collation are refused and not recorded as version 2.
func TestRereviewLiveHolds_MigrationVerifyChecksColumns(t *testing.T) {
	f := newFixture(t)
	cfg := f.cfg
	ctx := context.Background()
	full := migrations
	t.Cleanup(func() { migrations = full })
	migrations = full[:1]
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	migrations = full
	f.exec(t, "CREATE TABLE "+cfg.Database+".config_owners (owner_id VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL PRIMARY KEY, created_at DATETIME(6) NOT NULL) ENGINE=InnoDB")
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("weak pre-existing tables accepted: %v", err)
	}
	var recorded int
	_ = f.root.QueryRow("SELECT COUNT(*) FROM " + cfg.Database + ".schema_migrations WHERE version = 2").Scan(&recorded)
	if recorded != 0 {
		t.Fatal("version 2 recorded")
	}
}

// (5, F-12) Migration 2 with config_meta under parallel start from schema 1, then a repeat run: one
// version row, identical schema, and a second run is a no-op.
func TestRereviewLiveHolds_ParallelStartCreatesMetaOnce(t *testing.T) {
	f := newFixture(t)
	cfg := f.cfg
	ctx := context.Background()
	full := migrations
	t.Cleanup(func() { migrations = full })
	migrations = full[:1]
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	migrations = full
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = Migrate(ctx, cfg, MigrateOptions{})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("replica %d: %v", i, err)
		}
	}
	var n int
	if err := f.root.QueryRow("SELECT COUNT(*) FROM " + cfg.Database + ".schema_migrations WHERE version = 2").Scan(&n); err != nil || n != 1 {
		t.Fatalf("version 2 rows: %d %v", n, err)
	}
	report, err := Migrate(ctx, cfg, MigrateOptions{})
	if err != nil || len(report.Applied) != 0 {
		t.Fatalf("repeat run: %+v %v", report, err)
	}
	if _, err := openStore(t, cfg).ConfigRepo().IndexKey(ctx); err != nil {
		t.Fatal(err)
	}
}
