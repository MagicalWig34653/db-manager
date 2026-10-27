// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
)

const configOpTimeout = 15 * time.Second

// ConfigRepo returns the configstore persistence on this state store. Payloads
// are sealed with the server key (AEAD) and bound to owner, kind, id, scope and
// natural key, so rows cannot be swapped or moved between owners unnoticed.
func (s *Store) ConfigRepo() configstore.Repo { return &configRepo{s: s} }

type configRepo struct{ s *Store }

// envelope is the sealed plaintext of one item.
type envelope struct {
	Key       string          `json:"k,omitempty"`
	ClientRef string          `json:"r,omitempty"`
	Data      json.RawMessage `json:"d"`
}

func itemContext(owner string, kind configstore.Kind, id, scope, nkey string) []byte {
	return []byte("cfg1|" + owner + "|" + string(kind) + "|" + id + "|" + scope + "|" + nkey)
}

// cfgErr maps any error to a configstore sentinel or a context error.
func cfgErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, configstore.RepoExists), errors.Is(err, configstore.RepoNotFound), errors.Is(err, configstore.RepoVersion),
		errors.Is(err, configstore.RepoQuota), errors.Is(err, configstore.RepoStale), errors.Is(err, configstore.RepoCorrupt):
		return err
	case errors.Is(err, ErrDecrypt), errors.Is(err, ErrUnknownKey), errors.Is(err, ErrMalformed), errors.Is(err, ErrNoKey):
		return configstore.RepoCorrupt
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return configstore.RepoUnavailable
	}
}

func (r *configRepo) seal(owner string, row configstore.Row) ([]byte, error) {
	plain, err := json.Marshal(envelope{Key: row.Key, ClientRef: row.ClientRef, Data: row.Data})
	if err != nil {
		return nil, configstore.RepoCorrupt
	}
	return r.s.cipher.Seal(plain, itemContext(owner, row.Kind, row.ID, row.Scope, row.NKey))
}

func (r *configRepo) open(owner string, row configstore.Row, sealed []byte) (configstore.Row, error) {
	plain, err := r.s.cipher.Open(sealed, itemContext(owner, row.Kind, row.ID, row.Scope, row.NKey))
	if err != nil {
		return row, err
	}
	var env envelope
	if err := json.Unmarshal(plain, &env); err != nil {
		return row, configstore.RepoCorrupt
	}
	row.Key, row.ClientRef, row.Data = env.Key, env.ClientRef, []byte(env.Data)
	return row, nil
}

// withTx runs fn in a transaction after taking the owner's write lock. All
// writers of one owner serialize on that row, which keeps quotas exact.
func (r *configRepo) withTx(ctx context.Context, owner string, fn func(ctx context.Context, tx *sql.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, configOpTimeout)
	defer cancel()
	// Autocommit insert first: taking the lock row's shared lock inside the
	// transaction would deadlock when several writers upgrade it.
	if _, err := r.s.db.ExecContext(ctx, "INSERT IGNORE INTO config_owners (owner_id) VALUES (?)", owner); err != nil {
		return cfgErr(classify(err))
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := r.s.db.BeginTx(ctx, nil)
		if err != nil {
			return cfgErr(classify(err))
		}
		err = func() error {
			var locked string
			if err := tx.QueryRowContext(ctx, "SELECT owner_id FROM config_owners WHERE owner_id = ? FOR UPDATE", owner).Scan(&locked); err != nil {
				return err
			}
			return fn(ctx, tx)
		}()
		if err == nil {
			if err = tx.Commit(); err == nil {
				return nil
			}
		} else {
			_ = tx.Rollback()
		}
		var server *mysql.MySQLError
		if errors.As(err, &server) && (server.Number == 1213 || server.Number == 1205) {
			lastErr = err
			continue
		}
		if errors.Is(err, configstore.RepoExists) || errors.Is(err, configstore.RepoNotFound) || errors.Is(err, configstore.RepoVersion) ||
			errors.Is(err, configstore.RepoQuota) || errors.Is(err, configstore.RepoStale) || errors.Is(err, configstore.RepoCorrupt) {
			return err
		}
		return cfgErr(classify(unwrapPublic(err)))
	}
	return cfgErr(classify(lastErr))
}

