// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"strconv"
	"strings"
)

func readAccounts(ctx context.Context, env *opEnv, r core.AccountRequest) (out core.AccountsResult, err error) {
	out.Accounts = []core.AccountInfo{}
	limit := r.Limit
	if limit == 0 {
		limit = 100
	}
	var rows *sql.Rows
	if env.c.Driver == "postgres" {
		query := `SELECT rolname, rolcanlogin, rolconnlimit, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls, rolname = session_user OR rolname = current_user FROM pg_catalog.pg_roles`
		args := []any{}
		if r.Account != nil {
			query += " WHERE rolname = $1"
			args = append(args, r.Account.Name)
		}
		query += " ORDER BY rolname LIMIT " + strconv.Itoa(limit+1) + " OFFSET " + strconv.Itoa(r.Offset)
		rows, err = env.sess.QueryContext(ctx, query, args...)
	} else {
		_, lock, verErr := accountVersion(ctx, env)
		if verErr != nil {
			return out, verErr
		}
		locked := "NULL"
		if lock && !env.maria {
			locked = "account_locked = 'Y'"
		}
		// MariaDB exposes lock state in global_priv, never select its authentication data.
		if lock && env.maria {
			locked = "COALESCE(JSON_VALUE((SELECT Priv FROM mysql.global_priv gp WHERE gp.User=u.User AND gp.Host=u.Host), '$.account_locked'), 'false') = 'true'"
		}
		query := `SELECT User, Host, plugin, ssl_type, max_questions, max_updates, max_connections, max_user_connections, ` + locked + `, CONCAT(User,'@',Host) = CURRENT_USER() FROM mysql.user u`
		args := []any{}
		if r.Account != nil {
			query += " WHERE BINARY User = BINARY ? AND BINARY Host = BINARY ?"
			args = append(args, r.Account.Name, r.Account.Host)
		}
		query += " ORDER BY User, Host LIMIT ? OFFSET ?"
		args = append(args, limit+1, r.Offset)
		rows, err = env.sess.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		a := core.AccountInfo{Limits: map[string]int64{}, Attributes: []string{}}
		if env.c.Driver == "postgres" {
			var login, super, db, role, repl, bypass bool
			var connections int64
			err = rows.Scan(&a.Account.Name, &login, &connections, &super, &db, &role, &repl, &bypass, &a.Current)
			a.Login = &login
			locked := !login
			a.Locked = &locked
			a.Limits["connections"] = connections
			for i, v := range []bool{super, db, role, repl, bypass} {
				if v {
					a.Attributes = append(a.Attributes, []string{"SUPERUSER", "CREATEDB", "CREATEROLE", "REPLICATION", "BYPASSRLS"}[i])
				}
			}
		} else {
			var plugin, ssl sql.NullString
			var locked sql.NullBool
			var questions, updates, connections, userConnections int64
			err = rows.Scan(&a.Account.Name, &a.Account.Host, &plugin, &ssl, &questions, &updates, &connections, &userConnections, &locked, &a.Current)
			a.AuthPlugin = nullString(plugin)
			a.SSLType = nullString(ssl)
			if locked.Valid {
				a.Locked = &locked.Bool
			}
			a.Limits = map[string]int64{"queries_per_hour": questions, "updates_per_hour": updates, "connections_per_hour": connections, "user_connections": userConnections}
		}
		if err != nil {
			return out, err
		}
		if len(out.Accounts) == limit {
			out.Truncated = true
			break
		}
		out.Accounts = append(out.Accounts, a)
	}
	return out, rows.Err()
}

func resolveAccount(ctx context.Context, env *opEnv, a core.AccountRef) (core.AccountInfo, error) {
	out, err := readAccounts(ctx, env, core.AccountRequest{Account: &a, Limit: 1})
	if err != nil {
		return core.AccountInfo{}, err
	}
	if len(out.Accounts) != 1 || out.Accounts[0].Account != a {
		return core.AccountInfo{}, opNotFound()
	}
	if env.c.Driver == "mysql" && a.Host == "" {
		if !env.maria {
			return core.AccountInfo{}, accountInvalid()
		}
		var isRole string
		if err := env.sess.QueryRowContext(ctx, "SELECT is_role FROM mysql.user WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ?", a.Name, a.Host).Scan(&isRole); err != nil {
			return core.AccountInfo{}, err
		}
		if isRole != "Y" {
			return core.AccountInfo{}, accountInvalid()
		}
	}
	info := out.Accounts[0]
	// Conservatively protect the requested login too (proxy/session identities may differ).
	info.Current = info.Current || info.Account.Name == env.c.Username
	return info, nil
}

