// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
	"github.com/MagicalWig34653/db-manager/internal/configstore/repotest"
)

// Independent review attacks against the real MariaDB state (W-P20 review,
// docs/handoffs/W-P20-backend-review.md). Run via scripts/review-w-p20-live.py.
// TestReviewLiveGap_* pin current, undesirable behaviour of a finding and must
// be flipped when the finding is fixed.

func reviewPublic(err error) string {
	var public *configstore.Error
	if errors.As(err, &public) {
		return public.Code
	}
	if err == nil {
		return ""
	}
	return "other:" + err.Error()
}

// Row swaps inside one owner (ids, kinds, scopes) must never decrypt.
func TestReviewLiveHolds_RowSwapAndKindChange(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	a, _, err := service.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "s", Data: []byte(`{"name":"a","sql":"SELECT 1"}`)})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := service.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "s", Data: []byte(`{"name":"b","sql":"SELECT 2"}`)})
	if err != nil {
		t.Fatal(err)
	}
	db := cfg.Database + ".config_items"
	// Swap the sealed blobs of two rows of the same owner/kind/scope.
	f.exec(t, "UPDATE "+db+" x JOIN "+db+" y ON x.item_id = '"+a.ID+"' AND y.item_id = '"+b.ID+"' SET x.sealed = y.sealed, y.sealed = x.sealed")
	for _, id := range []string{a.ID, b.ID} {
		if _, err = service.Get(ctx, owner, "bookmarks", id); reviewPublic(err) != "state_unreadable" {
			t.Errorf("swapped row %s served: %v", id, reviewPublic(err))
		}
	}
	// Change the kind column (bookmarks -> searches): bound by the AEAD context.
	c, _, _ := service.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "s", Data: []byte(`{"name":"c","sql":"SELECT 3"}`)})
	f.exec(t, "UPDATE "+db+" SET kind = 'searches' WHERE item_id = '"+c.ID+"'")
	if _, err = service.Get(ctx, owner, "searches", c.ID); reviewPublic(err) != "state_unreadable" {
		t.Errorf("kind-changed row served: %v", reviewPublic(err))
	}
	// Changing nkey (natural key hash) invalidates the row as well.
	d, _, _ := service.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "s", Data: []byte(`{"table":"d"}`)})
	f.exec(t, "UPDATE "+db+" SET nkey = '"+strings.Repeat("0", 64)+"' WHERE item_id = '"+d.ID+"'")
	if _, err = service.Get(ctx, owner, "favorites", d.ID); reviewPublic(err) != "state_unreadable" {
		t.Errorf("nkey-changed row served: %v", reviewPublic(err))
	}
	// One broken row makes export fail closed (no partial plaintext of other rows leaks via error text).
	if _, err = service.Export(ctx, owner); reviewPublic(err) != "state_unreadable" {
		t.Errorf("export with broken rows: %v", reviewPublic(err))
	}
}

// F-02: the AEAD context does not contain the version. An older sealed blob put
// back by a database writer decrypts, and so does a deleted row restored from a
// backup or from a copy. The plaintext columns version/updated_at are not authenticated.
func TestReviewLiveGap_RollbackAndResurrection(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	db := cfg.Database + ".config_items"
	item, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":"old"}`)})
	if err != nil {
		t.Fatal(err)
	}
	var oldSealed []byte
	if err = f.root.QueryRow("SELECT sealed FROM "+db+" WHERE item_id = ?", item.ID).Scan(&oldSealed); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Update(ctx, owner, "settings", item.ID, []byte(`{"v":"new"}`), 1); err != nil {
		t.Fatal(err)
	}
	if _, err = f.root.Exec("UPDATE "+db+" SET sealed = ? WHERE item_id = ?", oldSealed, item.ID); err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, owner, "settings", item.ID)
	if err != nil || !strings.Contains(string(got.Data), "old") || got.Version != 2 {
		t.Errorf("rollback no longer succeeds (%v, %+v); F-02 fixed, flip", reviewPublic(err), got)
	}
	// Resurrection: copy the row, delete it through the API, put the copy back.
	f.exec(t, "CREATE TABLE "+cfg.Database+".bak AS SELECT * FROM "+db+" WHERE item_id = '"+item.ID+"'")
	if err = service.Delete(ctx, owner, "settings", item.ID, configstore.AnyVersion); err != nil {
		t.Fatal(err)
	}
	f.exec(t, "INSERT INTO "+db+" SELECT * FROM "+cfg.Database+".bak")
	if _, err = service.Get(ctx, owner, "settings", item.ID); err != nil {
		t.Errorf("resurrected row no longer readable (%v); F-02 fixed, flip", reviewPublic(err))
	}
}

