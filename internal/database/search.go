// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func invalidSearch() error {
	return public("invalid_search", "Suchanfrage oder Grenzen sind ungültig.")
}
func searchLimits(rows, preview, timeout *int) error {
	if *rows < 0 || *rows > core.SearchMaxRows || *preview < 0 || *preview > core.SearchMaxPreview || *timeout < 0 || *timeout > 60000 {
		return invalidSearch()
	}
	if *rows == 0 {
		*rows = 100
	}
	if *preview == 0 {
		*preview = 5
	}
	if *timeout == 0 {
		*timeout = 10000
	}
	return nil
}
func searchText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && len(s) <= core.SearchMaxValue
}
func searchColumn(c core.TableColumn) bool {
	typ := strings.ToUpper(c.DatabaseType)
	// Deliberately exclude binary, JSON, arrays, domains and arbitrary casts.
	if strings.ContainsAny(typ, "[]") || strings.Contains(typ, "BINARY") || strings.Contains(typ, "BLOB") {
		return false
	}
	for _, prefix := range []string{"TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "CHAR", "VARCHAR", "CHARACTER", "NCHAR", "NVARCHAR", "CLOB"} {
		if typ == prefix || strings.HasPrefix(typ, prefix+"(") || strings.HasPrefix(typ, prefix+" ") {
			return true
		}
	}
	return false
}

type searchBuilder struct {
	driver string
	args   []any
}

func (b *searchBuilder) bind(v any) string {
	b.args = append(b.args, v)
	if b.driver == "postgres" {
		return "$" + strconv.Itoa(len(b.args))
	}
	return "?"
}
func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}
func (b *searchBuilder) predicate(names []string, term, mode string) (string, error) {
	if !searchText(term) || term == "" || len(names) == 0 {
		return "", invalidSearch()
	}
	terms := []string{term}
	if mode == "all_words" {
		terms = strings.Fields(term)
		if len(terms) == 0 || len(terms) > 32 {
			return "", invalidSearch()
		}
	}
	groups := []string{}
	for _, word := range terms {
		clauses := []string{}
		for _, name := range names {
			switch mode {
			case "substring", "all_words":
				expr := name + " LIKE " + b.bind("%"+escapeLike(word)+"%") + " ESCAPE '!'"
				if b.driver == "sqlite" {
					expr = "typeof(" + name + ")='text' AND (" + expr + ")"
				}
				clauses = append(clauses, "("+expr+")")
			case "exact":
				expr := name + " = " + b.bind(word)
				if b.driver == "sqlite" {
					expr = "typeof(" + name + ")='text' AND (" + expr + ")"
				}
				clauses = append(clauses, "("+expr+")")
			case "regex":
				if b.driver == "sqlite" {
					return "", public("unsupported_search", "Regex ist für diesen Treiber nicht verfügbar.")
				}
				op := " REGEXP "
				if b.driver == "postgres" {
					op = " ~ "
				}
				clauses = append(clauses, name+op+b.bind(word))
			default:
				return "", invalidSearch()
			}
		}
		groups = append(groups, "("+strings.Join(clauses, " OR ")+")")
	}
	return strings.Join(groups, " AND "), nil
}
func searchNames(d core.TableDescription, requested []string, driver string) ([]string, error) {
	if len(requested) == 0 || len(requested) > 32 || d.Kind != "table" {
		return nil, invalidSearch()
	}
	names := []string{}
	seen := map[string]bool{}
	for _, name := range requested {
		found := false
		for _, col := range d.Columns {
			if col.Name == name && searchColumn(col) {
				found = true
				break
			}
		}
		if !found || seen[name] {
			return nil, public("unsupported_search", "Suchspalte ist nicht als Textspalte verfügbar.")
		}
		seen[name] = true
		names = append(names, quoteIdentifier(driver, name))
	}
	return names, nil
}
func searchTable(d core.TableDescription, driver string) string {
	return quoteIdentifier(driver, d.Table.Schema) + "." + quoteIdentifier(driver, d.Table.Name)
}
func searchRead(ctx context.Context, db *sql.DB, c core.Connection, q string, args []any, limit int) (core.ResultSet, error) {
	return boundTableRows(ctx, db, c, q, args, limit)
}
func (e *Engine) Search(ctx context.Context, r core.SearchRequest) (core.SearchResult, error) {
	empty := core.SearchResult{}
	if len(r.Targets) == 0 || len(r.Targets) > core.SearchMaxTables {
		return empty, invalidSearch()
	}
	if err := searchLimits(&r.MaxRows, &r.PreviewRows, &r.TimeoutMS); err != nil {
		return empty, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer closeDB()
	result := core.SearchResult{Tables: []core.SearchTableResult{}}
	seen := map[core.TableRef]bool{}
	remaining := r.MaxRows
	for _, target := range r.Targets {
		if !validTableRef(target.Table) {
			return empty, invalidSearch()
		}
		d, err := describe(ctx, db, r.Connection, target.Table)
		if err != nil {
			return empty, safe(ctx, err)
		}
		if seen[d.Table] {
			return empty, invalidSearch()
		}
		seen[d.Table] = true
		names, err := searchNames(d, target.Columns, r.Connection.Driver)
		if err != nil {
			return empty, err
		}
		b := searchBuilder{driver: r.Connection.Driver}
		pred, err := b.predicate(names, r.Term, r.Mode)
		if err != nil {
			return empty, err
		}
		// Fetch one bounded probe row beyond the count budget. Values are truncated
		// in SQL before transmission; SQLite storage classes exclude runtime blobs.
		expressions := []string{}
		for _, name := range names {
			expr := "SUBSTR(" + name + ",1,256)"
			if b.driver == "sqlite" {
				expr = "CASE WHEN typeof(" + name + ")='text' THEN " + expr + " ELSE NULL END"
			}
			expressions = append(expressions, expr)
		}
		q := "SELECT " + strings.Join(expressions, ",") + " FROM " + searchTable(d, b.driver) + " WHERE " + pred + " LIMIT " + b.bind(int64(remaining+1))
		set, err := searchRead(ctx, db, r.Connection, q, b.args, remaining+1)
		if err != nil {
			return empty, safe(ctx, err)
		}
		count := len(set.Rows)
		exact := !set.Truncated && count <= remaining
		if count > remaining {
			count = remaining
		}
		preview := count
		if preview > r.PreviewRows {
			preview = r.PreviewRows
		}
		result.Tables = append(result.Tables, core.SearchTableResult{Table: d.Table, Columns: target.Columns, Count: count, CountExact: exact, Rows: append([][]core.Cell{}, set.Rows[:preview]...)})
		remaining -= count
	}
	encoded, _ := json.Marshal(result)
	if len(encoded)+1 > MaxResponseBytes {
		return empty, public("result_too_large", "Suchergebnis ist zu groß.")
	}
	return result, nil
}

// SearchBrowse adds operators only on v14; the frozen v2 SQL builder stays unchanged.
func (e *Engine) SearchBrowse(ctx context.Context, r core.BrowseRequest) (core.BrowseResult, error) {
	if !validTableRef(r.Table) || len(r.Filters) > 16 || len(r.Sort) > 16 {
		return core.BrowseResult{}, invalidSearch()
	}
	preview := 1
	if err := searchLimits(&r.PageSize, &preview, &r.TimeoutMS); err != nil || r.Offset < 0 || r.Offset > 1000000 {
		return core.BrowseResult{}, invalidSearch()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutMS)*time.Millisecond)
	defer cancel()
	db, closeDB, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.BrowseResult{}, err
	}
	defer closeDB()
	d, err := describe(ctx, db, r.Connection, r.Table)
	if err != nil {
		return core.BrowseResult{}, safe(ctx, err)
	}
	query, args, err := searchBrowseSQL(r, d)
	if err != nil {
		return core.BrowseResult{}, err
	}
	set, err := boundTableRows(ctx, db, r.Connection, query, args, r.PageSize)
	if err != nil {
		return core.BrowseResult{}, safe(ctx, err)
	}
	more := set.Truncated
	if len(set.Rows) > r.PageSize {
		more = true
		set.Rows = set.Rows[:r.PageSize]
	}
	set.Truncated = false
	result := core.BrowseResult{Description: d, Result: set, Offset: r.Offset, PageSize: r.PageSize, StableOrder: d.StableOrder, HasMore: more}
	encoded, _ := json.Marshal(result)
	if len(encoded) > MaxResponseBytes {
		return core.BrowseResult{}, public("result_too_large", "Ergebnis ist zu groß.")
	}
	return result, nil
}

