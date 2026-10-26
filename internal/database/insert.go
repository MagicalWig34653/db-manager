// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	sqlite "github.com/ncruces/go-sqlite3"
)

func insertPlan(ctx context.Context, q metadataQuerier, c core.Connection, d core.TableDescription) (core.InsertEligibility, error) {
	result := core.InsertEligibility{Description: d, Reason: "unsupported_table", InsertableColumns: []string{}, RequiredPK: []string{}}
	if !eligibility(d).Eligible {
		return result, nil
	}
	if c.Driver == "postgres" {
		var unsupported, rules bool
		err := q.QueryRowContext(ctx, `SELECT c.relkind<>'r' OR c.relispartition OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid),c.relhasrules FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2`, d.Table.Schema, d.Table.Name).Scan(&unsupported, &rules)
		if err != nil {
			return result, err
		}
		if unsupported {
			result.Reason = "unsupported_inheritance"
			return result, nil
		}
		if rules {
			return result, nil
		}
	}
	result.Eligible = true
	result.Reason = "eligible"
	result.RequiredPK = append(result.RequiredPK, d.PrimaryKey...)
	for _, column := range d.Columns {
		if column.Generated == "none" {
			result.InsertableColumns = append(result.InsertableColumns, column.Name)
		}
	}
	if len(d.PrimaryKey) == 1 && d.PrimaryKey[0] != "" {
		var pk core.TableColumn
		for _, column := range d.Columns {
			if column.PKPosition == 1 {
				pk = column
				break
			}
		}
		switch c.Driver {
		case "sqlite":
			if strings.EqualFold(pk.DatabaseType, "INTEGER") {
				var exists int
				err := q.QueryRowContext(ctx, `SELECT 1 FROM pragma_index_list(?) WHERE origin='pk' LIMIT 1`, d.Table.Name).Scan(&exists)
				if errors.Is(err, sql.ErrNoRows) {
					result.AutoPK = pk.Name
				} else if err != nil {
					return result, err
				}
			}
		case "mysql":
			if pk.Generated == "identity" {
				result.AutoPK = pk.Name
			}
		case "postgres":
			if integerType(strings.ToUpper(pk.DatabaseType)) {
				var owned bool
				err := q.QueryRowContext(ctx, `SELECT pg_catalog.pg_get_serial_sequence($1,$2) IS NOT NULL`, quoteIdentifier(c.Driver, d.Table.Schema)+"."+quoteIdentifier(c.Driver, d.Table.Name), pk.Name).Scan(&owned)
				if err != nil {
					return result, err
				}
				if owned && (pk.Generated == "identity" || pk.DefaultValue != nil) {
					result.AutoPK = pk.Name
				}
			}
		}
	}
	if result.AutoPK != "" {
		result.RequiredPK = []string{}
	}
	// A generated PK outside the verified single-auto-key path cannot be supplied.
	for _, column := range d.Columns {
		if column.PKPosition > 0 && column.Generated != "none" && (result.AutoPK == "" || column.Name != result.AutoPK) {
			result.Eligible = false
			result.Reason = "unsupported_identity"
		}
	}
	if !result.Eligible {
		result.InsertableColumns = []string{}
		result.RequiredPK = []string{}
	}
	return result, nil
}
func (e *Engine) InsertEligibility(ctx context.Context, r core.DescribeRequest) (core.InsertEligibility, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !validTableRef(r.Table) {
		return core.InsertEligibility{}, public("invalid_table", "Tabellenbezug ist ungültig.")
	}
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.InsertEligibility{}, err
	}
	defer cleanup()
	d, err := describe(ctx, db, r.Connection, r.Table)
	if err != nil {
		return core.InsertEligibility{}, safe(ctx, err)
	}
	result, err := insertPlan(ctx, db, r.Connection, d)
	if err != nil {
		return result, safe(ctx, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return core.InsertEligibility{}, public("result_too_large", "Einfügemetadaten sind zu groß.")
	}
	return result, nil
}
func (e *Engine) Insert(ctx context.Context, r core.InsertRequest) (core.InsertResult, error) {
	empty := core.InsertResult{}
	if !r.Confirm || !validTableRef(r.Table) || len(r.Values) > 1024 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return empty, public("invalid_insert", "Einfügeanfrage oder Schreibbestätigung ist ungültig.")
	}
	rawBytes := len(r.Connection.Password) + len(r.Connection.Username) + len(r.Connection.Database) + len(r.Connection.Host) + len(r.Table.Name) + len(r.Table.Schema)
	for _, v := range r.Values {
		if _, err := mutationValue(v.Value); err != nil {
			return empty, err
		}
		rawBytes += len(v.Column) + len(v.Value.Value) + 32
		if rawBytes > 1<<20 {
			return empty, public("invalid_insert", "Einfügeanfrage ist zu groß.")
		}
	}
	encoded, err := json.Marshal(r)
	if err != nil || len(encoded) > 1<<20 {
		return empty, public("invalid_insert", "Einfügeanfrage ist zu groß.")
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
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
	session := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = session.begin(ctx); err != nil {
		return empty, safe(ctx, err)
	}
	defer session.rollback()
	result, err := rowsBulkInsert(ctx, session, r)
	if err != nil {
		return empty, err
	}
	if err = session.commit(ctx); err != nil {
		return empty, public("outcome_unknown", "Commit-Ergebnis ist unbekannt. Zustand neu laden; nicht blind wiederholen.")
	}
	return result, nil
}

