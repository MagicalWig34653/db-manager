// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
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
func TestReviewLiveHolds_RollbackRejected(t *testing.T) {
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
	if _, err = service.Get(ctx, owner, "settings", item.ID); reviewPublic(err) != "state_unreadable" {
		t.Errorf("rolled-back blob accepted: %v", reviewPublic(err))
	}
	// Setting the version column back as well is an undetectable full rollback of that row
	// (documented residual risk, ADR 9): the row reads as its old consistent state.
	if _, err = f.root.Exec("UPDATE "+db+" SET version = 1 WHERE item_id = ?", item.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Get(ctx, owner, "settings", item.ID); err != nil || got.Version != 1 {
		t.Logf("full row rollback is now detected (%v)", reviewPublic(err))
	}
}

// F-11 (fixed): nkey and scope are keyed (HMAC) hashes with an installation key that is only stored
// sealed. The dictionary attack of the review no longer recovers anything, the scope is not the
// plain SHA-256 of the identity, and the hashes survive a rotation of the encryption key.
func TestReviewLiveHolds_NKeyDictionaryAttackFails(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	store := openStore(t, cfg)
	service := configstore.NewService(store.ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	scopeSum := sha256.Sum256([]byte("mariadb://root@localhost:3306/shop"))
	scope := hex.EncodeToString(scopeSum[:])
	item, _, err := service.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: scope, Data: []byte(`{"database":"shop","table":"customers"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Create(ctx, owner, "layouts", configstore.CreateInput{Scope: scope, Key: "table:shop.salaries", Data: []byte(`{"w":1}`)}); err != nil {
		t.Fatal(err)
	}
	var dbScope, nkey string
	if err := f.root.QueryRow("SELECT scope, nkey FROM "+cfg.Database+".config_items WHERE item_id = ?", item.ID).Scan(&dbScope, &nkey); err != nil {
		t.Fatal(err)
	}
	h := func(parts ...string) string {
		x := sha256.New()
		for _, p := range parts {
			x.Write([]byte(p))
			x.Write([]byte{0})
		}
		return hex.EncodeToString(x.Sum(nil))
	}
	if dbScope == scope || dbScope == "" {
		t.Fatalf("scope stored as the client value or its plain hash")
	}
	for _, guess := range []string{scope, h("scope", owner, scope), h("key", "favorites", scope, "shop\x1fcustomers"), h("key", "favorites", dbScope, "shop\x1fcustomers")} {
		if guess == dbScope || guess == nkey {
			t.Fatalf("dictionary attack recovers a stored hash")
		}
	}
	// Read path still works, and the same installation key serves a second replica and a rotated cipher.
	rotated := cfg
	rotated.ServerKey = bytes.Repeat([]byte{9}, KeySize)
	rs := openStore(t, rotated)
	rs.cipher, err = NewCipher(rotated.ServerKey, cfg.ServerKey)
	if err != nil {
		t.Fatal(err)
	}
	rotatedService := configstore.NewService(rs.ConfigRepo())
	page, err := rotatedService.List(ctx, owner, "favorites", configstore.Filter{Scope: &scope})
	if err != nil || len(page.Items) != 1 || page.Items[0].Scope != scope {
		t.Fatalf("hashes changed with key rotation: %v %+v", err, page)
	}
	// The index key is not in the dump in clear.
	var sealed []byte
	if err := f.root.QueryRow("SELECT sealed FROM " + cfg.Database + ".config_meta WHERE name = 'index_key'").Scan(&sealed); err != nil || len(sealed) < 32+28 {
		t.Fatalf("index key row: %v", err)
	}
}

// F-05 (fixed): a secret in scope is rejected, and the stored scope is a hash anyway.
func TestReviewLiveHolds_ScopeNeverPlaintextInTheDump(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	service := configstore.NewService(openStore(t, cfg).ConfigRepo())
	owner := repotest.Owner()
	if _, _, err := service.Create(context.Background(), owner, "favorites", configstore.CreateInput{Scope: "ghp_ABCDEFGHIJKLMNOPQRSTUVWX1234567890", Data: []byte(`{"table":"t"}`)}); reviewPublic(err) != "secret_rejected" {
		t.Fatalf("scope secret: %v", reviewPublic(err))
	}
	if _, _, err := service.Create(context.Background(), owner, "favorites", configstore.CreateInput{Scope: "myconnection", Data: []byte(`{"table":"t"}`)}); err != nil {
		t.Fatal(err)
	}
	var scope string
	if err := f.root.QueryRow("SELECT scope FROM "+cfg.Database+".config_items WHERE owner_id = ?", owner).Scan(&scope); err != nil || strings.Contains(scope, "myconnection") || len(scope) != 64 {
		t.Errorf("scope column: %q %v", scope, err)
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

// F-12 (fixed): a pre-existing, wrongly defined config_items table is not recorded as migrated.
func TestReviewLiveHolds_ForeignTableRefusedByMigration(t *testing.T) {
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
	if _, err := Migrate(ctx, cfg, MigrateOptions{}); !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("foreign table accepted: %v", err)
	}
	var recorded int
	_ = f.root.QueryRow("SELECT COUNT(*) FROM " + cfg.Database + ".schema_migrations WHERE version = 2").Scan(&recorded)
	if recorded != 0 {
		t.Fatal("version 2 recorded despite the wrong schema")
	}
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
