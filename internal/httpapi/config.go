// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/configstore"
)

// OwnerResolver maps an authenticated request to the owner of comfort data.
// It is the single seam between the configuration store and whatever identity
// the server has: today every request belongs to one constant owner; W-P30 will
// return the account identifier of the session instead.
type OwnerResolver func(r *http.Request) (owner string, ok bool)

// SingleOwner is the identity model of the shared Bearer token: one owner.
const SingleOwner = "single"

// WithConfigStore enables the server-side comfort data routes (API 0.24).
func WithConfigStore(service *configstore.Service) Option {
	return func(a *API) { a.config = service }
}

// WithOwnerResolver replaces the single-owner identity (used by W-P30 and tests).
// model names the isolation guarantee reported by the capability route.
func WithOwnerResolver(resolve OwnerResolver, model string) Option {
	return func(a *API) { a.owners, a.ownerModel = resolve, model }
}

func (a *API) owner(w http.ResponseWriter, r *http.Request) (string, bool) {
	if a.owners == nil {
		return SingleOwner, true
	}
	owner, ok := a.owners(r)
	if !ok {
		fail(w, http.StatusUnauthorized, "unauthorized", "Anmeldung erforderlich.")
	}
	return owner, ok
}

func configFail(w http.ResponseWriter, err error) {
	var public *configstore.Error
	if errors.As(err, &public) {
		if public.Status == http.StatusServiceUnavailable {
			w.Header().Set("Retry-After", "5")
		}
		fail(w, public.Status, public.Code, public.Message)
		return
	}
	fail(w, http.StatusInternalServerError, "internal_error", "Interner Fehler.")
}

// configRoute guards a route that needs the server state: Bearer auth first,
// then fail-closed 503 while the store is unconfigured or unreachable.
func (a *API) configRoute(next http.HandlerFunc) http.Handler {
	return a.auth(a.RequireState(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.config == nil {
			w.Header().Set("Retry-After", "5")
			fail(w, http.StatusServiceUnavailable, "state_unavailable", "Serverzustand ist nicht erreichbar.")
			return
		}
		next(w, r)
	})))
}

var (
	ifMatchPattern = regexp.MustCompile(`^"([0-9]{1,18})"$`)
	cursorChars    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,120}$`)
)

// expectedVersion parses If-Match: a quoted version or "*" (any version).
func expectedVersion(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.Header.Get("If-Match")
	if raw == "" {
		configFail(w, configstore.ErrPreconditionNeed)
		return 0, false
	}
	if raw == "*" {
		return configstore.AnyVersion, true
	}
	match := ifMatchPattern.FindStringSubmatch(raw)
	if match == nil {
		configFail(w, configstore.ErrInvalid)
		return 0, false
	}
	version, _ := strconv.ParseInt(match[1], 10, 64)
	return version, true
}

const timeLayout = "2006-01-02T15:04:05.000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

type configItem struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Scope     string          `json:"scope"`
	Key       string          `json:"key"`
	ClientRef string          `json:"client_ref"`
	Version   int64           `json:"version"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Data      json.RawMessage `json:"data"`
}

func viewOf(item configstore.Item) configItem {
	return configItem{ID: item.ID, Kind: string(item.Kind), Scope: item.Scope, Key: item.Key, ClientRef: item.ClientRef,
		Version: item.Version, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt), Data: item.Data}
}

func writeItem(w http.ResponseWriter, status int, item configstore.Item) {
	w.Header().Set("ETag", `"`+strconv.FormatInt(item.Version, 10)+`"`)
	if status == http.StatusCreated {
		w.Header().Set("Location", "/api/v24/config/"+string(item.Kind)+"/"+item.ID)
	}
	writeJSON(w, status, viewOf(item))
}

func (a *API) registerConfigStore(mux *http.ServeMux) {
	mux.Handle("GET /api/v24/capabilities", a.auth(http.HandlerFunc(a.configCapabilities)))
	mux.Handle("GET /api/v24/config/status", a.configRoute(a.configStatus))
	mux.Handle("GET /api/v24/config/export", a.configRoute(a.configExport))
	mux.Handle("POST /api/v24/config/import", a.configRoute(a.configImport))
	mux.Handle("POST /api/v24/config/clear/plan", a.configRoute(a.configClearPlan))
	mux.Handle("POST /api/v24/config/clear/execute", a.configRoute(a.configClearExecute))
	mux.Handle("GET /api/v24/config/{kind}", a.configRoute(a.configList))
	mux.Handle("POST /api/v24/config/{kind}", a.configRoute(a.configCreate))
	mux.Handle("GET /api/v24/config/{kind}/{id}", a.configRoute(a.configGet))
	mux.Handle("PUT /api/v24/config/{kind}/{id}", a.configRoute(a.configUpdate))
	mux.Handle("DELETE /api/v24/config/{kind}/{id}", a.configRoute(a.configDelete))
}

