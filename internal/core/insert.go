// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const InsertContractVersion = "0.4.0"

type InsertRequest struct {
	Connection Connection  `json:"connection"`
	Table      TableRef    `json:"table"`
	Values     []RowChange `json:"values"`
	Confirm    bool        `json:"confirm"`
	TimeoutMS  int         `json:"timeout_ms,omitempty"`
}
type InsertEligibility struct {
	Description       TableDescription `json:"description"`
	Eligible          bool             `json:"eligible"`
	Reason            string           `json:"reason"`
	InsertableColumns []string         `json:"insertable_columns"`
	RequiredPK        []string         `json:"required_pk"`
	AutoPK            string           `json:"auto_pk,omitempty"`
}
type InsertResult struct {
	Outcome string      `json:"outcome"`
	Row     RowSnapshot `json:"row"`
}
type InsertEngine interface {
	InsertEligibility(context.Context, DescribeRequest) (InsertEligibility, error)
	Insert(context.Context, InsertRequest) (InsertResult, error)
}
