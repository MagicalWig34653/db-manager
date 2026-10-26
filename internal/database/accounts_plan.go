// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func buildAccountPlan(ctx context.Context, env *opEnv, action string, r core.AccountRequest) (out core.AccountPlan, err error) {
	driver := env.c.Driver
	systemSecond := false
	copyPrivileged := false
	roles, lock, err := accountVersion(ctx, env)
	if err != nil {
		return
	}
	if (action == "role_grant" || action == "role_revoke") && !roles || (action == "lock" || action == "unlock") && !lock || action == "flush" && driver == "postgres" {
		return out, accountUnsupported()
	}
	var info core.AccountInfo
	if r.Account != nil {
		info, err = resolveAccount(ctx, env, *r.Account)
		if err != nil && action == "password" && driver == "mysql" {
			// CURRENT_USER is server metadata available even without SELECT mysql.user.
			var current string
			readErr := env.sess.QueryRowContext(ctx, "SELECT CURRENT_USER()").Scan(&current)
			if readErr == nil && r.Account.Name == env.c.Username && current == r.Account.Name+"@"+r.Account.Host {
				info = core.AccountInfo{Account: core.AccountRef{Name: env.c.Username, Host: strings.TrimPrefix(current, env.c.Username+"@")}, Current: true}
				err = nil
			}
		}
		if err != nil {
			return
		}
	}
	if r.NewAccount != nil {
		_, existsErr := resolveAccount(ctx, env, *r.NewAccount)
		if existsErr == nil {
			return out, opExists()
		}
		var p *core.Error
		if !errors.As(existsErr, &p) || p.Code != "object_not_found" {
			return out, existsErr
		}
	}
	if driver == "mysql" && !env.maria {
		for _, name := range []*core.AccountRef{r.Account, r.NewAccount, r.Role} {
			if name != nil && len(name.Name) > 32 {
				return out, public("invalid_account_request", "Der Kontoname ist für MySQL auf 32 Zeichen begrenzt (MariaDB: 80).")
			}
		}
	}
	if r.Account != nil && action != "create" {
		if protected, second := accountSystem(driver, info.Account); protected {
			return out, public("system_account", "Systemkonten des Servers werden hier nicht verändert.")
		} else if second {
			systemSecond = true
		}
	}
	target := "Serverrechte"
	subject := ""
	affected := []string{}
	if r.Account != nil {
		target = accountLabel(info.Account)
		subject = accountQuote(driver, info.Account)
		affected = append(affected, target)
	}
	var dest string
	if r.NewAccount != nil {
		dest = accountQuote(driver, *r.NewAccount)
		affected = append(affected, accountLabel(*r.NewAccount))
		if action == "create" {
			target = accountLabel(*r.NewAccount)
		}
	}
	statements := []string{}
	needsSecret := action == "create" || action == "password" || action == "copy"
	destructive := action == "drop" || action == "rename" || action == "password" || action == "revoke" || action == "role_revoke" || action == "lock" || action == "flush"
	// Role revocation and DROP can affect a current login indirectly through
	// membership. Require the second confirmation conservatively for all of them.
	self := destructive && info.Current || action == "revoke" || action == "role_revoke" || action == "drop" || action == "flush" || systemSecond
	createSQL := func() string {
		if driver == "postgres" {
			return "CREATE ROLE " + dest + " LOGIN PASSWORD <protected-secret>"
		}
		return "CREATE USER " + dest + " IDENTIFIED BY <protected-secret>"
	}
	switch action {
	case "create":
		statements = append(statements, createSQL())
	case "password":
		suffix := " IDENTIFIED BY "
		if driver == "postgres" {
			suffix = " PASSWORD "
		}
		statements = append(statements, "ALTER USER "+subject+suffix+"<protected-secret>")
	case "rename":
		if driver == "postgres" {
			statements = append(statements, "ALTER ROLE "+subject+" RENAME TO "+dest)
		} else {
			statements = append(statements, "RENAME USER "+subject+" TO "+dest)
		}
	case "drop":
		keyword := "USER"
		if driver == "postgres" {
			keyword = "ROLE"
		}
		statements = append(statements, "DROP "+keyword+" "+subject)
	case "lock", "unlock":
		suffix := " ACCOUNT LOCK"
		if action == "unlock" {
			suffix = " ACCOUNT UNLOCK"
		}
		if driver == "postgres" {
			suffix = " NOLOGIN"
			if action == "unlock" {
				suffix = " LOGIN"
			}
		}
		statements = append(statements, "ALTER USER "+subject+suffix)
	case "grant", "revoke":
		statement, buildErr := accountGrantSQL(ctx, env, subject, *r.Grant, action == "revoke")
		if buildErr != nil {
			return out, buildErr
		}
		statements = append(statements, statement)
	case "role_grant", "role_revoke":
		role, resolveErr := resolveAccount(ctx, env, *r.Role)
		if resolveErr != nil {
			return out, resolveErr
		}
		keyword, join := "GRANT", " TO "
		if action == "role_revoke" {
			keyword, join = "REVOKE", " FROM "
		}
		roleSQL := accountQuote(driver, role.Account)
		if env.maria {
			var isRole string
			if err = env.sess.QueryRowContext(ctx, "SELECT is_role FROM mysql.user WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ?", role.Account.Name, role.Account.Host).Scan(&isRole); err != nil {
				return
			}
			if isRole != "Y" {
				return out, accountInvalid()
			}
			roleSQL = quoteIdentifier(driver, role.Account.Name)
		}
		statements = append(statements, keyword+" "+roleSQL+join+subject)
	case "copy":
		details, readErr := readAccountDetails(ctx, env, info.Account)
		if readErr != nil {
			return out, readErr
		}
		statements = append(statements, createSQL())
		for _, g := range details.Grants {
			statement, buildErr := accountGrantSQL(ctx, env, dest, g, false)
			if buildErr != nil {
				return out, buildErr
			}
			statements = append(statements, statement)
			if g.GrantOption || g.Scope == "global" && accountPowerful[g.Privilege] {
				copyPrivileged = true
			}
		}
		for _, role := range details.Roles {
			roleSQL := accountQuote(driver, role)
			if env.maria {
				roleSQL = quoteIdentifier(driver, role.Name)
			}
			statements = append(statements, "GRANT "+roleSQL+" TO "+dest)
		}
	case "flush":
		statements = append(statements, "FLUSH PRIVILEGES")
	default:
		return out, accountInvalid()
	}
	if len(statements) > 1001 {
		return out, public("result_too_large", "Der Plan enthält zu viele Anweisungen.")
	}
	// Copy order is deterministic even if information_schema has no fixed order.
	if action == "copy" && len(statements) > 1 {
		sort.Strings(statements[1:])
	}
	warnings := []string{"Serverrechte werden bei der Ausführung geprüft. Bestehende Sitzungen werden nicht beendet. Kein automatischer Retry; nach der Ausführung Konten und Rechte neu lesen."}
	if driver == "mysql" {
		warnings = append(warnings, "Kontenbefehle bestätigen einzeln und sind nicht gemeinsam rückrollbar; bei Teilfehlern ist der Ausgang unbekannt.")
	}
	privileged := false
	if (action == "grant" || action == "copy") && r.Grant != nil {
		if r.Grant.GrantOption {
			privileged = true
			warnings = append(warnings, "WITH GRANT OPTION erlaubt dem Konto, dieses Recht weiterzugeben (Rechteeskalation). Getippter Kontoname und Plan-Hash sind erforderlich.")
		}
		if r.Grant.Scope == "global" && accountPowerful[r.Grant.Privilege] {
			privileged = true
			warnings = append(warnings, "Das globale Recht "+r.Grant.Privilege+" gewährt weitreichende oder rechteändernde Befugnisse. Getippter Kontoname und Plan-Hash sind erforderlich.")
		}
	}
	if action == "role_grant" {
		privileged = true
		warnings = append(warnings, "Die Rolle überträgt alle ihre Rechte (bei PostgreSQL mit INHERIT-Wirkung) auf das Konto. Getippter Kontoname und Plan-Hash sind erforderlich.")
	}
	if copyPrivileged {
		privileged = true
		warnings = append(warnings, "Die Kopie überträgt weitreichende oder weitergebbare Rechte (GRANT OPTION oder mächtige globale Rechte).")
	}
	if privileged {
		destructive = true
	}
	if action == "revoke" && r.Grant.GrantOption && driver == "mysql" {
		warnings = append(warnings, "MySQL/MariaDB entziehen GRANT OPTION für den gesamten gewählten Geltungsbereich, nicht nur für das angegebene Einzelrecht.")
	}
	if action == "copy" {
		warnings = append(warnings, "Kopiert direkte SQL-Rechte und direkte Rollen ohne ADMIN OPTION. Keine Passwörter, Authentifizierungsplugins, TLS-Vorgaben, Ressourcenlimits, Besitzrechte, globalen PostgreSQL-Rollenattribute oder Standardrollen. Das Ziel erhält ein neues Passwort.")
	}
	if action == "rename" && driver == "postgres" {
		warnings = append(warnings, "Umbenennen kann einen alten MD5-Passwortprüfwert ungültig machen; ggf. Passwort anschließend neu setzen.")
	}
	if needsSecret {
		warnings = append(warnings, "<protected-secret> ist ein fester Platzhalter im Plan-Hash, kein SQL-Literal. Der Passwortwert wird nur bei der Ausführung eingesetzt und ist niemals Bestandteil des Plan-Hashes.")
	}
	bytes := 0
	for _, statement := range statements {
		bytes += len(statement)
		if len(statement) > 16384 || bytes > 1024*1024 {
			return out, public("result_too_large", "Der Kontenplan ist zu groß.")
		}
	}
	plan, err := dbplan.NewPlan(driver, statements, warnings, affected)
	if err != nil {
		return out, err
	}
	out = core.AccountPlan{Plan: plan, Action: action, Confirmation: core.AccountConfirm{Action: action, TargetName: target, AcknowledgeSelf: self}, Destructive: destructive, Self: self, RequiresSecret: needsSecret}
	return out, nil
}

