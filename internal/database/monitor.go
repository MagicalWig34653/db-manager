// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

var monitorNumber = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
var monitorLog = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[0-9]{1,20}$`)
var monitorDigest = regexp.MustCompile(`^[a-fA-F0-9]{16,64}$`)

// Deliberately small grammar: no functions, literals containing text, comments,
// CTEs, subqueries, session settings, INTO, or execution options.
var monitorSelect = regexp.MustCompile(`(?i)^SELECT\s+(?:[0-9]+|\*|[a-z_][a-z0-9_]*(?:\s*,\s*[a-z_][a-z0-9_]*)*)(?:\s+FROM\s+[a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)?(?:\s+WHERE\s+[a-z_][a-z0-9_]*\s*(?:=|<>|<=|>=|<|>)\s*[0-9]+)?(?:\s+LIMIT\s+[0-9]+)?$`)

func normalizeMonitor(action string, r *core.MonitorRequest) error {
	known := false
	for _, a := range core.MonitorActions {
		known = known || a == action
	}
	if !known || r.Limit < 0 || r.Limit > 200 || r.Offset < 0 || r.Offset > 10000 || r.TimeoutMS < 0 || r.TimeoutMS > 30000 || r.Position < 0 || r.Position > 9007199254740991 {
		return public("invalid_monitor", "Aktion oder Grenzen sind ungültig.")
	}
	if r.Limit == 0 {
		r.Limit = 50
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	if action == "binary-events" {
		if len(r.Log) > 256 || !monitorLog.MatchString(r.Log) {
			return public("invalid_monitor", "Binärlog-Kennung ist ungültig.")
		}
	} else if r.Log != "" || r.Position != 0 {
		return public("invalid_monitor", "Logparameter gehören ausschließlich zu Binärlog-Ereignissen.")
	}
	if action == "profiling" {
		r.SQL = strings.TrimSpace(r.SQL)
		if len(r.SQL) > 16384 || !monitorSelect.MatchString(r.SQL) {
			return public("invalid_monitor", "Profiling erlaubt nur ein einfaches SELECT ohne Funktionen, Kommentare oder Unterabfragen.")
		}
	} else if r.SQL != "" {
		return public("invalid_monitor", "SQL ist ausschließlich für Profiling zulässig.")
	}
	return nil
}

func (e *Engine) Monitor(ctx context.Context, action string, r core.MonitorRequest) (core.MonitorResult, error) {
	out := core.MonitorResult{Driver: r.Connection.Driver, Action: action, State: "available", Source: "none", Records: []core.MonitorRecord{}, Hints: []core.MonitorHint{}}
	if err := normalizeMonitor(action, &r); err != nil {
		return out, err
	}
	out.Limit, out.Offset = r.Limit, r.Offset
	support, reason := core.MonitorSupport(r.Connection.Driver, action)
	if support == "unsupported" {
		return out, public("unsupported_feature", reason)
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return out, err
	}
	defer cleanup()
	out.SampledAt = time.Now().UTC().Format(time.RFC3339Nano)
	switch r.Connection.Driver {
	case "sqlite":
		out.Source = "sqlite_version"
		var version string
		err = db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
		if err == nil && monitorNumber.MatchString(strings.ReplaceAll(version, ".", "")) {
			out.Records = append(out.Records, core.MonitorRecord{"version": version})
		}
		out.Reason = reason
		if r.Offset > 0 {
			out.Records = []core.MonitorRecord{}
		}
	case "mysql":
		err = monitorMySQL(ctx, db, action, r, &out)
	case "postgres":
		err = monitorPostgres(ctx, db, action, r, &out)
	default:
		return out, public("invalid_monitor", "Treiber ist ungültig.")
	}
	if err != nil {
		if ctx.Err() != nil {
			return out, safe(ctx, err)
		}
		if monitorUnavailable(err) {
			out.State = "unavailable"
			out.Reason = "Datenquelle ist nicht installiert, deaktiviert oder mit diesen Datenbankrechten nicht lesbar."
			out.Records = []core.MonitorRecord{}
			out.Hints = []core.MonitorHint{}
			out.HasMore = false
		} else {
			return out, safe(ctx, err)
		}
	}
	if action == "advisor" && out.State == "available" {
		out.Hints = monitorAdvice(out.Records, out.Source)
	}
	out.DurationMS = time.Since(started).Milliseconds()
	if err = validateOverviewSize(out); err != nil {
		return out, err
	}
	return out, nil
}

func monitorUnavailable(err error) bool {
	var m *mysql.MySQLError
	if errors.As(err, &m) {
		switch m.Number {
		case 1044, 1045, 1064, 1109, 1142, 1143, 1146, 1193, 1227, 1381:
			return true
		}
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "42501", "42P01", "42703", "3F000", "42883", "55000":
			return true
		}
	}
	return false
}

// Each field specifies an output name and a validator. No raw row is returned.
type monitorField struct{ name, kind string }

func monitorValue(value, kind string) bool {
	if len(value) > 256 {
		return false
	}
	switch kind {
	case "number":
		return monitorNumber.MatchString(value)
	case "digest":
		return monitorDigest.MatchString(value)
	case "log":
		return monitorLog.MatchString(value)
	}
	for _, allowed := range strings.Split(kind, "|") {
		if value == allowed {
			return true
		}
	}
	return false
}

type monitorQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func monitorRead(ctx context.Context, db monitorQuerier, query string, fields []monitorField, r core.MonitorRequest, out *core.MonitorResult, args ...any) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	// SHOW commands vary across versions. Match their columns by name rather than position.
	for index := 0; rows.Next(); index++ {
		values := make([]sql.NullString, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			return err
		}
		for _, value := range values {
			if len(value.String) > MaxCellBytes {
				return public("result_too_large", "Metadatenwert überschreitet die erlaubte Größe.")
			}
		}
		if index < r.Offset {
			continue
		}
		if len(out.Records) == r.Limit {
			out.HasMore = true
			break
		}
		record := core.MonitorRecord{}
		for i, col := range columns {
			for _, f := range fields {
				if strings.EqualFold(col, f.name) && values[i].Valid && monitorValue(values[i].String, f.kind) {
					record[strings.ToLower(f.name)] = values[i].String
				}
			}
		}
		out.Records = append(out.Records, record)
	}
	return rows.Err()
}
func numericFields(names ...string) []monitorField {
	f := []monitorField{}
	for _, n := range names {
		f = append(f, monitorField{n, "number"})
	}
	return f
}

func monitorAdvice(records []core.MonitorRecord, source string) []core.MonitorHint {
	hints := []core.MonitorHint{}
	rules := []struct {
		metric    string
		threshold float64
		message   string
	}{{"threads_running", 32, "Viele gleichzeitig aktive Threads; Arbeitslast und Wartezeiten prüfen."}, {"slow_queries", 0, "Langsame Abfragen wurden seit dem Zählerstart beobachtet; kein aktueller Ratenwert."}, {"deadlocks", 0, "Deadlocks seit dem Statistikstart beobachtet; Sperrreihenfolge prüfen."}, {"temp_files", 100, "Viele temporäre Dateien seit dem Statistikstart; Abfragepläne prüfen."}}
	for _, rule := range rules {
		for _, record := range records {
			value := record[rule.metric]
			number, err := strconv.ParseFloat(value, 64)
			if err == nil && number > rule.threshold {
				hints = append(hints, core.MonitorHint{Rule: "w-p27/" + rule.metric, Source: source, Metric: rule.metric, Threshold: fmt.Sprintf("> %g", rule.threshold), Observed: value, Message: rule.message})
			}
		}
	}
	return hints
}

func monitorMySQL(ctx context.Context, db *sql.DB, action string, r core.MonitorRequest, out *core.MonitorResult) error {
	switch action {
	case "sample", "advisor":
		out.Source = "global_status"
		rows, err := db.QueryContext(ctx, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Uptime','Questions','Bytes_received','Bytes_sent','Threads_running','Threads_connected','Slow_queries','Connections','Aborted_connects')")
		if err != nil {
			return err
		}
		defer rows.Close()
		record := core.MonitorRecord{}
		for rows.Next() {
			var key, value string
			if err = rows.Scan(&key, &value); err != nil {
				return err
			}
			switch strings.ToLower(key) {
			case "uptime", "questions", "bytes_received", "bytes_sent", "threads_running", "threads_connected", "slow_queries", "connections", "aborted_connects":
				if monitorValue(value, "number") {
					record[strings.ToLower(key)] = value
				}
			}
		}
		if err = rows.Err(); err != nil {
			return err
		}
		if r.Offset == 0 {
			out.Records = append(out.Records, record)
		}
		return nil
	case "analysis":
		out.Source = "performance_schema"
		var hasDigests bool
		probeErr := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM performance_schema.events_statements_summary_by_digest WHERE DIGEST IS NOT NULL)").Scan(&hasDigests)
		if probeErr != nil && !monitorUnavailable(probeErr) {
			return probeErr
		}
		fields := append(numericFields("calls", "total_seconds", "rows_sent"), monitorField{"digest", "digest"})
		query := fmt.Sprintf("SELECT DIGEST AS digest, COUNT_STAR AS calls, SUM_TIMER_WAIT / 1000000000000 AS total_seconds, SUM_ROWS_SENT AS rows_sent FROM performance_schema.events_statements_summary_by_digest WHERE DIGEST IS NOT NULL ORDER BY SUM_TIMER_WAIT DESC, DIGEST, SCHEMA_NAME LIMIT %d", r.Offset+r.Limit+1)
		if probeErr == nil && hasDigests {
			err := monitorRead(ctx, db, query, fields, r, out)
			if err == nil {
				return nil
			}
			if !monitorUnavailable(err) {
				return err
			}
		}
		out.Records = []core.MonitorRecord{}
		out.HasMore = false
		out.Source = "slow_log_table"
		var enabled bool
		var output string
		if err := db.QueryRowContext(ctx, "SELECT @@global.slow_query_log, @@global.log_output").Scan(&enabled, &output); err != nil {
			return err
		}
		if !enabled || !strings.Contains(output, "TABLE") {
			out.State = "unavailable"
			out.Reason = "Keine lesbare aktive Analysequelle: Performance-Schema ohne Messwerte und Slow-Log-Tabellenausgabe deaktiviert."
			return nil
		}
		out.Reason = "Nur aggregierte Laufzeiten; kein SQL- oder Log-Rohtext."
		return monitorRead(ctx, db, "SELECT COUNT(*) AS calls, COALESCE(SUM(TIME_TO_SEC(query_time)),0) AS total_seconds, COALESCE(SUM(rows_sent),0) AS rows_sent FROM mysql.slow_log", numericFields("calls", "total_seconds", "rows_sent"), r, out)
	case "replication":
		out.Source = "replica_status"
		fields := append(numericFields("Seconds_Behind_Source", "Seconds_Behind_Master", "Read_Source_Log_Pos", "Read_Master_Log_Pos", "Exec_Source_Log_Pos", "Exec_Master_Log_Pos", "Last_IO_Errno", "Last_SQL_Errno"), monitorField{"Replica_IO_Running", "Yes|No|Connecting"}, monitorField{"Slave_IO_Running", "Yes|No|Connecting"}, monitorField{"Replica_SQL_Running", "Yes|No"}, monitorField{"Slave_SQL_Running", "Yes|No"})
		var version string
		if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
			return err
		}
		command := "SHOW REPLICA STATUS"
		if strings.Contains(strings.ToLower(version), "mariadb") {
			command = "SHOW ALL SLAVES STATUS"
		}
		err := monitorRead(ctx, db, command, fields, r, out)
		if err != nil && monitorSyntax(err) {
			out.Records = []core.MonitorRecord{}
			err = monitorRead(ctx, db, "SHOW ALL SLAVES STATUS", fields, r, out)
			if err != nil && monitorSyntax(err) {
				err = monitorRead(ctx, db, "SHOW SLAVE STATUS", fields, r, out)
			}
		}
		return err
	case "binary-logs":
		out.Source = "binary_logs"
		return monitorRead(ctx, db, "SHOW BINARY LOGS", []monitorField{{"Log_name", "log"}, {"File_size", "number"}, {"Encrypted", "Yes|No"}}, r, out)
	case "binary-events":
		out.Source = "binlog_events"
		// Validate again against the readable log catalog; never accept a path or a SQL fragment.
		logs := core.MonitorResult{Records: []core.MonitorRecord{}}
		lr := r
		lr.Offset = 0
		lr.Limit = 10200
		if err := monitorRead(ctx, db, "SHOW BINARY LOGS", []monitorField{{"Log_name", "log"}}, lr, &logs); err != nil {
			return err
		}
		found := false
		for _, entry := range logs.Records {
			found = found || entry["log_name"] == r.Log
		}
		if !found {
			return public("invalid_monitor", "Binärlog ist nicht in der lesbaren Liste enthalten.")
		}
		query := fmt.Sprintf("SHOW BINLOG EVENTS IN '%s' FROM %d LIMIT %d", r.Log, r.Position, r.Offset+r.Limit+1)
		out.Reason = "Ereignis-Rohtexte werden unterdrückt; Paging ist bei Logrotation nicht stabil."
		return monitorRead(ctx, db, query, append(numericFields("Pos", "Server_id", "End_log_pos"), monitorField{"Event_type", "Start|Stop|Rotate|Format_desc|Previous_gtids|Gtid|Anonymous_Gtid|Query|Table_map|Write_rows|Update_rows|Delete_rows|Xid|Rows_query|Annotate_rows|Heartbeat|Ignorable"}), r, out)
	case "profiling":
		return monitorProfile(ctx, db, r, out)
	case "engines":
		return monitorEngines(ctx, db, r, out)
	}
	return nil
}
func monitorSyntax(err error) bool {
	var m *mysql.MySQLError
	return errors.As(err, &m) && m.Number == 1064
}

func monitorPostgres(ctx context.Context, db *sql.DB, action string, r core.MonitorRequest, out *core.MonitorResult) error {
	switch action {
	case "sample", "advisor":
		out.Source = "pg_stat_database"
		return monitorRead(ctx, db, "SELECT numbackends, xact_commit, xact_rollback, blks_read, blks_hit, tup_returned, tup_fetched, deadlocks, temp_files, temp_bytes FROM pg_catalog.pg_stat_database WHERE datname=current_database()", numericFields("numbackends", "xact_commit", "xact_rollback", "blks_read", "blks_hit", "tup_returned", "tup_fetched", "deadlocks", "temp_files", "temp_bytes"), r, out)
	case "analysis", "replication":
		var permitted bool
		if err := db.QueryRowContext(ctx, "SELECT rolsuper OR pg_has_role(current_user, 'pg_read_all_stats', 'MEMBER') FROM pg_catalog.pg_roles WHERE rolname=current_user").Scan(&permitted); err != nil {
			return err
		}
		if !permitted {
			out.State = "unavailable"
			out.Reason = "Für diese Ansicht ist pg_read_all_stats oder ein Superuser erforderlich."
			return nil
		}
		if action == "analysis" {
			out.Source = "pg_stat_statements"
			var schema string
			err := db.QueryRowContext(ctx, "SELECT n.nspname FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='pg_stat_statements'").Scan(&schema)
			if errors.Is(err, sql.ErrNoRows) {
				out.State = "unavailable"
				out.Reason = "pg_stat_statements ist nicht installiert."
				return nil
			}
			if err != nil {
				return err
			}
			query := fmt.Sprintf("SELECT queryid, calls, total_exec_time, rows FROM %s.pg_stat_statements ORDER BY total_exec_time DESC, queryid, userid, dbid, toplevel LIMIT %d", quoteIdentifier("postgres", schema), r.Offset+r.Limit+1)
			return monitorRead(ctx, db, query, numericFields("queryid", "calls", "total_exec_time", "rows"), r, out)
		}
		out.Source = "pg_replication"
		out.Reason = "Quell- und Empfängerstatus; Verbindungsinformationen, Slots und LSN-Texte werden unterdrückt."
		query := fmt.Sprintf("SELECT pid, state, sync_state, role FROM (SELECT pid, state, sync_state, 'sender' AS role FROM pg_catalog.pg_stat_replication UNION ALL SELECT pid, status AS state, NULL AS sync_state, 'receiver' AS role FROM pg_catalog.pg_stat_wal_receiver) AS replication ORDER BY role, pid LIMIT %d", r.Offset+r.Limit+1)
		return monitorRead(ctx, db, query, []monitorField{{"pid", "number"}, {"state", "startup|catchup|streaming|backup|stopped|starting|waiting|restarting|stopping"}, {"sync_state", "async|potential|sync|quorum"}, {"role", "sender|receiver"}}, r, out)
	case "profiling":
		return monitorProfile(ctx, db, r, out)
	case "engines":
		out.Source = "pg_am"
		out.Reason = "Nur Anzahl der Table-Access-Methods; keine MySQL-Engine-Statusdaten."
		return monitorRead(ctx, db, "SELECT count(*) AS table_access_methods FROM pg_catalog.pg_am WHERE amtype='t'", numericFields("table_access_methods"), r, out)
	}
	return nil
}
