// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Service applies the policy on top of a Repo.
type Service struct {
	repo      Repo
	forbidden []string
	now       func() time.Time
}

// NewService returns a service on repo.
func NewService(repo Repo) *Service { return &Service{repo: repo, now: time.Now} }

// AddForbidden registers literal values (for example the API token) that must
// never be stored. Call before serving requests.
func (s *Service) AddForbidden(values ...string) {
	for _, value := range values {
		if len(value) >= 8 {
			s.forbidden = append(s.forbidden, value)
		}
	}
}

// Item is the API view of a stored entry.
type Item struct {
	ID        string
	Kind      Kind
	Scope     string
	Key       string
	ClientRef string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Data      json.RawMessage
}

func itemOf(row Row) Item {
	return Item{ID: row.ID, Kind: row.Kind, Scope: row.Scope, Key: row.Key, ClientRef: row.ClientRef,
		Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Data: json.RawMessage(row.Data)}
}

// CreateInput is a create or import request for one item.
type CreateInput struct {
	Scope     string
	Key       string
	ClientRef string
	Data      json.RawMessage
	// Time preserves the original time of imported items; nil means now.
	Time *time.Time
}

func limitsOf(spec Spec) Limits {
	return Limits{PerScope: spec.PerScope, PerOwner: spec.PerOwner, FIFO: spec.FIFO, OwnerBytes: MaxOwnerBytes}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", ErrUnavailable
	}
	return hex.EncodeToString(b), nil
}

func specOf(kind string) (Spec, error) {
	spec, ok := SpecFor(kind)
	if !ok {
		return Spec{}, ErrUnknownKind
	}
	return spec, nil
}

func (s *Service) rejectSecret(strs []string, spec Spec) error {
	mode := ScanStructural
	if spec.SQL {
		mode = ScanSQL
	}
	for _, text := range strs {
		if LooksSecret(text, mode) {
			return ErrSecretRejected
		}
		for _, value := range s.forbidden {
			if strings.Contains(text, value) {
				return ErrSecretRejected
			}
		}
	}
	return nil
}

// prepare validates an input completely and builds the row to store.
func (s *Service) prepare(spec Spec, in CreateInput) (Row, error) {
	if !scopePattern.MatchString(in.Scope) {
		return Row{}, ErrInvalid
	}
	if in.ClientRef != "" && (!spec.UsesRef || !refPattern.MatchString(in.ClientRef)) {
		return Row{}, ErrInvalid
	}
	switch {
	case spec.Keyed && !spec.Derived:
		if spec.Kind == KindSettings && !settingsKey.MatchString(in.Key) {
			return Row{}, ErrInvalid
		}
		if spec.Kind == KindLayouts && (in.Key == "" || runes(in.Key) > MaxKeyLen || !okText(in.Key, false)) {
			return Row{}, ErrInvalid
		}
	case in.Key != "":
		return Row{}, ErrInvalid
	}
	if len(in.Data) > spec.MaxItemLen {
		return Row{}, ErrItemTooLarge
	}
	maxString := MaxJSONString
	if spec.SQL {
		maxString = MaxSQLBytes
	}
	data, strs, err := checkJSON(in.Data, maxString)
	if err != nil {
		return Row{}, err
	}
	derived, err := validateData(spec, in.Key, data)
	if err != nil {
		return Row{}, err
	}
	if err := s.rejectSecret(strs, spec); err != nil {
		return Row{}, err
	}
	if in.Key != "" && LooksSecret(in.Key, ScanIdentifier) {
		return Row{}, ErrSecretRejected
	}
	key := in.Key
	if spec.Derived {
		key = derived
	}
	row := Row{Kind: spec.Kind, Scope: in.Scope, Key: key, ClientRef: in.ClientRef, Data: data}
	if in.Time != nil {
		t := in.Time.UTC()
		if t.Year() < 2000 || t.After(s.now().Add(24*time.Hour)) {
			return Row{}, ErrInvalid
		}
		row.CreatedAt, row.UpdatedAt = t, t
	}
	id, err := newID()
	if err != nil {
		return Row{}, err
	}
	row.ID = id
	switch {
	case spec.Keyed:
		row.NKey = hashKey("key", string(spec.Kind), row.Scope, key)
	case row.ClientRef != "":
		row.NKey = hashKey("ref", string(spec.Kind), row.Scope, row.ClientRef)
	default:
		row.NKey = hashKey("id", string(spec.Kind), row.Scope, id)
	}
	return row, nil
}

func (s *Service) historyEnabled(ctx context.Context, owner string) (bool, error) {
	row, err := s.privacyRow(ctx, owner)
	if err != nil || row == nil {
		return false, err
	}
	var v struct {
		On bool `json:"history_enabled"`
	}
	if json.Unmarshal(row.Data, &v) != nil {
		return false, nil
	}
	return v.On, nil
}

