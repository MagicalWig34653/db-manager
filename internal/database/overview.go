// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

const maxSafeInteger = 9007199254740991

func invalidOverview(message string) error { return public("invalid_overview", message) }

// overviewParams validates paging and timeout values shared by all overview routes.
func overviewParams(limit, offset, timeoutMS int) (int, int, time.Duration, error) {
	if limit < 0 || limit > core.OverviewMaxLimit || offset < 0 || offset > core.OverviewMaxOffset || timeoutMS < 0 || timeoutMS > core.OverviewMaxTimeoutMS {
		return 0, 0, 0, invalidOverview("Seitengröße, Offset oder Zeitlimit sind ungültig.")
	}
	if limit == 0 {
		limit = core.OverviewDefaultLimit
	}
	if timeoutMS == 0 {
		timeoutMS = core.OverviewDefaultTimeoutMS
	}
	return limit, offset, time.Duration(timeoutMS) * time.Millisecond, nil
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}
func nullStringNonEmpty(value sql.NullString) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	return nullString(value)
}
func nullInt(value sql.NullInt64) *int64 {
	if !value.Valid || value.Int64 < 0 {
		return nil
	}
	number := value.Int64
	if number > maxSafeInteger {
		number = maxSafeInteger
	}
	return &number
}
func stringPointer(value string) *string { return &value }

func validateOverviewSize(result any) error {
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaxResponseBytes {
		return public("result_too_large", "Antwort überschreitet die erlaubte Größe.")
	}
	return nil
}

func (e *Engine) Databases(ctx context.Context, r core.DatabasesRequest) (core.DatabasesResult, error) {
	limit, offset, timeout, err := overviewParams(r.Limit, r.Offset, r.TimeoutMS)
	if err != nil {
		return core.DatabasesResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.DatabasesResult{}, err
	}
	defer cleanup()
	var result core.DatabasesResult
	switch r.Connection.Driver {
	case "sqlite":
		result, err = sqliteDatabases(ctx, db, r.Connection, limit, offset)
	case "mysql":
		result, err = mysqlDatabases(ctx, db, limit, offset)
	case "postgres":
		result, err = postgresDatabases(ctx, db, limit, offset)
	}
	if err != nil {
		return core.DatabasesResult{}, safe(ctx, err)
	}
	result.Driver, result.Offset, result.Limit = r.Connection.Driver, offset, limit
	if err = validateOverviewSize(result); err != nil {
		return core.DatabasesResult{}, err
	}
	return result, nil
}

func (e *Engine) Objects(ctx context.Context, r core.ObjectsRequest) (core.ObjectsResult, error) {
	limit, offset, timeout, err := overviewParams(r.Limit, r.Offset, r.TimeoutMS)
	if err != nil {
		return core.ObjectsResult{}, err
	}
	if strings.ContainsRune(r.Schema, 0) || len(r.Schema) > 1024 {
		return core.ObjectsResult{}, invalidOverview("Schemaangabe ist ungültig.")
	}
	if r.ExactRows {
		if r.Limit == 0 {
			limit = core.OverviewMaxExactObjects
		}
		if limit > core.OverviewMaxExactObjects {
			return core.ObjectsResult{}, invalidOverview("Exakte Zählung ist auf höchstens 100 Objekte je Seite begrenzt.")
		}
	}
	// The catalog read and the optional exact counting have separate budgets so a
	// slow COUNT never invalidates the already read catalog page.
	catalogCtx, cancel := context.WithTimeout(ctx, core.OverviewDefaultTimeoutMS*time.Millisecond)
	defer cancel()
	db, cleanup, err := e.open(catalogCtx, r.Connection)
	if err != nil {
		return core.ObjectsResult{}, err
	}
	defer cleanup()
	var result core.ObjectsResult
	switch r.Connection.Driver {
	case "sqlite":
		result, err = sqliteObjects(catalogCtx, db, r.Schema, limit, offset)
	case "mysql":
		result, err = mysqlObjects(catalogCtx, db, r.Connection, r.Schema, limit, offset)
	case "postgres":
		result, err = postgresObjects(catalogCtx, db, r.Schema, limit, offset)
	}
	if err != nil {
		return core.ObjectsResult{}, safe(catalogCtx, err)
	}
	result.Driver, result.Offset, result.Limit, result.ExactRequested = r.Connection.Driver, offset, limit, r.ExactRows
	if r.ExactRows {
		countCtx, countCancel := context.WithTimeout(ctx, timeout)
		defer countCancel()
		exactCounts(countCtx, db, r.Connection.Driver, &result)
		if err = ctx.Err(); err != nil {
			return core.ObjectsResult{}, err
		}
	}
	if err = validateOverviewSize(result); err != nil {
		return core.ObjectsResult{}, err
	}
	return result, nil
}

