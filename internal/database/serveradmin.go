// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func adminInvalid() error { return public("invalid_server_admin", "Ungültige Verwaltungsanfrage.") }
func adminContext(ctx context.Context, ms int) (context.Context, context.CancelFunc) {
	if ms == 0 {
		ms = 5000
	}
	return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
}
func adminCapability(driver, action string) error {
	if driver != "sqlite" && driver != "mysql" && driver != "postgres" {
		return adminInvalid()
	}
	cap := core.ServerAdminCapabilities().Get(driver, action)
	if cap.Support == dbplan.Unsupported {
		return public("unsupported_action", cap.Reason)
	}
	return nil
}
func adminSafeValue(name, value string, variable bool) core.ServerAdminValue {
	item := core.ServerAdminValue{Name: name, Masked: true}
	// Only known numeric variables can expose values. Unknown/plugin values,
	// paths, DSNs, key material and free-form strings remain masked.
	sensitive := false
	known := false
	// Exact known counters/limits that merely contain "connection" in their name.
	switch strings.ToLower(name) {
	case "max_connections", "connections", "max_used_connections", "max_user_connections":
		known = true
	}
	for _, part := range []string{"password", "passwd", "secret", "token", "key", "path", "file", "dir", "ssl", "tls", "connection", "credential"} {
		if !known && strings.Contains(strings.ToLower(name), part) {
			sensitive = true
		}
	}
	allowed := !variable || name == "max_heap_table_size" || name == "tmp_table_size" || name == "statement_timeout" || name == "lock_timeout" || name == "max_connections" || name == "max_user_connections" || name == "port"
	_, numericError := strconv.ParseFloat(value, 64)
	if !sensitive && allowed && numericError == nil && len(value) <= 64 && value != "" && strings.Trim(value, "0123456789.+-eE") == "" {
		v := value
		item.Value = &v
		item.Masked = false
	}
	return item
}
func (e *Engine) ServerAdminRead(ctx context.Context, kind string, r core.ServerAdminReadRequest) (core.ServerAdminReadResult, error) {
	empty := core.ServerAdminReadResult{}
	if (kind != "status" && kind != "variables" && kind != "processes") || r.Limit < 0 || r.Limit > 200 || r.Offset < 0 || r.Offset > 10000 || r.TimeoutMS < 0 || r.TimeoutMS > 10000 || len(r.Filter) > 128 || (r.Scope != "" && r.Scope != "GLOBAL" && r.Scope != "SESSION") || (kind != "variables" && r.Scope != "") {
		return empty, adminInvalid()
	}
	if err := adminCapability(r.Connection.Driver, kind); err != nil {
		return empty, err
	}
	if r.Limit == 0 {
		r.Limit = 100
	}
	if kind == "variables" && r.Scope == "" {
		r.Scope = "SESSION"
	}
	if kind == "variables" && r.Connection.Driver == "postgres" && r.Scope == "GLOBAL" {
		return empty, public("unsupported_action", "PostgreSQL liefert hier nur wirksame Sitzungswerte.")
	}
	ctx, cancel := adminContext(ctx, r.TimeoutMS)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer conn.Close()
	result := core.ServerAdminReadResult{Driver: r.Connection.Driver, Kind: kind, Scope: r.Scope, SampledAt: time.Now().UTC().Format(time.RFC3339Nano), Values: []core.ServerAdminValue{}, Processes: []core.ServerAdminProcess{}, Limit: r.Limit, Offset: r.Offset}
	if kind == "processes" {
		result.Processes, result.HasMore, err = adminProcesses(ctx, conn, r.Connection.Driver, r.Limit, r.Offset, r.Filter)
	} else if r.Connection.Driver == "sqlite" {
		var version string
		err = conn.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
		if r.Offset == 0 && strings.Contains("sqlite_version", strings.ToLower(r.Filter)) {
			result.Values = append(result.Values, core.ServerAdminValue{Name: "sqlite_version", Value: &version})
		}
	} else {
		query := "SHOW GLOBAL STATUS"
		if kind == "variables" {
			query = "SHOW " + r.Scope + " VARIABLES"
		}
		if r.Connection.Driver == "postgres" {
			query = "SELECT name,setting FROM pg_settings ORDER BY name"
			if kind == "status" {
				query = `SELECT key,value FROM pg_stat_database d CROSS JOIN LATERAL jsonb_each_text(jsonb_build_object('numbackends',numbackends,'xact_commit',xact_commit,'xact_rollback',xact_rollback,'blks_read',blks_read,'blks_hit',blks_hit,'tup_returned',tup_returned,'tup_fetched',tup_fetched,'tup_inserted',tup_inserted,'tup_updated',tup_updated,'tup_deleted',tup_deleted,'deadlocks',deadlocks,'temp_bytes',temp_bytes)) WHERE datname=current_database()`
			}
		}
		var rows *sql.Rows
		rows, err = conn.QueryContext(ctx, query)
		if err == nil {
			result.Values, result.HasMore, err = adminValues(rows, r, kind == "variables")
		}
	}
	if err != nil {
		return empty, safe(ctx, err)
	}
	if err = validateOverviewSize(result); err != nil {
		return empty, err
	}
	return result, nil
}

