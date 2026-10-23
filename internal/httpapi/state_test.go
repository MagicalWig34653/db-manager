// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeState struct{ up atomic.Bool }

func (f *fakeState) Healthy() bool { return f.up.Load() }

const queryBody = `{"connection":{"driver":"sqlite","database":"probe","tls_mode":"verify-full"},"sql":"SELECT 1"}`

func TestWithoutStateStoreBehaviorIsUnchanged(t *testing.T) {
	handler := New(&testEngine{}, testToken)
	response := perform(handler, "GET", "/healthz", "", "")
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("health changed: %d %s", response.Code, response.Body.String())
	}
	engine := &testEngine{}
	if response := perform(New(engine, testToken), "POST", "/api/v1/query", queryBody, testToken); response.Code != 200 || !engine.called {
		t.Fatalf("bearer mode changed: %d", response.Code)
	}
	ready := perform(handler, "GET", "/readyz", "", "")
	if ready.Code != 200 || !strings.Contains(ready.Body.String(), `"state_store":"disabled"`) {
		t.Fatalf("readyz without store: %d %s", ready.Code, ready.Body.String())
	}
}

// The Bearer API does not read server state, so an outage must not take it down,
// and liveness must not depend on the database (orchestrators replace unhealthy tasks).
func TestStateOutageDoesNotAffectBearerModeOrLiveness(t *testing.T) {
	state := &fakeState{}
	engine := &testEngine{}
	handler := New(engine, testToken, WithStateStore(state))
	if response := perform(handler, "POST", "/api/v1/query", queryBody, testToken); response.Code != 200 || !engine.called {
		t.Fatalf("bearer API affected by outage: %d", response.Code)
	}
	if response := perform(handler, "POST", "/api/v1/query", queryBody, "wrong"); response.Code != 401 {
		t.Fatalf("auth changed: %d", response.Code)
	}
	live := perform(handler, "GET", "/healthz", "", "")
	if live.Code != 200 || strings.TrimSpace(live.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("liveness depends on state: %d %s", live.Code, live.Body.String())
	}
}

func TestReadinessReportsStateStoreFromCache(t *testing.T) {
	state := &fakeState{}
	handler := New(&testEngine{}, testToken, WithStateStore(state))
	down := perform(handler, "GET", "/readyz", "", "")
	if down.Code != 503 || !strings.Contains(down.Body.String(), `"state_store":"unreachable"`) {
		t.Fatalf("readyz down: %d %s", down.Code, down.Body.String())
	}
	state.up.Store(true)
	up := perform(handler, "GET", "/readyz", "", "")
	if up.Code != 200 || !strings.Contains(up.Body.String(), `"state_store":"ok"`) {
		t.Fatalf("readyz up: %d %s", up.Code, up.Body.String())
	}
}

func TestRequireStateFailsClosedAndRecovers(t *testing.T) {
	state := &fakeState{}
	api := New(&testEngine{}, testToken, WithStateStore(state))
	var served atomic.Int32
	guarded := api.RequireState(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served.Add(1); w.WriteHeader(204) }))
	response := perform(guarded, "GET", "/x", "", testToken)
	if response.Code != 503 || served.Load() != 0 || response.Header().Get("Retry-After") == "" || !strings.Contains(response.Body.String(), "state_unavailable") {
		t.Fatalf("not failing closed: %d %s", response.Code, response.Body.String())
	}
	state.up.Store(true)
	if response := perform(guarded, "GET", "/x", "", testToken); response.Code != 204 || served.Load() != 1 {
		t.Fatalf("no recovery: %d", response.Code)
	}
	unconfigured := New(&testEngine{}, testToken).RequireState(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { served.Add(1) }))
	if response := perform(unconfigured, "GET", "/x", "", testToken); response.Code != 503 || served.Load() != 1 {
		t.Fatalf("route needing state served without a store: %d", response.Code)
	}
}