func readAccountDetails(ctx context.Context, env *opEnv, a core.AccountRef) (out core.AccountDetails, err error) {
	out.Info, err = resolveAccount(ctx, env, a)
	if err != nil {
		return
	}
	out.Grants = []core.AccountGrant{}
	out.Roles = []core.AccountRef{}
	out.Notes = []string{"Direkte Rechte und direkte Rollenmitgliedschaften; geerbte und öffentliche Rechte werden nicht als direkte Rechte ausgegeben. Ressourcen- und Authentifizierungseinstellungen sind nur lesend."}
	if env.c.Driver == "postgres" {
		out.Notes = append(out.Notes, "Objektrechte stammen nur aus der verbundenen Datenbank. Authentifizierungsverfahren und TLS-Regeln werden durch pg_hba.conf bestimmt und sind hier unbekannt. Rollenattribute stehen gesondert unter attributes; Besitzrechte und PUBLIC-Rechte sind nicht kopierbar.")
		err = readPostgresAccountGrants(ctx, env, a, &out)
	} else {
		out.Notes = append(out.Notes, "SQL-Rechte aus information_schema; dynamische MySQL-Rechte werden gesondert gelesen. MariaDB-Mehrfachauthentifizierung wird nicht vollständig durch die primäre Pluginanzeige beschrieben.")
		err = readMySQLAccountGrants(ctx, env, a, &out)
	}
	if err != nil {
		return
	}
	roles, _, err := accountVersion(ctx, env)
	if err != nil {
		return out, err
	}
	if !roles {
		out.Notes = append(out.Notes, "Dieser Server unterstützt keine Rollen.")
		return out, nil
	}
	var rows *sql.Rows
	switch {
	case env.c.Driver == "postgres":
		rows, err = env.sess.QueryContext(ctx, `SELECT r.rolname, '' FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member WHERE u.rolname=$1 ORDER BY r.rolname LIMIT 501`, a.Name)
	case env.maria:
		rows, err = env.sess.QueryContext(ctx, `SELECT Role, '' FROM mysql.roles_mapping WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ? ORDER BY Role LIMIT 501`, a.Name, a.Host)
	default:
		rows, err = env.sess.QueryContext(ctx, `SELECT FROM_USER, FROM_HOST FROM mysql.role_edges WHERE BINARY TO_USER=BINARY ? AND BINARY TO_HOST=BINARY ? ORDER BY FROM_USER, FROM_HOST LIMIT 501`, a.Name, a.Host)
	}
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var role core.AccountRef
		if err = rows.Scan(&role.Name, &role.Host); err != nil {
			return
		}
		out.Roles = append(out.Roles, role)
		if len(out.Roles) > core.AccountsMaxItems {
			return out, public("result_too_large", "Zu viele Rollenmitgliedschaften.")
		}
	}
	return out, rows.Err()
}

