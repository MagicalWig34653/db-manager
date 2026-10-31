// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const ExtendedExportContractVersion = "0.21.0"

// ExportObjectsContractVersion marks the additive DUMP-001 options on the v21 routes.
const ExportObjectsContractVersion = "0.25.0"

func ExportFormats() []string {
	return []string{"sql", "csv", "excel-csv", "json", "xml", "latex", "latex-longtable", "mediawiki", "yaml", "php-array", "texy", "codegen"}
}
func IsExportFormat(format string) bool {
	for _, item := range ExportFormats() {
		if item == format {
			return true
		}
	}
	return false
}

// HasExtendedExportOptions keeps the frozen v11 HTTP surface unchanged.
func HasExtendedExportOptions(r ExportRequest) bool {
	return r.extendedOptionsPresent || (r.Format != "" && r.Format != "sql" && r.Format != "csv" && r.Format != "excel-csv" && r.Format != "json") || r.ViewsAsTables || r.Definer != "" || r.Compatibility != "" || r.Comments || len(r.Aliases) > 0 || r.DataStatement != "" || len(r.Objects) > 0
}
func normalizeExportDetails(r *ExportRequest) error {
	if r.Format != "sql" && (r.Definer != "" || r.Compatibility != "" || r.Comments || r.DataStatement != "" || len(r.Aliases) > 0 || len(r.Objects) > 0) {
		return invalidExportOptions()
	}
	if r.Definer != "" && r.Definer != "omit" && r.Definer != "preserve" {
		return invalidExportOptions()
	}
	if r.Compatibility != "" && r.Compatibility != "native" && r.Compatibility != "data-only" {
		return invalidExportOptions()
	}
	if r.Compatibility == "data-only" {
		if (r.Structure != nil && *r.Structure) || r.Drop || r.IfNotExists {
			return invalidExportOptions()
		}
		v := false
		r.Structure = &v
	}
	if r.DataStatement != "" && r.DataStatement != "insert" && r.DataStatement != "insert-ignore" && r.DataStatement != "replace" {
		return invalidExportOptions()
	}
	if r.Connection.Driver == "postgres" && r.DataStatement != "" && r.DataStatement != "insert" {
		return invalidExportOptions()
	}
	if len(r.Aliases) > ExportMaxTables || (len(r.Aliases) > 0 && (r.Scope != "tables" || r.Structure == nil || *r.Structure)) {
		return invalidExportOptions()
	}
	seen := map[TableRef]bool{}
	for _, a := range r.Aliases {
		if seen[a.Source] {
			return invalidExportOptions()
		}
		seen[a.Source] = true
		for _, ref := range []TableRef{a.Source, a.Target} {
			if ref.Name == "" || len(ref.Name) > 1024 || len(ref.Schema) > 1024 || !utf8.ValidString(ref.Name) || !utf8.ValidString(ref.Schema) || strings.ContainsRune(ref.Name, 0) || strings.ContainsRune(ref.Schema, 0) {
				return invalidExportOptions()
			}
		}
	}
	return nil
}

// UnmarshalJSON preserves strict field validation and rejects malformed UTF-8
// before encoding/json can silently replace it. Presence matters on legacy HTTP.
func (r *ExportRequest) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid export encoding")
	}
	type plain ExportRequest
	value := plain(*r)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key := range fields {
		switch strings.ToLower(key) {
		case "views_as_tables", "definer", "compatibility", "comments", "aliases", "data_statement", "objects":
			value.extendedOptionsPresent = true
		}
	}
	*r = ExportRequest(value)
	return nil
}

// normalizeExportObjects validates the optional object classes (DUMP-001). It runs
// after the structure default is known. The result is canonical: unique classes in
// dependency order. Objects always describe the whole database/schema, never a
// table selection, and need the structure part of the dump.
func normalizeExportObjects(r *ExportRequest) error {
	driver := r.Connection.Driver
	if len(r.Objects) == 0 {
		if r.Definer == "preserve" {
			return invalidExportOptions()
		}
		r.Objects = nil
		return nil
	}
	if r.Format != "sql" || r.Structure == nil || !*r.Structure || (r.Scope != "database" && r.Scope != "server") || r.ViewsAsTables || len(r.Aliases) > 0 {
		return invalidExportOptions()
	}
	want := map[string]bool{}
	for _, class := range r.Objects {
		known := false
		for _, item := range ExportObjectClasses() {
			known = known || item == class
		}
		if !known || want[class] {
			return invalidExportOptions()
		}
		want[class] = true
	}
	if driver == "sqlite" && (want["routines"] || want["events"]) || driver == "postgres" && want["events"] {
		return invalidExportOptions()
	}
	if r.Definer == "preserve" && driver != "mysql" {
		return invalidExportOptions()
	}
	// IF NOT EXISTS only where the dialect has it for the class; otherwise use drop.
	if r.IfNotExists && (driver == "mysql" && want["views"] || driver == "postgres" && (want["views"] || want["triggers"])) {
		return invalidExportOptions()
	}
	r.Objects = nil
	for _, class := range ExportObjectClasses() {
		if want[class] {
			r.Objects = append(r.Objects, class)
		}
	}
	return nil
}

// HasExportObject reports whether the dump includes the object class.
func (r ExportRequest) HasExportObject(class string) bool {
	for _, item := range r.Objects {
		if item == class {
			return true
		}
	}
	return false
}
