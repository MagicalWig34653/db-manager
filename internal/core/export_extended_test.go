// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"encoding/json"
	"testing"
)

func TestExtendedExportOptions(t *testing.T) {
	base := ExportRequest{Connection: Connection{Driver: "sqlite", Database: "placeholder"}, Tables: []TableRef{{Name: "placeholder"}}}
	for _, format := range ExportFormats() {
		r := base
		r.Format = format
		if _, err := NormalizeExtendedExport(r); err != nil {
			t.Fatal(format, err)
		}
	}
	for _, change := range []func(*ExportRequest){
		func(r *ExportRequest) { r.Definer = "preserve" }, func(r *ExportRequest) { r.Compatibility = "mysql40" }, func(r *ExportRequest) { r.DataStatement = "insert;DROP" },
		func(r *ExportRequest) { r.Connection.Driver = "postgres"; r.DataStatement = "replace" }, func(r *ExportRequest) { r.Format = "xml"; r.Comments = true },
		func(r *ExportRequest) { r.Tables[0].Name = string([]byte{255}) },
		func(r *ExportRequest) {
			r.Aliases = []ExportAlias{{Source: TableRef{Name: "placeholder"}, Target: TableRef{Name: "target"}}}
		},
	} {
		r := base
		r.Tables = append([]TableRef(nil), base.Tables...)
		change(&r)
		if _, err := NormalizeExtendedExport(r); err == nil {
			t.Fatalf("invalid options accepted: %#v", r)
		}
	}
	var r ExportRequest
	if err := json.Unmarshal([]byte(`{"connection":{"driver":"sqlite"},"comments":false}`), &r); err != nil || !HasExtendedExportOptions(r) {
		t.Fatal("legacy option presence lost", err)
	}
	if json.Unmarshal([]byte(`{"unknown":false}`), &r) == nil {
		t.Fatal("unknown field accepted")
	}
	if json.Unmarshal([]byte("{\"filename\":\"\xff\"}"), &r) == nil {
		t.Fatal("invalid UTF-8 replaced")
	}
}
