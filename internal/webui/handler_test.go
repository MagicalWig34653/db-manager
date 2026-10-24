// SPDX-License-Identifier: AGPL-3.0-only
package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testAssets() fs.FS {
	return fstest.MapFS{
		"index.html":                {Data: []byte(`<!doctype html><html lang="de"><title>Arbeitsbereich</title></html>`)},
		"assets/index-abcdefgh.js":  {Data: []byte(`const app = "synthetic";`)},
		"assets/index-abcdefgh.css": {Data: []byte(`body { color: black; }`)},
		"assets/unhashed.js":        {Data: []byte(`const other = true;`)},
		"assets/hidden.map":         {Data: []byte(`{"sources":[]}`)},
		"secret.json":               {Data: []byte(`{"not":"public"}`)},
		"manifest.webmanifest":      {Data: []byte(`{"name":"DB Manager"}`)},
		"sw.js":                     {Data: []byte(`self.addEventListener("fetch", () => {});`)},
		"icon.svg":                  {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		"icon-192.png":              {Data: []byte("\x89PNG")},
		"icon-512.png":              {Data: []byte("\x89PNG")},
		"icon-maskable-512.png":     {Data: []byte("\x89PNG")},
		"apple-touch-icon.png":      {Data: []byte("\x89PNG")},
		"favicon-32.png":            {Data: []byte("\x89PNG")},
		"icon-unlisted.png":         {Data: []byte("\x89PNG")},
		"sw.js.map":                 {Data: []byte("{}")},
		"LICENSE.txt":               {Data: []byte("GNU AFFERO GENERAL PUBLIC LICENSE")},
		"THIRD-PARTY-NOTICES.txt":   {Data: []byte("Original third-party notices")},
	}
}

func TestAPIPassthroughPreservesRequestAndResponse(t *testing.T) {
	for _, target := range []string{"/api", "/api/v1/query?synthetic=1", "/api/../index.html", "/api/v1/missing", "/healthz", "/healthz/extra"} {
		t.Run(target, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("synthetic-body"))
			request.Header.Set("Authorization", "Bearer synthetic-token")
			called := false
			expectedStatus := http.StatusUnprocessableEntity
			if strings.Contains(target, "missing") {
				expectedStatus = http.StatusNotFound
			}
			if target == "/api" {
				expectedStatus = http.StatusUnauthorized
			}
			if target == "/healthz" {
				expectedStatus = http.StatusOK
			}
			api := http.HandlerFunc(func(w http.ResponseWriter, got *http.Request) {
				called = true
				if got != request {
					t.Fatal("API request was replaced or normalized")
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Referrer-Policy", "api-sentinel")
				w.WriteHeader(expectedStatus)
				_, _ = w.Write([]byte(`{"error":{"code":"synthetic","message":"API"}}`))
			})
			recorder := httptest.NewRecorder()
			wrapFS(testAssets(), api).ServeHTTP(recorder, request)
			if !called || recorder.Code != expectedStatus || !strings.Contains(recorder.Body.String(), `"code":"synthetic"`) {
				t.Fatal("API status/body was masked by the UI")
			}
			if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Referrer-Policy") != "api-sentinel" || recorder.Header().Get("Content-Security-Policy") != "" {
				t.Fatal("API response headers were altered")
			}
		})
	}
}

func TestStaticMIMECacheAndSecurityPolicy(t *testing.T) {
	handler := wrapFS(testAssets(), http.NotFoundHandler())
	for _, entry := range []struct {
		target, mediaType string
		immutable         bool
	}{
		{"/", "text/html; charset=utf-8", false},
		{"/index.html", "text/html; charset=utf-8", false},
		{"/assets/index-abcdefgh.js", "text/javascript; charset=utf-8", true},
		{"/assets/index-abcdefgh.css", "text/css; charset=utf-8", true},
		{"/assets/unhashed.js", "text/javascript; charset=utf-8", false},
		{"/manifest.webmanifest", "application/manifest+json", false},
		{"/sw.js", "text/javascript; charset=utf-8", false},
		{"/icon.svg", "image/svg+xml", false},
		{"/icon-192.png", "image/png", false},
		{"/icon-512.png", "image/png", false},
		{"/icon-maskable-512.png", "image/png", false},
		{"/apple-touch-icon.png", "image/png", false},
		{"/favicon-32.png", "image/png", false},
		{"/LICENSE.txt", "text/plain; charset=utf-8", false},
		{"/THIRD-PARTY-NOTICES.txt", "text/plain; charset=utf-8", false},
	} {
		t.Run(entry.target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, entry.target, nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != entry.mediaType {
				t.Fatalf("wrong status/MIME: %d %s", recorder.Code, recorder.Header().Get("Content-Type"))
			}
			if strings.Contains(recorder.Header().Get("Cache-Control"), "immutable") != entry.immutable {
				t.Fatal("wrong immutable-cache policy")
			}
			if entry.target == "/" && recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("index must not be stored")
			}
			if !immutableAsset.MatchString(strings.TrimPrefix(entry.target, "/")) && recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("ungehashte Datei muss no-store liefern")
			}
			if recorder.Header().Get("Service-Worker-Allowed") != "" {
				t.Fatal("service worker scope must not be widened")
			}
			policy := recorder.Header().Get("Content-Security-Policy")
			if strings.Contains(policy, "unsafe-eval") || !strings.Contains(policy, "script-src 'self';") || !strings.Contains(policy, "script-src-attr 'none';") || !strings.Contains(policy, "frame-ancestors 'none'") || !strings.Contains(policy, "manifest-src 'self';") || !strings.Contains(policy, "worker-src 'self';") {
				t.Fatal("script/frame CSP is insufficient")
			}
			if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Referrer-Policy") != "no-referrer" || recorder.Header().Get("Cross-Origin-Resource-Policy") != "same-origin" {
				t.Fatal("static security headers are missing")
			}
		})
	}
}

