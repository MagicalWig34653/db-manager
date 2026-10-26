// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func accountInvalid() error {
	return public("invalid_account_request", "Die Kontenanfrage ist ungültig.")
}
func accountUnsupported() error {
	return public("unsupported_action", "Diese Kontenfunktion wird vom Server oder für diesen Geltungsbereich nicht unterstützt.")
}
func accountSafe(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return opNotFound()
	}
	var p *core.Error
	if errors.As(err, &p) {
		return err
	}
	if reason, definite := opReason(err); definite {
		return public(reason, reasonText[reason])
	}
	return safe(ctx, err)
}
func accountName(driver string, a core.AccountRef) bool {
	max := 32
	if driver == "postgres" {
		max = 63
	}
	return a.Name != "" && utf8.ValidString(a.Name) && len(a.Name) <= max && !strings.ContainsAny(a.Name, "\x00\r\n") && utf8.ValidString(a.Host) && len(a.Host) <= 255 && !strings.ContainsAny(a.Host, "\x00\r\n") && (driver != "postgres" || a.Host == "")
}
func accountLabel(a core.AccountRef) string {
	// Length-prefixed identity avoids ambiguity between name and host.
	return strconv.Itoa(len(a.Name)) + ":" + a.Name + "@" + a.Host
}
func accountQuote(driver string, a core.AccountRef) string {
	if driver == "postgres" {
		return quoteIdentifier(driver, a.Name)
	}
	return accountLiteral(a.Name) + "@" + accountLiteral(a.Host)
}
func accountLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func validateAccountRequest(r core.AccountRequest, action string) error {
	if r.Connection.Driver == "sqlite" {
		return accountUnsupported()
	}
	if r.Connection.Driver != "mysql" && r.Connection.Driver != "postgres" {
		return accountInvalid()
	}
	if r.TimeoutMS < 0 || r.TimeoutMS > 60000 || r.Limit < 0 || r.Limit > core.AccountsMaxItems || r.Offset < 0 || r.Offset > 1000000 {
		return accountInvalid()
	}
	if action != "list" && (r.Limit != 0 || r.Offset != 0) {
		return accountInvalid()
	}
	if action == "list" || action == "flush" {
		if r.Account != nil {
			return accountInvalid()
		}
	} else if r.Account == nil || !accountName(r.Connection.Driver, *r.Account) {
		return accountInvalid()
	}
	if r.Connection.Driver == "mysql" && r.Account != nil && r.Account.Host == "" && action != "inspect" {
		return accountInvalid()
	}
	if (action == "rename" || action == "copy") != (r.NewAccount != nil) {
		return accountInvalid()
	}
	// Create uses account as its new identity, rather than two identities.
	if action == "create" {
		return accountInvalid()
	}
	if r.NewAccount != nil && (!accountName(r.Connection.Driver, *r.NewAccount) || r.Connection.Driver == "mysql" && r.NewAccount.Host == "") {
		return accountInvalid()
	}
	if (action == "grant" || action == "revoke") != (r.Grant != nil) {
		return accountInvalid()
	}
	if (action == "role_grant" || action == "role_revoke") != (r.Role != nil) {
		return accountInvalid()
	}
	if r.Role != nil && !accountName(r.Connection.Driver, *r.Role) {
		return accountInvalid()
	}
	return nil
}

// The create route has only a new account; all other actions resolve an existing account.
func validateAccounts(r core.AccountRequest, action string) error {
	if action == "create" {
		if r.Account != nil || r.NewAccount == nil || r.Connection.Driver == "mysql" && r.NewAccount.Host == "" {
			return accountInvalid()
		}
		check := r
		check.Account = r.NewAccount
		check.NewAccount = nil
		return validateAccountRequest(check, "inspect")
	}
	known := action == "list" || action == "inspect"
	for _, a := range core.AccountActions {
		known = known || a == action
	}
	if !known {
		return accountInvalid()
	}
	return validateAccountRequest(r, action)
}
func accountContext(parent context.Context, r core.AccountRequest) (context.Context, context.CancelFunc) {
	ms := r.TimeoutMS
	if ms == 0 {
		ms = 30000
	}
	return context.WithTimeout(parent, time.Duration(ms)*time.Millisecond)
}

