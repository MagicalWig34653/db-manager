// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests need an exclusively owned disposable MariaDB with TLS, provided
// by scripts/test-server-state.py through DBM_TEST_STATE_* variables.

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func loadTestRoots(t *testing.T) *x509.CertPool {
	t.Helper()
	data, err := os.ReadFile(os.Getenv("DBM_TEST_STATE_CA_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		t.Fatal("test CA")
	}
	return roots
}

type fixture struct {
	cfg  Config
	root *sql.DB
	port int
}

func (f fixture) exec(t *testing.T, statement string) {
	t.Helper()
	if _, err := f.root.Exec(statement); err != nil {
		t.Fatalf("fixture statement failed: %v", err)
	}
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	rawPort := os.Getenv("DBM_TEST_STATE_PORT")
	if rawPort == "" {
		t.Skip("disposable state database fixture required")
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	host := os.Getenv("DBM_TEST_STATE_HOST")
	if host == "" {
		host = "localhost"
	}
	suffix := randomHex(t, 5)
	cfg := Config{
		Host: host, Port: port, Database: "sz1_" + suffix,
		User: "rt_" + suffix, Password: randomHex(t, 16),
		MigrationUser: "mg_" + suffix, MigrationPassword: randomHex(t, 16),
		ServerKey: bytes.Repeat([]byte{7}, KeySize), roots: loadTestRoots(t),
	}
	root, err := openPool(Config{Host: host, Port: port, roots: cfg.roots}, "root", os.Getenv("DBM_TEST_STATE_ROOT_PASSWORD"))
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{cfg: cfg, root: root, port: port}
	for _, statement := range []string{
		"CREATE DATABASE " + cfg.Database + " CHARACTER SET utf8mb4",
		fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s' REQUIRE SSL", cfg.User, cfg.Password),
		fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s' REQUIRE SSL", cfg.MigrationUser, cfg.MigrationPassword),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON %s.* TO '%s'@'%%'", cfg.Database, cfg.User),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%'", cfg.Database, cfg.MigrationUser),
	} {
		f.exec(t, statement)
	}
	t.Cleanup(func() {
		for _, statement := range []string{"DROP DATABASE IF EXISTS " + cfg.Database, "DROP USER IF EXISTS '" + cfg.User + "'@'%'", "DROP USER IF EXISTS '" + cfg.MigrationUser + "'@'%'"} {
			_, _ = root.Exec(statement)
		}
		_ = root.Close()
	})
	return f
}

func (f fixture) migrated(t *testing.T) Config {
	t.Helper()
	if _, err := Migrate(context.Background(), f.cfg, MigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	return f.cfg
}

func openStore(t *testing.T, cfg Config) *Store {
	t.Helper()
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestConcurrentMigrationAppliesExactlyOnce(t *testing.T) {
	f := newFixture(t)
	const replicas = 8
	reports := make([]Report, replicas)
	errs := make([]error, replicas)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < replicas; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			reports[i], errs[i] = Migrate(context.Background(), f.cfg, MigrateOptions{})
		}(i)
	}
	close(start)
	wg.Wait()
	appliers := 0
	for i := range reports {
		if errs[i] != nil {
			t.Fatalf("replica %d: %v", i, errs[i])
		}
		if len(reports[i].Applied) > 0 {
			appliers++
		}
		if reports[i].Current != LatestVersion() {
			t.Fatalf("replica %d current %d", i, reports[i].Current)
		}
	}
	if appliers != 1 {
		t.Fatalf("migration applied by %d replicas, want exactly 1", appliers)
	}
	var rows int
	if err := f.root.QueryRow("SELECT COUNT(*) FROM " + f.cfg.Database + ".schema_migrations").Scan(&rows); err != nil || rows != len(migrations) {
		t.Fatalf("version rows %d %v", rows, err)
	}
	if again, err := Migrate(context.Background(), f.cfg, MigrateOptions{}); err != nil || len(again.Applied) != 0 {
		t.Fatalf("second run not a no-op: %+v %v", again, err)
	}
}

func TestMigrationWaitsForLockAndTimesOut(t *testing.T) {
	f := newFixture(t)
	holder, err := f.root.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	var got int
	if err := holder.QueryRowContext(context.Background(), "SELECT GET_LOCK(?, 5)", lockName(f.cfg.Database)).Scan(&got); err != nil || got != 1 {
		t.Fatalf("hold lock: %v %d", err, got)
	}
	if _, err := Migrate(context.Background(), f.cfg, MigrateOptions{LockTimeout: time.Second}); !errors.Is(err, ErrMigrationLocked) {
		t.Fatalf("expected lock timeout, got %v", err)
	}
	var tables int
	_ = f.root.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?", f.cfg.Database).Scan(&tables)
	if tables != 0 {
		t.Fatal("migration ran without holding the lock")
	}
	done := make(chan Report, 1)
	go func() {
		report, err := Migrate(context.Background(), f.cfg, MigrateOptions{LockTimeout: 20 * time.Second})
		if err != nil {
			t.Error(err)
		}
		done <- report
	}()
	time.Sleep(700 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("migrated while lock was held")
	default:
	}
	if _, err := holder.ExecContext(context.Background(), "DO RELEASE_LOCK(?)", lockName(f.cfg.Database)); err != nil {
		t.Fatal(err)
	}
	select {
	case report := <-done:
		if len(report.Applied) != 1 {
			t.Fatalf("waiter result %+v", report)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("waiter never proceeded")
	}
}

func TestPlanIsDryRun(t *testing.T) {
	f := newFixture(t)
	report, err := Plan(context.Background(), f.cfg)
	if err != nil || len(report.Applied) != len(migrations) || report.Current != 0 {
		t.Fatalf("plan %+v %v", report, err)
	}
	var tables int
	_ = f.root.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?", f.cfg.Database).Scan(&tables)
	if tables != 0 {
		t.Fatalf("plan created %d tables", tables)
	}
	f.migrated(t)
	if report, err := Plan(context.Background(), f.cfg); err != nil || len(report.Applied) != 0 || report.Current != LatestVersion() {
		t.Fatalf("plan after migrate %+v %v", report, err)
	}
	f.exec(t, "UPDATE "+f.cfg.Database+".schema_migrations SET checksum = REPEAT('0', 64) WHERE version = 1")
	if _, err := Plan(context.Background(), f.cfg); !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("checksum drift not detected: %v", err)
	}
}

func TestWaitReadyReportsOutdatedSchemaAndUnavailability(t *testing.T) {
	f := newFixture(t)
	store := openStore(t, f.cfg)
	if err := store.WaitReady(context.Background(), 5*time.Second); !errors.Is(err, ErrSchemaOutdated) {
		t.Fatalf("outdated schema: %v", err)
	}
	f.migrated(t)
	if err := store.WaitReady(context.Background(), 5*time.Second); err != nil || !store.Healthy() {
		t.Fatalf("ready: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	dead := f.cfg
	dead.Port = closedPort
	deadStore := openStore(t, dead)
	if err := deadStore.WaitReady(context.Background(), 1500*time.Millisecond); !errors.Is(err, ErrUnavailable) || deadStore.Healthy() {
		t.Fatalf("unreachable: %v healthy=%v", err, deadStore.Healthy())
	}
}

func TestTLSCertificateIsVerified(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)

	untrusted := f.cfg
	untrusted.roots = nil // system roots only; the fixture CA is unknown
	if err := openStore(t, untrusted).Ping(context.Background()); !errors.Is(err, ErrTLS) {
		t.Fatalf("untrusted CA accepted: %v", err)
	}
	wrongHost := f.cfg
	wrongHost.Host = "127.0.0.1" // certificate only names localhost
	if err := openStore(t, wrongHost).Ping(context.Background()); !errors.Is(err, ErrTLS) {
		t.Fatalf("hostname mismatch accepted: %v", err)
	}
	if err := openStore(t, f.cfg).Ping(context.Background()); err != nil {
		t.Fatalf("trusted TLS: %v", err)
	}
	if raw := os.Getenv("DBM_TEST_STATE_NOTLS_PORT"); raw != "" {
		port, _ := strconv.Atoi(raw)
		plain := f.cfg
		plain.Port = port
		if err := openStore(t, plain).Ping(context.Background()); !errors.Is(err, ErrTLS) {
			t.Fatalf("downgrade to plaintext accepted: %v", err)
		}
	}
	// Verified-TLS users cannot be reached without TLS even when the client opts out.
	insecure := f.cfg
	insecure.TLSDisabled = true
	if err := openStore(t, insecure).Ping(context.Background()); err == nil {
		t.Fatal("REQUIRE SSL user connected without TLS")
	}
}

func TestRuntimeUserHasLeastPrivilege(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	store := openStore(t, f.cfg)
	ctx := context.Background()
	if err := store.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.PutProbe(ctx, "dml", []byte("works")); err != nil {
		t.Fatalf("runtime DML denied: %v", err)
	}
	other := "other_" + randomHex(t, 4)
	f.exec(t, "CREATE DATABASE "+other)
	f.exec(t, "CREATE TABLE "+other+".t (a INT)")
	t.Cleanup(func() { _, _ = f.root.Exec("DROP DATABASE IF EXISTS " + other) })
	for _, statement := range []string{
		"CREATE TABLE runtime_ddl (a INT)",
		"ALTER TABLE state_key_probe ADD COLUMN extra INT",
		"DROP TABLE state_key_probe",
		"TRUNCATE TABLE state_key_probe",
		"CREATE INDEX runtime_idx ON state_key_probe (key_id)",
		"SELECT * FROM " + other + ".t",
		"INSERT INTO " + other + ".t VALUES (1)",
		"CREATE DATABASE runtime_escape",
		"SELECT * FROM mysql.user",
	} {
		if _, err := store.db.ExecContext(ctx, statement); err == nil {
			t.Fatalf("runtime user allowed: %s", statement)
		} else if classify(err) != ErrAuth {
			t.Fatalf("unexpected error class for %q", statement)
		}
	}
	// Without a migration account the runtime account must not be able to migrate.
	fresh := newFixture(t)
	freshRuntimeOnly := fresh.cfg
	freshRuntimeOnly.MigrationUser, freshRuntimeOnly.MigrationPassword = "", ""
	if _, err := Migrate(ctx, freshRuntimeOnly, MigrateOptions{}); !errors.Is(err, ErrAuth) {
		t.Fatalf("runtime account performed DDL: %v", err)
	}
}

func TestEncryptedFieldsAreNotPlaintextInDump(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	store := openStore(t, f.cfg)
	ctx := context.Background()
	secret := "plaintext-marker-" + randomHex(t, 6)
	if err := store.PutProbe(ctx, "a", []byte(secret)); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetProbe(ctx, "a"); err != nil || string(got) != secret {
		t.Fatalf("round trip: %v", err)
	}
	rows, err := f.root.Query("SELECT id, key_id, sealed FROM " + f.cfg.Database + ".state_key_probe")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var dump bytes.Buffer
	for rows.Next() {
		var id, keyID string
		var sealed []byte
		if err := rows.Scan(&id, &keyID, &sealed); err != nil {
			t.Fatal(err)
		}
		if keyID != store.cipher.ActiveKeyID() {
			t.Fatalf("key id column %q", keyID)
		}
		dump.WriteString(id + "|" + keyID + "|")
		dump.Write(sealed)
		dump.WriteString(hex.EncodeToString(sealed))
	}
	if dump.Len() == 0 || bytes.Contains(dump.Bytes(), []byte(secret)) {
		t.Fatal("plaintext visible in stored rows or no rows")
	}

	// Wrong server key: clean error, no plaintext.
	wrong := f.cfg
	wrong.ServerKey = bytes.Repeat([]byte{9}, KeySize)
	if _, err := openStore(t, wrong).GetProbe(ctx, "a"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("wrong key: %v", err)
	}
	// Missing server key.
	missing := &Store{db: store.db}
	if _, err := missing.GetProbe(ctx, "a"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("missing key: %v", err)
	}
	if err := missing.PutProbe(ctx, "b", []byte("x")); !errors.Is(err, ErrNoKey) {
		t.Fatalf("missing key put: %v", err)
	}
	// A sealed value moved to another row id fails authentication.
	f.exec(t, "INSERT INTO "+f.cfg.Database+".state_key_probe (id, key_id, sealed) SELECT 'moved', key_id, sealed FROM "+f.cfg.Database+".state_key_probe WHERE id = 'a'")
	if _, err := store.GetProbe(ctx, "moved"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("moved value accepted: %v", err)
	}
	if _, err := store.GetProbe(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
	if err := store.PutProbe(ctx, "bad id/../", nil); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestErrorsNeverContainSecrets(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	bad := f.cfg
	bad.Password = "wrong-" + randomHex(t, 8)
	err := openStore(t, bad).Ping(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("wrong password: %v", err)
	}
	for _, secret := range []string{bad.Password, f.cfg.Password, f.cfg.MigrationPassword, f.cfg.User, f.cfg.Database} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("secret in error text")
		}
	}
}

// proxy is a restartable TCP forwarder that simulates a database outage.
type proxy struct {
	t        *testing.T
	target   string
	mu       sync.Mutex
	listener net.Listener
	conns    []net.Conn
	port     int
}

func newProxy(t *testing.T, target string) *proxy {
	p := &proxy{t: t, target: target}
	p.start("127.0.0.1:0")
	t.Cleanup(p.stop)
	return p
}

func (p *proxy) start(address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		p.t.Fatal(err)
	}
	p.mu.Lock()
	p.listener = listener
	p.port = listener.Addr().(*net.TCPAddr).Port
	p.mu.Unlock()
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			upstream, err := net.Dial("tcp", p.target)
			if err != nil {
				_ = client.Close()
				continue
			}
			p.mu.Lock()
			p.conns = append(p.conns, client, upstream)
			p.mu.Unlock()
			go func() { _, _ = io.Copy(upstream, client); _ = upstream.Close() }()
			go func() { _, _ = io.Copy(client, upstream); _ = client.Close() }()
		}
	}()
}