// F-11: nkey and scope are unsalted SHA-256 over low-entropy names. A database
// reader recovers favorite/table names and layout keys with a dictionary.
func TestReviewLiveGap_NKeyDictionaryRecoversTableNames(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	scopeIdentity := "mariadb://root@localhost:3306/shop"
	scopeSum := sha256.Sum256([]byte(scopeIdentity)) // what the web client uses as scope
	scope := hex.EncodeToString(scopeSum[:])
	if _, _, err := service.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"database":"shop","table":"customers"}`)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Create(ctx, owner, "layouts", configstore.CreateInput{Scope: scope, Key: "table:shop.salaries", Data: []byte(`{"w":1}`)}); err != nil {
		t.Fatal(err)
	}
	rows, err := f.root.Query("SELECT kind, scope, nkey FROM "+cfg.Database+".config_items WHERE owner_id = ?", owner)
	if err != nil {
		t.Fatal(err)
	}
	type dumped struct{ kind, scope, nkey string }
	var dump []dumped
	for rows.Next() {
		var d dumped
		if err = rows.Scan(&d.kind, &d.scope, &d.nkey); err != nil {
			t.Fatal(err)
		}
		dump = append(dump, d)
	}
	_ = rows.Close()
	hash := func(parts ...string) string {
		h := sha256.New()
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	// Attacker side: only the dump and wordlists. Step 1: the scope from known connection strings.
	guessedScope := ""
	for _, guess := range []string{"mariadb://root@localhost:3306/mysql", "mariadb://root@localhost:3306/shop"} {
		sum := sha256.Sum256([]byte(guess))
		if hex.EncodeToString(sum[:]) == dump[0].scope {
			guessedScope = hex.EncodeToString(sum[:])
		}
	}
	recovered := map[string]string{}
	for _, d := range dump {
		for _, table := range []string{"users", "orders", "customers", "payments"} {
			if d.kind == "favorites" && hash("key", "favorites", d.scope, "shop\x1f"+table) == d.nkey {
				recovered["favorite"] = table
			}
		}
		for _, table := range []string{"table:shop.users", "table:shop.salaries"} {
			if d.kind == "layouts" && hash("key", "layouts", d.scope, table) == d.nkey {
				recovered["layout"] = table
			}
		}
	}
	if recovered["favorite"] != "customers" || recovered["layout"] != "table:shop.salaries" || guessedScope == "" {
		t.Errorf("dictionary attack no longer works (%v, scope %q); F-11 fixed, flip", recovered, guessedScope)
	}
}

// F-05: a secret placed in scope is stored in plaintext in the scope column.
func TestReviewLiveGap_ScopeIsPlaintextInTheDump(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	marker := "ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234567890"
	if _, _, err := service.Create(context.Background(), owner, "favorites", configstore.CreateInput{Scope: marker, Data: []byte(`{"table":"t"}`)}); err != nil {
		t.Logf("scope secret rejected (%v); F-05 fixed, flip", reviewPublic(err))
		return
	}
	var scope string
	if err := f.root.QueryRow("SELECT scope FROM "+cfg.Database+".config_items WHERE owner_id = ?", owner).Scan(&scope); err != nil || scope != marker {
		t.Errorf("scope not plaintext: %q %v", scope, err)
	}
}

// Migration 2 under real conditions: aborted halfway, foreign leftovers,
// concurrent instances, outdated schema at start and old binary on new schema.
func TestReviewLiveMigration2(t *testing.T) {
	f := newFixture(t)
	cfg := f.cfg
	ctx := context.Background()
	full := migrations
	t.Cleanup(func() { migrations = full })

	// 1. Roll out only v1 (old binary), then start the new runtime on it: ErrSchemaOutdated, fail closed.
	migrations = full[:1]
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	migrations = full
	store := openStore(t, cfg)
	if err := store.WaitReady(ctx, 3*time.Second); !errors.Is(err, ErrSchemaOutdated) {
		t.Errorf("new runtime on schema v1: %v", err)
	}
	// The config repo must not be usable (no table) and must report unavailable, not leak driver text.
	service := configstore.NewService(store.ConfigRepo())
	_, err := service.Status(ctx, "single")
	if reviewPublic(err) != "state_unavailable" {
		t.Errorf("repo on missing tables: %v", reviewPublic(err))
	}

	// 2. Crash after statement 1 of 2: config_owners exists, no schema_migrations row for v2.
	f.exec(t, "CREATE TABLE "+cfg.Database+".config_owners (owner_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY, created_at DATETIME(6) NOT NULL DEFAULT UTC_TIMESTAMP(6)) ENGINE=InnoDB")
	report, err := Migrate(ctx, cfg, MigrateOptions{})
	if err != nil || len(report.Applied) != 1 || report.Applied[0] != 2 {
		t.Fatalf("resume after partial v2: %+v %v", report, err)
	}
	if err = openStore(t, cfg).WaitReady(ctx, 3*time.Second); err != nil {
		t.Errorf("not ready after resume: %v", err)
	}
	// 3. Repeated migration is a no-op.
	if report, err = Migrate(ctx, cfg, MigrateOptions{}); err != nil || len(report.Applied) != 0 {
		t.Errorf("second run: %+v %v", report, err)
	}

	// 4. Old binary (knows only v1) against schema v2: refuses (rollback of the program is blocked).
	migrations = full[:1]
	old := openStore(t, cfg)
	waitErr := old.WaitReady(ctx, 3*time.Second)
	migrations = full
	if !errors.Is(waitErr, ErrSchemaMismatch) {
		t.Errorf("old binary on v2 schema: %v (documented as blocked rollback)", waitErr)
	}
}

// F-12: a pre-existing, wrong config_items table is accepted by IF NOT EXISTS
// and recorded as migrated; the checksum covers the statement text, not the
// resulting schema.
func TestReviewLiveGap_ForeignTableAcceptedByMigration(t *testing.T) {
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
	f.exec(t, "CREATE TABLE "+cfg.Database+".config_items (owner_id VARCHAR(64) NOT NULL, kind VARCHAR(16) NOT NULL, item_id CHAR(32) NOT NULL, scope VARCHAR(128) NOT NULL, nkey CHAR(64) NOT NULL, key_id VARCHAR(40) NOT NULL, sealed MEDIUMBLOB NOT NULL, plain_size INT UNSIGNED NOT NULL, version BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL) ENGINE=InnoDB")
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); err != nil {
		t.Logf("foreign table now detected (%v); F-12 fixed, flip", err)
		return
	}
	// No PK / unique key: duplicate natural keys and duplicate ids are now possible; the quota/idempotency logic relies on the lock only.
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	item, _, err := service.Create(context.Background(), owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"a":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, "INSERT INTO "+cfg.Database+".config_items SELECT * FROM "+cfg.Database+".config_items WHERE item_id = '"+item.ID+"'")
	t.Logf("duplicate primary key accepted by the foreign table (migration recorded as applied)")
}

// Eight instances start at once on an empty database: exactly one applies each version.
func TestReviewLiveHolds_ConcurrentMigrationBothVersions(t *testing.T) {
	f := newFixture(t)
	const replicas = 8
	var wg sync.WaitGroup
	reports := make([]Report, replicas)
	errs := make([]error, replicas)
	start := make(chan struct{})
	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			reports[i], errs[i] = Migrate(context.Background(), f.cfg, MigrateOptions{LockTimeout: 30 * time.Second})
		}(i)
	}
	close(start)
	wg.Wait()
	applied := map[int]int{}
	for i := range reports {
		if errs[i] != nil {
			t.Fatalf("replica %d: %v", i, errs[i])
		}
		for _, v := range reports[i].Applied {
			applied[v]++
		}
	}
	for _, m := range migrations {
		if applied[m.Version] != 1 {
			t.Errorf("version %d applied %d times", m.Version, applied[m.Version])
		}
	}
	var n int
	if err := f.root.QueryRow("SELECT COUNT(*) FROM " + f.cfg.Database + ".schema_migrations").Scan(&n); err != nil || n != len(migrations) {
		t.Errorf("schema_migrations rows: %d %v", n, err)
	}
}

// Many owners racing on one owner row and cross-owner independence under load:
// the quota of owner A is never consumed by owner B.
func TestReviewLiveHolds_QuotaIsPerOwnerUnderLoad(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	a, b := configstore.NewService(openStore(t, cfg).ConfigRepo()), configstore.NewService(openStore(t, cfg).ConfigRepo())
	saved := configstore.Specs[3]
	configstore.Specs[3].PerOwner = 5 // favorites
	t.Cleanup(func() { configstore.Specs[3] = saved })
	ownerA, ownerB := repotest.Owner(), repotest.Owner()
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := map[string]int{}
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			owner, s := ownerA, a
			if i%2 == 1 {
				owner, s = ownerB, b
			}
			_, ok, err := s.Create(context.Background(), owner, "favorites", configstore.CreateInput{Data: []byte(`{"table":"t` + string(rune('a'+i)) + `"}`)})
			if err == nil && ok {
				mu.Lock()
				created[owner]++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if created[ownerA] != 5 || created[ownerB] != 5 {
		t.Errorf("per-owner quota: %v", created)
	}
}