func (s *Service) privacyRow(ctx context.Context, owner string) (*Row, error) {
	rows, err := s.repo.List(ctx, owner, ListQuery{Kind: KindSettings, Scope: "", NKey: hashKey("key", string(KindSettings), "", privacyKey), Limit: 1})
	if err != nil {
		return nil, mapRepo(err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// Create stores one item. The bool is true when a new entry was created; for
// idempotent kinds and repeated client references the existing entry is returned.
func (s *Service) Create(ctx context.Context, owner, kind string, in CreateInput) (Item, bool, error) {
	if !validOwner(owner) {
		return Item{}, false, ErrInvalid
	}
	spec, err := specOf(kind)
	if err != nil {
		return Item{}, false, err
	}
	row, err := s.prepare(spec, in)
	if err != nil {
		return Item{}, false, err
	}
	if spec.Kind == KindHistory {
		on, err := s.historyEnabled(ctx, owner)
		if err != nil {
			return Item{}, false, err
		}
		if !on {
			return Item{}, false, ErrHistoryDisabled
		}
	}
	stored, err := s.repo.Insert(ctx, owner, row, limitsOf(spec))
	if errors.Is(err, RepoExists) {
		switch {
		case spec.Touch:
			bumped, err := s.repo.Update(ctx, owner, spec.Kind, stored.ID, stored.Data, AnyVersion, limitsOf(spec))
			if err != nil {
				return Item{}, false, mapRepo(err)
			}
			return itemOf(bumped), false, nil
		case spec.Idempotent || row.ClientRef != "":
			return itemOf(stored), false, nil
		}
		return Item{}, false, ErrAlreadyExists
	}
	if err != nil {
		return Item{}, false, mapRepo(err)
	}
	return itemOf(stored), true, nil
}

// Get returns one item.
func (s *Service) Get(ctx context.Context, owner, kind, id string) (Item, error) {
	spec, err := s.checkRef(owner, kind, id)
	if err != nil {
		return Item{}, err
	}
	row, err := s.repo.Get(ctx, owner, spec.Kind, id)
	if err != nil {
		return Item{}, mapRepo(err)
	}
	return itemOf(row), nil
}

func (s *Service) checkRef(owner, kind, id string) (Spec, error) {
	spec, err := specOf(kind)
	if err != nil {
		return Spec{}, err
	}
	if !validOwner(owner) {
		return Spec{}, ErrInvalid
	}
	if !idPattern.MatchString(id) {
		return Spec{}, ErrNotFound
	}
	return spec, nil
}

// Filter selects a page of items. Scope nil means all scopes.
type Filter struct {
	Scope  *string
	Key    string
	Limit  int
	Cursor string
}

// Page is one page of a listing.
type Page struct {
	Items      []Item
	NextCursor string
}

// List returns items newest first.
func (s *Service) List(ctx context.Context, owner, kind string, f Filter) (Page, error) {
	spec, err := specOf(kind)
	if err != nil {
		return Page{}, err
	}
	if !validOwner(owner) {
		return Page{}, ErrInvalid
	}
	q := ListQuery{Kind: spec.Kind, AllScopes: f.Scope == nil, Limit: f.Limit}
	if f.Scope != nil {
		if !scopePattern.MatchString(*f.Scope) {
			return Page{}, ErrInvalid
		}
		q.Scope = *f.Scope
	}
	if q.Limit == 0 {
		q.Limit = DefaultPage
	}
	if q.Limit < 0 || q.Limit > MaxPage {
		return Page{}, ErrInvalid
	}
	if f.Key != "" {
		if !spec.Keyed || spec.Derived || f.Scope == nil {
			return Page{}, ErrInvalid
		}
		q.NKey = hashKey("key", string(spec.Kind), q.Scope, f.Key)
	}
	if f.Cursor != "" {
		cursor, ok := decodeCursor(f.Cursor)
		if !ok {
			return Page{}, ErrInvalid
		}
		q.After = &cursor
	}
	want := q.Limit
	q.Limit = want + 1
	rows, err := s.repo.List(ctx, owner, q)
	if err != nil {
		return Page{}, mapRepo(err)
	}
	page := Page{Items: []Item{}}
	total := 0
	for i, row := range rows {
		if i >= want || (i > 0 && total+len(row.Data) > MaxPageBytes) {
			last := rows[i-1]
			page.NextCursor = encodeCursor(Cursor{UpdatedAt: last.UpdatedAt, ID: last.ID})
			break
		}
		total += len(row.Data)
		page.Items = append(page.Items, itemOf(row))
	}
	return page, nil
}

func encodeCursor(c Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.UpdatedAt.UnixMicro(), 10) + "." + c.ID))
}

