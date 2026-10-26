// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/sqlitesandbox"
	sqlite "github.com/ncruces/go-sqlite3"
)

const MaxCellBytes = 1 << 20
const MaxResponseBytes = 8 << 20
const MaxConcurrentRequests = 4

// MaxConcurrentExports bounds simultaneous exports; each holds a connection and a snapshot for up to 300 s.
const MaxConcurrentExports = 2

// Engine retains administrative policy, never request credentials or connections.
type Engine struct {
	dataDir string
	hosts   map[string]bool
	slots   chan struct{}
	exports chan struct{} // bounds long-running exports (MaxConcurrentExports)
	roots   *x509.CertPool
	plans   planLedger // single-use ledger for confirmed import/bulk plans
}

func New(dataDir string, allowedHosts []string, options ...Option) (*Engine, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, public("configuration_error", "Datenverzeichnis ist erforderlich.")
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, public("configuration_error", "Datenverzeichnis ist ungültig.")
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return nil, public("configuration_error", "Datenverzeichnis konnte nicht angelegt werden.")
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, public("configuration_error", "Datenverzeichnis konnte nicht geöffnet werden.")
	}
	root.Close()
	hosts := map[string]bool{}
	if len(allowedHosts) == 0 {
		allowedHosts = []string{"127.0.0.1", "::1", "localhost"}
	}
	for _, host := range allowedHosts {
		host = canonicalHost(strings.TrimSpace(host))
		if !validHost(host) {
			return nil, public("configuration_error", "Erlaubtes Datenbankziel ist ungültig.")
		}
		hosts[host] = true
	}
	engine := &Engine{dataDir: abs, hosts: hosts, slots: make(chan struct{}, MaxConcurrentRequests), exports: make(chan struct{}, MaxConcurrentExports)}
	for _, option := range options {
		if err := option(engine); err != nil {
			return nil, err
		}
	}
	return engine, nil
}
func (e *Engine) Drivers() []string     { return []string{"sqlite", "mysql", "postgres"} }
func public(code, message string) error { return &core.Error{Code: code, Message: message} }
func safe(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, sqlite.TOOBIG) {
		return public("result_too_large", "Ergebniszelle überschreitet die erlaubte Größe.")
	}
	if errors.Is(err, sqlite.BUSY) {
		return public("database_busy", "Datenbank ist bereits geöffnet. Erneut versuchen.")
	}
	var p *core.Error
	if errors.As(err, &p) {
		return p
	}
	return public("database_error", "Datenbankoperation fehlgeschlagen. Verbindung und SQL prüfen.")
}
func (e *Engine) open(ctx context.Context, c core.Connection) (*sql.DB, func(), error) {
	return e.openMode(ctx, c, false)
}

