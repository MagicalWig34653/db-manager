// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"sync"
	"time"
)

// MemoryRepo is an in-memory Repo with the same semantics as the MariaDB
// implementation. It backs unit tests and the shared conformance suite; it is
// never wired into the server.
type MemoryRepo struct {
	mu   sync.Mutex
	rows map[string][]*Row // owner -> rows
	// Down makes every call fail like an unreachable store.
	Down bool
	last time.Time
}

// NewMemoryRepo returns an empty repository.
func NewMemoryRepo() *MemoryRepo { return &MemoryRepo{rows: map[string][]*Row{}} }

func (m *MemoryRepo) tick() time.Time {
	now := time.Now().UTC().Truncate(time.Microsecond)
	if !now.After(m.last) {
		now = m.last.Add(time.Microsecond)
	}
	m.last = now
	return now
}

func (m *MemoryRepo) find(owner string, kind Kind, id string) *Row {
	for _, row := range m.rows[owner] {
		if row.Kind == kind && row.ID == id {
			return row
		}
	}
	return nil
}

func sizeOf(row *Row) int64 { return PlainSize(row.Key, row.ClientRef, row.Data) }

func (m *MemoryRepo) ownerBytes(owner string) int64 {
	var total int64
	for _, row := range m.rows[owner] {
		total += sizeOf(row)
	}
	return total
}

func older(a, b *Row) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.Before(b.UpdatedAt)
	}
	return a.ID < b.ID
}

func (m *MemoryRepo) evict(owner string, kind Kind, scope string, scoped bool) bool {
	var oldest *Row
	for _, row := range m.rows[owner] {
		if row.Kind != kind || (scoped && row.Scope != scope) {
			continue
		}
		if oldest == nil || older(row, oldest) {
			oldest = row
		}
	}
	if oldest == nil {
		return false
	}
	m.remove(owner, oldest)
	return true
}

func (m *MemoryRepo) remove(owner string, target *Row) {
	rows := m.rows[owner][:0]
	for _, row := range m.rows[owner] {
		if row != target {
			rows = append(rows, row)
		}
	}
	m.rows[owner] = rows
}

func (m *MemoryRepo) count(owner string, kind Kind, scope string, scoped bool) int {
	n := 0
	for _, row := range m.rows[owner] {
		if row.Kind == kind && (!scoped || row.Scope == scope) {
			n++
		}
	}
	return n
}

// Insert implements Repo.
func (m *MemoryRepo) Insert(ctx context.Context, owner string, row Row, lim Limits) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return Row{}, RepoUnavailable
	}
	for _, existing := range m.rows[owner] {
		if existing.Kind == row.Kind && existing.Scope == row.Scope && existing.NKey == row.NKey {
			return *existing, RepoExists
		}
	}
	size := PlainSize(row.Key, row.ClientRef, row.Data)
	if size > lim.OwnerBytes {
		return Row{}, RepoQuota
	}
	for m.count(owner, row.Kind, row.Scope, true) >= lim.PerScope {
		if !lim.FIFO || !m.evict(owner, row.Kind, row.Scope, true) {
			return Row{}, RepoQuota
		}
	}
	for m.count(owner, row.Kind, "", false) >= lim.PerOwner {
		if !lim.FIFO || !m.evict(owner, row.Kind, "", false) {
			return Row{}, RepoQuota
		}
	}
	for m.ownerBytes(owner)+size > lim.OwnerBytes {
		if !lim.FIFO || !m.evict(owner, row.Kind, "", false) {
			return Row{}, RepoQuota
		}
	}
	stored := row
	stored.Version = 1
	if stored.CreatedAt.IsZero() {
		stored.CreatedAt = m.tick()
		stored.UpdatedAt = stored.CreatedAt
	}
	m.rows[owner] = append(m.rows[owner], &stored)
	return stored, nil
}

// Get implements Repo.
func (m *MemoryRepo) Get(ctx context.Context, owner string, kind Kind, id string) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return Row{}, RepoUnavailable
	}
	if row := m.find(owner, kind, id); row != nil {
		return *row, nil
	}
	return Row{}, RepoNotFound
}

// List implements Repo.
func (m *MemoryRepo) List(ctx context.Context, owner string, q ListQuery) ([]Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return nil, RepoUnavailable
	}
	var rows []Row
	for _, row := range m.rows[owner] {
		if row.Kind != q.Kind || (!q.AllScopes && row.Scope != q.Scope) || (q.NKey != "" && row.NKey != q.NKey) {
			continue
		}
		if q.After != nil {
			after := q.After.UpdatedAt.UnixMicro()
			mine := row.UpdatedAt.UnixMicro()
			if mine > after || (mine == after && row.ID >= q.After.ID) {
				continue
			}
		}
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].ID > rows[j].ID
	})
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	return rows, nil
}