func unwrapPublic(err error) error {
	for _, public := range []*Error{ErrDecrypt, ErrUnknownKey, ErrMalformed, ErrNoKey} {
		if errors.Is(err, public) {
			return public
		}
	}
	return err
}

const itemColumns = "item_id, scope, nkey, sealed, version, created_at, updated_at"

type scanner interface{ Scan(dest ...any) error }

func (r *configRepo) scan(owner string, kind configstore.Kind, rows scanner) (configstore.Row, error) {
	var row configstore.Row
	var sealed []byte
	var version uint64
	row.Kind = kind
	if err := rows.Scan(&row.ID, &row.Scope, &row.NKey, &sealed, &version, &row.CreatedAt, &row.UpdatedAt); err != nil {
		return row, err
	}
	row.Version = int64(version)
	row.CreatedAt, row.UpdatedAt = row.CreatedAt.UTC(), row.UpdatedAt.UTC()
	return r.open(owner, row, sealed)
}

// Insert implements configstore.Repo.
func (r *configRepo) Insert(ctx context.Context, owner string, row configstore.Row, lim configstore.Limits) (configstore.Row, error) {
	size := configstore.PlainSize(row.Key, row.ClientRef, row.Data)
	if size > lim.OwnerBytes {
		return configstore.Row{}, configstore.RepoQuota
	}
	sealed, err := r.seal(owner, row)
	if err != nil {
		return configstore.Row{}, cfgErr(err)
	}
	var out configstore.Row
	err = r.withTx(ctx, owner, func(ctx context.Context, tx *sql.Tx) error {
		existing := tx.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind = ? AND scope = ? AND nkey = ?", owner, string(row.Kind), row.Scope, row.NKey)
		found, err := r.scan(owner, row.Kind, existing)
		if err == nil {
			out = found
			return configstore.RepoExists
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := r.makeRoom(ctx, tx, owner, row, size, lim); err != nil {
			return err
		}
		created, updated := row.CreatedAt, row.UpdatedAt
		query := "INSERT INTO config_items (owner_id, kind, item_id, scope, nkey, key_id, sealed, plain_size, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, "
		args := []any{owner, string(row.Kind), row.ID, row.Scope, row.NKey, r.s.cipher.ActiveKeyID(), sealed, size}
		if created.IsZero() {
			query += "UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))"
		} else {
			query += "?, ?)"
			args = append(args, created.UTC(), updated.UTC())
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
		stored, err := r.scan(owner, row.Kind, tx.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ?", owner, string(row.Kind), row.ID))
		out = stored
		return err
	})
	if errors.Is(err, configstore.RepoExists) {
		return out, configstore.RepoExists
	}
	return out, cfgErr(err)
}

// makeRoom enforces the limits before an insert, evicting the oldest entries of
// FIFO kinds. It runs under the owner lock, so counts cannot change meanwhile.
func (r *configRepo) makeRoom(ctx context.Context, tx *sql.Tx, owner string, row configstore.Row, size int64, lim configstore.Limits) error {
	kind := string(row.Kind)
	var inScope, inOwner int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM config_items WHERE owner_id = ? AND kind = ? AND scope = ?", owner, kind, row.Scope).Scan(&inScope); err != nil {
		return err
	}
	if over := inScope - lim.PerScope + 1; over > 0 {
		if !lim.FIFO {
			return configstore.RepoQuota
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM config_items WHERE owner_id = ? AND kind = ? AND scope = ? ORDER BY updated_at, item_id LIMIT ?", owner, kind, row.Scope, over); err != nil {
			return err
		}
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM config_items WHERE owner_id = ? AND kind = ?", owner, kind).Scan(&inOwner); err != nil {
		return err
	}
	if over := inOwner - lim.PerOwner + 1; over > 0 {
		if !lim.FIFO {
			return configstore.RepoQuota
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM config_items WHERE owner_id = ? AND kind = ? ORDER BY updated_at, item_id LIMIT ?", owner, kind, over); err != nil {
			return err
		}
	}
	var used int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(plain_size), 0) FROM config_items WHERE owner_id = ?", owner).Scan(&used); err != nil {
		return err
	}
	need := used + size - lim.OwnerBytes
	if need <= 0 {
		return nil
	}
	if !lim.FIFO {
		return configstore.RepoQuota
	}
	rows, err := tx.QueryContext(ctx, "SELECT item_id, plain_size FROM config_items WHERE owner_id = ? AND kind = ? ORDER BY updated_at, item_id", owner, kind)
	if err != nil {
		return err
	}
	var victims []any
	for rows.Next() && need > 0 {
		var id string
		var freed int64
		if err := rows.Scan(&id, &freed); err != nil {
			_ = rows.Close()
			return err
		}
		victims = append(victims, id)
		need -= freed
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if need > 0 {
		return configstore.RepoQuota
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(victims)), ",")
	_, err = tx.ExecContext(ctx, "DELETE FROM config_items WHERE owner_id = ? AND kind = ? AND item_id IN ("+marks+")", append([]any{owner, kind}, victims...)...)
	return err
}