func (e *Engine) openMode(ctx context.Context, c core.Connection, maintenance bool) (*sql.DB, func(), error) {
	if strings.ContainsRune(c.Username, 0) || strings.ContainsRune(c.Password, 0) || strings.ContainsRune(c.Database, 0) {
		return nil, nil, public("invalid_connection", "Verbindungsparameter sind ungültig.")
	}
	if c.Port < 0 || c.Port > 65535 || (c.TLSMode != "" && c.TLSMode != "verify-full" && c.TLSMode != "disable") {
		return nil, nil, public("invalid_connection", "Verbindungsparameter sind ungültig.")
	}
	select {
	case e.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	release := func() { <-e.slots }
	var db *sql.DB
	var cleanup func()
	var err error
	switch c.Driver {
	case "sqlite":
		if maintenance {
			db, cleanup, err = sqlitesandbox.OpenMaintenance(e.dataDir, c.Database)
		} else {
			db, cleanup, err = sqlitesandbox.Open(e.dataDir, c.Database)
		}
	case "mysql", "postgres":
		db, err = e.network(ctx, c)
	default:
		release()
		return nil, nil, public("unsupported_driver", "Datenbanktreiber ist nicht verfügbar.")
	}
	if err != nil {
		release()
		return nil, nil, safe(ctx, err)
	}
	if cleanup == nil {
		cleanup = func() {}
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, func() { db.Close(); cleanup(); release() }, nil
}
func (e *Engine) Test(ctx context.Context, c core.Connection) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, closeDB, err := e.open(ctx, c)
	if err != nil {
		return err
	}
	defer closeDB()
	return safe(ctx, db.PingContext(ctx))
}
func (e *Engine) Query(ctx context.Context, r core.QueryRequest) (core.QueryResult, error) {
	parentContext := ctx
	start := time.Now()
	empty := core.QueryResult{}
	if len(r.SQL) > MaxCellBytes || strings.TrimSpace(r.SQL) == "" || r.MaxRows < 0 || r.MaxRows > 10000 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return empty, public("invalid_query", "SQL oder Abfragegrenzen sind ungültig.")
	}
	if r.MaxRows == 0 {
		r.MaxRows = 1000
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeDB()
	if r.Connection.Driver == "sqlite" {
		return sqliteQuery(ctx, db, r, start)
	}
	// Prepare rejects batches before execution; no transaction spans requests.
	stmt, err := db.PrepareContext(ctx, r.SQL)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return empty, safe(ctx, err)
	}
	defer rows.Close()
	result := core.QueryResult{Results: []core.ResultSet{}}
	// Reserve framing, duration and resultset separators; final JSON is checked exactly.
	bytesUsed := 128
	for {
		if len(result.Results) >= 16 {
			cancel()
			return empty, public("result_too_large", "Zu viele Ergebnissätze.")
		}
		set, used, err := readSet(ctx, rows, r.MaxRows, MaxResponseBytes+512-bytesUsed)
		if err != nil {
			mapped := safe(ctx, err)
			cancel()
			return empty, mapped
		}
		bytesUsed += used + 1
		result.Results = append(result.Results, set)
		if set.Truncated {
			if ctx.Err() != nil {
				return empty, safe(ctx, ctx.Err())
			}
			cancel()
			_ = rows.Close()
			if parentContext.Err() != nil {
				return empty, parentContext.Err()
			}
			result.DurationMS = time.Since(start).Milliseconds()
			encoded, _ := json.Marshal(result)
			if len(encoded)+1 > MaxResponseBytes {
				return empty, public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
			}
			return result, nil
		}
		if !rows.NextResultSet() {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return empty, safe(ctx, err)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	encoded, err := json.Marshal(result)
	if err != nil {
		return empty, public("result_error", "Ergebnis konnte nicht verarbeitet werden.")
	}
	if len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
	}
	return result, nil
}
func readSet(ctx context.Context, rows *sql.Rows, maxRows, budget int) (core.ResultSet, int, error) {
	set := core.ResultSet{Columns: []core.Column{}, Rows: [][]core.Cell{}}
	types, err := rows.ColumnTypes()
	if err != nil {
		return set, 0, err
	}
	if len(types) > 1024 {
		return set, 0, public("result_too_large", "Zu viele Ergebnisspalten.")
	}
	for _, c := range types {
		if len(c.Name()) > MaxCellBytes {
			return set, 0, public("result_too_large", "Spaltenname ist zu groß.")
		}
		set.Columns = append(set.Columns, core.Column{Name: c.Name(), DatabaseType: c.DatabaseTypeName()})
	}
	header, _ := json.Marshal(set)
	used := len(header)
	if used > budget {
		return set, 0, public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
	}
	for rows.Next() {
		if ctx.Err() != nil {
			return set, used, ctx.Err()
		}
		if len(set.Rows) == maxRows {
			set.Truncated = true
			break
		}
		raw := make([]any, len(types))
		dest := make([]any, len(types))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return set, used, err
		}
		row := make([]core.Cell, len(types))
		for i := range raw {
			row[i], err = cell(raw[i], types[i].DatabaseTypeName())
			if err != nil {
				return set, used, err
			}
		}
		data, _ := json.Marshal(row)
		used += len(data) + 1
		if used > budget {
			return set, used, public("result_too_large", "Ergebnis überschreitet die erlaubte Größe.")
		}
		set.Rows = append(set.Rows, row)
	}
	return set, used, rows.Err()
}
func (e *Engine) Catalog(ctx context.Context, c core.Connection) ([]core.Table, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, closeDB, err := e.open(ctx, c)
	if err != nil {
		return nil, err
	}
	defer closeDB()
	var query string
	switch c.Driver {
	case "sqlite":
		query = "SELECT 'main', name, type FROM sqlite_schema WHERE type IN ('table','view') AND name NOT GLOB 'sqlite_*' ORDER BY name"
	case "mysql":
		query = "SELECT TABLE_SCHEMA, TABLE_NAME, CASE WHEN TABLE_TYPE='VIEW' THEN 'view' ELSE 'table' END FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() ORDER BY TABLE_NAME"
	case "postgres":
		query = "SELECT table_schema,table_name,CASE WHEN table_type='VIEW' THEN 'view' ELSE 'table' END FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_schema,table_name"
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, safe(ctx, err)
	}
	defer rows.Close()
	tables := []core.Table{}
	used := 32
	for rows.Next() {
		if len(tables) >= 10000 {
			return nil, public("result_too_large", "Objektliste überschreitet die erlaubte Größe.")
		}
		var table core.Table
		if err := rows.Scan(&table.Schema, &table.Name, &table.Kind); err != nil {
			return nil, safe(ctx, err)
		}
		if len(table.Schema) > MaxCellBytes || len(table.Name) > MaxCellBytes {
			return nil, public("result_too_large", "Objektname ist zu groß.")
		}
		data, _ := json.Marshal(table)
		used += len(data) + 1
		if used > MaxResponseBytes {
			return nil, public("result_too_large", "Objektliste überschreitet die erlaubte Größe.")
		}
		tables = append(tables, table)
	}
	return tables, safe(ctx, rows.Err())
}

// Option applies administrator-controlled policy at engine initialization.
type Option func(*Engine) error

func WithCAFile(path string) Option {
	return func(e *Engine) error {
		if path == "" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > MaxCellBytes {
			return public("configuration_error", "Datenbank-CA-Datei konnte nicht geladen werden.")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return public("configuration_error", "Datenbank-CA-Datei enthält keine gültigen Zertifikate.")
		}
		e.roots = roots
		return nil
	}
}
