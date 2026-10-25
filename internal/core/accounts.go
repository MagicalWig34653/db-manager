// SPDX-License-Identifier: AGPL-3.0-only
package core

import (
	"context"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const AccountsContractVersion = "0.16.0"
const AccountsMaxItems = 500

var AccountActions = []string{"create", "rename", "drop", "password", "grant", "revoke", "lock", "unlock", "role_grant", "role_revoke", "copy", "flush"}

type AccountRef struct {
	Name string `json:"name"`
	Host string `json:"host,omitempty"`
}

type AccountGrant struct {
	Scope       string `json:"scope"`
	Database    string `json:"database,omitempty"`
	Schema      string `json:"schema,omitempty"`
	Object      string `json:"object,omitempty"`
	Column      string `json:"column,omitempty"`
	RoutineKind string `json:"routine_kind,omitempty"`
	// RoutineID is a PostgreSQL catalog OID, never an SQL signature from a client.
	RoutineID   string `json:"routine_id,omitempty"`
	Privilege   string `json:"privilege"`
	GrantOption bool   `json:"grant_option"`
}

type AccountRequest struct {
	Connection Connection    `json:"connection"`
	Account    *AccountRef   `json:"account,omitempty"`
	NewAccount *AccountRef   `json:"new_account,omitempty"`
	Role       *AccountRef   `json:"role,omitempty"`
	Grant      *AccountGrant `json:"grant,omitempty"`
	TimeoutMS  int           `json:"timeout_ms,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	Offset     int           `json:"offset,omitempty"`
}

// Secret is accepted only by execute, never by planning or reading routes.
// It is intentionally excluded from every response and plan digest.
type AccountExecuteRequest struct {
	AccountRequest
	Secret             *string        `json:"secret,omitempty"`
	Confirm            AccountConfirm `json:"confirm"`
	ExpectedPlanSHA256 string         `json:"expected_plan_sha256"`
}

type AccountConfirm struct {
	Action          string `json:"action"`
	TargetName      string `json:"target_name,omitempty"`
	AcknowledgeSelf bool   `json:"acknowledge_self,omitempty"`
}

type AccountInfo struct {
	Account    AccountRef       `json:"account"`
	Current    bool             `json:"current"`
	Login      *bool            `json:"login"`
	Locked     *bool            `json:"locked"`
	AuthPlugin *string          `json:"auth_plugin"`
	SSLType    *string          `json:"ssl_type"`
	Limits     map[string]int64 `json:"limits"`
	Attributes []string         `json:"attributes"`
}

type AccountsResult struct {
	Accounts  []AccountInfo `json:"accounts"`
	Truncated bool          `json:"truncated"`
}

type AccountDetails struct {
	Info   AccountInfo    `json:"info"`
	Grants []AccountGrant `json:"grants"`
	Roles  []AccountRef   `json:"roles"`
	Notes  []string       `json:"notes"`
}

type AccountPlan struct {
	dbplan.Plan
	Action         string         `json:"action"`
	Confirmation   AccountConfirm `json:"confirmation"`
	Destructive    bool           `json:"destructive"`
	Self           bool           `json:"self"`
	RequiresSecret bool           `json:"requires_secret"`
}

type AccountResult struct {
	Outcome            string `json:"outcome"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message"`
	StatementsExecuted int    `json:"statements_executed"`
	Atomic             bool   `json:"atomic"`
}

type AccountsCapabilities struct {
	ContractVersion string              `json:"contract_version"`
	Drivers         dbplan.Capabilities `json:"drivers"`
}

func AccountCapabilities() dbplan.Capabilities {
	c := dbplan.Capabilities{}
	for _, action := range append([]string{"list", "inspect"}, AccountActions...) {
		c.Set("sqlite", action, dbplan.Capability{Support: dbplan.Unsupported, Reason: "SQLite verwaltet keine Datenbankkonten oder Serverrechte."})
		c.Set("mysql", action, dbplan.Capability{Support: dbplan.Limited, Reason: "Serverrechte erforderlich; Rollen und Kontosperren werden anhand der Serverversion geprüft. Kopie umfasst direkte SQL-Rechte und Rollen, keine Anmeldedaten oder Ressourcenlimits."})
		c.Set("postgres", action, dbplan.Capability{Support: dbplan.Limited, Reason: "Serverrechte erforderlich; Rechte im verbundenen Datenbankkatalog, globale Rollenattribute nur lesend. Sperren setzt NOLOGIN; bestehende Sitzungen bleiben aktiv."})
	}
	c.Set("postgres", "flush", dbplan.Capability{Support: dbplan.Unsupported, Reason: "PostgreSQL benötigt kein FLUSH PRIVILEGES."})
	return c
}

type AccountsEngine interface {
	Accounts(context.Context, AccountRequest) (AccountsResult, error)
	AccountDetails(context.Context, AccountRequest) (AccountDetails, error)
	AccountPlan(context.Context, string, AccountRequest) (AccountPlan, error)
	AccountExecute(context.Context, string, AccountExecuteRequest) (AccountResult, error)
}

// AccountAccess is conservative connection-specific action visibility. Object
// rights remain server-enforced at execution, including privileges via roles.
type AccountAccess struct {
	Actions []string `json:"actions"`
	Notes   []string `json:"notes"`
}
type AccountsAccessEngine interface {
	AccountAccess(context.Context, AccountRequest) (AccountAccess, error)
}
