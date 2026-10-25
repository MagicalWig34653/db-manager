// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tokens are opaque, process-local metadata fingerprints, never query hashes.
var adminTokenKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("serveradmin: entropy unavailable")
	}
	return key
}()

func adminToken(parts ...string) string {
	data, _ := json.Marshal(parts)
	mac := hmac.New(sha256.New, adminTokenKey)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
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
	for _, part := range []string{"password", "passwd", "secret", "token", "key", "path", "file", "dir", "ssl", "tls", "connection", "credential"} {
		if strings.Contains(strings.ToLower(name), part) {
			sensitive = true
		}
	}
	allowed := !variable || name == "max_heap_table_size" || name == "tmp_table_size" || name == "statement_timeout" || name == "lock_timeout" || name == "max_connections" || name == "port"
	if !sensitive && allowed && len(value) <= 64 && value != "" && strings.Trim(value, "0123456789.+-eE") == "" {
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
		result.Processes, result.HasMore, err = adminProcesses(ctx, conn, r.Connection.Driver, r.Limit, r.Offset, r.Filter, "")
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
			values := []core.ServerAdminValue{}
			scanned := 0
			for rows.Next() {
				var name, value string
				if err = rows.Scan(&name, &value); err != nil {
					break
				}
				scanned++
				if scanned > 4096 || len(name) > 256 || len(value) > MaxCellBytes {
					err = public("result_too_large", "Servermetadaten sind zu groß.")
					break
				}
				if strings.Contains(strings.ToLower(name), strings.ToLower(r.Filter)) {
					values = append(values, adminSafeValue(name, value, kind == "variables"))
				}
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
			start := min(r.Offset, len(values))
			end := min(start+r.Limit, len(values))
			result.Values = values[start:end]
			result.HasMore = end < len(values)
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

func adminProcesses(ctx context.Context, conn *sql.Conn, driver string, limit, offset int, filter, id string) ([]core.ServerAdminProcess, bool, error) {
	query := `SELECT CAST(ID AS CHAR),USER,COALESCE(DB,''),CAST(TIME AS SIGNED),ID=CONNECTION_ID(),HOST,COALESCE(INFO,'') FROM information_schema.PROCESSLIST WHERE (?='' OR CAST(ID AS CHAR)=?) AND LOCATE(?,USER)>0 ORDER BY ID LIMIT ? OFFSET ?`
	args := []any{id, id, filter, limit + 1, offset}
	if driver == "postgres" {
		query = `SELECT pid::text,COALESCE(usename,''),COALESCE(datname,''),GREATEST(0,COALESCE(EXTRACT(EPOCH FROM clock_timestamp()-query_start)::bigint,0)),pid=pg_backend_pid(),COALESCE(backend_start::text,''),COALESCE(query_start::text,'') FROM pg_stat_activity WHERE backend_type='client backend' AND ($1='' OR pid::text=$1) AND strpos(COALESCE(usename,''),$2)>0 ORDER BY pid LIMIT $3 OFFSET $4`
		args = []any{id, filter, limit + 1, offset}
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
		var identity, queryIdentity string
		if err = rows.Scan(&p.ID, &p.User, &p.Database, &p.Seconds, &p.Self, &identity, &queryIdentity); err != nil {
			return nil, false, err
		}
		if len(p.User) > 256 || len(p.Database) > 256 || len(identity) > 512 || len(queryIdentity) > MaxCellBytes {
			return nil, false, public("result_too_large", "Prozessmetadaten sind zu groß.")
		}
		p.Identity = adminToken(driver, p.ID, p.User, p.Database, identity)
		p.QueryIdentity = adminToken(p.Identity, queryIdentity)
		result = append(result, p)
	}
	return result, false, rows.Err()
}

func adminVariable(driver, name, scope, value string) bool {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != value {
		return false
	}
	if driver == "mysql" {
		return (scope == "SESSION" || scope == "GLOBAL") && (name == "max_heap_table_size" || name == "tmp_table_size") && n >= 16384 && n <= 1073741824
	}
	return driver == "postgres" && scope == "SESSION" && (name == "statement_timeout" || name == "lock_timeout") && n >= 100 && n <= 600000
}
func adminBuild(ctx context.Context, conn *sql.Conn, action string, r core.ServerAdminRequest) (core.ServerAdminPlan, error) {
	empty := core.ServerAdminPlan{}
	var statement, target string
	var previous string
	warnings := []string{"Kein automatischer Retry; Vorschau und Ausführung sind kein atomarer Snapshot."}
	if action == "kill" {
		if r.Name != "" || r.Scope != "" || r.Value != "" || (r.Mode != "query" && r.Mode != "connection") {
			return empty, adminInvalid()
		}
		n, err := strconv.ParseUint(r.ID, 10, 32)
		if err != nil || n == 0 || strconv.FormatUint(n, 10) != r.ID || (len(r.Identity) != 64 || strings.Trim(r.Identity, "0123456789abcdef") != "") {
			return empty, adminInvalid()
		}
		processes, _, err := adminProcesses(ctx, conn, r.Connection.Driver, 1, 0, "", r.ID)
		if err != nil {
			return empty, safe(ctx, err)
		}
		if len(processes) != 1 {
			return empty, public("object_not_found", "Prozess nicht sichtbar oder nicht mehr vorhanden.")
		}
		p := processes[0]
		if p.Self || p.User == "" || p.User == "system user" || p.User == "event_scheduler" {
			return empty, public("unsupported_action", "Eigene Verwaltungssitzung und Systemprozesse sind geschützt.")
		}
		token := p.Identity
		if r.Mode == "query" {
			token = p.QueryIdentity
		}
		if !hmac.Equal([]byte(token), []byte(r.Identity)) {
			return empty, public("plan_mismatch", "Prozess oder Abfrage hat sich geändert.")
		}
		target = p.ID + ":" + p.User
		statement = "KILL " + strings.ToUpper(r.Mode) + " " + p.ID
		if r.Connection.Driver == "postgres" {
			fn := "pg_cancel_backend"
			if r.Mode == "connection" {
				fn = "pg_terminate_backend"
			}
			statement = "SELECT " + fn + "(" + p.ID + ")"
		}
		statement += " /* identity:" + token + " */"
		warnings = append(warnings, "Nur die bestätigte ID; unmittelbar nach Prüfung kann der Prozess enden oder ersetzt werden. Keine ABA-/Exactly-once-Garantie.")
	} else if action == "set_variable" {
		if r.ID != "" || r.Identity != "" || r.Mode != "" || !adminVariable(r.Connection.Driver, r.Name, r.Scope, r.Value) {
			return empty, adminInvalid()
		}
		var name string
		if r.Connection.Driver == "mysql" {
			rows, err := conn.QueryContext(ctx, "SHOW "+r.Scope+" VARIABLES")
			if err != nil {
				return empty, safe(ctx, err)
			}
			var value string
			scanned := 0
			for rows.Next() {
				scanned++
				if scanned > 4096 {
					err = public("result_too_large", "Servermetadaten sind zu groß.")
					break
				}
				var candidate string
				err = rows.Scan(&candidate, &value)
				if err != nil {
					break
				}
				if candidate == r.Name {
					name = candidate
					previous = value
					break
				}
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return empty, safe(ctx, err)
			}
		} else {
			if err := conn.QueryRowContext(ctx, "SELECT name,setting FROM pg_settings WHERE name=$1 AND context IN ('user','superuser')", r.Name).Scan(&name, &previous); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return empty, public("object_not_found", "Dynamische Variable nicht sichtbar.")
				}
				return empty, safe(ctx, err)
			}
		}
		if name != r.Name {
			return empty, public("object_not_found", "Variable nicht sichtbar.")
		}
		target = r.Scope + ":" + name
		// Name is both allowlisted and resolved from server metadata; value is a canonical integer.
		statement = "SET " + r.Scope + " " + name + " = " + r.Value
		if r.Scope == "SESSION" {
			warnings = append(warnings, "SESSION gilt nur für diese API-Anfrage und endet beim Schließen der Verbindung.")
		} else {
			warnings = append(warnings, "GLOBAL wirkt serverweit auf neue Sitzungen; keine persistente Konfigurationsänderung.")
		}
	} else {
		return empty, adminInvalid()
	}
	statement += " /* target:" + adminToken(r.Connection.Driver, r.Connection.Host, strconv.Itoa(r.Connection.Port), r.Connection.Database, r.Connection.Username, target, previous) + " */"
	plan, err := dbplan.NewPlan(r.Connection.Driver, []string{statement}, warnings, []string{target})
	if err != nil {
		return empty, adminInvalid()
	}
	return core.ServerAdminPlan{Plan: plan, Action: action, TargetName: target}, nil
}
func adminValidateWrite(action string, r core.ServerAdminRequest) error {
	if r.TimeoutMS < 0 || r.TimeoutMS > 10000 {
		return adminInvalid()
	}
	if action != "kill" && action != "set_variable" {
		return adminInvalid()
	}
	return adminCapability(r.Connection.Driver, action)
}
func (e *Engine) ServerAdminPlan(ctx context.Context, action string, r core.ServerAdminRequest) (core.ServerAdminPlan, error) {
	if err := adminValidateWrite(action, r); err != nil {
		return core.ServerAdminPlan{}, err
	}
	ctx, cancel := adminContext(ctx, r.TimeoutMS)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.ServerAdminPlan{}, err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return core.ServerAdminPlan{}, safe(ctx, err)
	}
	defer conn.Close()
	return adminBuild(ctx, conn, action, r)
}
func (e *Engine) ServerAdminExecute(ctx context.Context, action string, r core.ServerAdminExecuteRequest) (core.ServerAdminResult, error) {
	empty := core.ServerAdminResult{}
	if err := adminValidateWrite(action, r.ServerAdminRequest); err != nil {
		return empty, err
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
	plan, err := adminBuild(ctx, conn, action, r.ServerAdminRequest)
	if err != nil {
		return empty, err
	}
	if err = dbplan.Check(plan.Plan, r.ExpectedPlanSHA256, r.Confirm, action, plan.TargetName, true); err != nil {
		if errors.Is(err, dbplan.ErrPlanMismatch) {
			return empty, public("plan_mismatch", "Plan neu laden und erneut bestätigen.")
		}
		return empty, public("confirmation_mismatch", "Plan-Hash, Aktion oder getippter Zielname stimmen nicht überein.")
	}
	return adminRun(ctx, conn, action, r.ServerAdminRequest, plan)
}

func adminRun(ctx context.Context, conn *sql.Conn, action string, r core.ServerAdminRequest, plan core.ServerAdminPlan) (core.ServerAdminResult, error) {
	var err error
	result := core.ServerAdminResult{Outcome: dbplan.StatusRejected, Reason: "canceled", Message: "Vor Ausführung abgebrochen."}
	if ctx.Err() != nil {
		return result, nil
	}
	if action == "kill" && r.Connection.Driver == "postgres" {
		var signaled bool
		err = conn.QueryRowContext(ctx, plan.Statements[0]).Scan(&signaled)
		if err == nil && !signaled {
			result.Reason = "not_signaled"
			result.Message = "Der Server hat kein Signal gesendet."
			return result, nil
		}
	} else {
		_, err = conn.ExecContext(ctx, plan.Statements[0])
	}
	if err != nil {
		reason, definite := adminReason(err)
		if definite {
			result.Reason = reason
			result.Message = "Der Server hat die Aktion abgelehnt."
		} else {
			result.Outcome = dbplan.StatusOutcomeUnknown
			result.Reason = "connection_lost"
			result.Message = "Ausgang unbekannt. Zustand prüfen; nicht automatisch wiederholen."
		}
		return result, nil
	}
	result.Outcome = dbplan.StatusCompleted
	result.Reason = ""
	result.Message = "Vom Server ausgeführt; ein Prozesssignal bestätigt keinen Prozessabschluss."
	if action == "set_variable" {
		var value string
		query := "SELECT @@" + r.Scope + "." + r.Name
		if r.Connection.Driver == "postgres" {
			query = fmt.Sprintf("SHOW %s", r.Name)
		}
		if err = conn.QueryRowContext(ctx, query).Scan(&value); err == nil && len(value) <= 64 {
			if r.Connection.Driver == "postgres" || !adminSafeValue(r.Name, value, true).Masked {
				result.ObservedValue = &value
			}
		}
		result.Message = "Variable gesetzt; Rücklesen erfolgt getrennt. SESSION endet mit dieser Anfrage."
	}
	return result, nil
}

// Server-side disconnect/cancel errors do not prove that a signal or setting
// was never applied. Preserve uncertainty rather than claiming rejection.
func adminReason(err error) (string, bool) {
	var my *mysql.MySQLError
	var pg *pgconn.PgError
	if errors.As(err, &my) && (my.Number == 1927 || my.Number == 2006 || my.Number == 2013 || my.Number == 1317) {
		return "", false
	}
	if errors.As(err, &pg) && (strings.HasPrefix(pg.Code, "08") || pg.Code == "57P01" || pg.Code == "57P02" || pg.Code == "57P03" || pg.Code == "57014") {
		return "", false
	}
	return opReason(err)
}