// Get implements configstore.Repo.
func (r *configRepo) Get(ctx context.Context, owner string, kind configstore.Kind, id string) (configstore.Row, error) {
	ctx, cancel := context.WithTimeout(ctx, configOpTimeout)
	defer cancel()
	row, err := r.scan(owner, kind, r.s.db.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ?", owner, string(kind), id))
	if errors.Is(err, sql.ErrNoRows) {
		return configstore.Row{}, configstore.RepoNotFound
	}
	return row, cfgErr(unwrapPublicClassify(err))
}

// unwrapPublicClassify keeps sentinel and key errors and classifies driver errors.
func unwrapPublicClassify(err error) error {
	if err == nil {
		return nil
	}
	if public := unwrapPublic(err); public != err {
		return public
	}
	var sentinel = errors.Is(err, configstore.RepoCorrupt)
	if sentinel {
		return err
	}
	return classify(err)
}

// List implements configstore.Repo.
func (r *configRepo) List(ctx context.Context, owner string, q configstore.ListQuery) ([]configstore.Row, error) {
	ctx, cancel := context.WithTimeout(ctx, configOpTimeout)
	defer cancel()
	query := "SELECT " + itemColumns + " FROM config_items WHERE owner_id = ? AND kind = ?"
	args := []any{owner, string(q.Kind)}
	if !q.AllScopes {
		query += " AND scope = ?"
		args = append(args, q.Scope)
	}
	if q.NKey != "" {
		query += " AND nkey = ?"
		args = append(args, q.NKey)
	}
	if q.After != nil {
		at := q.After.UpdatedAt.UTC()
		query += " AND (updated_at < ? OR (updated_at = ? AND item_id < ?))"
		args = append(args, at, at, q.After.ID)
	}
	query += " ORDER BY updated_at DESC, item_id DESC"
	if q.Limit > 0 {
		query += " LIMIT " + strconv.Itoa(q.Limit)
	}
	rows, err := r.s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, cfgErr(classify(err))
	}
	defer rows.Close()
	var out []configstore.Row
	for rows.Next() {
		row, err := r.scan(owner, q.Kind, rows)
		if err != nil {
			return nil, cfgErr(unwrapPublicClassify(err))
		}
		out = append(out, row)
	}
	return out, cfgErr(unwrapPublicClassify(rows.Err()))
}