func decodeCursor(text string) (Cursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil || len(raw) > 80 {
		return Cursor{}, false
	}
	micro, id, ok := strings.Cut(string(raw), ".")
	n, perr := strconv.ParseInt(micro, 10, 64)
	if !ok || perr != nil || !idPattern.MatchString(id) {
		return Cursor{}, false
	}
	return Cursor{UpdatedAt: time.UnixMicro(n).UTC(), ID: id}, true
}

// Update replaces the data of an entry when expect matches its version.
func (s *Service) Update(ctx context.Context, owner, kind, id string, data json.RawMessage, expect int64) (Item, error) {
	spec, err := s.checkRef(owner, kind, id)
	if err != nil {
		return Item{}, err
	}
	if spec.Derived {
		return Item{}, ErrInvalid
	}
	current, err := s.repo.Get(ctx, owner, spec.Kind, id)
	if err != nil {
		return Item{}, mapRepo(err)
	}
	row, err := s.prepare(spec, CreateInput{Scope: current.Scope, Key: current.Key, ClientRef: current.ClientRef, Data: data})
	if err != nil {
		return Item{}, err
	}
	if spec.Kind == KindHistory {
		return Item{}, ErrInvalid // history entries are immutable; delete instead
	}
	updated, err := s.repo.Update(ctx, owner, spec.Kind, id, row.Data, expect, limitsOf(spec))
	if err != nil {
		return Item{}, mapRepo(err)
	}
	return itemOf(updated), nil
}

// Delete removes one entry when expect matches its version.
func (s *Service) Delete(ctx context.Context, owner, kind, id string, expect int64) error {
	spec, err := s.checkRef(owner, kind, id)
	if err != nil {
		return err
	}
	return mapRepo(s.repo.Delete(ctx, owner, spec.Kind, id, expect))
}

// ImportItem is one entry of an import batch.
type ImportItem struct {
	Kind string
	CreateInput
}

// ImportResult reports one item.
type ImportResult struct {
	Index  int
	Status string // created, exists, updated, rejected
	ID     string
	Reason string
}

// Import stores a batch item by item. Items are independent: a rejected item
// never prevents the others. Existing entries are kept unless overwrite is set
// and the kind is a document kind (settings, layouts).
func (s *Service) Import(ctx context.Context, owner string, items []ImportItem, overwrite bool) ([]ImportResult, error) {
	if !validOwner(owner) || len(items) == 0 || len(items) > MaxImportItems {
		return nil, ErrInvalid
	}
	results := make([]ImportResult, len(items))
	historyOn, historyChecked := false, false
	for i, item := range items {
		result := ImportResult{Index: i, Status: "rejected"}
		reject := func(err error) {
			var public *Error
			if errors.As(err, &public) {
				result.Reason = public.Code
			} else {
				result.Reason = ErrUnavailable.Code
			}
		}
		spec, err := specOf(item.Kind)
		if err != nil {
			reject(err)
			results[i] = result
			continue
		}
		row, err := s.prepare(spec, item.CreateInput)
		if err != nil {
			reject(err)
			results[i] = result
			continue
		}
		if spec.Kind == KindHistory {
			if !historyChecked {
				var herr error
				historyOn, herr = s.historyEnabled(ctx, owner)
				if herr != nil {
					return nil, herr
				}
				historyChecked = true
			}
			if !historyOn {
				reject(ErrHistoryDisabled)
				results[i] = result
				continue
			}
		}
		stored, err := s.repo.Insert(ctx, owner, row, limitsOf(spec))
		switch {
		case err == nil:
			result.Status, result.ID = "created", stored.ID
		case errors.Is(err, RepoExists):
			result.ID = stored.ID
			if overwrite && spec.Keyed && !spec.Derived {
				if _, uerr := s.repo.Update(ctx, owner, spec.Kind, stored.ID, row.Data, AnyVersion, limitsOf(spec)); uerr != nil {
					reject(mapRepo(uerr))
					break
				}
				result.Status = "updated"
			} else {
				result.Status = "exists"
			}
		case errors.Is(err, RepoUnavailable):
			return nil, ErrUnavailable
		default:
			reject(mapRepo(err))
		}
		results[i] = result
	}
	return results, nil
}

// ExportItem is one entry of a complete export.
type ExportItem struct {
	Kind      Kind
	Scope     string
	Key       string
	ClientRef string
	Time      time.Time
	Data      json.RawMessage
}

