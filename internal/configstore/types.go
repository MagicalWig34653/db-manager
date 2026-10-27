// SPDX-License-Identifier: AGPL-3.0-only

// Package configstore is the server-side comfort data store (W-P20, ADR-W-P20):
// SQL history, bookmarks, saved searches, table favorites, recently used tables,
// settings documents and layouts. It holds the policy (kinds, limits, validation,
// secret rejection, optimistic concurrency, plan/confirm for clearing) and talks
// to persistence through the narrow Repo interface, which the MariaDB server
// state implements. Every Repo call is scoped to an owner; today the API passes
// one constant owner, W-P30 will pass the account identifier.
package configstore

import (
	"context"
	"time"
)

// ContractVersion is the API revision introduced with this package.
const ContractVersion = "0.24.0"

// Kind names a collection of comfort data.
type Kind string

const (
	KindHistory   Kind = "history"
	KindBookmarks Kind = "bookmarks"
	KindSearches  Kind = "searches"
	KindFavorites Kind = "favorites"
	KindRecents   Kind = "recents"
	KindSettings  Kind = "settings"
	KindLayouts   Kind = "layouts"
)

// Hard limits shared by all kinds.
const (
	MaxScopeLen        = 128
	MaxKeyLen          = 200 // runes
	MaxClientRefLen    = 64
	MaxNameLen         = 100 // runes, as in the browser-local W-P16 store
	MaxSQLChars        = 100000
	MaxSQLBytes        = 256 << 10
	MaxPage            = 100
	DefaultPage        = 50
	MaxPageBytes       = 4 << 20
	MaxImportItems     = 100
	MaxJSONDepth       = 5
	MaxJSONNodes       = 2000
	MaxJSONString      = 4096 // bytes per string value in documents
	MaxJSONArray       = 256
	MaxJSONObjectKeys  = 64
	MaxVariables       = 20
	MaxIdentifier      = 128 // runes, database and table names
	MaxRecentsPerScope = 100
	MaxClearKinds      = 7
	idLength           = 32 // hex characters
	ownerIDMax         = 64
)

// MaxOwnerBytes caps the plaintext bytes per owner over all kinds. It is a
// variable only so that tests can lower it; production code never writes it.
var MaxOwnerBytes int64 = 32 << 20

// Spec is the static policy of one kind.
type Spec struct {
	Kind Kind
	// Keyed kinds carry a caller-supplied key and are unique per (scope, key);
	// favorites and recents derive the key from database and table instead.
	Keyed bool
	// Derived means the natural key is computed from the data (favorites, recents).
	Derived bool
	// FIFO kinds evict their oldest entries instead of rejecting when full.
	FIFO bool
	// Touch kinds treat a duplicate create as "use again" and bump the entry.
	Touch bool
	// Idempotent kinds return the existing entry for a duplicate create.
	Idempotent bool
	// UsesRef kinds accept client_ref (idempotent imports of local records).
	UsesRef bool
	// SQL kinds carry SQL text and get the full word-based secret rules.
	SQL        bool
	PerScope   int
	PerOwner   int
	MaxItemLen int // maximum plaintext bytes of one item (data envelope)
}

// Specs lists all kinds in a stable order.
var Specs = []Spec{
	{Kind: KindHistory, FIFO: true, UsesRef: true, SQL: true, PerScope: 200, PerOwner: 5000, MaxItemLen: MaxSQLBytes + 1024},
	{Kind: KindBookmarks, UsesRef: true, SQL: true, PerScope: 200, PerOwner: 2000, MaxItemLen: MaxSQLBytes + 4096},
	{Kind: KindSearches, UsesRef: true, SQL: true, PerScope: 100, PerOwner: 1000, MaxItemLen: 32 << 10},
	{Kind: KindFavorites, Keyed: true, Derived: true, Idempotent: true, PerScope: 500, PerOwner: 5000, MaxItemLen: 2 << 10},
	{Kind: KindRecents, Keyed: true, Derived: true, FIFO: true, Touch: true, Idempotent: true, PerScope: MaxRecentsPerScope, PerOwner: 500, MaxItemLen: 2 << 10},
	{Kind: KindSettings, Keyed: true, PerScope: 100, PerOwner: 200, MaxItemLen: 32 << 10},
	{Kind: KindLayouts, Keyed: true, PerScope: 1000, PerOwner: 2000, MaxItemLen: 64 << 10},
}