func (r *configRepo) lockedRow(ctx context.Context, tx *sql.Tx, owner string, kind configstore.Kind, id string, expect int64) (configstore.Row, error) {
	row, err := r.scan(owner, kind, tx.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ? FOR UPDATE", owner, string(kind), id))
	if errors.Is(err, sql.ErrNoRows) {
		return row, configstore.RepoNotFound
	}
	if err != nil {
		return row, err
	}
	if expect != configstore.AnyVersion && row.Version != expect {
		return row, configstore.RepoVersion
	}
	return row, nil
}

// Update implements configstore.Repo.
func (r *configRepo) Update(ctx context.Context, owner string, kind configstore.Kind, id string, data []byte, expect int64, lim configstore.Limits) (configstore.Row, error) {
	var out configstore.Row
	err := r.withTx(ctx, owner, func(ctx context.Context, tx *sql.Tx) error {
		row, err := r.lockedRow(ctx, tx, owner, kind, id, expect)
		if err != nil {
			return err
		}
		oldSize := configstore.PlainSize(row.Key, row.ClientRef, row.Data)
		row.Data = data
		size := configstore.PlainSize(row.Key, row.ClientRef, row.Data)
		var used int64
		if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(plain_size), 0) FROM config_items WHERE owner_id = ?", owner).Scan(&used); err != nil {
			return err
		}
		if used-oldSize+size > lim.OwnerBytes {
			return configstore.RepoQuota
		}
		sealed, err := r.seal(owner, row)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE config_items SET sealed = ?, key_id = ?, plain_size = ?, version = version + 1, updated_at = UTC_TIMESTAMP(6) WHERE owner_id = ? AND kind = ? AND item_id = ?",
			sealed, r.s.cipher.ActiveKeyID(), size, owner, string(kind), id); err != nil {
			return err
		}
		out, err = r.scan(owner, kind, tx.QueryRowContext(ctx, "SELECT "+itemColumns+" FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ?", owner, string(kind), id))
		return err
	})
	return out, cfgErr(err)
}

// Delete implements configstore.Repo.
func (r *configRepo) Delete(ctx context.Context, owner string, kind configstore.Kind, id string, expect int64) error {
	return cfgErr(r.withTx(ctx, owner, func(ctx context.Context, tx *sql.Tx) error {
		var version uint64
		err := tx.QueryRowContext(ctx, "SELECT version FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ? FOR UPDATE", owner, string(kind), id).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) {
			return configstore.RepoNotFound
		}
		if err != nil {
			return err
		}
		if expect != configstore.AnyVersion && int64(version) != expect {
			return configstore.RepoVersion
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM config_items WHERE owner_id = ? AND kind = ? AND item_id = ?", owner, string(kind), id)
		return err
	}))
}

// Export implements configstore.Repo.
func (r *configRepo) Export(ctx context.Context, owner string) ([]configstore.Row, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*configOpTimeout)
	defer cancel()
	rows, err := r.s.db.QueryContext(ctx, "SELECT kind, "+itemColumns+" FROM config_items WHERE owner_id = ? ORDER BY updated_at, item_id", owner)
	if err != nil {
		return nil, cfgErr(classify(err))
	}
	defer rows.Close()
	byKind := map[configstore.Kind][]configstore.Row{}
	for rows.Next() {
		var kind string
		var row configstore.Row
		var sealed []byte
		var version uint64
		if err := rows.Scan(&kind, &row.ID, &row.Scope, &row.NKey, &sealed, &version, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, cfgErr(classify(err))
		}
		row.Kind, row.Version = configstore.Kind(kind), int64(version)
		row.CreatedAt, row.UpdatedAt = row.CreatedAt.UTC(), row.UpdatedAt.UTC()
		opened, err := r.open(owner, row, sealed)
		if err != nil {
			return nil, cfgErr(unwrapPublicClassify(err))
		}
		byKind[opened.Kind] = append(byKind[opened.Kind], opened)
	}
	if err := rows.Err(); err != nil {
		return nil, cfgErr(classify(err))
	}
	var out []configstore.Row
	for _, spec := range configstore.Specs {
		out = append(out, byKind[spec.Kind]...)
	}
	return out, nil
}