func adminProcesses(ctx context.Context, conn *sql.Conn, driver string, limit, offset int, filter string) ([]core.ServerAdminProcess, bool, error) {
	query := `SELECT CAST(ID AS CHAR),USER,COALESCE(DB,''),GREATEST(0,CAST(TIME AS SIGNED)),ID=CONNECTION_ID() FROM information_schema.PROCESSLIST WHERE LOCATE(?,USER)>0 ORDER BY ID LIMIT ? OFFSET ?`
	args := []any{filter, limit + 1, offset}
	if driver == "postgres" {
		query = `SELECT pid::text,COALESCE(usename,''),COALESCE(datname,''),GREATEST(0,COALESCE(EXTRACT(EPOCH FROM clock_timestamp()-query_start)::bigint,0)),pid=pg_backend_pid() FROM pg_stat_activity WHERE backend_type='client backend' AND strpos(COALESCE(usename,''),$1)>0 ORDER BY pid LIMIT $2 OFFSET $3`
		args = []any{filter, limit + 1, offset}
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	result := []core.ServerAdminProcess{}
	for rows.Next() {
		if len(result) == limit {
			return result, true, nil
		}
		var p core.ServerAdminProcess
		if err = rows.Scan(&p.ID, &p.User, &p.Database, &p.Seconds, &p.Self); err != nil {
			return nil, false, err
		}
		if len(p.User) > 256 || len(p.Database) > 256 || len(p.ID) > 20 || p.Seconds < 0 || p.Seconds > 9007199254740991 {
			return nil, false, public("result_too_large", "Prozessmetadaten sind zu groß.")
		}
		result = append(result, p)
	}
	return result, false, rows.Err()
}

// Bound scanning before sorting and paging; response limits alone do not bound work.
func adminValues(rows *sql.Rows, r core.ServerAdminReadRequest, variable bool) ([]core.ServerAdminValue, bool, error) {
	defer rows.Close()
	values := []core.ServerAdminValue{}
	scanned, bytes := 0, 0
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, false, err
		}
		scanned++
		bytes += len(name) + len(value)
		if scanned > 4096 || len(name) > 256 || len(value) > MaxCellBytes || bytes > MaxResponseBytes {
			return nil, false, public("result_too_large", "Servermetadaten sind zu groß.")
		}
		if strings.Contains(strings.ToLower(name), strings.ToLower(r.Filter)) {
			values = append(values, adminSafeValue(name, value, variable))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	start := min(r.Offset, len(values))
	end := min(start+r.Limit, len(values))
	return values[start:end], end < len(values), nil
}