func readMySQLAccountGrants(ctx context.Context, env *opEnv, a core.AccountRef, out *core.AccountDetails) error {
	// GRANTEE is a bound metadata value, not an SQL literal. The catalog
	// wraps raw account parts in quotes without SQL-escaping embedded quotes.
	grantee := "'" + a.Name + "'@'" + a.Host + "'"
	// Embedded quote/@ sequences can make the catalog's raw display identity
	// ambiguous. Refuse a collision rather than copying another account's grants.
	var identities int
	if err := env.sess.QueryRowContext(ctx, `SELECT COUNT(*) FROM mysql.user WHERE BINARY CONCAT(CHAR(39),User,CHAR(39,64,39),Host,CHAR(39))=BINARY ?`, grantee).Scan(&identities); err != nil {
		return err
	}
	if identities == 0 {
		return opNotFound()
	}
	if identities != 1 {
		return accountUnsupported()
	}
	queries := []struct{ scope, query string }{
		{"global", `SELECT '', '', '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.USER_PRIVILEGES WHERE BINARY GRANTEE=BINARY ?`},
		{"database", `SELECT TABLE_SCHEMA, '', '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.SCHEMA_PRIVILEGES WHERE BINARY GRANTEE=BINARY ?`},
		{"table", `SELECT TABLE_SCHEMA, TABLE_NAME, '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.TABLE_PRIVILEGES WHERE BINARY GRANTEE=BINARY ?`},
		{"column", `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.COLUMN_PRIVILEGES WHERE BINARY GRANTEE=BINARY ?`},
		{"routine", `SELECT ROUTINE_SCHEMA, ROUTINE_NAME, '', ROUTINE_TYPE, PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.ROUTINE_PRIVILEGES WHERE BINARY GRANTEE=BINARY ?`},
	}
	// MySQL has no ROUTINE_PRIVILEGES view; use mysql.procs_priv with a fixed privilege expansion.
	queries[4].query = `SELECT Db, Routine_name, '', Routine_type, p.privilege_name, IF(FIND_IN_SET('Grant',Proc_priv)>0,'YES','NO') FROM mysql.procs_priv CROSS JOIN (SELECT 'EXECUTE' AS privilege_name, 'Execute' AS set_name UNION ALL SELECT 'ALTER ROUTINE','Alter Routine') p WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ? AND FIND_IN_SET(p.set_name,Proc_priv)>0`
	for _, q := range queries {
		args := []any{grantee}
		if q.scope == "routine" {
			args = []any{a.Name, a.Host}
		}
		rows, err := env.sess.QueryContext(ctx, q.query+" LIMIT 501", args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			g := core.AccountGrant{Scope: q.scope}
			var grantable string
			if err = rows.Scan(&g.Database, &g.Object, &g.Column, &g.RoutineKind, &g.Privilege, &grantable); err != nil {
				rows.Close()
				return err
			}
			g.GrantOption = grantable == "YES"
			g.RoutineKind = strings.ToLower(g.RoutineKind)
			out.Grants = append(out.Grants, g)
			if len(out.Grants) > core.AccountsMaxItems {
				rows.Close()
				return public("result_too_large", "Zu viele direkte Rechte.")
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if env.maria && a.Host == "" {
		return readMariaRoleGlobalGrants(ctx, env, a, out)
	}
	if !env.maria {
		roles, _, err := accountVersion(ctx, env)
		if err != nil {
			return err
		}
		if roles {
			rows, err := env.sess.QueryContext(ctx, `SELECT PRIV, WITH_GRANT_OPTION FROM mysql.global_grants WHERE BINARY USER=BINARY ? AND BINARY HOST=BINARY ? LIMIT 501`, a.Name, a.Host)
			if err != nil {
				return err
			}
			defer rows.Close()
			// Recent MySQL versions also expose dynamic grants in USER_PRIVILEGES.
			// Keep the explicit catalog read for compatibility, without duplicates.
			seen := make(map[core.AccountGrant]bool, len(out.Grants))
			for _, grant := range out.Grants {
				seen[grant] = true
			}
			for rows.Next() {
				g := core.AccountGrant{Scope: "global"}
				var option string
				if err = rows.Scan(&g.Privilege, &option); err != nil {
					return err
				}
				g.GrantOption = option == "Y"
				if seen[g] {
					continue
				}
				seen[g] = true
				out.Grants = append(out.Grants, g)
				if len(out.Grants) > core.AccountsMaxItems {
					return public("result_too_large", "Zu viele direkte Rechte.")
				}
			}
			return rows.Err()
		}
	}
	return nil
}

func readPostgresAccountGrants(ctx context.Context, env *opEnv, a core.AccountRef, out *core.AccountDetails) error {
	// Expand only stored ACLs: no inferred owner or PUBLIC privileges.
	query := `WITH acl AS (
 SELECT 'database' AS scope, d.datname AS db, ''::text AS schema, ''::text AS object, ''::text AS col, ''::text AS kind, ''::text AS rid, x.* FROM pg_catalog.pg_database d CROSS JOIN LATERAL aclexplode(d.datacl) x
 UNION ALL SELECT 'table', current_database(), n.nspname,c.relname,'','','',x.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(c.relacl) x WHERE c.relkind IN ('r','p','v','m','f')
 UNION ALL SELECT 'column',current_database(),n.nspname,c.relname,at.attname,'','',x.* FROM pg_catalog.pg_attribute at JOIN pg_catalog.pg_class c ON c.oid=at.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(at.attacl) x WHERE at.attnum>0 AND NOT at.attisdropped
 UNION ALL SELECT 'routine',current_database(),n.nspname,p.proname,'',CASE WHEN p.prokind='p' THEN 'procedure' ELSE 'function' END,p.oid::text,x.* FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL aclexplode(p.proacl) x WHERE p.prokind IN ('f','p')
 ) SELECT scope,db,schema,object,col,kind,rid,privilege_type,is_grantable FROM acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee WHERE r.rolname=$1 ORDER BY scope,db,schema,object,col,rid,privilege_type LIMIT 501`
	rows, err := env.sess.QueryContext(ctx, query, a.Name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g core.AccountGrant
		if err = rows.Scan(&g.Scope, &g.Database, &g.Schema, &g.Object, &g.Column, &g.RoutineKind, &g.RoutineID, &g.Privilege, &g.GrantOption); err != nil {
			return err
		}
		out.Grants = append(out.Grants, g)
		if len(out.Grants) > core.AccountsMaxItems {
			return public("result_too_large", "Zu viele direkte Rechte.")
		}
	}
	return rows.Err()
}

// MariaDB keeps roles outside acl_users, so USER_PRIVILEGES omits their global
// grants. Read only catalog-verified privilege flags, never authentication data.
func readMariaRoleGlobalGrants(ctx context.Context, env *opEnv, a core.AccountRef, out *core.AccountDetails) error {
	var modernCatalog int
	if err := env.sess.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='mysql' AND TABLE_NAME='global_priv'").Scan(&modernCatalog); err != nil {
		return err
	}
	if modernCatalog != 0 {
		var rawAccess string
		// Select only the public privilege bitmap, never the authentication JSON.
		if err := env.sess.QueryRowContext(ctx, "SELECT COALESCE(JSON_VALUE(Priv,'$.access'),'0') FROM mysql.global_priv WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ?", a.Name, a.Host).Scan(&rawAccess); err != nil {
			return err
		}
		access, err := strconv.ParseUint(rawAccess, 10, 64)
		// mysql.user exposes only the legacy bits 0..29. Do not silently omit
		// modern or future global privileges from a supposedly complete result.
		if err != nil || access>>30 != 0 {
			return accountUnsupported()
		}
	}
	rows, err := env.sess.QueryContext(ctx, "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='mysql' AND TABLE_NAME='user' AND RIGHT(LOWER(COLUMN_NAME),5)='_priv' ORDER BY ORDINAL_POSITION")
	if err != nil {
		return err
	}
	columns := []string{}
	privileges := []string{}
	special := map[string]string{"grant": "GRANT OPTION", "show_db": "SHOW DATABASES", "create_tmp_table": "CREATE TEMPORARY TABLES", "repl_slave": "REPLICATION SLAVE", "repl_client": "REPLICATION CLIENT", "delete_history": "DELETE HISTORY"}
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			break
		}
		key := strings.TrimSuffix(strings.ToLower(column), "_priv")
		privilege := strings.ToUpper(strings.ReplaceAll(key, "_", " "))
		if mapped, ok := special[key]; ok {
			privilege = mapped
		}
		if !accountPrivileges["global"][privilege] && privilege != "GRANT OPTION" && privilege != "DELETE HISTORY" {
			err = accountUnsupported()
			break
		}
		columns = append(columns, quoteIdentifier("mysql", column))
		privileges = append(privileges, privilege)
		if len(columns) > 128 {
			err = accountUnsupported()
			break
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return accountUnsupported()
	}
	values := make([]string, len(columns))
	args := make([]any, len(columns))
	for i := range values {
		args[i] = &values[i]
	}
	if err = env.sess.QueryRowContext(ctx, "SELECT "+strings.Join(columns, ",")+" FROM mysql.user WHERE BINARY User=BINARY ? AND BINARY Host=BINARY ?", a.Name, a.Host).Scan(args...); err != nil {
		return err
	}
	grantOption := false
	for i, privilege := range privileges {
		grantOption = grantOption || privilege == "GRANT OPTION" && values[i] == "Y"
	}
	seen := map[core.AccountGrant]bool{}
	for _, grant := range out.Grants {
		seen[grant] = true
	}
	for i, privilege := range privileges {
		if values[i] != "Y" || privilege == "GRANT OPTION" {
			continue
		}
		g := core.AccountGrant{Scope: "global", Privilege: privilege, GrantOption: grantOption}
		if !seen[g] {
			out.Grants = append(out.Grants, g)
			seen[g] = true
		}
	}
	if len(out.Grants) > core.AccountsMaxItems {
		return public("result_too_large", "Zu viele direkte Rechte.")
	}
	return nil
}