func clearWhere(owner string, scope configstore.ClearScope) (string, []any) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(scope.Kinds)), ",")
	args := []any{owner}
	for _, kind := range scope.Kinds {
		args = append(args, string(kind))
	}
	where := "owner_id = ? AND kind IN (" + marks + ")"
	if !scope.AllScopes {
		where += " AND scope = ?"
		args = append(args, scope.Scope)
	}
	return where, args
}

type queryContexter interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *configRepo) digest(ctx context.Context, q queryContexter, owner string, scope configstore.ClearScope) (configstore.Digest, error) {
	where, args := clearWhere(owner, scope)
	rows, err := q.QueryContext(ctx, "SELECT kind, item_id, version, plain_size FROM config_items WHERE "+where+" ORDER BY kind, item_id", args...)
	if err != nil {
		return configstore.Digest{}, err
	}
	defer rows.Close()
	d := configstore.Digest{Counts: map[configstore.Kind]int{}, Bytes: map[configstore.Kind]int64{}}
	sum := sha256.New()
	for rows.Next() {
		var kind, id string
		var version uint64
		var size int64
		if err := rows.Scan(&kind, &id, &version, &size); err != nil {
			return d, err
		}
		d.Counts[configstore.Kind(kind)]++
		d.Bytes[configstore.Kind(kind)] += size
		sum.Write([]byte(kind + "|" + id + "|" + strconv.FormatUint(version, 10) + "\n"))
	}
	d.Hash = hex.EncodeToString(sum.Sum(nil))
	return d, rows.Err()
}

// Digest implements configstore.Repo.
func (r *configRepo) Digest(ctx context.Context, owner string, scope configstore.ClearScope) (configstore.Digest, error) {
	ctx, cancel := context.WithTimeout(ctx, configOpTimeout)
	defer cancel()
	d, err := r.digest(ctx, r.s.db, owner, scope)
	return d, cfgErr(classify(err))
}

// Clear implements configstore.Repo.
func (r *configRepo) Clear(ctx context.Context, owner string, scope configstore.ClearScope, hash string) (map[configstore.Kind]int, error) {
	deleted := map[configstore.Kind]int{}
	err := r.withTx(ctx, owner, func(ctx context.Context, tx *sql.Tx) error {
		d, err := r.digest(ctx, tx, owner, scope)
		if err != nil {
			return err
		}
		if d.Hash != hash {
			return configstore.RepoStale
		}
		where, args := clearWhere(owner, scope)
		if _, err := tx.ExecContext(ctx, "DELETE FROM config_items WHERE "+where, args...); err != nil {
			return err
		}
		for kind, n := range d.Counts {
			deleted[kind] = n
		}
		return nil
	})
	return deleted, cfgErr(err)
}

// Usage implements configstore.Repo.
func (r *configRepo) Usage(ctx context.Context, owner string) (configstore.Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, configOpTimeout)
	defer cancel()
	rows, err := r.s.db.QueryContext(ctx, "SELECT kind, COUNT(*), COALESCE(SUM(plain_size), 0) FROM config_items WHERE owner_id = ? GROUP BY kind", owner)
	if err != nil {
		return configstore.Usage{}, cfgErr(classify(err))
	}
	defer rows.Close()
	u := configstore.Usage{Counts: map[configstore.Kind]int{}, Bytes: map[configstore.Kind]int64{}, SchemaVersion: LatestVersion()}
	for rows.Next() {
		var kind string
		var n int
		var size int64
		if err := rows.Scan(&kind, &n, &size); err != nil {
			return u, cfgErr(classify(err))
		}
		u.Counts[configstore.Kind(kind)], u.Bytes[configstore.Kind(kind)] = n, size
	}
	return u, cfgErr(classify(rows.Err()))
}