// SpecFor returns the policy of a kind.
func SpecFor(kind string) (Spec, bool) {
	for _, spec := range Specs {
		if string(spec.Kind) == kind {
			return spec, true
		}
	}
	return Spec{}, false
}

// Row is one stored item. Data is compact JSON; metadata that is secret-free
// by construction (ids, scope, version, times) travels next to it.
type Row struct {
	Kind      Kind
	ID        string
	Scope     string // keyed hash of the client scope (what the database stores)
	ScopeName string // the client's scope; only ever stored inside the sealed payload
	NKey      string // keyed hash of the natural key, set by the service
	Key       string
	ClientRef string
	Data      []byte
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Requirement is a precondition the repository checks under the owner lock
// before an insert (the history opt-in): the row identified by Kind/Scope/NKey
// must exist and Check must accept its data.
type Requirement struct {
	Kind  Kind
	Scope string
	NKey  string
	Check func(data []byte) bool
}

// Limits is what a Repo enforces atomically on insert and update.
type Limits struct {
	Require    *Requirement
	PerScope   int
	PerOwner   int
	FIFO       bool
	OwnerBytes int64
}

// AnyVersion disables the optimistic-concurrency check (If-Match: *).
const AnyVersion int64 = -1

// ListQuery selects items. Scope "" with AllScopes false means the global scope.
type ListQuery struct {
	Kind      Kind
	Scope     string
	AllScopes bool
	Key       string // exact key filter (hashed by the service into NKey)
	NKey      string
	Limit     int
	After     *Cursor
}

// Cursor continues a listing after the given position (newest first).
type Cursor struct {
	UpdatedAt time.Time
	ID        string
}

// ClearScope selects what a clear operation affects.
type ClearScope struct {
	Kinds     []Kind
	Scope     string
	AllScopes bool
}

// Digest fingerprints the rows a clear would delete.
type Digest struct {
	Counts map[Kind]int
	Bytes  map[Kind]int64
	Hash   string
}

// Usage is the per-kind consumption of one owner.
type Usage struct {
	Counts        map[Kind]int
	Bytes         map[Kind]int64
	SchemaVersion int
}

// Repo is the persistence contract. Implementations must be safe for
// concurrent use and must serialize writes per owner so that limits hold
// under concurrency. They return only the sentinel errors below (or
// context errors), never driver text.
type Repo interface {
	// Insert stores row. When an entry with the same (kind, scope, nkey) exists it
	// returns that entry and ErrExists without modifying anything.
	Insert(ctx context.Context, owner string, row Row, lim Limits) (Row, error)
	Get(ctx context.Context, owner string, kind Kind, id string) (Row, error)
	List(ctx context.Context, owner string, q ListQuery) ([]Row, error)
	// Update replaces the data of an entry; expect is the required current
	// version or AnyVersion. The version is incremented and updated_at bumped.
	Update(ctx context.Context, owner string, kind Kind, id string, data []byte, expect int64, lim Limits) (Row, error)
	Delete(ctx context.Context, owner string, kind Kind, id string, expect int64) error
	Export(ctx context.Context, owner string) ([]Row, error)
	Digest(ctx context.Context, owner string, scope ClearScope) (Digest, error)
	// Clear deletes the selected rows only if their digest still equals hash.
	Clear(ctx context.Context, owner string, scope ClearScope, hash string) (map[Kind]int, error)
	Usage(ctx context.Context, owner string) (Usage, error)
	// IndexKey returns the installation-wide key for hashing scopes and natural
	// keys. It is created once, stored sealed and never changes, so rotating the
	// encryption key does not change any stored hash.
	IndexKey(ctx context.Context) ([]byte, error)
}

// PlainSize is the quota weight of an item: the bytes of its data, key and
// client reference. Every Repo must account quotas with exactly this value.
func PlainSize(key, clientRef string, data []byte) int64 {
	return int64(len(data) + len(key) + len(clientRef))
}
