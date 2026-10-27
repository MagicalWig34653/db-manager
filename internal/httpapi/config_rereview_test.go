// SPDX-License-Identifier: AGPL-3.0-only
package httpapi

import (
	"strings"
	"testing"
	"time"
)

// Independent re-review of the W-P20 follow-ups (docs/handoffs/W-P20-backend-rereview.md).

// Gap (F-13 incomplete): Go's struct decoding matches field names case-insensitively, so a
// duplicate that differs only in case is not detected and the last one wins.
func TestRereviewHolds_CaseVariantDuplicateEnvelopeKeys(t *testing.T) {
	handler, _, _ := configAPI(t)
	for _, body := range []string{`{"scope":"a","Scope":"b","data":{"table":"x"}}`, `{"data":{"table":"x"},"DATA":{"table":"y"}}`} {
		if got := do(handler, "POST", "/api/v24/config/favorites", body, nil).Code; got != 400 {
			t.Errorf("%s: %d, want 400", body, got)
		}
	}
	// Unicode-escaped and plain duplicates are caught (must hold).
	for _, body := range []string{`{"scope":"a","scope":"b","data":{"table":"x"}}`, `{"data":{"table":"x"},"data":{"table":"y"}}`, `{"items":[{"data":{"table":"x"},"data":{"table":"y"}}]}`} {
		path := "/api/v24/config/favorites"
		if strings.HasPrefix(body, `{"items"`) {
			path = "/api/v24/config/import"
		}
		if got := do(handler, "POST", path, body, nil).Code; got != 400 {
			t.Errorf("%s: %d, want 400", body, got)
		}
	}
}

// Blocker-class defect: hasDuplicateKeys never terminates on malformed or too deeply nested arrays
// (dec.More() stays true after a decoder error), so a single authenticated request pins a core for
// good. The demonstration spins a CPU for the rest of the test binary, so it only runs on request:
//
//	DBM_REVIEW_DEMO_HANG=1 go test ./internal/httpapi -run RereviewHang
//
// Expected after the fix: every case returns within the budget.
func TestRereviewHolds_DuplicateKeyWalkTerminates(t *testing.T) {
	cases := map[string]string{
		"unterminated array":  `{"data":[1,2`,
		"missing comma":       `{"data":[1 2]}`,
		"bare bracket":        `[`,
		"valid but 100k deep": strings.Repeat("[", 100000) + strings.Repeat("]", 100000),
		"65 deep":             strings.Repeat("[", 65) + strings.Repeat("]", 65),
		"object deep":         strings.Repeat(`{"a":`, 70) + "1" + strings.Repeat("}", 70),
	}
	for name, raw := range cases {
		done := make(chan bool, 1)
		go func() { done <- hasDuplicateKeys([]byte(raw)) }()
		select {
		case bad := <-done:
			if !bad {
				t.Errorf("%s: accepted", name)
			}
		case <-time.After(3 * time.Second):
			t.Errorf("%s: hasDuplicateKeys does not terminate", name)
		}
	}
}

// Must hold: resolver owners that differ only in case from the reserved legacy owner are separate
// owners (ascii_bin) and never see its data; separators and spaces are rejected.
func TestRereviewHolds_ResolverOwnerVariants(t *testing.T) {
	handler := ownerResolverAPI(t, "X-Test-User")
	do(handler, "POST", "/api/v24/config/favorites", `{"data":{"table":"t"}}`, map[string]string{"X-Test-User": "Single"})
	for _, owner := range []string{"single", "single ", " single", "SINGLE", "sing1e", "single\t", "meta", "cfg2|meta"} {
		code := do(handler, "GET", "/api/v24/config/favorites", "", map[string]string{"X-Test-User": owner}).Code
		switch owner {
		case "single":
			if code != 401 {
				t.Errorf("%q: %d, want 401", owner, code)
			}
		case "SINGLE", "sing1e", "meta":
			if code != 200 {
				t.Errorf("%q: %d, want 200 (separate owner)", owner, code)
			}
		default:
			if code != 400 {
				t.Errorf("%q: %d, want 400", owner, code)
			}
		}
	}
}

// The HTTP route answers 400 quickly for the hanging inputs.
func TestRereviewHolds_HangingBodiesAnswer400(t *testing.T) {
	handler, _, _ := configAPI(t)
	for _, body := range []string{`{"data":[1,2`, `{"data":[1 2]}`, strings.Repeat("[", 20000)} {
		done := make(chan int, 1)
		go func() { done <- do(handler, "POST", "/api/v24/config/layouts", body, nil).Code }()
		select {
		case code := <-done:
			if code != 400 {
				t.Errorf("%.20s: %d", body, code)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%.20s: no answer", body)
		}
	}
}
