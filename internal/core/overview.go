// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// OverviewContractVersion is the API minor of the read-only overview routes.
const OverviewContractVersion = "0.7.0"

// Limits of the overview routes. They are part of the public contract.
const (
	OverviewDefaultLimit     = 200
	OverviewMaxLimit         = 1000
	OverviewMaxOffset        = 1000000
	OverviewMaxExactObjects  = 100
	OverviewDefaultTimeoutMS = 10000
	OverviewMaxTimeoutMS     = 30000
	OverviewMaxListItems     = 1000
	OverviewMaxDefinition    = 1 << 20
)

// DatabasesRequest lists the databases visible to the connection.
type DatabasesRequest struct {
	Connection Connection `json:"connection"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

// DatabaseInfo is one visible database. Null counts mean "not available".
type DatabaseInfo struct {
	Name        string  `json:"name"`
	Charset     *string `json:"charset"`
	Collation   *string `json:"collation"`
	Current     bool    `json:"current"`
	System      bool    `json:"system"`
	ObjectCount *int64  `json:"object_count"`
	SizeBytes   *int64  `json:"size_bytes"`
	Estimated   bool    `json:"estimated"`
}

// DatabasesResult is one page of the database list.
type DatabasesResult struct {
	Driver    string         `json:"driver"`
	Databases []DatabaseInfo `json:"databases"`
	Total     int64          `json:"total"`
	Offset    int            `json:"offset"`
	Limit     int            `json:"limit"`
	HasMore   bool           `json:"has_more"`
}

// ObjectsRequest lists tables and views of one schema (MySQL database).
type ObjectsRequest struct {
	Connection Connection `json:"connection"`
	Schema     string     `json:"schema,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	ExactRows  bool       `json:"exact_rows,omitempty"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

// RowCount distinguishes estimates, exact counts and unknown values.
type RowCount struct {
	Value *int64 `json:"value"`
	State string `json:"state"`
}

// ObjectInfo is one visible table-like object.
type ObjectInfo struct {
	Schema    string   `json:"schema"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Rows      RowCount `json:"rows"`
	Engine    *string  `json:"engine"`
	Collation *string  `json:"collation"`
	SizeBytes *int64   `json:"size_bytes"`
	Comment   *string  `json:"comment"`
}

// ObjectsResult is one page of the object overview.
type ObjectsResult struct {
	Driver          string       `json:"driver"`
	Schema          string       `json:"schema"`
	Objects         []ObjectInfo `json:"objects"`
	Total           int64        `json:"total"`
	Offset          int          `json:"offset"`
	Limit           int          `json:"limit"`
	HasMore         bool         `json:"has_more"`
	ExactRequested  bool         `json:"exact_requested"`
	ExactIncomplete bool         `json:"exact_incomplete"`
}

// DefinitionRequest asks for the definition of one catalog object.
type DefinitionRequest struct {
	Connection Connection `json:"connection"`
	Object     TableRef   `json:"object"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

// DefinitionResult carries the definition text. Derived marks a text that was
// reconstructed from the catalog because the system has no native counterpart.
type DefinitionResult struct {
	Driver     string   `json:"driver"`
	Object     Table    `json:"object"`
	Definition string   `json:"definition"`
	Source     string   `json:"source"`
	Derived    bool     `json:"derived"`
	Notice     *string  `json:"notice"`
	Warnings   []string `json:"warnings"`
}

// ServerInfoRequest asks for server information visible to the connection.
type ServerInfoRequest struct {
	Connection Connection `json:"connection"`
	TimeoutMS  int        `json:"timeout_ms,omitempty"`
}

// ServerConnection echoes non-secret connection data.
type ServerConnection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	TLSMode  string `json:"tls_mode"`
}

type NameValue struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}
type CharacterSetInfo struct {
	Name             string  `json:"name"`
	DefaultCollation *string `json:"default_collation"`
	Description      *string `json:"description"`
	MaxBytes         *int    `json:"max_bytes"`
}
type CollationInfo struct {
	Name    string  `json:"name"`
	Charset *string `json:"charset"`
	Default bool    `json:"default"`
}
type EngineInfo struct {
	Name         string  `json:"name"`
	Support      string  `json:"support"`
	Comment      *string `json:"comment"`
	Transactions *string `json:"transactions"`
}
type PluginInfo struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Status  string  `json:"status"`
	Version *string `json:"version"`
}