// exactCounts runs COUNT(*) for base tables of the page. Names come from the
// catalog rows just read, never from the request. It stops at the first timeout
// and marks the rest unknown; it never reports an estimate as exact.
func exactCounts(ctx context.Context, db *sql.DB, driver string, result *core.ObjectsResult) {
	stopped := false
	for i := range result.Objects {
		object := &result.Objects[i]
		if object.Kind != "table" {
			if object.Rows.State == "estimated" {
				object.Rows = core.RowCount{State: "unknown"}
			}
			continue
		}
		if stopped {
			object.Rows = core.RowCount{State: "unknown"}
			result.ExactIncomplete = true
			continue
		}
		schema := object.Schema
		if driver == "sqlite" {
			schema = "main"
		}
		name, err := dbplan.QualifiedName(driver, schema, object.Name)
		if err != nil {
			object.Rows = core.RowCount{State: "unknown"}
			result.ExactIncomplete = true
			continue
		}
		var count int64
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+name).Scan(&count)
		if err != nil {
			object.Rows = core.RowCount{State: "unknown"}
			result.ExactIncomplete = true
			if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
				stopped = true
			}
			continue
		}
		if count > maxSafeInteger {
			count = maxSafeInteger
		}
		object.Rows = core.RowCount{Value: &count, State: "exact"}
	}
}

func (e *Engine) Definition(ctx context.Context, r core.DefinitionRequest) (core.DefinitionResult, error) {
	_, _, timeout, err := overviewParams(0, 0, r.TimeoutMS)
	if err != nil {
		return core.DefinitionResult{}, err
	}
	if !validTableRef(r.Object) {
		return core.DefinitionResult{}, invalidOverview("Objektbezug ist ungültig.")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.DefinitionResult{}, err
	}
	defer cleanup()
	var result core.DefinitionResult
	switch r.Connection.Driver {
	case "sqlite":
		result, err = sqliteDefinition(ctx, db, r.Object)
	case "mysql":
		result, err = mysqlDefinition(ctx, db, r.Connection, r.Object)
	case "postgres":
		result, err = postgresDefinition(ctx, db, r.Object)
	}
	if err != nil {
		return core.DefinitionResult{}, safe(ctx, err)
	}
	if len(result.Definition) > core.OverviewMaxDefinition {
		return core.DefinitionResult{}, public("result_too_large", "Definition überschreitet die erlaubte Größe.")
	}
	result.Driver = r.Connection.Driver
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	if err = validateOverviewSize(result); err != nil {
		return core.DefinitionResult{}, err
	}
	return result, nil
}

func (e *Engine) ServerInfo(ctx context.Context, r core.ServerInfoRequest) (core.ServerInfoResult, error) {
	_, _, timeout, err := overviewParams(0, 0, r.TimeoutMS)
	if err != nil {
		return core.ServerInfoResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.ServerInfoResult{}, err
	}
	defer cleanup()
	result := core.ServerInfoResult{Driver: r.Connection.Driver, Session: []core.NameValue{}, CharacterSets: []core.CharacterSetInfo{}, Collations: []core.CollationInfo{}, Engines: []core.EngineInfo{}, Plugins: []core.PluginInfo{}}
	// Only non-secret connection data is echoed; the password is never copied.
	result.Connection = core.ServerConnection{Host: r.Connection.Host, Port: r.Connection.Port, Database: r.Connection.Database, Username: r.Connection.Username, TLSMode: r.Connection.TLSMode}
	if r.Connection.Driver == "sqlite" {
		result.Connection.Host, result.Connection.Port, result.Connection.Username = "", 0, ""
	}
	switch r.Connection.Driver {
	case "sqlite":
		err = sqliteServerInfo(ctx, db, &result)
	case "mysql":
		err = mysqlServerInfo(ctx, db, &result)
	case "postgres":
		err = postgresServerInfo(ctx, db, &result)
	}
	if err != nil {
		return core.ServerInfoResult{}, safe(ctx, err)
	}
	if err = validateOverviewSize(result); err != nil {
		return core.ServerInfoResult{}, err
	}
	return result, nil
}

// scanStrings reads rows of N nullable text columns and stops after max rows;
// it reports whether more rows existed.
func scanStrings(rows *sql.Rows, columns, max int, each func([]sql.NullString) error) (bool, error) {
	defer rows.Close()
	count := 0
	for rows.Next() {
		if count == max {
			return true, rows.Err()
		}
		values := make([]sql.NullString, columns)
		dest := make([]any, columns)
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return false, err
		}
		for _, value := range values {
			if len(value.String) > MaxCellBytes {
				return false, public("result_too_large", "Metadatenwert ist zu groß.")
			}
		}
		if err := each(values); err != nil {
			return false, err
		}
		count++
	}
	return false, rows.Err()
}
