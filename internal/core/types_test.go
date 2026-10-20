// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"encoding/json"
	"testing"
)

func TestCellSerializationPreservesNullAndEmptyValues(t *testing.T) {
	for _, tc := range []struct {
		cell Cell
		want string
	}{
		{Cell{Kind: "null"}, `{"kind":"null"}`},
		{Cell{Kind: "text", Value: ""}, `{"kind":"text","value":""}`},
		{Cell{Kind: "binary", Value: ""}, `{"kind":"binary","value":""}`},
		{Cell{Kind: "integer", Value: "9007199254740993"}, `{"kind":"integer","value":"9007199254740993"}`},
	} {
		data, err := json.Marshal(tc.cell)
		if err != nil || string(data) != tc.want {
			t.Fatalf("unexpected cell serialization: %s, %v", data, err)
		}
	}
}