var accountPrivileges = map[string]map[string]bool{
	"global":   privilegeSet("SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|RELOAD|SHUTDOWN|PROCESS|FILE|REFERENCES|INDEX|ALTER|SHOW DATABASES|SUPER|CREATE TEMPORARY TABLES|LOCK TABLES|EXECUTE|REPLICATION SLAVE|REPLICATION CLIENT|CREATE VIEW|SHOW VIEW|CREATE ROUTINE|ALTER ROUTINE|CREATE USER|EVENT|TRIGGER|CREATE TABLESPACE|CREATE ROLE|DROP ROLE|USAGE|CONNECTION_ADMIN|SYSTEM_USER|SYSTEM_VARIABLES_ADMIN|SESSION_VARIABLES_ADMIN|BACKUP_ADMIN|ROLE_ADMIN|BINLOG_ADMIN|REPLICATION_SLAVE_ADMIN|REPLICATION_APPLIER|SET_USER_ID|SET_ANY_DEFINER|ALLOW_NONEXISTENT_DEFINER"),
	"database": privilegeSet("SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|REFERENCES|INDEX|ALTER|CREATE TEMPORARY TABLES|LOCK TABLES|EXECUTE|CREATE VIEW|SHOW VIEW|CREATE ROUTINE|ALTER ROUTINE|EVENT|TRIGGER|CONNECT|TEMPORARY"),
	"table":    privilegeSet("SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|REFERENCES|INDEX|ALTER|CREATE VIEW|SHOW VIEW|TRIGGER|TRUNCATE|MAINTAIN"),
	"column":   privilegeSet("SELECT|INSERT|UPDATE|REFERENCES"),
	"routine":  privilegeSet("EXECUTE|ALTER ROUTINE"),
}

func privilegeSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(s, "|") {
		out[p] = true
	}
	return out
}

func accountGrantSQL(ctx context.Context, env *opEnv, subject string, g core.AccountGrant, revoke bool) (string, error) {

	allowed := accountPrivileges[g.Scope]
	if !allowed[g.Privilege] {
		return "", accountUnsupported()
	}
	if len(g.Database) > 1024 || len(g.Schema) > 1024 || len(g.Object) > 1024 || len(g.Column) > 1024 || strings.ContainsRune(g.Database+g.Schema+g.Object+g.Column, 0) {
		return "", accountInvalid()
	}
	driver := env.c.Driver
	q := func(v string) string { return quoteIdentifier(driver, v) }
	target := ""
	privilege := g.Privilege
	// Reject stray object fields rather than silently dropping them.
	switch g.Scope {
	case "global":
		if driver == "postgres" {
			return "", accountUnsupported()
		}
		if g.Database != "" || g.Schema != "" || g.Object != "" || g.Column != "" || g.RoutineID != "" || g.RoutineKind != "" {
			return "", accountInvalid()
		}
		target = "*.*"
	case "database":
		if g.Schema != "" || g.Object != "" || g.Column != "" || g.RoutineID != "" || g.RoutineKind != "" {
			return "", accountInvalid()
		}
		var name string
		query := "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE BINARY SCHEMA_NAME=BINARY ?"
		if driver == "postgres" {
			query = "SELECT datname FROM pg_catalog.pg_database WHERE datname=$1"
		}
		if err := env.sess.QueryRowContext(ctx, query, g.Database).Scan(&name); err != nil {
			return "", accountSafe(ctx, err)
		}
		if name != g.Database {
			return "", opNotFound()
		}
		target = q(name) + ".*"
		if driver == "postgres" {
			if !privilegeSet("CREATE|CONNECT|TEMPORARY")[privilege] {
				return "", accountUnsupported()
			}
			target = "DATABASE " + q(name)
		} else if strings.ContainsAny(name, "_%\\") {
			// MySQL database grants treat these as wildcard patterns even in quotes.
			// Refuse them rather than accidentally granting access to other databases.
			return "", accountUnsupported()
		}
	case "table", "column":
		if g.RoutineID != "" || g.RoutineKind != "" || g.Scope == "table" && g.Column != "" {
			return "", accountInvalid()
		}
		schema := g.Database
		if driver == "postgres" {
			var current string
			if err := env.sess.QueryRowContext(ctx, "SELECT current_database()").Scan(&current); err != nil {
				return "", err
			}
			if g.Database != current {
				return "", accountUnsupported()
			}
			schema = g.Schema
			if !privilegeSet("SELECT|INSERT|UPDATE|DELETE|TRUNCATE|REFERENCES|TRIGGER|MAINTAIN")[privilege] {
				return "", accountUnsupported()
			}
		} else if g.Schema != "" {
			return "", accountInvalid()
		}
		var actualSchema, actualObject string
		query := "SELECT TABLE_SCHEMA,TABLE_NAME FROM information_schema.TABLES WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ?"
		if driver == "postgres" {
			query = "SELECT n.nspname,c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind IN ('r','p','v','m','f')"
		}
		if err := env.sess.QueryRowContext(ctx, query, schema, g.Object).Scan(&actualSchema, &actualObject); err != nil {
			return "", accountSafe(ctx, err)
		}
		if actualSchema != schema || actualObject != g.Object {
			return "", opNotFound()
		}
		target = q(actualSchema) + "." + q(actualObject)
		if driver == "postgres" {
			target = "TABLE " + target
		}
		if g.Scope == "column" {
			var col string
			query = "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE BINARY TABLE_SCHEMA=BINARY ? AND BINARY TABLE_NAME=BINARY ? AND BINARY COLUMN_NAME=BINARY ?"
			if driver == "postgres" {
				query = "SELECT a.attname FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2 AND a.attname=$3 AND a.attnum>0 AND NOT a.attisdropped AND c.relkind IN ('r','p','v','m','f')"
			}
			if err := env.sess.QueryRowContext(ctx, query, schema, g.Object, g.Column).Scan(&col); err != nil {
				return "", accountSafe(ctx, err)
			}
			if col != g.Column {
				return "", opNotFound()
			}
			privilege += " (" + q(col) + ")"
		}
	case "routine":
		if g.Column != "" || g.RoutineKind != "function" && g.RoutineKind != "procedure" {
			return "", accountInvalid()
		}
		var schema, name, kind, signature string
		if driver == "postgres" {
			if privilege != "EXECUTE" {
				return "", accountUnsupported()
			}
			if _, err := strconv.ParseUint(g.RoutineID, 10, 32); err != nil {
				return "", accountInvalid()
			}
			query := `SELECT n.nspname,p.proname,CASE WHEN p.prokind='p' THEN 'procedure' ELSE 'function' END,pg_catalog.pg_get_function_identity_arguments(p.oid) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE p.oid=$1::oid AND p.prokind IN ('f','p') AND current_database()=$2`
			if err := env.sess.QueryRowContext(ctx, query, g.RoutineID, g.Database).Scan(&schema, &name, &kind, &signature); err != nil {
				return "", accountSafe(ctx, err)
			}
			if schema != g.Schema || name != g.Object || kind != g.RoutineKind {
				return "", opNotFound()
			}
			// Signature is formatted only by PostgreSQL from its own catalog.
			target = strings.ToUpper(kind) + " " + q(schema) + "." + q(name) + "(" + signature + ")"
		} else {
			if g.Schema != "" || g.RoutineID != "" {
				return "", accountInvalid()
			}
			query := `SELECT ROUTINE_SCHEMA,ROUTINE_NAME,ROUTINE_TYPE FROM information_schema.ROUTINES WHERE BINARY ROUTINE_SCHEMA=BINARY ? AND BINARY ROUTINE_NAME=BINARY ? AND ROUTINE_TYPE=?`
			if err := env.sess.QueryRowContext(ctx, query, g.Database, g.Object, strings.ToUpper(g.RoutineKind)).Scan(&schema, &name, &kind); err != nil {
				return "", accountSafe(ctx, err)
			}
			if schema != g.Database || name != g.Object {
				return "", opNotFound()
			}
			target = strings.ToUpper(kind) + " " + q(schema) + "." + q(name)
		}
	default:
		return "", accountInvalid()
	}
	if revoke {
		prefix := "REVOKE "
		if g.GrantOption {
			if driver == "mysql" {
				return "REVOKE GRANT OPTION ON " + target + " FROM " + subject, nil
			}
			prefix += "GRANT OPTION FOR "
		}
		return prefix + privilege + " ON " + target + " FROM " + subject, nil
	}
	suffix := ""
	if g.GrantOption {
		suffix = " WITH GRANT OPTION"
	}
	return "GRANT " + privilege + " ON " + target + " TO " + subject + suffix, nil
}

// accountPowerful lists global privileges that allow privilege changes, impersonation or definer abuse.
var accountPowerful = privilegeSet("SUPER|CREATE USER|SET_USER_ID|SYSTEM_USER|SET_ANY_DEFINER|ROLE_ADMIN|CREATE ROLE|DROP ROLE|SHUTDOWN|FILE|RELOAD|REPLICATION_SLAVE_ADMIN|BINLOG_ADMIN|SYSTEM_VARIABLES_ADMIN|ALLOW_NONEXISTENT_DEFINER")

// accountSystem reports accounts the server owns (protected: refused outright) and
// administrative accounts that require the second confirmation like the own account.
func accountSystem(driver string, a core.AccountRef) (protected, second bool) {
	name := strings.ToLower(a.Name)
	if driver == "postgres" {
		return strings.HasPrefix(name, "pg_"), name == "postgres"
	}
	switch name {
	case "mysql.sys", "mysql.session", "mysql.infoschema", "mariadb.sys":
		return true, false
	case "root":
		return false, true
	}
	return false, false
}