func (p *proxy) stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listener != nil {
		_ = p.listener.Close()
		p.listener = nil
	}
	for _, conn := range p.conns {
		_ = conn.Close()
	}
	p.conns = nil
}

func TestOutageFailsClosedAndRecoversWithoutRestart(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	p := newProxy(t, net.JoinHostPort(f.cfg.Host, strconv.Itoa(f.port)))
	cfg := f.cfg
	cfg.Host, cfg.Port = "localhost", p.port
	store := openStore(t, cfg)
	if err := store.WaitReady(context.Background(), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	store.Monitor(100 * time.Millisecond)
	ctx := context.Background()
	if err := store.PutProbe(ctx, "survivor", []byte("persisted")); err != nil {
		t.Fatal(err)
	}
	if !store.Healthy() {
		t.Fatal("not healthy while up")
	}

	port := p.port
	p.stop() // outage
	eventually(t, 5*time.Second, "unhealthy after outage", func() bool { return !store.Healthy() })
	if err := store.Check(ctx); err == nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("check during outage: %v", err)
	}
	if _, err := store.GetProbe(ctx, "survivor"); err == nil {
		t.Fatal("read succeeded during outage")
	}

	p.start(net.JoinHostPort("127.0.0.1", strconv.Itoa(port))) // recovery, same Store instance
	eventually(t, 10*time.Second, "recovery after outage", func() bool { return store.Healthy() })
	if got, err := store.GetProbe(ctx, "survivor"); err != nil || string(got) != "persisted" {
		t.Fatalf("state lost after outage: %v", err)
	}
}

func eventually(t *testing.T, limit time.Duration, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout: %s", what)
}

func TestStoreCloseStopsMonitorAndIsRaceFree(t *testing.T) {
	f := newFixture(t)
	f.migrated(t)
	store, err := Open(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	store.Monitor(20 * time.Millisecond)
	store.Monitor(20 * time.Millisecond) // second call is a no-op
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_ = store.Healthy()
				_ = store.Check(context.Background())
			}
		}()
	}
	wg.Wait()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if store.Healthy() {
		t.Fatal("healthy after close")
	}
}
