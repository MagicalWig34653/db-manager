// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const ImportContractVersion = "0.18.0"
const MaxImportBytes = 64 << 20
const MaxImportRows = 100000
const MaxImportStatements = 10000

// ImportRequest contains metadata only. File bytes travel as a multipart part.
type ImportRequest struct {
	Connection         Connection           `json:"connection"`
	Format             string               `json:"format"`
	Compression        string               `json:"compression,omitempty"`
	Table              TableRef             `json:"table,omitempty"`
	CSV                CSVImportOptions     `json:"csv,omitempty"`
	Mode               string               `json:"mode,omitempty"`
	MaxRows            int                  `json:"max_rows,omitempty"`
	MaxBytes           int                  `json:"max_bytes,omitempty"`
	TimeoutMS          int                  `json:"timeout_ms,omitempty"`
	Delimiter          string               `json:"delimiter,omitempty"`
	Confirm            *dbplan.Confirmation `json:"confirm,omitempty"`
	ExpectedPlanSHA256 string               `json:"expected_plan_sha256,omitempty"`
}
type CSVImportOptions struct {
	Columns   []string `json:"columns,omitempty"`
	Separator string   `json:"separator,omitempty"`
	Quote     string   `json:"quote,omitempty"`
	Escape    string   `json:"escape,omitempty"`
	Charset   string   `json:"charset,omitempty"`
	Header    bool     `json:"header,omitempty"`
	Null      *string  `json:"null,omitempty"`
}
type ImportIssue struct {
	Row    int    `json:"row"`
	Column int    `json:"column"`
	Code   string `json:"code"`
}
type ImportPlan struct {
	dbplan.Plan
	Rows            int           `json:"rows"`
	ValidRows       int           `json:"valid_rows"`
	Preview         [][]Cell      `json:"preview"`
	Errors          []ImportIssue `json:"errors"`
	ErrorsTruncated bool          `json:"errors_truncated"`
}
type ImportProgress struct {
	Processed int `json:"processed"`
	Total     int `json:"total"`
}
type ImportResult struct {
	Outcome         string               `json:"outcome"`
	Processed       int                  `json:"processed"`
	Inserted        int                  `json:"inserted"`
	Skipped         int                  `json:"skipped"`
	Statements      []SQLStatementResult `json:"statements"`
	Errors          []ImportIssue        `json:"errors"`
	ErrorsTruncated bool                 `json:"errors_truncated"`
	Warnings        []string             `json:"warnings"`
	Reason          string               `json:"reason,omitempty"`
}
type ImportEngine interface {
	PlanImport(context.Context, ImportRequest, []byte) (ImportPlan, error)
	ExecuteImport(context.Context, ImportRequest, []byte, func(ImportProgress)) (ImportResult, error)
}
