// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const SQLBatchContractVersion = "0.16.0"

type SQLBatchRequest struct {
	Connection Connection `json:"connection"`
	SQL        string     `json:"sql"`
	Delimiter  string     `json:"delimiter,omitempty"`
	Mode       string     `json:"mode,omitempty"`
	Confirm    bool       `json:"confirm"`
	MaxRows    int        `json:"max_rows,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}
type SQLStatementResult struct {
	Index        int          `json:"index"`
	Outcome      string       `json:"outcome"`
	Results      []ResultSet  `json:"results"`
	RowsAffected *string      `json:"rows_affected,omitempty"`
	Warnings     []SQLWarning `json:"warnings"`
	DurationMS   int64        `json:"duration_ms"`
	ErrorCode    string       `json:"error_code,omitempty"`
	ServerCode   string       `json:"server_code,omitempty"`
}
type SQLWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type SQLBatchResult struct {
	Statements []SQLStatementResult `json:"statements"`
	DurationMS int64                `json:"duration_ms"`
}
type SQLSimulationResult struct {
	Outcome      string       `json:"outcome"`
	RowsAffected *string      `json:"rows_affected,omitempty"`
	Warnings     []SQLWarning `json:"warnings"`
	DurationMS   int64        `json:"duration_ms"`
}
type SQLBatchEngine interface {
	SQLBatch(context.Context, SQLBatchRequest) (SQLBatchResult, error)
	SQLExplain(context.Context, SQLBatchRequest) (SQLBatchResult, error)
	SQLSimulate(context.Context, SQLBatchRequest) (SQLSimulationResult, error)
}