// Export returns every entry of the owner. Records without a client reference
// get their id as reference so that re-importing the export is idempotent.
func (s *Service) Export(ctx context.Context, owner string) ([]ExportItem, error) {
	if !validOwner(owner) {
		return nil, ErrInvalid
	}
	rows, err := s.repo.Export(ctx, owner)
	if err != nil {
		return nil, mapRepo(err)
	}
	items := make([]ExportItem, 0, len(rows))
	for _, row := range rows {
		spec, _ := SpecFor(string(row.Kind))
		item := ExportItem{Kind: row.Kind, Scope: row.Scope, Time: row.UpdatedAt, Data: json.RawMessage(row.Data)}
		if spec.Keyed && !spec.Derived {
			item.Key = row.Key
		}
		if spec.UsesRef {
			item.ClientRef = row.ClientRef
			if item.ClientRef == "" {
				item.ClientRef = row.ID
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// KindCount is one line of a clear plan.
type KindCount struct {
	Kind  Kind
	Count int
	Bytes int64
}

// ClearPlan is the preview of a clear operation.
type ClearPlan struct {
	Kinds        []KindCount
	TotalItems   int
	TotalBytes   int64
	PlanHash     string
	Confirmation string
}

func (s *Service) clearScope(kinds []string, scope *string) (ClearScope, error) {
	if len(kinds) == 0 || len(kinds) > MaxClearKinds {
		return ClearScope{}, ErrInvalid
	}
	out := ClearScope{AllScopes: scope == nil}
	if scope != nil {
		if !scopePattern.MatchString(*scope) {
			return ClearScope{}, ErrInvalid
		}
		out.Scope = *scope
	}
	seen := map[string]bool{}
	for _, name := range kinds {
		spec, err := specOf(name)
		if err != nil || seen[name] {
			return ClearScope{}, ErrInvalid
		}
		seen[name] = true
		out.Kinds = append(out.Kinds, spec.Kind)
	}
	return out, nil
}

func confirmationFor(total int) string { return fmt.Sprintf("LÖSCHEN %d", total) }

// PlanClear previews deleting whole collections (optionally for one scope).
func (s *Service) PlanClear(ctx context.Context, owner string, kinds []string, scope *string) (ClearPlan, error) {
	if !validOwner(owner) {
		return ClearPlan{}, ErrInvalid
	}
	cs, err := s.clearScope(kinds, scope)
	if err != nil {
		return ClearPlan{}, err
	}
	digest, err := s.repo.Digest(ctx, owner, cs)
	if err != nil {
		return ClearPlan{}, mapRepo(err)
	}
	plan := ClearPlan{PlanHash: digest.Hash}
	for _, kind := range cs.Kinds {
		plan.Kinds = append(plan.Kinds, KindCount{Kind: kind, Count: digest.Counts[kind], Bytes: digest.Bytes[kind]})
		plan.TotalItems += digest.Counts[kind]
		plan.TotalBytes += digest.Bytes[kind]
	}
	plan.Confirmation = confirmationFor(plan.TotalItems)
	return plan, nil
}

// ExecuteClear deletes what the plan showed, if nothing changed meanwhile and
// the typed confirmation matches.
func (s *Service) ExecuteClear(ctx context.Context, owner string, kinds []string, scope *string, planHash, confirmation string) (map[Kind]int, error) {
	if !validOwner(owner) {
		return nil, ErrInvalid
	}
	cs, err := s.clearScope(kinds, scope)
	if err != nil {
		return nil, err
	}
	digest, err := s.repo.Digest(ctx, owner, cs)
	if err != nil {
		return nil, mapRepo(err)
	}
	total := 0
	for _, kind := range cs.Kinds {
		total += digest.Counts[kind]
	}
	if confirmation != confirmationFor(total) {
		if digest.Hash != planHash {
			return nil, ErrPlanStale
		}
		return nil, ErrConfirmation
	}
	deleted, err := s.repo.Clear(ctx, owner, cs, planHash)
	if err != nil {
		return nil, mapRepo(err)
	}
	return deleted, nil
}

// Status is the usage report of the owner.
type Status struct {
	Usage         []KindCount
	TotalBytes    int64
	SchemaVersion int
}

// Status reports consumption and limits for the owner.
func (s *Service) Status(ctx context.Context, owner string) (Status, error) {
	if !validOwner(owner) {
		return Status{}, ErrInvalid
	}
	usage, err := s.repo.Usage(ctx, owner)
	if err != nil {
		return Status{}, mapRepo(err)
	}
	out := Status{SchemaVersion: usage.SchemaVersion}
	for _, spec := range Specs {
		out.Usage = append(out.Usage, KindCount{Kind: spec.Kind, Count: usage.Counts[spec.Kind], Bytes: usage.Bytes[spec.Kind]})
		out.TotalBytes += usage.Bytes[spec.Kind]
	}
	return out, nil
}
