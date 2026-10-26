// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func monitorProfile(ctx context.Context, db *sql.DB, r core.MonitorRequest, out *core.MonitorResult) error {
	out.Source = "explain"
	out.Reason = "Nur Planung einfacher SELECT-Anweisungen; keine Ausführung, kein ANALYZE und keine Laufzeitprofilierung. Plan-Rohtexte werden unterdrückt."
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if r.Connection.Driver == "mysql" {
		err = monitorRead(ctx, tx, "EXPLAIN "+r.SQL, append(numericFields("id", "rows", "filtered"), monitorField{"type", "system|const|eq_ref|ref|fulltext|ref_or_null|index_merge|unique_subquery|index_subquery|range|index|ALL"}), r, out)
		return err
	}
	var raw string
	if err = tx.QueryRowContext(ctx, "EXPLAIN (FORMAT JSON) "+r.SQL).Scan(&raw); err != nil {
		return err
	}
	return monitorPostgresPlan(raw, r, out)
}

func monitorPostgresPlan(raw string, r core.MonitorRequest, out *core.MonitorResult) error {
	if len(raw) > MaxCellBytes {
		return public("result_too_large", "Abfrageplan überschreitet die erlaubte Größe.")
	}
	var plan any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&plan); err != nil {
		return err
	}
	all := []core.MonitorRecord{}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, item := range node {
				walk(item)
			}
		case map[string]any:
			if _, ok := node["Node Type"]; ok {
				record := core.MonitorRecord{}
				for _, key := range []string{"Startup Cost", "Total Cost", "Plan Rows", "Plan Width"} {
					if number, ok := node[key].(json.Number); ok && monitorNumber.MatchString(string(number)) {
						record[strings.ToLower(strings.ReplaceAll(key, " ", "_"))] = string(number)
					}
				}
				// Node names are server enum labels; no identifiers, expressions or filters.
				if kind, ok := node["Node Type"].(string); ok && monitorValue(kind, "Result|Seq Scan|Index Scan|Index Only Scan|Bitmap Heap Scan|Bitmap Index Scan|Limit|Sort|Gather|Gather Merge|Append|Aggregate|Group|Nested Loop|Hash Join|Merge Join|Hash|Materialize|Memoize|Subquery Scan|Unique|LockRows|SetOp|WindowAgg") {
					record["node_type"] = kind
				}
				all = append(all, record)
			}
			if p, ok := node["Plan"]; ok {
				walk(p)
			}
			if p, ok := node["Plans"]; ok {
				walk(p)
			}
		}
	}
	walk(plan)
	for i, record := range all {
		if i < r.Offset {
			continue
		}
		if len(out.Records) == r.Limit {
			out.HasMore = true
			break
		}
		out.Records = append(out.Records, record)
	}
	return nil
}

var innodbMetric = regexp.MustCompile(`(?m)^(History list length|Buffer pool size|Free buffers|Database pages|Modified db pages)\s+([0-9]+)\s*$`)

func monitorEngines(ctx context.Context, db *sql.DB, r core.MonitorRequest, out *core.MonitorResult) error {
	out.Source = "show_engines"
	out.Reason = "Engine-Fähigkeiten und begrenzte numerische InnoDB-Details; SQL-, Transaktions- und Dateitexte werden unterdrückt."
	fields := []monitorField{{"Engine", "InnoDB|MyISAM|MEMORY|CSV|ARCHIVE|BLACKHOLE|MRG_MYISAM|PERFORMANCE_SCHEMA|SEQUENCE|Aria|FEDERATED|partition"}, {"Support", "YES|NO|DEFAULT|DISABLED"}, {"Transactions", "YES|NO"}, {"XA", "YES|NO"}, {"Savepoints", "YES|NO"}}
	if err := monitorRead(ctx, db, "SHOW ENGINES", fields, r, out); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "SHOW ENGINE INNODB STATUS")
	if err != nil {
		if monitorUnavailable(err) {
			out.Reason += " InnoDB-Status ist mit diesen Rechten nicht lesbar."
			return nil
		}
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var engine, name, status string
		if err = rows.Scan(&engine, &name, &status); err != nil {
			return err
		}
		if len(status) > MaxCellBytes {
			return public("result_too_large", "Engine-Status überschreitet die erlaubte Größe.")
		}
		// Never return raw InnoDB status: it can contain user SQL, credentials and paths.
		if len(status) > 16384 {
			status = status[:16384]
			out.Truncated = true
		}
		record := core.MonitorRecord{"engine": "InnoDB"}
		for _, match := range innodbMetric.FindAllStringSubmatch(status, -1) {
			if _, err = strconv.ParseUint(match[2], 10, 64); err == nil {
				record[strings.ToLower(strings.ReplaceAll(match[1], " ", "_"))] = match[2]
			}
		}
		// Attach the detail to an existing InnoDB entry to keep the page bound exact.
		for _, entry := range out.Records {
			if entry["engine"] == "InnoDB" {
				for key, value := range record {
					entry[key] = value
				}
			}
		}
	}
	return rows.Err()
}