// Update implements Repo.
func (m *MemoryRepo) Update(ctx context.Context, owner string, kind Kind, id string, data []byte, expect int64, lim Limits) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return Row{}, RepoUnavailable
	}
	row := m.find(owner, kind, id)
	if row == nil {
		return Row{}, RepoNotFound
	}
	if expect != AnyVersion && row.Version != expect {
		return Row{}, RepoVersion
	}
	if m.ownerBytes(owner)-sizeOf(row)+PlainSize(row.Key, row.ClientRef, data) > lim.OwnerBytes {
		return Row{}, RepoQuota
	}
	row.Data = append([]byte(nil), data...)
	row.Version++
	row.UpdatedAt = m.tick()
	return *row, nil
}

// Delete implements Repo.
func (m *MemoryRepo) Delete(ctx context.Context, owner string, kind Kind, id string, expect int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return RepoUnavailable
	}
	row := m.find(owner, kind, id)
	if row == nil {
		return RepoNotFound
	}
	if expect != AnyVersion && row.Version != expect {
		return RepoVersion
	}
	m.remove(owner, row)
	return nil
}

// Export implements Repo.
func (m *MemoryRepo) Export(ctx context.Context, owner string) ([]Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return nil, RepoUnavailable
	}
	var rows []Row
	for _, spec := range Specs {
		var part []Row
		for _, row := range m.rows[owner] {
			if row.Kind == spec.Kind {
				part = append(part, *row)
			}
		}
		sort.Slice(part, func(i, j int) bool {
			if !part[i].UpdatedAt.Equal(part[j].UpdatedAt) {
				return part[i].UpdatedAt.Before(part[j].UpdatedAt)
			}
			return part[i].ID < part[j].ID
		})
		rows = append(rows, part...)
	}
	return rows, nil
}

func selected(row *Row, scope ClearScope) bool {
	if !scope.AllScopes && row.Scope != scope.Scope {
		return false
	}
	for _, kind := range scope.Kinds {
		if row.Kind == kind {
			return true
		}
	}
	return false
}

func (m *MemoryRepo) digest(owner string, scope ClearScope) Digest {
	d := Digest{Counts: map[Kind]int{}, Bytes: map[Kind]int64{}}
	var picked []*Row
	for _, row := range m.rows[owner] {
		if selected(row, scope) {
			picked = append(picked, row)
		}
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].Kind != picked[j].Kind {
			return picked[i].Kind < picked[j].Kind
		}
		return picked[i].ID < picked[j].ID
	})
	sum := sha256.New()
	for _, row := range picked {
		d.Counts[row.Kind]++
		d.Bytes[row.Kind] += sizeOf(row)
		sum.Write([]byte(string(row.Kind) + "|" + row.ID + "|" + strconv.FormatInt(row.Version, 10) + "\n"))
	}
	d.Hash = hex.EncodeToString(sum.Sum(nil))
	return d
}

// Digest implements Repo.
func (m *MemoryRepo) Digest(ctx context.Context, owner string, scope ClearScope) (Digest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return Digest{}, RepoUnavailable
	}
	return m.digest(owner, scope), nil
}

// Clear implements Repo.
func (m *MemoryRepo) Clear(ctx context.Context, owner string, scope ClearScope, hash string) (map[Kind]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return nil, RepoUnavailable
	}
	if m.digest(owner, scope).Hash != hash {
		return nil, RepoStale
	}
	deleted := map[Kind]int{}
	kept := m.rows[owner][:0]
	for _, row := range m.rows[owner] {
		if selected(row, scope) {
			deleted[row.Kind]++
		} else {
			kept = append(kept, row)
		}
	}
	m.rows[owner] = kept
	return deleted, nil
}

// Usage implements Repo.
func (m *MemoryRepo) Usage(ctx context.Context, owner string) (Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Down {
		return Usage{}, RepoUnavailable
	}
	u := Usage{Counts: map[Kind]int{}, Bytes: map[Kind]int64{}, SchemaVersion: 2}
	for _, row := range m.rows[owner] {
		u.Counts[row.Kind]++
		u.Bytes[row.Kind] += sizeOf(row)
	}
	return u, nil
}
