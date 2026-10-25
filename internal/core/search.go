// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const SearchContractVersion = "0.15.0"
const SearchMaxTables = 20
const SearchMaxRows = 1000
const SearchMaxPreview = 20
const SearchMaxValue = 4096

type SearchTarget struct {
	Table   TableRef `json:"table"`
	Columns []string `json:"columns"`
}
type SearchRequest struct {
	Connection  Connection     `json:"connection"`
	Targets     []SearchTarget `json:"targets"`
	Term        string         `json:"term"`
	Mode        string         `json:"mode"`
	MaxRows     int            `json:"max_rows,omitempty"`
	PreviewRows int            `json:"preview_rows,omitempty"`
	TimeoutMS   int            `json:"timeout_ms,omitempty"`
}
type SearchTableResult struct {
	Table      TableRef `json:"table"`
	Columns    []string `json:"columns"`
	Count      int      `json:"count"`
	CountExact bool     `json:"count_exact"`
	Rows       [][]Cell `json:"rows"`
}
type SearchResult struct {
	Tables []SearchTableResult `json:"tables"`
}
type ReplaceRequest struct {
	Connection  Connection `json:"connection"`
	Table       TableRef   `json:"table"`
	Column      *string    `json:"column"`
	Term        string     `json:"term"`
	Replacement *string    `json:"replacement"`
	Mode        string     `json:"mode"`
	MaxRows     int        `json:"max_rows,omitempty"`
	TimeoutMS   int        `json:"timeout_ms,omitempty"`
}
type ReplaceExample struct {
	Key    []Cell `json:"key"`
	Before Cell   `json:"before"`
	After  Cell   `json:"after"`
}
type ReplacePlan struct {
	Driver       string           `json:"driver"`
	Table        TableRef         `json:"table"`
	Column       string           `json:"column"`
	AffectedRows int              `json:"affected_rows"`
	Examples     []ReplaceExample `json:"examples"`
	Statements   []string         `json:"statements"`
	Warnings     []string         `json:"warnings"`
	SHA256       string           `json:"plan_sha256"`
}
type ReplaceConfirm struct {
	Action       string `json:"action"`
	TargetName   string `json:"target_name"`
	AffectedRows int    `json:"affected_rows"`
}
type ReplaceExecuteRequest struct {
	ReplaceRequest
	Confirm            ReplaceConfirm `json:"confirm"`
	ExpectedPlanSHA256 string         `json:"expected_plan_sha256"`
}
type ReplaceResult struct {
	Outcome      dbplan.Status `json:"outcome"`
	AffectedRows int           `json:"affected_rows"`
	Reason       string        `json:"reason,omitempty"`
}
type SearchDriverCapability struct {
	SearchModes  []string `json:"search_modes"`
	ReplaceModes []string `json:"replace_modes"`
	Reason       string   `json:"reason"`
}

func SearchCapabilities() map[string]SearchDriverCapability {
	return map[string]SearchDriverCapability{
		"sqlite":   {[]string{"substring", "all_words", "exact"}, []string{"substring", "exact"}, "Nur Textspalten; keine Regex-Erweiterung geladen; Ersetzen nur mit stabilem Primärschlüssel und vollständig begrenztem Tabellenbestand."},
		"mysql":    {[]string{"substring", "all_words", "exact", "regex"}, []string{"substring", "exact", "regex"}, "MariaDB über mysql; native REGEXP/REGEXP_REPLACE-Semantik; Ersetzen nur InnoDB mit stabilem Primärschlüssel und vollständig begrenztem Tabellenbestand."},
		"postgres": {[]string{"substring", "all_words", "exact", "regex"}, []string{"substring", "exact", "regex"}, "Native POSIX-Regex; Ersetzen nur Basistabellen ohne Vererbung, Partitionen oder Rewrite-Regeln mit stabilem Primärschlüssel."},
	}
}

type SearchEngine interface {
	Search(context.Context, SearchRequest) (SearchResult, error)
	ReplacePlan(context.Context, ReplaceRequest) (ReplacePlan, error)
	ReplaceExecute(context.Context, ReplaceExecuteRequest) (ReplaceResult, error)
	SearchBrowse(context.Context, BrowseRequest) (BrowseResult, error)
}
