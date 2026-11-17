// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"maps"
	"sort"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// Only routine-local privileges are removed by DROP. Read the complete catalog,
// not information_schema's visibility-filtered grants, before changing anything.
type mysqlRoutineACL map[core.AccountRef]uint8

var mysqlRoutinePrivileges = []string{"EXECUTE", "ALTER ROUTINE", "GRANT OPTION"}

func readMySQLRoutineACL(ctx context.Context, env *opEnv, ref core.ProgramRef) (mysqlRoutineACL, error) {
	rows, err := env.sess.conn.QueryContext(ctx, `SELECT User, Host, Proc_priv FROM mysql.procs_priv WHERE BINARY Db=BINARY ? AND BINARY Routine_name=BINARY ? AND Routine_type=? LIMIT 501`, ref.Schema, ref.Name, strings.ToUpper(ref.Kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	acl := mysqlRoutineACL{}
	for rows.Next() {
		var account core.AccountRef
		var privileges string
		if err := rows.Scan(&account.Name, &account.Host, &privileges); err != nil {
			return nil, err
		}
		var bits uint8
		for _, privilege := range strings.Split(privileges, ",") {
			switch strings.ToUpper(strings.TrimSpace(privilege)) {
			case "":
			case "EXECUTE":
				bits |= 1
			case "ALTER ROUTINE":
				bits |= 2
			case "GRANT":
				bits |= 4
			default:
				return nil, errors.New("unsupported routine privilege")
			}
		}
		if bits != 0 {
			acl[account] = bits
		}
		if len(acl) > core.AccountsMaxItems {
			return nil, errors.New("too many routine grants")
		}
	}
	return acl, rows.Err()
}

// restoreMySQLRoutineACL removes newly automatic creator rights as well as
// restoring the old creator's and explicit grantees' rights. Verify by rereading.
func restoreMySQLRoutineACL(ctx context.Context, env *opEnv, ref core.ProgramRef, saved mysqlRoutineACL) error {
	current, err := readMySQLRoutineACL(ctx, env, ref)
	if err != nil {
		return err
	}
	qualified, err := opQualified("mysql", ref.Schema, ref.Name)
	if err != nil {
		return err
	}
	target := strings.ToUpper(ref.Kind) + " " + qualified
	accounts := make([]core.AccountRef, 0, len(current)+len(saved))
	for account := range current {
		accounts = append(accounts, account)
	}
	for account := range saved {
		if _, found := current[account]; !found {
			accounts = append(accounts, account)
		}
	}
	sort.Slice(accounts, func(i, j int) bool { return accountLabel(accounts[i]) < accountLabel(accounts[j]) })
	privilegeList := func(bits uint8) string {
		var names []string
		for i, name := range mysqlRoutinePrivileges {
			if bits&(1<<i) != 0 {
				names = append(names, name)
			}
		}
		return strings.Join(names, ", ")
	}
	for _, account := range accounts {
		extra, missing := current[account]&^saved[account], saved[account]&^current[account]
		if extra != 0 {
			if _, err := env.sess.conn.ExecContext(ctx, "REVOKE "+privilegeList(extra)+" ON "+target+" FROM "+accountQuote("mysql", account)); err != nil {
				return err
			}
		}
		if missing != 0 {
			if _, err := env.sess.conn.ExecContext(ctx, "GRANT "+privilegeList(missing)+" ON "+target+" TO "+accountQuote("mysql", account)); err != nil {
				return err
			}
		}
	}
	observed, err := readMySQLRoutineACL(ctx, env, ref)
	if err != nil {
		return err
	}
	if !maps.Equal(saved, observed) {
		return errors.New("routine privileges not restored")
	}
	return nil
}
