// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
	"github.com/MagicalWig34653/db-manager/internal/configstore/repotest"
)

// These tests need the disposable TLS MariaDB of scripts/test-config-store.py
// (or test-server-state.py) and are skipped without DBM_TEST_STATE_PORT.

func TestConfigRepoConformanceOnMariaDB(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	store := openStore(t, cfg)
	repotest.Run(t, func(t *testing.T) *configstore.Service {
		return configstore.NewService(store.ConfigRepo())
	})
}

func TestConfigRepoTwoReplicasShareOneState(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	a, b := openStore(t, cfg), openStore(t, cfg)
	sa, sb := configstore.NewService(a.ConfigRepo()), configstore.NewService(b.ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	item, _, err := sa.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := sb.Get(ctx, owner, "settings", item.ID)
	if err != nil || string(got.Data) != `{"v":1}` {
		t.Fatalf("second replica does not see the first: %v", err)
	}
	// Writers on two replicas race for one version: exactly one wins.
	var wg sync.WaitGroup
	var wins atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := sa
			if i%2 == 1 {
				s = sb
			}
			if _, err := s.Update(ctx, owner, "settings", item.ID, []byte(`{"v":2}`), 1); err == nil {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d replicas won the same version", wins.Load())
	}
	// Quota stays exact across replicas.
	saved := configstore.Specs[1]
	configstore.Specs[1].PerScope = 6
	t.Cleanup(func() { configstore.Specs[1] = saved })
	var created atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := sa
			if i%2 == 1 {
				s = sb
			}
			if _, _, err := s.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "q", Data: []byte(`{"name":"b","sql":"SELECT ` + string(rune('a'+i)) + `"}`)}); err == nil {
				created.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if created.Load() != 6 {
		t.Fatalf("cross-replica quota not exact: %d", created.Load())
	}
}

func TestConfigRowsAreEncryptedAndBoundToTheirOwner(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	store := openStore(t, cfg)
	service := configstore.NewService(store.ConfigRepo())
	ctx := context.Background()
	owner, other := repotest.Owner(), repotest.Owner()
	marker := "marker_" + randomHex(t, 6)
	item, _, err := service.Create(ctx, owner, "bookmarks", configstore.CreateInput{Scope: "sc", Data: []byte(`{"name":"` + marker + `","sql":"SELECT '` + marker + `'"}`)})
	if err != nil {
		t.Fatal(err)
	}
	fav, _, err := service.Create(ctx, owner, "favorites", configstore.CreateInput{Scope: "sc", Data: []byte(`{"database":"d","table":"` + marker + `"}`)})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.root.Query("SELECT owner_id, kind, item_id, scope, nkey, key_id, sealed FROM " + cfg.Database + ".config_items")
	if err != nil {
		t.Fatal(err)
	}
	var dump bytes.Buffer
	n := 0
	for rows.Next() {
		var owner, kind, id, scope, nkey, keyID string
		var sealed []byte
		if err := rows.Scan(&owner, &kind, &id, &scope, &nkey, &keyID, &sealed); err != nil {
			t.Fatal(err)
		}
		dump.WriteString(owner + kind + id + scope + nkey + keyID)
		dump.Write(sealed)
		n++
	}
	_ = rows.Close()
	if n != 2 || bytes.Contains(dump.Bytes(), []byte(marker)) || bytes.Contains(dump.Bytes(), []byte("SELECT")) || bytes.Contains(dump.Bytes(), []byte(`"table"`)) {
		t.Fatalf("plaintext or key material visible in %d stored rows", n)
	}
	// A row copied to another owner (or id, scope) fails authentication instead of being served.
	f.exec(t, "INSERT INTO "+cfg.Database+".config_owners (owner_id) VALUES ('"+other+"')")
	f.exec(t, "INSERT INTO "+cfg.Database+".config_items (owner_id, kind, item_id, scope, nkey, key_id, sealed, plain_size, version, created_at, updated_at) "+
		"SELECT '"+other+"', kind, item_id, scope, nkey, key_id, sealed, plain_size, version, created_at, updated_at FROM "+cfg.Database+".config_items WHERE item_id = '"+item.ID+"'")
	_, err = service.Get(ctx, other, "bookmarks", item.ID)
	var public *configstore.Error
	if !errors.As(err, &public) || public.Code != "state_unreadable" {
		t.Fatalf("row moved between owners was served: %v", err)
	}
	f.exec(t, "UPDATE "+cfg.Database+".config_items SET scope = 'other' WHERE item_id = '"+fav.ID+"'")
	if _, err = service.Get(ctx, owner, "favorites", fav.ID); !errors.As(err, &public) || public.Code != "state_unreadable" {
		t.Fatalf("row moved between scopes was served: %v", err)
	}
	// Wrong server key: clean error class, no plaintext, no driver text.
	wrong := cfg
	wrong.ServerKey = bytes.Repeat([]byte{9}, KeySize)
	wrongService := configstore.NewService(openStore(t, wrong).ConfigRepo())
	_, err = wrongService.Get(ctx, owner, "bookmarks", item.ID)
	if !errors.As(err, &public) || public.Code != "state_unreadable" || strings.Contains(public.Message, marker) {
		t.Fatalf("wrong key: %v", err)
	}
	// Rotation: a store with the new key first and the old one second reads and rewrites.
	rotated := cfg
	rotated.ServerKey = bytes.Repeat([]byte{9}, KeySize)
	rotatedStore := openStore(t, rotated)
	rotatedStore.cipher, err = NewCipher(rotated.ServerKey, cfg.ServerKey)
	if err != nil {
		t.Fatal(err)
	}
	rotatedService := configstore.NewService(rotatedStore.ConfigRepo())
	if got, err := rotatedService.Get(ctx, owner, "bookmarks", item.ID); err != nil || !strings.Contains(string(got.Data), marker) {
		t.Fatalf("old-key rows unreadable after rotation: %v", err)
	}
	if _, err := rotatedService.Update(ctx, owner, "bookmarks", item.ID, []byte(`{"name":"n","sql":"SELECT 2"}`), 1); err != nil {
		t.Fatal(err)
	}
	var keyID string
	if err := f.root.QueryRow("SELECT key_id FROM "+cfg.Database+".config_items WHERE item_id = ?", item.ID).Scan(&keyID); err != nil || keyID != rotatedStore.cipher.ActiveKeyID() {
		t.Fatalf("rewrite did not move to the new key: %q %v", keyID, err)
	}
}