func (e *Engine) Accounts(parent context.Context, r core.AccountRequest) (out core.AccountsResult, err error) {
	if err = validateAccounts(r, "list"); err != nil {
		return
	}
	ctx, cancel := accountContext(parent, r)
	defer cancel()
	env, closeAll, err := e.openAccounts(ctx, r.Connection)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	defer closeAll()
	out, err = readAccounts(ctx, env, r)
	return out, accountSafe(ctx, err)
}
func (e *Engine) AccountDetails(parent context.Context, r core.AccountRequest) (out core.AccountDetails, err error) {
	if err = validateAccounts(r, "inspect"); err != nil {
		return
	}
	ctx, cancel := accountContext(parent, r)
	defer cancel()
	env, closeAll, err := e.openAccounts(ctx, r.Connection)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	defer closeAll()
	out, err = readAccountDetails(ctx, env, *r.Account)
	return out, accountSafe(ctx, err)
}
func (e *Engine) AccountPlan(parent context.Context, action string, r core.AccountRequest) (out core.AccountPlan, err error) {
	if err = validateAccounts(r, action); err != nil {
		return
	}
	ctx, cancel := accountContext(parent, r)
	defer cancel()
	env, closeAll, err := e.openAccounts(ctx, r.Connection)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	defer closeAll()
	out, err = buildAccountPlan(ctx, env, action, r)
	return out, accountSafe(ctx, err)
}
func (e *Engine) AccountExecute(parent context.Context, action string, r core.AccountExecuteRequest) (out core.AccountResult, err error) {
	if err = validateAccounts(r.AccountRequest, action); err != nil {
		return
	}
	needsSecret := action == "create" || action == "copy" || action == "password"
	if needsSecret != (r.Secret != nil) || r.Secret != nil && (len(*r.Secret) == 0 || len(*r.Secret) > 4096 || !utf8.ValidString(*r.Secret) || strings.ContainsRune(*r.Secret, 0)) {
		return out, accountInvalid()
	}
	ctx, cancel := accountContext(parent, r.AccountRequest)
	defer cancel()
	env, closeAll, err := e.openAccounts(ctx, r.Connection)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	defer closeAll()
	plan, err := buildAccountPlan(ctx, env, action, r.AccountRequest)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	if err = dbplan.Check(plan.Plan, r.ExpectedPlanSHA256, dbplan.Confirmation{Action: r.Confirm.Action, TargetName: r.Confirm.TargetName}, action, plan.Confirmation.TargetName, plan.Destructive); err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) {
			return out, public("plan_mismatch", "Der Plan hat sich geändert.")
		}
		return out, public("confirmation_mismatch", "Bestätigung oder Plan-Hash ist ungültig.")
	}
	if plan.Self && !r.Confirm.AcknowledgeSelf {
		return out, public("confirmation_mismatch", "Diese Aktion am eigenen Konto erfordert eine zweite ausdrückliche Bestätigung.")
	}
	// Plans contain a non-SQL secret marker. Only execute substitutes the protected
	// value after validation; neither the expanded statement nor driver errors escape.
	statements := append([]string{}, plan.Statements...)
	if needsSecret {
		// Replace only the final marker of the first statement, never identifier text.
		statements[0] = strings.TrimSuffix(statements[0], "<protected-secret>") + accountLiteral(*r.Secret)
	}
	return executeAccounts(ctx, env, statements), nil
}
func executeAccounts(ctx context.Context, env *opEnv, statements []string) (out core.AccountResult) {
	transactional := env.c.Driver == "postgres"
	if transactional {
		if err := env.sess.begin(ctx); err != nil {
			return core.AccountResult{Outcome: "rejected", Message: "Transaktion konnte nicht gestartet werden."}
		}
		defer env.sess.rollback()
	}
	for i, statement := range statements {
		if ctx.Err() != nil && i == 0 {
			return core.AccountResult{Outcome: "rejected", Reason: "canceled", Message: "Vor der Ausführung abgebrochen."}
		}
		if _, err := env.sess.exec(ctx, statement); err != nil {
			reason, definite := opReason(err)
			out.Outcome = "outcome_unknown"
			out.Message = "Ausgang unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			if definite && (transactional || i == 0) {
				out.Outcome = "rejected"
				out.Reason = reason
				out.Message = reasonText[reason]
				out.StatementsExecuted = 0
			}
			return
		}
		out.StatementsExecuted = i + 1
	}
	if transactional {
		if err := env.sess.commit(ctx); err != nil {
			out.Outcome = "outcome_unknown"
			out.Message = "Commit-Ausgang unbekannt. Zustand prüfen; nicht automatisch wiederholen."
			return
		}
		out.Atomic = true
	}
	out.Outcome = "completed"
	out.Message = "Ausgeführt. Konten und Rechte anschließend neu lesen; bestehende Sitzungen bleiben möglicherweise aktiv."
	return
}

