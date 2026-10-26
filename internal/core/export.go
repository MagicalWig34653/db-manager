// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

const ExportContractVersion = "0.11.0"
const ExportMaxBytes int64 = 1 << 30
const ExportDefaultBytes int64 = 64 << 20
const ExportMaxRows int64 = 10000000
const ExportDefaultRows int64 = 1000000
const ExportMaxTimeoutMS = 300000
const ExportDefaultTimeoutMS = 60000
const ExportMaxTables = 100

// ExportRequest contains download options, never a server-side destination.
type ExportAlias struct {
	Source TableRef `json:"source"`
	Target TableRef `json:"target"`
}

type ExportRequest struct {
	extendedOptionsPresent bool
	ViewsAsTables          bool          `json:"views_as_tables,omitempty"`
	Definer                string        `json:"definer,omitempty"`
	Compatibility          string        `json:"compatibility,omitempty"`
	Comments               bool          `json:"comments,omitempty"`
	Aliases                []ExportAlias `json:"aliases,omitempty"`
	DataStatement          string        `json:"data_statement,omitempty"`
	Connection             Connection    `json:"connection"`
	Format                 string        `json:"format,omitempty"`
	Scope                  string        `json:"scope,omitempty"`
	Tables                 []TableRef    `json:"tables,omitempty"`
	Databases              []string      `json:"databases,omitempty"`
	SQL                    string        `json:"sql,omitempty"`
	Structure              *bool         `json:"structure,omitempty"`
	Data                   *bool         `json:"data,omitempty"`
	Drop                   bool          `json:"drop,omitempty"`
	IfNotExists            bool          `json:"if_not_exists,omitempty"`
	Extended               bool          `json:"extended,omitempty"`
	Dialect                string        `json:"dialect,omitempty"`
	Charset                string        `json:"charset,omitempty"`
	Filename               string        `json:"filename,omitempty"`
	Gzip                   bool          `json:"gzip,omitempty"`
	Transaction            bool          `json:"transaction,omitempty"`
	DisableFK              bool          `json:"disable_fk,omitempty"`
	Delimiter              string        `json:"delimiter,omitempty"`
	CRLF                   bool          `json:"crlf,omitempty"`
	MaxStatementBytes      int           `json:"max_statement_bytes,omitempty"`
	MaxRows                int64         `json:"max_rows,omitempty"`
	MaxBytes               int64         `json:"max_bytes,omitempty"`
	TimeoutMS              int           `json:"timeout_ms,omitempty"`
}

type ExportEngine interface {
	Export(context.Context, ExportRequest, io.Writer) error
}

