// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"encoding/json"
	"testing"
)

func TestIndexesStrictJSON(t *testing.T) {
	for _, data := range []string{
		`{"connection":{},"table":{"name":"x"},"action":"primary_drop","index":{}}`,
		`{"connection":{},"table":{"name":"x"},"action":"index_drop","name":"i","new_name":""}`,
		`{"connection":{},"table":{"name":"x"},"action":"index_create"}`,
		`{"connection":{},"table":{"name":"x"},"action":"index_create","index":{"name":"i","kind":"index","terms":[{"column":"id","sql":"DROP"}]}}`,
	} {
		var r IndexesRequest
		if json.Unmarshal([]byte(data), &r) == nil {
			t.Fatal(data)
		}
	}
	caps := IndexesCapabilities()
	if caps.Allows("sqlite", "primary_set") || caps.Allows("sqlite", "index_alter") || caps.Allows("postgres", "prefix") || !caps.Allows("postgres", "partial") || !caps.Allows("mysql", "spatial") {
		t.Fatal(caps)
	}
}
