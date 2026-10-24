// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func maintenanceTimeout(n, ms int) (time.Duration, error) {
	if n < 1 || n > core.MaintenanceMaxItems || ms < 0 || ms > core.MaintenanceMaxTimeoutMS {
		return 0, public("invalid_maintenance", "Tabellenanzahl oder Zeitlimit ist ungültig.")
	}
	if ms == 0 {
		ms = core.MaintenanceDefaultTimeoutMS
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// maintenancePreview resolves exact catalog names before quoting. No client SQL
// or free-form options reach a statement. Unsupported items remain independent.
func maintenancePreview(ctx context.Context, db *sql.DB, c core.Connection, item core.MaintenanceItem) core.MaintenancePreview {
	p := core.MaintenancePreview{Table: item.Table, Action: item.Action, State: "rejected", Reason: "invalid_maintenance"}
	if !validTableRef(item.Table) || (item.Full || item.Analyze) && item.Action != "vacuum" {
		return p
	}
	caps := core.MaintenanceCapabilities()
	cap := caps[c.Driver][item.Action]
	if cap.Support == "" || cap.Support == dbplan.Unsupported {
		p.Reason = "unsupported_action"
		return p
	}
	schema := strings.ToLower(item.Table.Schema)
	if schema == "" && c.Driver == "mysql" {
		schema = strings.ToLower(c.Database)
	}
	if schema == "information_schema" || schema == "mysql" || schema == "performance_schema" || schema == "sys" || strings.HasPrefix(schema, "pg_") || strings.HasPrefix(strings.ToLower(item.Table.Name), "sqlite_") {
		p.Reason = "system_table"
		return p
	}
	d, err := describe(ctx, db, c, item.Table)
	if err != nil {
		p.Reason = maintenanceReason(ctx, err)
		return p
	}
	p.Table = d.Table
	if d.Kind != "table" {
		p.Reason = "unsupported_table"
		return p
	}
	// Catalog matching must be exact even on case-insensitive MySQL catalogs.
	if d.Table.Name != item.Table.Name || item.Table.Schema != "" && d.Table.Schema != item.Table.Schema {
		p.Reason = "object_not_found"
		return p
	}
	if c.Driver == "postgres" {
		var eligible, permitted bool
		err = db.QueryRowContext(ctx, `SELECT c.relkind='r' AND NOT c.relispartition AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhparent=c.oid OR i.inhrelid=c.oid), pg_catalog.pg_has_role(current_user,c.relowner,'USAGE') OR pg_catalog.has_table_privilege(c.oid,'MAINTAIN') FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&eligible, &permitted)
		if err != nil {
			p.Reason = maintenanceReason(ctx, err)
			return p
		}
		if !eligible {
			p.Reason = "unsupported_table"
			return p
		}
		if !permitted {
			p.Reason = "permission_denied"
			return p
		}
	}
	statement, typed, reason := maintenanceStatement(c.Driver, d.StorageEngine, d.Table, item)
	if reason != "" {
		p.Reason = reason
		return p
	}
	plan, err := dbplan.NewPlan(c.Driver, []string{statement}, []string{"Die Laufzeit ist nicht vorhersagbar; Sperren können andere Sitzungen blockieren. Das Zeitlimit umfasst den gesamten Stapel.", "Ein Abbruch garantiert keinen Rollback. Nach unbekanntem Ausgang nicht automatisch wiederholen."}, []string{d.Table.Schema + "." + d.Table.Name})
	if err != nil {
		return p
	}
	if typed {
		plan.Warnings = append(plan.Warnings, "Diese Aktion kann Tabellen oder Indizes neu aufbauen und lange exklusive Sperren sowie zusätzlichen Speicher benötigen.")
	}
	p.State, p.Reason, p.Plan, p.TypedConfirmation, p.TargetName = "planned", "", &plan, typed, d.Table.Name
	return p
}

func maintenanceStatement(driver, engine string, table core.TableRef, item core.MaintenanceItem) (string, bool, string) {
	name, err := dbplan.QualifiedName(driver, table.Schema, table.Name)
	if err != nil {
		return "", false, "invalid_maintenance"
	}
	switch driver {
	case "sqlite":
		switch item.Action {
		case "check":
			q, _ := dbplan.QuoteIdentifier(driver, table.Name)
			return "PRAGMA main.integrity_check(" + q + ")", false, ""
		case "analyze":
			return "ANALYZE " + name, false, ""
		}
	case "mysql":
		engine = strings.ToUpper(engine)
		supported := engine == "INNODB" || engine == "MYISAM" || engine == "ARIA"
		if item.Action == "repair" {
			supported = engine == "MYISAM" || engine == "ARIA" || engine == "ARCHIVE" || engine == "CSV"
		}
		if !supported {
			return "", false, "unsupported_engine"
		}
		switch item.Action {
		case "check", "analyze", "optimize", "repair", "checksum":
			return strings.ToUpper(item.Action) + " TABLE " + name, item.Action == "optimize" || item.Action == "repair", ""
		}
	case "postgres":
		switch item.Action {
		case "analyze":
			return "ANALYZE " + name, false, ""
		case "reindex":
			return "REINDEX TABLE " + name, true, ""
		case "vacuum":
			opts := []string{}
			if item.Full {
				opts = append(opts, "FULL")
			}
			if item.Analyze {
				opts = append(opts, "ANALYZE")
			}
			prefix := "VACUUM "
			if len(opts) > 0 {
				prefix = "VACUUM (" + strings.Join(opts, ", ") + ") "
			}
			return prefix + name, item.Full, ""
		}
	}
	return "", false, "unsupported_action"
}

func (e *Engine) PlanMaintenance(ctx context.Context, r core.MaintenancePlanRequest) (core.MaintenancePlanResult, error) {
	result := core.MaintenancePlanResult{Items: []core.MaintenancePreview{}}
	timeout, err := maintenanceTimeout(len(r.Items), r.TimeoutMS)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	db, closeDB, err := e.openMode(ctx, r.Connection, true)
	if err != nil {
		return result, err
	}
	defer closeDB()
	for _, item := range r.Items {
		result.Items = append(result.Items, maintenancePreview(ctx, db, r.Connection, item))
	}
	return result, nil
}

func (e *Engine) ExecuteMaintenance(ctx context.Context, r core.MaintenanceExecuteRequest) (core.MaintenanceResult, error) {
	result := core.MaintenanceResult{Items: []core.MaintenanceOutcome{}}
	timeout, err := maintenanceTimeout(len(r.Items), r.TimeoutMS)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	db, closeDB, err := e.openMode(ctx, r.Connection, true)
	if err != nil {
		return result, err
	}
	defer closeDB()
	completed := 0
	for _, item := range r.Items {
		start := time.Now()
		p := maintenancePreview(ctx, db, r.Connection, item.MaintenanceItem)
		out := core.MaintenanceOutcome{Table: p.Table, Action: p.Action, Outcome: dbplan.StatusRejected, Reason: p.Reason}
		if p.Plan != nil {
			err = dbplan.Check(*p.Plan, item.ExpectedPlanSHA256, item.Confirm, p.Action, p.TargetName, p.TypedConfirmation)
			if errors.Is(err, dbplan.ErrPlanMismatch) || errors.Is(err, dbplan.ErrInvalidDigest) {
				out.Reason = "plan_mismatch"
			} else if err != nil {
				out.Reason = "confirmation_mismatch"
			} else if ctx.Err() != nil {
				out.Reason = "canceled"
			} else {
				if err = maintenanceSessionLimits(ctx, db, r.Connection.Driver); err != nil {
					out.Reason = "session_setup_failed"
				} else {
					out.Outcome, out.Reason, out.Checksum = runMaintenance(ctx, db, r.Connection.Driver, item.Action, p.Plan.Statements[0])
				}
			}
		}
		if out.Outcome == dbplan.StatusCompleted {
			completed++
		}
		out.DurationMS = time.Since(start).Milliseconds()
		result.Items = append(result.Items, out)
	}
	result.Partial = completed > 0 && completed < len(result.Items)
	return result, nil
}

func maintenanceReason(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return "canceled"
	}
	var p *core.Error
	if errors.As(err, &p) && p.Code == "table_not_found" {
		return "object_not_found"
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "object_not_found"
	}
	if reason, ok := maintenanceDefiniteRejection(err); ok {
		return reason
	}
	return "database_error"
}

// The SQL call may have taken effect before any error is delivered. Conservatively
// classify failures after dispatch as unknown, including reported repair failures.
func runMaintenance(ctx context.Context, db *sql.DB, driver, action, statement string) (dbplan.Status, string, *string) {
	unknown := func() (dbplan.Status, string, *string) {
		return dbplan.StatusOutcomeUnknown, "execution_unconfirmed", nil
	}
	if driver == "postgres" || driver == "sqlite" && action == "analyze" {
		_, err := db.ExecContext(ctx, statement)
		if err != nil {
			if reason, ok := maintenanceDefiniteRejection(err); ok {
				return dbplan.StatusRejected, reason, nil
			}
			return unknown()
		}
		return dbplan.StatusCompleted, "", nil
	}
	rows, err := db.QueryContext(ctx, statement)
	if err != nil {
		if reason, ok := maintenanceDefiniteRejection(err); ok {
			return dbplan.StatusRejected, reason, nil
		}
		return unknown()
	}
	defer rows.Close()
	count := 0
	ok := false
	var checksum *string
	for rows.Next() {
		count++
		if count > 64 {
			return unknown()
		}
		switch {
		case driver == "sqlite":
			var message string
			if rows.Scan(&message) != nil {
				return unknown()
			}
			if message != "ok" {
				return dbplan.StatusRejected, "integrity_check_failed", nil
			}
			ok = true
		case action == "checksum":
			var table string
			var value sql.NullString
			if rows.Scan(&table, &value) != nil {
				return unknown()
			}
			if !value.Valid || len(value.String) > 20 || strings.Trim(value.String, "0123456789") != "" || value.String == "" {
				return dbplan.StatusRejected, "checksum_unavailable", nil
			}
			v := value.String
			checksum = &v
			ok = true
		default:
			var table, op, kind, message string
			if rows.Scan(&table, &op, &kind, &message) != nil {
				return unknown()
			}
			// Never expose server diagnostic text: it can contain SQL, paths or secrets.
			if kind == "error" || kind == "warning" {
				return unknown()
			}
			if kind == "status" && (message == "OK" || message == "Table is already up to date") {
				ok = true
			}
		}
	}
	if rows.Err() != nil || !ok {
		return unknown()
	}
	return dbplan.StatusCompleted, "", checksum
}

// Session settings use only server-generated integers and the dedicated single
// connection. They bound server work as well as client-side context waiting.
func maintenanceSessionLimits(ctx context.Context, db *sql.DB, driver string) error {
	deadline, _ := ctx.Deadline()
	ms := time.Until(deadline).Milliseconds()
	if ms < 1 {
		return context.DeadlineExceeded
	}
	switch driver {
	case "postgres":
		if _, err := db.ExecContext(ctx, "SET statement_timeout = "+strconv.FormatInt(ms, 10)); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, "SET lock_timeout = "+strconv.FormatInt(min(ms, 1000), 10))
		return err
	case "mysql":
		_, err := db.ExecContext(ctx, "SET SESSION lock_wait_timeout = "+strconv.FormatInt(max(1, (ms+999)/1000), 10))
		return err
	}
	return nil
}

// Only explicit permission/object lookup errors prove that maintenance did not
// start. In particular, a repair or timeout error can follow partial work.
func maintenanceDefiniteRejection(err error) (string, bool) {
	var my *mysql.MySQLError
	if errors.As(err, &my) {
		switch my.Number {
		case 1044, 1045, 1142, 1143, 1227:
			return "permission_denied", true
		case 1146:
			return "object_not_found", true
		}
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "42501":
			return "permission_denied", true
		case "42P01":
			return "object_not_found", true
		}
	}
	return "", false
}