func (a *API) configCapabilities(w http.ResponseWriter, r *http.Request) {
	available, reason := false, "ok"
	switch {
	case a.state == nil || a.config == nil:
		reason = "not_configured"
	case !a.state.Healthy():
		reason = "unreachable"
	default:
		available = true
	}
	mode := "local_only"
	if available {
		mode = "server"
	}
	model := a.ownerModel
	if model == "" {
		model = "single"
	}
	kinds := make([]map[string]any, 0, len(configstore.Specs))
	for _, spec := range configstore.Specs {
		kinds = append(kinds, map[string]any{"kind": string(spec.Kind), "keyed": spec.Keyed && !spec.Derived, "fifo": spec.FIFO,
			"max_per_scope": spec.PerScope, "max_per_owner": spec.PerOwner, "max_item_bytes": spec.MaxItemLen})
	}
	writeJSON(w, 200, map[string]any{
		"contract_version": configstore.ContractVersion, "available": available, "mode": mode, "reason": reason,
		"owner_model": model, "history_requires_opt_in": true, "secrets_policy": "reject", "kinds": kinds,
		"max_owner_bytes": configstore.MaxOwnerBytes, "max_page": configstore.MaxPage, "default_page": configstore.DefaultPage,
		"max_import_items": configstore.MaxImportItems, "max_request_bytes": MaxRequestBytes,
	})
}

func (a *API) configStatus(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	status, err := a.config.Status(r.Context(), owner)
	if err != nil {
		configFail(w, err)
		return
	}
	usage := make([]map[string]any, 0, len(status.Usage))
	for _, line := range status.Usage {
		spec, _ := configstore.SpecFor(string(line.Kind))
		usage = append(usage, map[string]any{"kind": string(line.Kind), "count": line.Count, "bytes": line.Bytes, "max_items": spec.PerOwner})
	}
	writeJSON(w, 200, map[string]any{"schema_version": status.SchemaVersion, "usage": usage, "total_bytes": status.TotalBytes, "max_owner_bytes": configstore.MaxOwnerBytes})
}

func (a *API) configList(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	if !onlyParams(query, "scope", "key", "limit", "cursor") {
		configFail(w, configstore.ErrInvalid)
		return
	}
	filter := configstore.Filter{Key: query.Get("key"), Cursor: query.Get("cursor")}
	if _, present := query["scope"]; present {
		scope := query.Get("scope")
		filter.Scope = &scope
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			configFail(w, configstore.ErrInvalid)
			return
		}
		filter.Limit = limit
	}
	if filter.Cursor != "" && !cursorChars.MatchString(filter.Cursor) {
		configFail(w, configstore.ErrInvalid)
		return
	}
	page, err := a.config.List(r.Context(), owner, r.PathValue("kind"), filter)
	if err != nil {
		configFail(w, err)
		return
	}
	items := make([]configItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, viewOf(item))
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": page.NextCursor})
}

func onlyParams(values url.Values, allowed ...string) bool {
	for name, list := range values {
		known := false
		for _, a := range allowed {
			known = known || a == name
		}
		if !known || len(list) != 1 {
			return false
		}
	}
	return true
}

type configCreateBody struct {
	Scope     string          `json:"scope"`
	Key       string          `json:"key"`
	ClientRef string          `json:"client_ref"`
	Data      json.RawMessage `json:"data"`
}