func TestConfigRepoRuntimeUserCannotChangeSchemaAndOutageIsClassified(t *testing.T) {
	f := newFixture(t)
	cfg := f.migrated(t)
	store := openStore(t, cfg)
	for _, statement := range []string{"DROP TABLE config_items", "ALTER TABLE config_items ADD COLUMN x INT", "TRUNCATE TABLE config_owners"} {
		if _, err := store.db.Exec(statement); err == nil {
			t.Fatalf("runtime user ran %s", statement)
		}
	}
}

func TestConfigRepoOutageMapsToUnavailableAndRecovers(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	p := newProxy(t, net.JoinHostPort(f.cfg.Host, strconv.Itoa(f.port)))
	cfg := f.cfg
	cfg.Host, cfg.Port = "localhost", p.port
	store := openStore(t, cfg)
	if err := store.WaitReady(context.Background(), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	service := configstore.NewService(store.ConfigRepo())
	ctx := context.Background()
	owner := repotest.Owner()
	item, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "navigation", Data: []byte(`{"v":1}`)})
	if err != nil {
		t.Fatal(err)
	}
	port := p.port
	p.stop()
	start := time.Now()
	for name, call := range map[string]func() error{
		"get":  func() error { _, err := service.Get(ctx, owner, "settings", item.ID); return err },
		"list": func() error { _, err := service.List(ctx, owner, "settings", configstore.Filter{}); return err },
		"create": func() error {
			_, _, err := service.Create(ctx, owner, "settings", configstore.CreateInput{Key: "x", Data: []byte(`{}`)})
			return err
		},
		"delete": func() error { return service.Delete(ctx, owner, "settings", item.ID, configstore.AnyVersion) },
		"export": func() error { _, err := service.Export(ctx, owner); return err },
	} {
		var public *configstore.Error
		if err := call(); !errors.As(err, &public) || public.Code != "state_unavailable" || public.Status != 503 {
			t.Fatalf("%s during outage: %v", name, err)
		}
	}
	if time.Since(start) > 40*time.Second {
		t.Fatal("operations hang during outage")
	}
	p.start(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	eventually(t, 15*time.Second, "recovery without restart", func() bool {
		got, err := service.Get(ctx, owner, "settings", item.ID)
		return err == nil && string(got.Data) == `{"v":1}`
	})
}
