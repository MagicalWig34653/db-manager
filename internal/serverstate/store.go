// SPDX-License-Identifier: AGPL-3.0-only

// Package serverstate is the persistent server state storage in an external
// MariaDB (ADR-0006 option c). It owns its own connection pool, which is
// strictly separate from the engine pools used for user-managed databases.
package serverstate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

const (
	pingTimeout     = 2 * time.Second
	connectTimeout  = 5 * time.Second
	ioTimeout       = 10 * time.Second
	defaultInterval = 3 * time.Second
)

// Store is the runtime handle to the server state. It is safe for concurrent use.
type Store struct {
	db      *sql.DB
	cipher  *Cipher
	healthy atomic.Bool

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// Open prepares the runtime pool and cipher. It does not contact the database.
func Open(cfg Config) (*Store, error) {
	if !cfg.Enabled() {
		return nil, ErrConfig
	}
	cipher, err := NewCipher(cfg.ServerKey)
	if err != nil {
		return nil, err
	}
	db, err := openPool(cfg, cfg.User, cfg.Password)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)
	return &Store{db: db, cipher: cipher}, nil
}

// openPool builds a pool with certificate-verified TLS unless TLS was
// explicitly disabled for local development. With TLS configured the driver
// never falls back to plaintext.
func openPool(cfg Config, user, password string) (*sql.DB, error) {
	return openPoolWithIO(cfg, user, password, ioTimeout)
}

// openPoolWithIO is openPool with an explicit read/write timeout, which the
// migration pool needs to be much longer than the runtime pool.
func openPoolWithIO(cfg Config, user, password string, io time.Duration) (*sql.DB, error) {
	c := mysql.NewConfig()
	c.Logger = &mysql.NopLogger{}
	c.Net = "tcp"
	c.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	c.User = user
	c.Passwd = password
	c.DBName = cfg.Database
	c.Timeout = connectTimeout
	c.ReadTimeout = io
	c.WriteTimeout = io
	c.ParseTime = true
	c.Loc = time.UTC
	c.MultiStatements = false
	c.AllowAllFiles = false
	c.AllowCleartextPasswords = false
	c.AllowFallbackToPlaintext = false
	if !cfg.TLSDisabled {
		roots := cfg.roots
		if roots == nil {
			var err error
			if roots, err = x509.SystemCertPool(); err != nil {
				roots = x509.NewCertPool()
			}
		}
		c.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.Host, RootCAs: roots}
	}
	connector, err := mysql.NewConnector(c)
	if err != nil {
		return nil, ErrConfig
	}
	return sql.OpenDB(connector), nil
}

// Ping reports whether the state store answers within a short deadline and
// updates the cached health.
func (s *Store) Ping(ctx context.Context) error {
	parent := ctx
	ctx, cancel := context.WithTimeout(parent, pingTimeout)
	defer cancel()
	var one int
	err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	if err != nil && parent.Err() != nil {
		// The caller gave up (cancellation or its own deadline). That says nothing
		// about the database, so the cached health must not change.
		return classify(err)
	}
	s.healthy.Store(err == nil)
	return classify(err)
}

// Healthy returns the last observed reachability without touching the network.
// It is false before the first successful check, so callers fail closed.
func (s *Store) Healthy() bool { return s.healthy.Load() }

// Check performs a live reachability check; it implements the HTTP health hook.
func (s *Store) Check(ctx context.Context) error { return s.Ping(ctx) }

// WaitReady blocks until the store is reachable and the schema matches this
// program. Certificate and authentication failures are configuration errors and
// return immediately; plain unavailability is retried until timeout.
func (s *Store) WaitReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		err := s.Ping(ctx)
		if err == nil {
			err = s.VerifySchema(ctx)
		}
		if err == nil {
			return nil
		}
		if err == ErrTLS || err == ErrAuth || err == ErrSchemaOutdated || err == ErrSchemaMismatch {
			return err
		}
		select {
		case <-ctx.Done():
			return ErrUnavailable
		case <-time.After(time.Second):
		}
	}
}

// Monitor refreshes the cached health until Close. Recovery after an outage
// needs no restart because database/sql reconnects on demand.
func (s *Store) Monitor(interval time.Duration) {
	if interval <= 0 {
		interval = defaultInterval
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go func(done chan struct{}) {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.Ping(ctx)
			}
		}
	}(s.done)
}

// Close stops the monitor and closes the pool.
func (s *Store) Close() error {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	s.healthy.Store(false)
	return s.db.Close()
}
