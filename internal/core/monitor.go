// SPDX-License-Identifier: AGPL-3.0-only
package core

import "context"

const MonitorContractVersion = "0.20.0"

// MonitorRequest is a single, stateless read. SQL is used only by profiling.
type MonitorRequest struct {
	Connection Connection `json:"connection"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
	Log        string     `json:"log,omitempty"`
	Position   int64      `json:"position,omitempty"`
	SQL        string     `json:"sql,omitempty"`
}
type MonitorHint struct {
	Rule      string `json:"rule"`
	Source    string `json:"source"`
	Metric    string `json:"metric"`
	Threshold string `json:"threshold"`
	Observed  string `json:"observed"`
	Message   string `json:"message"`
}

// MonitorRecord contains only explicitly selected numeric or enumerated fields.
type MonitorRecord map[string]string

type MonitorResult struct {
	Driver     string          `json:"driver"`
	Action     string          `json:"action"`
	State      string          `json:"state"`
	Reason     string          `json:"reason"`
	Source     string          `json:"source"`
	SampledAt  string          `json:"sampled_at"`
	DurationMS int64           `json:"duration_ms"`
	Records    []MonitorRecord `json:"records"`
	Hints      []MonitorHint   `json:"hints"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	HasMore    bool            `json:"has_more"`
	Truncated  bool            `json:"truncated"`
}
type MonitorCapability struct {
	Driver  string `json:"driver"`
	Action  string `json:"action"`
	Support string `json:"support"`
	Reason  string `json:"reason"`
}
type MonitorCapabilities struct {
	ContractVersion  string              `json:"contract_version"`
	Available        bool                `json:"available"`
	ReadOnly         bool                `json:"read_only"`
	MaxLimit         int                 `json:"max_limit"`
	MaxOffset        int                 `json:"max_offset"`
	MaxTimeoutMS     int                 `json:"max_timeout_ms"`
	MaxResponseBytes int                 `json:"max_response_bytes"`
	Drivers          []MonitorCapability `json:"drivers"`
}

var MonitorActions = []string{"sample", "analysis", "advisor", "replication", "binary-logs", "binary-events", "profiling", "engines"}

func MonitorSupport(driver, action string) (string, string) {
	if driver == "sqlite" {
		if action == "sample" || action == "engines" {
			return "limited", "Nur grundlegende SQLite-Informationen; keine Serverzähler oder Dateipfade."
		}
		return "unsupported", "SQLite bietet diese Serverfunktion nicht an."
	}
	if driver == "postgres" && (action == "binary-logs" || action == "binary-events") {
		return "unsupported", "PostgreSQL besitzt kein MySQL-Binärlog."
	}
	return "limited", "Nur lesend; Datenquelle, Serverversion und tatsächliche Datenbankrechte bestimmen die Verfügbarkeit. Rohtexte werden unterdrückt."
}

type MonitorEngine interface {
	Monitor(context.Context, string, MonitorRequest) (MonitorResult, error)
}
