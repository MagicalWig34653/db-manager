// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

// Migration is one forward-only schema step. Statements must be idempotent
// (CREATE ... IF NOT EXISTS) because MariaDB DDL commits implicitly and a crash
// between the DDL and the version row must be repairable by running it again.
type Migration struct {
	Version    int
	Name       string
	Statements []string
}

// Checksum fingerprints the statements so edited history is detected.
func (m Migration) Checksum() string {
	sum := sha256.Sum256([]byte(strings.Join(m.Statements, "\x00")))
	return hex.EncodeToString(sum[:])
}

// migrations is the ordered, append-only list. Never edit an applied entry.
var migrations = []Migration{
	{
		Version: 1,
		Name:    "schema_info_and_key_probe",
		Statements: []string{
			// Test table for key identifiers and sealed fields; no user-facing data.
			"CREATE TABLE IF NOT EXISTS state_key_probe (" +
				"id VARCHAR(64) NOT NULL PRIMARY KEY, " +
				"key_id VARCHAR(40) NOT NULL, " +
				"sealed VARBINARY(4096) NOT NULL, " +
				"updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
	},
}

// LatestVersion is the schema version this program expects.
func LatestVersion() int { return migrations[len(migrations)-1].Version }

const createVersionTable = "CREATE TABLE IF NOT EXISTS schema_migrations (" +
	"version INT NOT NULL PRIMARY KEY, " +
	"name VARCHAR(100) NOT NULL, " +
	"checksum CHAR(64) NOT NULL, " +
	"applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)" +
	") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"

// MigrateOptions tunes the migration run.
type MigrateOptions struct {
	// LockTimeout is how long to wait for a concurrent migrator. Default 60s.
	LockTimeout time.Duration
}

// Report describes a migration or plan run.
type Report struct {
	// Applied lists versions applied by this call (Plan: versions that would be applied).
	Applied []int
	// Current is the highest applied version after the call (Plan: before).
	Current int
}

func lockName(database string) string {
	sum := sha256.Sum256([]byte(database))
	return "dbm_state_migrate_" + hex.EncodeToString(sum[:])[:32]
}

func (c Config) migrationAccount() (string, string) {
	if c.MigrationUser != "" {
		return c.MigrationUser, c.MigrationPassword
	}
	return c.User, c.Password
}

// Plan reports pending migrations without executing any DDL or creating tables.
func Plan(ctx context.Context, cfg Config) (Report, error) {
	user, password := cfg.migrationAccount()
	db, err := openPool(cfg, user, password)
	if err != nil {
		return Report{}, err
	}
	defer db.Close()
	applied, err := readApplied(ctx, db)
	if err != nil {
		return Report{}, classify(err)
	}
	return plan(applied)
}

func plan(applied map[int]string) (Report, error) {
	var report Report
	for _, m := range migrations {
		if sum, ok := applied[m.Version]; ok {
			if sum != m.Checksum() {
				return Report{}, ErrSchemaMismatch
			}
			report.Current = m.Version
			continue
		}
		report.Applied = append(report.Applied, m.Version)
	}
	for version := range applied {
		if version > LatestVersion() {
			return Report{}, ErrSchemaMismatch
		}
	}
	return report, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// readApplied returns applied versions with checksums. A missing version table means none.
func readApplied(ctx context.Context, q queryer) (map[int]string, error) {
	rows, err := q.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		var server *mysql.MySQLError
		if errors.As(err, &server) && server.Number == 1146 {
			return map[int]string{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	applied := map[int]string{}
	for rows.Next() {
		var version int
		var sum string
		if err := rows.Scan(&version, &sum); err != nil {
			return nil, err
		}
		applied[version] = sum
	}
	return applied, rows.Err()
}

// Migrate applies pending migrations while holding a database-wide named lock
// (GET_LOCK) on a dedicated connection, so concurrent replicas migrate exactly
// once: the others wait, then find the version already applied. DDL runs only
// with the migration account (or the runtime account if none is configured,
// which only succeeds when that account was granted DDL).
func Migrate(ctx context.Context, cfg Config, opts MigrateOptions) (Report, error) {
	if opts.LockTimeout <= 0 {
		opts.LockTimeout = 60 * time.Second
	}
	user, password := cfg.migrationAccount()
	db, err := openPool(cfg, user, password)
	if err != nil {
		return Report{}, err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return Report{}, classify(err)
	}
	defer conn.Close()

	name := lockName(cfg.Database)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", name, int(opts.LockTimeout/time.Second)).Scan(&got); err != nil {
		return Report{}, classify(err)
	}
	if !got.Valid || got.Int64 != 1 {
		return Report{}, ErrMigrationLocked
	}
	// Release with a fresh context so cancellation cannot leave the lock held
	// on a pooled connection; closing the connection releases it as well.
	defer func() {
		release, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(release, "SELECT RELEASE_LOCK(?)", name)
	}()

	if _, err := conn.ExecContext(ctx, createVersionTable); err != nil {
		return Report{}, classify(err)
	}
	applied, err := readApplied(ctx, conn)
	if err != nil {
		return Report{}, classify(err)
	}
	pending, err := plan(applied)
	if err != nil {
		return Report{}, err
	}
	report := Report{Current: pending.Current}
	for _, m := range migrations {
		if _, done := applied[m.Version]; done {
			continue
		}
		var mine sql.NullInt64
		if err := conn.QueryRowContext(ctx, "SELECT IS_USED_LOCK(?) = CONNECTION_ID()", name).Scan(&mine); err != nil {
			return report, classify(err)
		}
		if !mine.Valid || mine.Int64 != 1 {
			return report, ErrMigrationLocked
		}
		for _, statement := range m.Statements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				if c := classify(err); c == ErrAuth || errors.Is(c, context.Canceled) || errors.Is(c, context.DeadlineExceeded) {
					return report, c
				}
				return report, ErrMigrationFailed
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, checksum) VALUES (?, ?, ?)", m.Version, m.Name, m.Checksum()); err != nil {
			return report, classify(err)
		}
		report.Applied = append(report.Applied, m.Version)
		report.Current = m.Version
	}
	return report, nil
}

// VerifySchema checks with the runtime account that exactly the expected
// migrations are applied. It never changes the schema.
func (s *Store) VerifySchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	applied, err := readApplied(ctx, s.db)
	if err != nil {
		return classify(err)
	}
	report, err := plan(applied)
	if err != nil {
		return err
	}
	if len(report.Applied) > 0 {
		return ErrSchemaOutdated
	}
	return nil
}
