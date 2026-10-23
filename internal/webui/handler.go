// SPDX-License-Identifier: AGPL-3.0-only
package webui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

//go:embed static
var bundled embed.FS

const contentSecurityPolicy = "default-src 'none'; script-src 'self'; script-src-attr 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"

var immutableAsset = regexp.MustCompile(`^assets/[A-Za-z0-9._-]+-[A-Za-z0-9_-]{8,}\.(js|css)$`)

// Wrap serves the bundled UI and delegates API, liveness and readiness requests without alteration.
func Wrap(api http.Handler) http.Handler {
	assets, err := fs.Sub(bundled, "static")
	if err != nil {
		panic("embedded web assets are missing")
	}
	return wrapFS(assets, api)
}

func wrapFS(assets fs.FS, api http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") ||
			r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/healthz/") ||
			r.URL.Path == "/readyz" || strings.HasPrefix(r.URL.Path, "/readyz/") {
			api.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Methode nicht erlaubt.", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if r.URL.Path == "/" {
			name = "index.html"
		} else if path.Clean(r.URL.Path) != r.URL.Path || strings.ContainsAny(name, "\\\x00") || !fs.ValidPath(name) {
			http.Error(w, "Nicht gefunden.", http.StatusNotFound)
			return
		}
		mediaType := staticMediaType(name)
		if mediaType == "" {
			http.Error(w, "Nicht gefunden.", http.StatusNotFound)
			return
		}
		content, err := fs.ReadFile(assets, name)
		if err != nil {
			http.Error(w, "Nicht gefunden.", http.StatusNotFound)
			return
		}
		if immutableAsset.MatchString(name) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		w.Header().Set("Content-Type", mediaType)
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(content)))
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(content))
	})
}

func staticMediaType(name string) string {
	if name == "index.html" {
		return "text/html; charset=utf-8"
	}
	if name == "LICENSE.txt" || name == "THIRD-PARTY-NOTICES.txt" {
		return "text/plain; charset=utf-8"
	}
	if !strings.HasPrefix(name, "assets/") {
		return ""
	}
	switch path.Ext(name) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	default:
		return ""
	}
}
