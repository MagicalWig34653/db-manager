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
	grantee := accountLiteral(a.Name) + "@" + accountLiteral(a.Host)
	queries := []struct{ scope, query string }{
		{"global", `SELECT '', '', '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.USER_PRIVILEGES WHERE GRANTEE=?`},
		{"database", `SELECT TABLE_SCHEMA, '', '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE=?`},
		{"table", `SELECT TABLE_SCHEMA, TABLE_NAME, '', '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.TABLE_PRIVILEGES WHERE GRANTEE=?`},
		{"column", `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, '', PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.COLUMN_PRIVILEGES WHERE GRANTEE=?`},
		{"routine", `SELECT ROUTINE_SCHEMA, ROUTINE_NAME, '', ROUTINE_TYPE, PRIVILEGE_TYPE, IS_GRANTABLE FROM information_schema.ROUTINE_PRIVILEGES WHERE GRANTEE=?`},
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
			for rows.Next() {
				g := core.AccountGrant{Scope: "global"}
				var option string
				if err = rows.Scan(&g.Privilege, &option); err != nil {
					return err
				}
				g.GrantOption = option == "Y"
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
