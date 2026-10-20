// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const MutationContractVersion = "0.3.0"

type RowSnapshot struct {
	Columns []string `json:"columns"`
	Values  []Cell   `json:"values"`
}
type RowChange struct {
	Column string `json:"column"`
	Value  Cell   `json:"value"`
}
type MutationRequest struct {
	Connection Connection  `json:"connection"`
	Table      TableRef    `json:"table"`
	Expected   RowSnapshot `json:"expected"`
	Changes    []RowChange `json:"changes,omitempty"`
	Confirm    bool        `json:"confirm"`
	TimeoutMS  int         `json:"timeout_ms,omitempty"`
}
type MutationEligibility struct {
	Description     TableDescription `json:"description"`
	Eligible        bool             `json:"eligible"`
	Reason          string           `json:"reason"`
	WritableColumns []string         `json:"writable_columns"`
}
type MutationResult struct {
	Outcome string       `json:"outcome"`
	Changed bool         `json:"changed"`
	Row     *RowSnapshot `json:"row,omitempty"`
}
type MutationEngine interface {
	Eligibility(context.Context, DescribeRequest) (MutationEligibility, error)
	Update(context.Context, MutationRequest) (MutationResult, error)
	Delete(context.Context, MutationRequest) (MutationResult, error)
}