func (a *API) configCreate(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	var body configCreateBody
	if !decode(w, r, &body) {
		return
	}
	item, created, err := a.config.Create(r.Context(), owner, r.PathValue("kind"), configstore.CreateInput{Scope: body.Scope, Key: body.Key, ClientRef: body.ClientRef, Data: body.Data})
	if err != nil {
		configFail(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeItem(w, status, item)
}

func (a *API) configGet(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	item, err := a.config.Get(r.Context(), owner, r.PathValue("kind"), r.PathValue("id"))
	if err != nil {
		configFail(w, err)
		return
	}
	writeItem(w, http.StatusOK, item)
}

func (a *API) configUpdate(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	expect, ok := expectedVersion(w, r)
	if !ok {
		return
	}
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if !decode(w, r, &body) {
		return
	}
	item, err := a.config.Update(r.Context(), owner, r.PathValue("kind"), r.PathValue("id"), body.Data, expect)
	if err != nil {
		configFail(w, err)
		return
	}
	writeItem(w, http.StatusOK, item)
}

func (a *API) configDelete(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	expect, ok := expectedVersion(w, r)
	if !ok {
		return
	}
	if err := a.config.Delete(r.Context(), owner, r.PathValue("kind"), r.PathValue("id"), expect); err != nil {
		configFail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type configImportItem struct {
	Kind      string          `json:"kind"`
	Scope     string          `json:"scope"`
	Key       string          `json:"key"`
	ClientRef string          `json:"client_ref"`
	Time      string          `json:"time"`
	Data      json.RawMessage `json:"data"`
}

func (a *API) configImport(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	var body struct {
		Items              []configImportItem `json:"items"`
		OverwriteDocuments bool               `json:"overwrite_documents"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Items) == 0 || len(body.Items) > configstore.MaxImportItems {
		configFail(w, configstore.ErrInvalid)
		return
	}
	items := make([]configstore.ImportItem, len(body.Items))
	badTime := make([]bool, len(body.Items))
	for i, in := range body.Items {
		items[i] = configstore.ImportItem{Kind: in.Kind, CreateInput: configstore.CreateInput{Scope: in.Scope, Key: in.Key, ClientRef: in.ClientRef, Data: in.Data}}
		if in.Time != "" {
			t, err := time.Parse(time.RFC3339Nano, in.Time)
			if err != nil {
				badTime[i] = true
				items[i].Kind = "" // not stored; reported as rejected below
				continue
			}
			items[i].Time = &t
		}
	}
	results, err := a.config.Import(r.Context(), owner, items, body.OverwriteDocuments)
	if err != nil {
		configFail(w, err)
		return
	}
	counts := map[string]int{}
	out := make([]map[string]any, len(results))
	for i, result := range results {
		if badTime[i] {
			result.Status, result.Reason, result.ID = "rejected", configstore.ErrInvalid.Code, ""
		}
		counts[result.Status]++
		out[i] = map[string]any{"index": result.Index, "status": result.Status, "id": result.ID, "reason": result.Reason}
	}
	writeJSON(w, 200, map[string]any{"results": out, "created": counts["created"], "exists": counts["exists"], "updated": counts["updated"], "rejected": counts["rejected"]})
}

func (a *API) configExport(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	items, err := a.config.Export(r.Context(), owner)
	if err != nil {
		configFail(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"kind": string(item.Kind), "scope": item.Scope, "key": item.Key, "client_ref": item.ClientRef, "time": formatTime(item.Time), "data": item.Data})
	}
	w.Header().Set("Content-Disposition", `attachment; filename="db-manager-config.json"`)
	writeJSON(w, 200, map[string]any{"format": "dbm-config-export", "format_version": 1, "exported_at": formatTime(time.Now()), "items": out})
}

func (a *API) configClearPlan(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	var body struct {
		Kinds []string `json:"kinds"`
		Scope *string  `json:"scope"`
	}
	if !decode(w, r, &body) {
		return
	}
	plan, err := a.config.PlanClear(r.Context(), owner, body.Kinds, body.Scope)
	if err != nil {
		configFail(w, err)
		return
	}
	kinds := make([]map[string]any, 0, len(plan.Kinds))
	for _, line := range plan.Kinds {
		kinds = append(kinds, map[string]any{"kind": string(line.Kind), "count": line.Count, "bytes": line.Bytes})
	}
	writeJSON(w, 200, map[string]any{"kinds": kinds, "total_items": plan.TotalItems, "total_bytes": plan.TotalBytes, "plan_hash": plan.PlanHash, "confirmation": plan.Confirmation})
}

func (a *API) configClearExecute(w http.ResponseWriter, r *http.Request) {
	owner, ok := a.owner(w, r)
	if !ok {
		return
	}
	var body struct {
		Kinds        []string `json:"kinds"`
		Scope        *string  `json:"scope"`
		PlanHash     string   `json:"plan_hash"`
		Confirmation string   `json:"confirmation"`
	}
	if !decode(w, r, &body) {
		return
	}
	deleted, err := a.config.ExecuteClear(r.Context(), owner, body.Kinds, body.Scope, body.PlanHash, body.Confirmation)
	if err != nil {
		configFail(w, err)
		return
	}
	lines, total := make([]map[string]any, 0, len(body.Kinds)), 0
	for _, name := range body.Kinds {
		n := deleted[configstore.Kind(name)]
		total += n
		lines = append(lines, map[string]any{"kind": name, "count": n})
	}
	writeJSON(w, 200, map[string]any{"deleted": lines, "total": total})
}
