// SPDX-License-Identifier: AGPL-3.0-only
package core

import "testing"

func TestExportOptionValidation(t *testing.T) {
	base := ExportRequest{Connection: Connection{Driver: "sqlite", Database: "fixture"}, Tables: []TableRef{{Name: "table"}}}
	r, err := NormalizeExport(base)
	if err != nil || r.MaxBytes != ExportDefaultBytes || r.Format != "sql" || !*r.Structure || !*r.Data {
		t.Fatalf("defaults %#v %v", r, err)
	}
	tests := []func(*ExportRequest){
		func(r *ExportRequest) { r.MaxRows = -1 }, func(r *ExportRequest) { r.MaxBytes = ExportMaxBytes + 1 }, func(r *ExportRequest) { r.TimeoutMS = ExportMaxTimeoutMS + 1 },
		func(r *ExportRequest) { r.Charset = "latin1" }, func(r *ExportRequest) { r.Dialect = "postgres" }, func(r *ExportRequest) { r.Scope = "server" },
		func(r *ExportRequest) { r.Tables = []TableRef{{Name: "bad\x00name"}} }, func(r *ExportRequest) { r.Filename = "../../path" },
		func(r *ExportRequest) { r.Delimiter = "\r\n" }, func(r *ExportRequest) { r.Format = "xml" }, func(r *ExportRequest) { r.SQL = "DELETE FROM x" },
	}
	for i, change := range tests {
		r := base
		change(&r)
		if _, err := NormalizeExport(r); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}