var exportFilename = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,119}$`)

func invalidExportOptions() error {
	return &Error{Code: "invalid_export", Message: "Exportoptionen sind ungültig oder nicht unterstützt."}
}

// NormalizeExport applies bounded defaults before any I/O, also used by HTTP.
func NormalizeExport(r ExportRequest) (ExportRequest, error) {
	if HasExtendedExportOptions(r) {
		return r, invalidExportOptions()
	}
	return NormalizeExtendedExport(r)
}

func NormalizeExtendedExport(r ExportRequest) (ExportRequest, error) {
	if r.Format == "" {
		r.Format = "sql"
	}
	if r.Scope == "" {
		r.Scope = "tables"
	}
	if r.Charset == "" {
		r.Charset = "utf-8"
	}
	if r.Dialect == "" {
		r.Dialect = r.Connection.Driver
	}
	if r.MaxStatementBytes == 0 {
		r.MaxStatementBytes = 1 << 20
	}
	if r.MaxStatementBytes < 1024 || r.MaxStatementBytes > 4<<20 {
		return r, invalidExportOptions()
	}
	if r.MaxBytes == 0 {
		r.MaxBytes = ExportDefaultBytes
	}
	if r.MaxRows == 0 {
		r.MaxRows = ExportDefaultRows
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = ExportDefaultTimeoutMS
	}
	if r.MaxBytes < 1 || r.MaxBytes > ExportMaxBytes || r.MaxRows < 1 || r.MaxRows > ExportMaxRows || r.TimeoutMS < 1 || r.TimeoutMS > ExportMaxTimeoutMS {
		return r, invalidExportOptions()
	}
	if !IsExportFormat(r.Format) {
		return r, invalidExportOptions()
	}
	if r.Scope != "tables" && r.Scope != "database" && r.Scope != "server" && r.Scope != "query" {
		return r, invalidExportOptions()
	}
	if r.Connection.Driver != "sqlite" && r.Connection.Driver != "mysql" && r.Connection.Driver != "postgres" {
		return r, invalidExportOptions()
	}
	if r.Dialect != r.Connection.Driver || (r.Charset != "utf-8" && r.Charset != "utf-8-bom") {
		return r, invalidExportOptions()
	}
	if len(r.Tables) > ExportMaxTables || len(r.Databases) > 20 || (r.Filename != "" && !exportFilename.MatchString(r.Filename)) {
		return r, invalidExportOptions()
	}
	if r.Delimiter == "" {
		r.Delimiter = ","
		if r.Format == "excel-csv" {
			r.Delimiter = ";"
		}
	}
	if r.Delimiter != "," && r.Delimiter != ";" && r.Delimiter != "\t" {
		return r, invalidExportOptions()
	}
	if r.Format == "excel-csv" {
		r.CRLF = true
		r.Charset = "utf-8-bom"
	}
	if err := normalizeExportDetails(&r); err != nil {
		return r, err
	}
	if r.Structure == nil {
		v := r.Format == "sql"
		r.Structure = &v
	}
	if r.Data == nil {
		v := true
		r.Data = &v
	}
	if !*r.Structure && !*r.Data {
		return r, invalidExportOptions()
	}
	if r.Format != "sql" && (*r.Structure || r.Drop || r.IfNotExists || r.Extended || r.Transaction || r.DisableFK) {
		return r, invalidExportOptions()
	}
	if r.Scope == "query" {
		if r.Format == "sql" || strings.TrimSpace(r.SQL) == "" || len(r.SQL) > 1<<20 || len(r.Tables) != 0 || len(r.Databases) != 0 {
			return r, invalidExportOptions()
		}
	} else if r.SQL != "" {
		return r, invalidExportOptions()
	}
	if r.Scope == "tables" && len(r.Tables) == 0 {
		return r, invalidExportOptions()
	}
	if r.Scope != "tables" && len(r.Tables) != 0 {
		return r, invalidExportOptions()
	}
	if r.Scope == "server" {
		// Never discover or export databases that the caller did not select.
		if len(r.Databases) == 0 || r.Format == "csv" || r.Format == "excel-csv" || (r.Format == "sql" && r.Connection.Driver != "mysql") {
			return r, invalidExportOptions()
		}
	} else if len(r.Databases) != 0 {
		return r, invalidExportOptions()
	}
	if r.Scope == "query" && (r.Drop || r.IfNotExists || r.DisableFK || r.Transaction) {
		return r, invalidExportOptions()
	}
	if !*r.Structure && (r.Drop || r.IfNotExists) {
		return r, invalidExportOptions()
	}
	seenDatabases := map[string]bool{}
	for _, name := range r.Databases {
		if strings.TrimSpace(name) == "" || len(name) > 1024 || strings.ContainsRune(name, 0) || !utf8.ValidString(name) || seenDatabases[name] {
			return r, invalidExportOptions()
		}
		seenDatabases[name] = true
	}
	if r.DisableFK && r.Connection.Driver == "postgres" {
		return r, invalidExportOptions()
	}
	for _, ref := range r.Tables {
		if ref.Name == "" || len(ref.Name) > 1024 || len(ref.Schema) > 1024 || strings.ContainsRune(ref.Name, 0) || strings.ContainsRune(ref.Schema, 0) || !utf8.ValidString(ref.Name) || !utf8.ValidString(ref.Schema) {
			return r, invalidExportOptions()
		}
	}
	return r, nil
}
