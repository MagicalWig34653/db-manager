// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"net/http"
)

func (a *API) registerAccounts(mux *http.ServeMux) {
	mux.Handle("GET /api/v18/capabilities", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, core.AccountsCapabilities{ContractVersion: core.AccountsContractVersion, Drivers: core.AccountCapabilities()})
	})))
	mux.Handle("POST /api/v18/accounts/access", a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input core.AccountRequest
		input.Connection.TLSMode = "verify-full"
		if !decodeAccounts(w, r, "access", false, &input) || !validConnection(w, input.Connection) {
			return
		}
		engine, ok := a.engine.(core.AccountsAccessEngine)
		if !ok {
			fail(w, 422, "unsupported_feature", "Rechteanzeige ist nicht verfügbar.")
			return
		}
		result, err := engine.AccountAccess(r.Context(), input)
		if err != nil {
			accountsFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})))
	for _, kind := range []string{"list", "inspect"} {
		mux.Handle("POST /api/v18/accounts/"+kind, a.auth(a.accountsRead(kind)))
	}
	for _, action := range core.AccountActions {
		mux.Handle("POST /api/v18/accounts/"+action+"/plan", a.auth(a.accountsWrite(action, false)))
		mux.Handle("POST /api/v18/accounts/"+action, a.auth(a.accountsWrite(action, true)))
	}
}
func (a *API) accountsEngine(w http.ResponseWriter) (core.AccountsEngine, bool) {
	engine, ok := a.engine.(core.AccountsEngine)
	if !ok {
		fail(w, 422, "unsupported_feature", "Kontenverwaltung ist nicht verfügbar.")
	}
	return engine, ok
}
func accountsFail(w http.ResponseWriter, err error) {
	var p *core.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "invalid_account_request", "confirmation_mismatch":
			fail(w, 400, p.Code, p.Message)
			return
		case "plan_mismatch", "object_exists":
			fail(w, 409, p.Code, p.Message)
			return
		case "object_not_found":
			fail(w, 404, p.Code, p.Message)
			return
		case "permission_denied":
			fail(w, 403, p.Code, p.Message)
			return
		}
	}
	engineError(w, err)
}
func (a *API) accountsRead(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input core.AccountRequest
		input.Connection.TLSMode = "verify-full"
		if !decodeAccounts(w, r, kind, false, &input) || !validConnection(w, input.Connection) {
			return
		}
		engine, ok := a.accountsEngine(w)
		if !ok {
			return
		}
		if kind == "list" {
			result, err := engine.Accounts(r.Context(), input)
			if err != nil {
				accountsFail(w, err)
				return
			}
			writeJSON(w, 200, result)
		} else {
			result, err := engine.AccountDetails(r.Context(), input)
			if err != nil {
				accountsFail(w, err)
				return
			}
			writeJSON(w, 200, result)
		}
	})
}
func (a *API) accountsWrite(action string, execute bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !execute {
			var input core.AccountRequest
			input.Connection.TLSMode = "verify-full"
			if !decodeAccounts(w, r, action, false, &input) || !validConnection(w, input.Connection) {
				return
			}
			engine, ok := a.accountsEngine(w)
			if !ok {
				return
			}
			result, err := engine.AccountPlan(r.Context(), action, input)
			if err != nil {
				accountsFail(w, err)
				return
			}
			writeJSON(w, 200, result)
			return
		}
		var input core.AccountExecuteRequest
		input.Connection.TLSMode = "verify-full"
		if !decodeAccounts(w, r, action, true, &input) || !validConnection(w, input.Connection) {
			return
		}
		engine, ok := a.accountsEngine(w)
		if !ok {
			return
		}
		result, err := engine.AccountExecute(r.Context(), action, input)
		if err != nil {
			accountsFail(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
}

// decodeAccounts enforces action-specific field presence before typed decoding.
// Zero values of forbidden fields are forbidden too; secrets never reach errors.
func decodeAccounts(w http.ResponseWriter, r *http.Request, action string, execute bool, target any) bool {
	var fields map[string]json.RawMessage
	if !decode(w, r, &fields) {
		return false
	}
	allowed := map[string]bool{"connection": true, "timeout_ms": true}
	required := []string{"connection"}
	add := func(name string) { allowed[name] = true; required = append(required, name) }
	if action == "list" {
		allowed["limit"] = true
		allowed["offset"] = true
	}
	if action != "list" && action != "access" && action != "flush" && action != "create" {
		add("account")
	}
	if action == "create" || action == "rename" || action == "copy" {
		add("new_account")
	}
	if action == "grant" || action == "revoke" {
		add("grant")
	}
	if action == "role_grant" || action == "role_revoke" {
		add("role")
	}
	if execute {
		add("confirm")
		add("expected_plan_sha256")
		if action == "create" || action == "copy" || action == "password" {
			add("secret")
		}
	}
	invalid := func() bool {
		fail(w, 400, "invalid_request", "Die Felder entsprechen nicht der Kontenaktion.")
		return false
	}
	for name := range fields {
		if !allowed[name] {
			return invalid()
		}
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return invalid()
		}
	}
	if raw, ok := fields["grant"]; ok {
		var grant map[string]json.RawMessage
		if json.Unmarshal(raw, &grant) != nil {
			return invalid()
		}
		for _, name := range []string{"scope", "privilege", "grant_option"} {
			if _, ok := grant[name]; !ok {
				return invalid()
			}
		}
	}
	if raw, ok := fields["confirm"]; ok {
		var confirm core.AccountConfirm
		if json.Unmarshal(raw, &confirm) != nil || confirm.Action != action {
			return invalid()
		}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return invalid()
	}
	return true
}
