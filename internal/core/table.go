// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const TableContractVersion = "0.2.0"

type TableRef struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
}
type DescribeRequest struct {
	Connection Connection `json:"connection"`
	Table      TableRef   `json:"table"`
}
type TableColumn struct {
	Name         string  `json:"name"`
	DatabaseType string  `json:"database_type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"default_value,omitempty"`
	PKPosition   int     `json:"pk_position"`
	Generated    string  `json:"generated"`
	ReadOnly     bool    `json:"read_only"`
}
type TableDescription struct {
	Table         TableRef      `json:"table"`
	Kind          string        `json:"kind"`
	Columns       []TableColumn `json:"columns"`
	PrimaryKey    []string      `json:"primary_key"`
	ReadOnly      bool          `json:"read_only"`
	StableOrder   bool          `json:"stable_order"`
	StorageEngine string        `json:"storage_engine"`
	Transactional string        `json:"transactional"`
}
type TableSort struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}
type TableFilter struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    *Cell  `json:"value,omitempty"`
}
type BrowseRequest struct {
	Connection Connection    `json:"connection"`
	Table      TableRef      `json:"table"`
	PageSize   int           `json:"page_size,omitempty"`
	Offset     int           `json:"offset,omitempty"`
	Sort       []TableSort   `json:"sort,omitempty"`
	Filters    []TableFilter `json:"filters,omitempty"`
	TimeoutMS  int           `json:"timeout_ms,omitempty"`
}
type BrowseResult struct {
	Description TableDescription `json:"description"`
	Result      ResultSet        `json:"result"`
	Offset      int              `json:"offset"`
	PageSize    int              `json:"page_size"`
	HasMore     bool             `json:"has_more"`
	StableOrder bool             `json:"stable_order"`
	DurationMS  int64            `json:"duration_ms"`
}
type TableEngine interface {
	Describe(context.Context, DescribeRequest) (TableDescription, error)
	Browse(context.Context, BrowseRequest) (BrowseResult, error)
}