func accountVersion(ctx context.Context, env *opEnv) (roles, lock bool, err error) {
	if env.c.Driver == "postgres" {
		return true, true, nil
	}
	var version string
	if err = env.sess.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return
	}

	return accountServerFeatures(version, env.maria)
}

func accountServerFeatures(version string, maria bool) (roles, lock bool, err error) {
	if maria {
		version = strings.TrimPrefix(version, "5.5.5-")
	}
	var major, minor, patch int
	if _, err = fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return false, false, accountUnsupported()
	}
	if maria {
		return major > 10 || major == 10 && (minor > 0 || minor == 0 && patch >= 5), major > 10 || major == 10 && (minor > 4 || minor == 4 && patch >= 2), nil
	}
	return major >= 8, major > 5 || major == 5 && (minor > 7 || minor == 7 && patch >= 6), nil
}

func (e *Engine) AccountAccess(parent context.Context, r core.AccountRequest) (out core.AccountAccess, err error) {
	out.Actions = []string{}
	out.Notes = []string{"Konservative Anzeigehilfe, keine Autorisierung. Objektbezogene Rechte und geerbte Rechte können zusätzliche Aktionen erlauben; der Datenbankserver entscheidet bei Ausführung."}
	if err = validateAccounts(r, "list"); err != nil {
		return
	}
	if r.Limit != 0 || r.Offset != 0 {
		return out, accountInvalid()
	}
	ctx, cancel := accountContext(parent, r)
	defer cancel()
	env, closeAll, err := e.openAccounts(ctx, r.Connection)
	if err != nil {
		return out, accountSafe(ctx, err)
	}
	defer closeAll()
	var admin bool
	if env.c.Driver == "postgres" {
		err = env.sess.QueryRowContext(ctx, "SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=current_user").Scan(&admin)
	} else {
		// Access to mysql.user itself is restricted. Failure hides mutations while
		// the actual operations still independently enforce server privileges.
		err = env.sess.QueryRowContext(ctx, "SELECT Super_priv='Y' AND Create_user_priv='Y' AND Grant_priv='Y' FROM mysql.user WHERE CONCAT(User,'@',Host)=CURRENT_USER() LIMIT 1").Scan(&admin)
	}
	if err != nil {
		out.Notes = append(out.Notes, "Administrationsrechte konnten nicht bestätigt werden; Schreibaktionen sind ausgeblendet.")
		return out, nil
	}
	if admin {
		roles, lock, versionErr := accountVersion(ctx, env)
		if versionErr != nil {
			return out, accountSafe(ctx, versionErr)
		}
		for _, action := range core.AccountActions {
			if !roles && strings.HasPrefix(action, "role_") || !lock && (action == "lock" || action == "unlock") || env.c.Driver == "postgres" && action == "flush" {
				continue
			}
			out.Actions = append(out.Actions, action)
		}
	}
	return out, nil
}

// openAccounts prepares deterministic quoting without table-statistics settings
// that do not exist on older supported MySQL servers.
func (e *Engine) openAccounts(ctx context.Context, c core.Connection) (*opEnv, func(), error) {
	db, cleanup, err := e.open(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		cleanup()
		return nil, nil, accountSafe(ctx, err)
	}
	closeAll := func() { conn.Close(); cleanup() }
	env := &opEnv{c: c, sess: &mutationSession{conn: conn, driver: c.Driver}}
	if c.Driver == "mysql" {
		var modes, version string
		if err = conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode,VERSION()").Scan(&modes, &version); err == nil {
			env.maria = strings.Contains(version, "MariaDB")
			_, err = conn.ExecContext(ctx, "SET SESSION sql_mode = ?", modes+",NO_BACKSLASH_ESCAPES,STRICT_ALL_TABLES")
		}
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET NAMES utf8mb4")
		}
		if err == nil {
			_, err = conn.ExecContext(ctx, "SET SESSION lock_wait_timeout = 10")
		}
	} else {
		if _, err = conn.ExecContext(ctx, "SET standard_conforming_strings = on"); err == nil {
			_, err = conn.ExecContext(ctx, "SET lock_timeout = '10s'")
		}
	}
	if err != nil {
		closeAll()
		return nil, nil, accountSafe(ctx, err)
	}
	return env, closeAll, nil
}