// ServerInfoResult never contains credentials, host paths or data directories.
type ServerInfoResult struct {
	Driver              string             `json:"driver"`
	Variant             string             `json:"variant"`
	Version             string             `json:"version"`
	VersionComment      *string            `json:"version_comment"`
	UptimeSeconds       *int64             `json:"uptime_seconds"`
	CurrentDatabase     *string            `json:"current_database"`
	Connection          ServerConnection   `json:"connection"`
	Session             []NameValue        `json:"session"`
	CharacterSets       []CharacterSetInfo `json:"character_sets"`
	CharacterSetsCapped bool               `json:"character_sets_truncated"`
	Collations          []CollationInfo    `json:"collations"`
	CollationsCapped    bool               `json:"collations_truncated"`
	Engines             []EngineInfo       `json:"engines"`
	Plugins             []PluginInfo       `json:"plugins"`
	PluginsCapped       bool               `json:"plugins_truncated"`
}

// OverviewEngine is the optional read-only overview capability of an engine.
type OverviewEngine interface {
	Databases(context.Context, DatabasesRequest) (DatabasesResult, error)
	Objects(context.Context, ObjectsRequest) (ObjectsResult, error)
	Definition(context.Context, DefinitionRequest) (DefinitionResult, error)
	ServerInfo(context.Context, ServerInfoRequest) (ServerInfoResult, error)
}

// Overview action names used in the capability table.
const (
	ActionDatabases  = "databases"
	ActionObjects    = "objects"
	ActionDefinition = "definition"
	ActionServerInfo = "server_info"
	ActionExactRows  = "exact_rows"
)

// OverviewCapabilities is the static per-driver capability table. MariaDB is
// driver "mysql" with the variant reported by server-info.
func OverviewCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	sup := dbplan.Capability{Support: dbplan.Supported}
	lim := func(reason string) dbplan.Capability {
		return dbplan.Capability{Support: dbplan.Limited, Reason: reason}
	}
	c.Set("sqlite", ActionDatabases, lim("Nur die verwaltete Datenbank der Verbindung; kein Serverkatalog, keine Größe."))
	c.Set("sqlite", ActionObjects, sup)
	c.Set("sqlite", ActionDefinition, sup)
	c.Set("sqlite", ActionServerInfo, lim("Keine Laufzeit, Engines oder Plugins; Kollationen sind die eingebauten."))
	c.Set("sqlite", ActionExactRows, sup)
	c.Set("mysql", ActionDatabases, lim("Größen stammen aus Statistikwerten (Schätzung); Objektzahlen sind Katalogzählungen sichtbarer Objekte."))
	c.Set("mysql", ActionObjects, lim("Zeilenzahl und Größe sind Schätzwerte aus den Tabellenstatistiken."))
	c.Set("mysql", ActionDefinition, sup)
	c.Set("mysql", ActionServerInfo, sup)
	c.Set("mysql", ActionExactRows, sup)
	c.Set("postgres", ActionDatabases, lim("Objektzahlen nur für die verbundene Datenbank; andere Datenbanken sind nicht ohne Verbindungswechsel lesbar."))
	c.Set("postgres", ActionObjects, lim("Zeilenzahl ist die Planerschätzung (reltuples)."))
	c.Set("postgres", ActionDefinition, lim("Aus dem Katalog rekonstruiert (derived); kein natives SHOW CREATE."))
	c.Set("postgres", ActionServerInfo, sup)
	c.Set("postgres", ActionExactRows, sup)
	return c
}