func TestStaticTraversalUnknownPathsAndMethods(t *testing.T) {
	handler := wrapFS(testAssets(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("static path reached the API") }))
	for _, target := range []string{"/assets/", "/assets", "/../LICENSE.txt", "/assets/../index.html", "/assets/%2e%2e/index.html", "/%2e%2e/", "/assets%5cindex-abcdefgh.js", "/secret.json", "/assets/hidden.map", "/node_modules/package.json", "/unknown", "/icon-unlisted.png", "/sw.js.map", "/sw.js/", "/Sw.js", "/%2e%2e/sw.js", "/assets/sw.js", "/manifest.json", "/api-missing", "/healthz-missing"} {
		t.Run(target, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "<!doctype") || strings.Contains(recorder.Body.String(), "directory") {
				t.Fatalf("unknown/traversal path exposed UI content: %d %q", recorder.Code, recorder.Body.String())
			}
		})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET, HEAD" {
		t.Fatal("static POST was not rejected")
	}
}

func TestStaticHEADConditionalAndRange(t *testing.T) {
	handler := wrapFS(testAssets(), http.NotFoundHandler())
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/assets/index-abcdefgh.js", nil))
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/assets/index-abcdefgh.js", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") {
		t.Fatal("HEAD has wrong status/body/length")
	}
	conditional := httptest.NewRequest(http.MethodGet, "/assets/index-abcdefgh.js", nil)
	conditional.Header.Set("If-None-Match", get.Header().Get("ETag"))
	notModified := httptest.NewRecorder()
	handler.ServeHTTP(notModified, conditional)
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatal("ETag revalidation failed")
	}
	ranged := httptest.NewRequest(http.MethodGet, "/assets/index-abcdefgh.js", nil)
	ranged.Header.Set("Range", "bytes=0-4")
	partial := httptest.NewRecorder()
	handler.ServeHTTP(partial, ranged)
	if partial.Code != http.StatusPartialContent || partial.Body.String() != "const" {
		t.Fatal("byte range handling failed")
	}
}

func TestBundledAssetsMatchManifest(t *testing.T) {
	manifestBytes, err := fs.ReadFile(manifestFS, "assets-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Source struct {
			Repository  string `json:"repository"`
			Commit      string `json:"commit"`
			ContractSHA string `json:"contract_sha256"`
		} `json:"source"`
		Files []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
			Size int    `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Source.Repository != "https://github.com/MagicalWig34653/db-manager-web.git" || len(manifest.Source.Commit) != 40 || manifest.Source.ContractSHA != "4b598dc69a1904685e67a717997412654440ac10f47c17f7a025ccd114e294be" {
		t.Fatal("frontend provenance is missing or incompatible")
	}
	assets, err := fs.Sub(bundled, "static")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if seen[file.Path] {
			t.Fatalf("duplicate manifest entry: %s", file.Path)
		}
		seen[file.Path] = true
		data, err := fs.ReadFile(assets, file.Path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if len(data) != file.Size || hex.EncodeToString(sum[:]) != file.SHA {
			t.Fatalf("modified embedded artifact: %s", file.Path)
		}
		if strings.Contains(file.Path, "node_modules") || strings.HasSuffix(file.Path, ".map") || strings.Contains(string(data), "/Users/") {
			t.Fatalf("private/development artifact embedded: %s", file.Path)
		}
	}
	for _, name := range []string{"index.html", "LICENSE.txt", "THIRD-PARTY-NOTICES.txt"} {
		if !seen[name] {
			t.Fatalf("missing required asset: %s", name)
		}
	}
	for name, mediaType := range pwaFiles {
		if !seen[name] {
			t.Fatalf("missing PWA asset: %s", name)
		}
		rec := httptest.NewRecorder()
		Wrap(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+name, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != mediaType || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("PWA asset %s served with status %d, type %q, cache %q", name, rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"))
		}
	}
	if err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !seen[name] {
			t.Errorf("unmanifested embedded asset: %s", name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	root := httptest.NewRecorder()
	Wrap(http.NotFoundHandler()).ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusOK || !strings.Contains(root.Body.String(), `lang="de"`) {
		t.Fatal("real embedded German UI is unavailable")
	}
}

func TestCSPDoesNotLoosenBeyondManifestAndWorker(t *testing.T) {
	want := "default-src 'none'; script-src 'self'; script-src-attr 'none'; style-src 'self'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self'; font-src 'self'; manifest-src 'self'; worker-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; object-src 'none'"
	if contentSecurityPolicy != want {
		t.Fatalf("unexpected CSP: %s", contentSecurityPolicy)
	}
}

func TestPWAFilesRejectUnsafeMethodsAndKeepAPIOutOfWorkerPaths(t *testing.T) {
	handler := wrapFS(testAssets(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, target := range []string{"/sw.js", "/manifest.webmanifest", "/icon-192.png"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, target, nil))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s accepted POST", target)
		}
	}
	for _, target := range []string{"/api/v1/query", "/healthz", "/readyz"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusNoContent || recorder.Header().Get("Content-Security-Policy") != "" {
			t.Fatalf("%s did not pass through to the API handler", target)
		}
	}
}
