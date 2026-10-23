// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeState struct{ up atomic.Bool }

func (f *fakeState) Healthy() bool { return f.up.Load() }
func (f *fakeState) Check(context.Context) error {
	if f.up.Load() {
		return nil
	}
	return errors.New("sentinel-driver-detail host=secret.internal")
}

func TestWithoutStateStoreBehaviorIsUnchanged(t *testing.T) {
	response := perform(New(&testEngine{}, testToken), "GET", "/healthz", "", "")
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("health changed: %d %s", response.Code, response.Body.String())
	}
	engine := &testEngine{}
	if response := perform(New(engine, testToken), "POST", "/api/v1/query", `{"connection":{"driver":"sqlite","database":"probe","tls_mode":"verify-full"},"sql":"SELECT 1"}`, testToken); response.Code != 200 || !engine.called {
		t.Fatalf("bearer mode changed: %d", response.Code)
	}
}

func TestStateStoreFailsClosedAndRecovers(t *testing.T) {
	state := &fakeState{}
	engine := &testEngine{}
	handler := New(engine, testToken, WithStateStore(state))
	query := `{"connection":{"driver":"sqlite","database":"probe","tls_mode":"verify-full"},"sql":"SELECT 1"}`

	// Down: even a valid bearer token gets 503, the engine is never called, health is unhealthy.
	response := perform(handler, "POST", "/api/v1/query", query, testToken)
	if response.Code != 503 || engine.called || response.Header().Get("Retry-After") == "" || !strings.Contains(response.Body.String(), "state_unavailable") {
		t.Fatalf("not failing closed: %d %s called=%v", response.Code, response.Body.String(), engine.called)
	}
	if response := perform(handler, "GET", "/api/v1/capabilities", "", ""); response.Code != 503 {
		t.Fatalf("unauthenticated request not gated: %d", response.Code)
	}
	health := perform(handler, "GET", "/healthz", "", "")
	if health.Code != 503 || strings.Contains(health.Body.String(), "sentinel") || strings.Contains(health.Body.String(), "secret.internal") {
		t.Fatalf("health down: %d %s", health.Code, health.Body.String())
	}

	// Up again: the same handler recovers without restart and authentication still applies.
	state.up.Store(true)
	if response := perform(handler, "POST", "/api/v1/query", query, testToken); response.Code != 200 || !engine.called {
		t.Fatalf("no recovery: %d", response.Code)
	}
	if response := perform(handler, "POST", "/api/v1/query", query, "wrong"); response.Code != 401 {
		t.Fatalf("auth bypass while up: %d", response.Code)
	}
	health = perform(handler, "GET", "/healthz", "", "")
	if health.Code != 200 || !strings.Contains(health.Body.String(), `"state_store":"ok"`) {
		t.Fatalf("health up: %d %s", health.Code, health.Body.String())
	}
}
