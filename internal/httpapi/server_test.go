// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

const testToken = "synthetic-test-token-with-32-characters"

type testEngine struct {
	called   bool
	request  core.QueryRequest
	err      error
	deadline time.Time
}

func (e *testEngine) Drivers() []string { return []string{"sqlite"} }
func (e *testEngine) Test(ctx context.Context, c core.Connection) error {
	e.called = true
	return e.err
}
func (e *testEngine) Catalog(ctx context.Context, c core.Connection) ([]core.Table, error) {
	e.called = true
	return nil, e.err
}
func (e *testEngine) Query(ctx context.Context, r core.QueryRequest) (core.QueryResult, error) {
	e.called = true
	e.request = r
	e.deadline, _ = ctx.Deadline()
	return core.QueryResult{Results: []core.ResultSet{}}, e.err
}
func perform(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
func TestAuthorizationPrecedesDatabaseOperations(t *testing.T) {
	for _, token := range []string{"", "incorrect-token"} {
		engine := &testEngine{}
		response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"sql":"SELECT 1"}`, token)
		if response.Code != 401 || engine.called {
			t.Fatalf("unauthenticated database call: %d, %v", response.Code, engine.called)
		}
	}
	if response := perform(New(&testEngine{}, testToken), "GET", "/healthz", "", ""); response.Code != 200 {
		t.Fatal(response.Code)
	}
}
func TestInvalidRequestsNeverReachEngine(t *testing.T) {
	for _, body := range []string{
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","unknown":true}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1"} {}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","max_rows":10001}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","max_rows":-1}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","timeout_ms":60001}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","timeout_ms":-1}`,
		`{"connection":{"driver":"sqlite","database":"synthetic"},"sql":" "}`,
	} {
		engine := &testEngine{}
		response := perform(New(engine, testToken), "POST", "/api/v1/query", body, testToken)
		if response.Code != 400 || engine.called {
			t.Fatalf("body %q: %d, %v", body, response.Code, engine.called)
		}
	}
}
func TestBodyLimitAndContentType(t *testing.T) {
	engine := &testEngine{}
	response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"sql":"`+strings.Repeat("x", MaxRequestBytes)+`"}`, testToken)
	if response.Code != 413 || engine.called {
		t.Fatalf("oversized body reached engine: %d", response.Code)
	}
	request := httptest.NewRequest("POST", "/api/v1/query", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	New(engine, testToken).ServeHTTP(response, request)
	if response.Code != 415 {
		t.Fatal(response.Code)
	}
}
func TestQueryDefaultsAndErrorRedaction(t *testing.T) {
	engine := &testEngine{err: errors.New("password=synthetic-private-value host=private-host")}
	response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1"}`, testToken)
	if response.Code != 422 || strings.Contains(response.Body.String(), "synthetic-private-value") || strings.Contains(response.Body.String(), "private-host") {
		t.Fatalf("driver error leaked: %s", response.Body.String())
	}
	if engine.request.MaxRows != 1000 || engine.request.TimeoutMS != 10000 {
		t.Fatal(engine.request)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("credentials response must not be cached")
	}
}
func TestCancellationIsRecognizable(t *testing.T) {
	engine := &testEngine{err: context.DeadlineExceeded}
	response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1"}`, testToken)
	if response.Code != 408 || !strings.Contains(response.Body.String(), "query_canceled") {
		t.Fatal(response.Body.String())
	}
}

func TestConnectionValidationAcrossOperations(t *testing.T) {
	for _, path := range []string{"/api/v1/query", "/api/v1/catalog", "/api/v1/connections/test"} {
		for _, body := range []string{
			`null`, `{}`, `{"connection":null}`,
			`{"connection":{"driver":"unknown"}}`,
			`{"connection":{"driver":"mysql","port":65536}}`,
			`{"connection":{"driver":"mysql","port":-1}}`,
			`{"connection":{"driver":"postgres","tls_mode":"insecure"}}`,
			`{"connection":{"driver":"sqlite"}}`,
			`{"connection":{"driver":"sqlite","database":"../outside"}}`,
			`{"connection":{"driver":"sqlite","database":"/tmp/outside"}}`,
			`{"connection":{"driver":"sqlite","database":"file:outside?mode=ro"}}`,
		} {
			engine := &testEngine{}
			response := perform(New(engine, testToken), "POST", path, body, testToken)
			if response.Code != 400 || engine.called {
				t.Fatalf("%s accepted invalid connection: %d", path, response.Code)
			}
		}
	}
}

func TestValidCatalogAndConnectionTest(t *testing.T) {
	for _, path := range []string{"/api/v1/catalog", "/api/v1/connections/test"} {
		engine := &testEngine{}
		response := perform(New(engine, testToken), "POST", path, `{"connection":{"driver":"sqlite","database":"synthetic"}}`, testToken)
		if response.Code != 200 || !engine.called {
			t.Fatalf("%s rejected valid connection: %d", path, response.Code)
		}
	}
}

func TestQuerySuccessAndDeadline(t *testing.T) {
	engine := &testEngine{}
	response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1","max_rows":5,"timeout_ms":25}`, testToken)
	if response.Code != 200 || !engine.called || engine.request.MaxRows != 5 || engine.request.TimeoutMS != 25 {
		t.Fatalf("query limits were not forwarded: %d", response.Code)
	}
	remaining := time.Until(engine.deadline)
	if engine.deadline.IsZero() || remaining > 25*time.Millisecond {
		t.Fatalf("query deadline was not applied: %v", remaining)
	}
}

func TestOversizedTrailingWhitespaceAndParameterizedMediaType(t *testing.T) {
	engine := &testEngine{}
	body := `{"connection":{"driver":"sqlite","database":"synthetic"},"sql":"SELECT 1"}` + strings.Repeat(" ", MaxRequestBytes)
	response := perform(New(engine, testToken), "POST", "/api/v1/query", body, testToken)
	if response.Code != 413 || engine.called {
		t.Fatalf("oversized trailing whitespace accepted: %d", response.Code)
	}
	request := httptest.NewRequest("POST", "/api/v1/catalog", strings.NewReader(`{"connection":{"driver":"sqlite","database":"synthetic"}}`))
	request.Header.Set("Content-Type", "Application/JSON; charset=utf-8")
	request.Header.Set("Authorization", "Bearer "+testToken)
	response = httptest.NewRecorder()
	New(engine, testToken).ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("valid media type rejected: %d", response.Code)
	}
}
