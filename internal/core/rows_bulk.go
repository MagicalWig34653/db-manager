// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"io"
)

const RowsBulkContractVersion = "0.19.0"
const MaxBulkRows = 100
const MaxFileValueBytes = 512 * 1024

type BulkRow struct {
	Expected        RowSnapshot `json:"expected"`
	StabilitySHA256 string      `json:"stability_sha256,omitempty"`
	CopyValues      []RowChange `json:"copy_values,omitempty"`
}
type RowsBulkRequest struct {
	Connection         Connection  `json:"connection"`
	Table              TableRef    `json:"table"`
	Action             string      `json:"action"`
	Rows               []BulkRow   `json:"rows"`
	Changes            []RowChange `json:"changes,omitempty"`
	TimeoutMS          int         `json:"timeout_ms,omitempty"`
	Confirm            bool        `json:"confirm,omitempty"`
	ExpectedPlanSHA256 string      `json:"expected_plan_sha256,omitempty"`
}
type BulkPlannedRow struct {
	Key             []RowChange `json:"key"`
	StabilitySHA256 string      `json:"stability_sha256"`
}
type RowsBulkPlan struct {
	Table      TableRef         `json:"table"`
	Action     string           `json:"action"`
	Rows       []BulkPlannedRow `json:"rows"`
	RowCount   int              `json:"row_count"`
	Warnings   []string         `json:"warnings"`
	PlanSHA256 string           `json:"plan_sha256"`
}
type BulkRowResult struct {
	Index   int          `json:"index"`
	Outcome string       `json:"outcome"`
	Row     *RowSnapshot `json:"row,omitempty"`
}
type RowsBulkResult struct {
	Outcome string          `json:"outcome"`
	Rows    []BulkRowResult `json:"rows"`
	Reason  string          `json:"reason,omitempty"`
}
type FileValueRequest struct {
	Connection Connection  `json:"connection"`
	Table      TableRef    `json:"table"`
	Expected   RowSnapshot `json:"expected"`
	Column     string      `json:"column"`
	Confirm    bool        `json:"confirm,omitempty"`
	TimeoutMS  int         `json:"timeout_ms,omitempty"`
	Preview    bool        `json:"preview,omitempty"`
}
type ForeignKeyLookupRequest struct {
	Connection    Connection `json:"connection"`
	Table         TableRef   `json:"table"`
	Column        string     `json:"column"`
	DisplayColumn string     `json:"display_column"`
	Search        string     `json:"search,omitempty"`
	PageSize      int        `json:"page_size,omitempty"`
	Offset        int        `json:"offset,omitempty"`
	TimeoutMS     int        `json:"timeout_ms,omitempty"`
}
type ForeignKeyChoice struct {
	Value   Cell `json:"value"`
	Display Cell `json:"display"`
}
type ForeignKeyLookupResult struct {
	Table         TableRef           `json:"table"`
	ValueColumn   string             `json:"value_column"`
	DisplayColumn string             `json:"display_column"`
	Choices       []ForeignKeyChoice `json:"choices"`
	HasMore       bool               `json:"has_more"`
}
type RowsBulkEngine interface {
	RowsBulkPlan(context.Context, RowsBulkRequest) (RowsBulkPlan, error)
	RowsBulkExecute(context.Context, RowsBulkRequest) (RowsBulkResult, error)
	FileValueUpload(context.Context, FileValueRequest, io.Reader) (MutationResult, error)
	FileValueDownload(context.Context, FileValueRequest, func(string, int64) (io.Writer, error)) error
	ForeignKeyLookup(context.Context, ForeignKeyLookupRequest) (ForeignKeyLookupResult, error)
}