func searchBrowseSQL(r core.BrowseRequest, d core.TableDescription) (string, []any, error) {
	base := r
	base.Filters = nil
	query, args, err := browseSQL(base, d)
	if err != nil {
		return "", nil, err
	}
	// Build predicates and pagination parameters in a single binding sequence.
	b := searchBuilder{driver: r.Connection.Driver}
	clauses := []string{}
	for _, f := range r.Filters {
		found := false
		for _, col := range d.Columns {
			if col.Name == f.Column {
				found = true
			}
		}
		if !found {
			return "", nil, invalidSearch()
		}
		name := quoteIdentifier(b.driver, f.Column)
		if f.Operator == "is_null" || f.Operator == "is_not_null" {
			if f.Value != nil {
				return "", nil, invalidSearch()
			}
			suffix := " IS NULL"
			if f.Operator == "is_not_null" {
				suffix = " IS NOT NULL"
			}
			clauses = append(clauses, name+suffix)
			continue
		}
		if f.Value == nil {
			return "", nil, invalidSearch()
		}
		v, err := filterValue(*f.Value)
		if err != nil {
			return "", nil, err
		}
		op := map[string]string{"eq": "=", "ne": "<>", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}[f.Operator]
		if f.Operator == "contains" {
			if f.Value.Kind != "text" {
				return "", nil, invalidSearch()
			}
			clauses = append(clauses, name+" LIKE "+b.bind("%"+escapeLike(f.Value.Value)+"%")+" ESCAPE '!'")
			continue
		}
		if f.Operator == "like" {
			if f.Value.Kind != "text" {
				return "", nil, invalidSearch()
			}
			clauses = append(clauses, name+" LIKE "+b.bind(v)+" ESCAPE '!'")
			continue
		}
		if op == "" {
			return "", nil, invalidSearch()
		}
		clauses = append(clauses, name+" "+op+" "+b.bind(v))
	}
	// Split at the exact catalog-derived SELECT prefix, never at SQL-looking
	// text that may occur inside a quoted table/column name.
	names := []string{}
	for _, col := range d.Columns {
		names = append(names, quoteIdentifier(b.driver, col.Name))
	}
	prefix := "SELECT " + strings.Join(names, ",") + " FROM " + searchTable(d, b.driver)
	suffix := strings.TrimPrefix(query, prefix)
	limit := " LIMIT ? OFFSET ?"
	if b.driver == "postgres" {
		limit = " LIMIT $1 OFFSET $2"
	}
	order := strings.TrimSuffix(suffix, limit)
	query = prefix
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += order + " LIMIT " + b.bind(args[0]) + " OFFSET " + b.bind(args[1])
	return query, b.args, nil
}