func rowsBulkInsert(ctx context.Context, session *mutationSession, r core.InsertRequest) (core.InsertResult, error) {
	empty := core.InsertResult{}
	d, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	quote := func(v string) string { return quoteIdentifier(session.driver, v) }
	table := quote(d.Table.Schema) + "." + quote(d.Table.Name)
	names := []string{}
	columns := []string{}
	positions := map[string]int{}
	for i, col := range d.Columns {
		names = append(names, quote(col.Name))
		columns = append(columns, col.Name)
		positions[col.Name] = i
	}
	readTable := table
	if session.driver == "postgres" {
		readTable = "ONLY " + table
	}
	if session.driver != "sqlite" {
		if _, err = session.read(ctx, "SELECT "+strings.Join(names, ",")+" FROM "+readTable+" WHERE 1=0 FOR UPDATE", nil); err != nil {
			return empty, safe(ctx, err)
		}
	}
	fresh, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !reflect.DeepEqual(d, fresh) {
		return empty, rowConflict()
	}
	plan, err := insertPlan(ctx, session, r.Connection, d)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !plan.Eligible {
		return empty, public("insert_not_supported", "Tabelle ist für strukturiertes Einfügen nicht freigegeben.")
	}
	seen := map[string]core.Cell{}
	args := []any{}
	insertNames := []string{}
	parameters := []string{}
	for _, v := range r.Values {
		pos, ok := positions[v.Column]
		_, duplicate := seen[v.Column]
		if !ok || duplicate || d.Columns[pos].Generated != "none" {
			return empty, public("invalid_insert", "Einfügespalte ist unbekannt, doppelt oder generiert.")
		}
		value, err := mutationValue(v.Value)
		if err != nil {
			return empty, err
		}
		seen[v.Column] = v.Value
		insertNames = append(insertNames, quote(v.Column))
		args = append(args, value)
		marker := "?"
		if session.driver == "postgres" {
			marker = "$" + strconv.Itoa(len(args))
		}
		parameters = append(parameters, marker)
	}
	auto := false
	keyCells := make([]core.Cell, len(d.PrimaryKey))
	for i, key := range d.PrimaryKey {
		value, ok := seen[key]
		if !ok && len(d.PrimaryKey) == 1 && plan.AutoPK == key {
			auto = true
			continue
		}
		if !ok || value.Kind == "null" {
			return empty, public("invalid_insert", "Vollständiger nicht-NULL-Primärschlüssel ist erforderlich.")
		}
		keyCells[i] = value
	}
	// Fresh per-request connection plus explicit reset excludes stale session IDs.
	if auto && session.driver == "mysql" {
		var reset string
		if err = session.QueryRowContext(ctx, "SELECT CAST(LAST_INSERT_ID(0) AS CHAR)").Scan(&reset); err != nil {
			return empty, safe(ctx, err)
		}
	}
	insertPrefix := "INSERT INTO "
	if session.driver == "sqlite" {
		insertPrefix = "INSERT OR ABORT INTO "
	}
	query := insertPrefix + table + " (" + strings.Join(insertNames, ",") + ") VALUES (" + strings.Join(parameters, ",") + ")"
	if len(insertNames) == 0 {
		if session.driver == "mysql" {
			query = "INSERT INTO " + table + " () VALUES ()"
		} else {
			query = insertPrefix + table + " DEFAULT VALUES"
		}
	}
	if auto && session.driver == "postgres" {
		returned, err := session.read(ctx, query+" RETURNING "+quote(plan.AutoPK), args)
		if err != nil {
			return empty, insertError(ctx, err)
		}
		if returned.Truncated || len(returned.Rows) != 1 || len(returned.Rows[0]) != 1 {
			return empty, public("insert_not_applied", "Einfügen wurde nicht als Einzelzeile ausgeführt.")
		}
		keyCells[0] = returned.Rows[0][0]
	} else {
		applied, err := session.exec(ctx, query, args...)
		if err != nil {
			return empty, insertError(ctx, err)
		}
		count, err := applied.RowsAffected()
		if err != nil {
			return empty, safe(ctx, err)
		}
		if count != 1 {
			return empty, public("insert_not_applied", "Einfügen wurde nicht als Einzelzeile ausgeführt.")
		}
		if auto {
			if session.driver == "mysql" {
				var id string
				if err = session.QueryRowContext(ctx, "SELECT CAST(LAST_INSERT_ID() AS CHAR)").Scan(&id); err != nil {
					return empty, safe(ctx, err)
				}
				parsed, err := strconv.ParseUint(id, 10, 64)
				if err != nil || parsed == 0 {
					return empty, insertIdentityError()
				}
				keyCells[0] = core.Cell{Kind: "integer", Value: strconv.FormatUint(parsed, 10)}
			} else {
				id, err := applied.LastInsertId()
				if err != nil {
					return empty, safe(ctx, err)
				}
				keyCells[0] = core.Cell{Kind: "integer", Value: strconv.FormatInt(id, 10)}
			}
		}
	}
	keyArgs := []any{}
	predicates := []string{}
	for i, key := range d.PrimaryKey {
		if keyCells[i].Kind == "null" {
			return empty, insertIdentityError()
		}
		value, err := mutationValue(keyCells[i])
		if err != nil {
			return empty, insertIdentityError()
		}
		keyArgs = append(keyArgs, value)
		name := quote(key)
		if session.driver == "sqlite" {
			name += " COLLATE BINARY"
		}
		marker := "?"
		if session.driver == "postgres" {
			marker = "$" + strconv.Itoa(len(keyArgs))
		}
		predicates = append(predicates, name+" = "+marker)
	}
	selectSQL := "SELECT " + strings.Join(names, ",") + " FROM " + readTable + " WHERE " + strings.Join(predicates, " AND ")
	if session.driver != "sqlite" {
		selectSQL += " FOR UPDATE"
	}
	after, err := session.read(ctx, selectSQL, keyArgs)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if after.Truncated || len(after.Rows) != 1 {
		return empty, insertIdentityError()
	}
	for i, key := range d.PrimaryKey {
		if after.Rows[0][positions[key]] != keyCells[i] {
			return empty, insertIdentityError()
		}
	}
	fresh, err = describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !reflect.DeepEqual(d, fresh) {
		return empty, rowConflict()
	}
	result := core.InsertResult{Outcome: "committed", Row: core.RowSnapshot{Columns: columns, Values: after.Rows[0]}}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Einfügeergebnis ist zu groß.")
	}
	return result, nil
}

func insertIdentityError() error {
	return public("insert_identity_unknown", "Eindeutige eingefügte Zeile konnte nicht bestätigt werden; Transaktion wird zurückgerollt.")
}
func insertError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var my *mysql.MySQLError
	var pg *pgconn.PgError
	if errors.Is(err, sqlite.CONSTRAINT_PRIMARYKEY) || errors.Is(err, sqlite.CONSTRAINT_UNIQUE) || (errors.As(err, &my) && my.Number == 1062) || (errors.As(err, &pg) && pg.Code == "23505") {
		return public("insert_conflict", "Eindeutigkeitsbedingung verletzt; keine Zeile überschrieben.")
	}
	if errors.Is(err, sqlite.CONSTRAINT) || (my != nil && (my.Number == 1048 || my.Number == 1452 || my.Number == 3819 || my.Number == 4025)) || (pg != nil && strings.HasPrefix(pg.Code, "23")) {
		return public("constraint_violation", "Einfügen verletzt eine Datenbankbedingung.")
	}
	return safe(ctx, err)
}
