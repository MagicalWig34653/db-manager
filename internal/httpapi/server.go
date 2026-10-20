// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

const MaxRequestBytes = 1024 * 1024

var databaseID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type API struct {
	engine    core.Engine
	tokenHash [32]byte
	handler   http.Handler
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(engine core.Engine, token string) *API {
	a := &API{engine: engine, tokenHash: sha256.Sum256([]byte(token))}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.Handle("GET /api/v1/capabilities", a.auth(http.HandlerFunc(a.capabilities)))
	mux.Handle("POST /api/v1/connections/test", a.auth(http.HandlerFunc(a.test)))
	mux.Handle("POST /api/v1/catalog", a.auth(http.HandlerFunc(a.catalog)))
	mux.Handle("POST /api/v1/query", a.auth(http.HandlerFunc(a.query)))
	a.registerTables(mux)
	a.handler = mux
	return a
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	a.handler.ServeHTTP(w, r)
}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		hash := sha256.Sum256([]byte(strings.TrimPrefix(raw, "Bearer ")))
		if !strings.HasPrefix(raw, "Bearer ") || subtle.ConstantTimeCompare(hash[:], a.tokenHash[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, 401, "unauthorized", "Anmeldung erforderlich.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		fail(w, 415, "unsupported_media_type", "JSON ist erforderlich.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBytes)
	var body bytes.Buffer
	decoder := json.NewDecoder(io.TeeReader(r.Body, &body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			fail(w, 413, "request_too_large", "Anfrage ist zu groß.")
		} else {
			fail(w, 400, "invalid_request", "Ungültige Anfrage.")
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			fail(w, 413, "request_too_large", "Anfrage ist zu groß.")
		} else {
			fail(w, 400, "invalid_request", "Genau ein JSON-Objekt ist erforderlich.")
		}
		return false
	}
	// None of the request schema fields are nullable. Scan every token so a
	// duplicate object key cannot hide an explicit null from typed decoding.
	tokens := json.NewDecoder(bytes.NewReader(body.Bytes()))
	for {
		token, err := tokens.Token()
		if err == io.EOF {
			break
		}
		if err != nil || token == nil {
			fail(w, 400, "invalid_request", "Nullwerte sind nicht zulässig.")
			return false
		}
	}
	return true
}

func validConnection(w http.ResponseWriter, c core.Connection) bool {
	if (c.Driver != "sqlite" && c.Driver != "mysql" && c.Driver != "postgres") ||
		c.Port < 0 || c.Port > 65535 ||
		(c.TLSMode != "verify-full" && c.TLSMode != "disable") ||
		(c.Driver == "sqlite" && !databaseID.MatchString(c.Database)) {
		fail(w, 400, "invalid_request", "Verbindungsparameter sind ungültig.")
		return false
	}
	return true
}
func (a *API) capabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"contract_version": core.ContractVersion, "drivers": a.engine.Drivers(), "credential_storage": false, "max_rows": 10000, "max_timeout_ms": 60000})
}
func (a *API) test(w http.ResponseWriter, r *http.Request) {
	input := core.CatalogRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := a.engine.Test(ctx, input.Connection); err != nil {
		engineError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (a *API) catalog(w http.ResponseWriter, r *http.Request) {
	input := core.CatalogRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tables, err := a.engine.Catalog(ctx, input.Connection)
	if err != nil {
		engineError(w, err)
		return
	}
	if tables == nil {
		tables = []core.Table{}
	}
	writeJSON(w, 200, map[string]any{"tables": tables})
}
func (a *API) query(w http.ResponseWriter, r *http.Request) {
	input := core.QueryRequest{Connection: core.Connection{TLSMode: "verify-full"}}
	if !decode(w, r, &input) || !validConnection(w, input.Connection) {
		return
	}
	if strings.TrimSpace(input.SQL) == "" || input.MaxRows < 0 || input.MaxRows > 10000 || input.TimeoutMS < 0 || input.TimeoutMS > 60000 {
		fail(w, 400, "invalid_request", "SQL oder Abfragegrenzen sind ungültig.")
		return
	}
	if input.MaxRows == 0 {
		input.MaxRows = 1000
	}
	if input.TimeoutMS == 0 {
		input.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(input.TimeoutMS)*time.Millisecond)
	defer cancel()
	result, err := a.engine.Query(ctx, input)
	if err != nil {
		engineError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func engineError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		fail(w, 408, "query_canceled", "Abfrage abgebrochen oder Zeitlimit erreicht.")
		return
	}
	var public *core.Error
	if errors.As(err, &public) {
		fail(w, 422, public.Code, public.Message)
		return
	}
	fail(w, 422, "database_error", "Datenbankoperation fehlgeschlagen. Verbindung und SQL prüfen.")
}
func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": errorBody{Code: code, Message: message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
