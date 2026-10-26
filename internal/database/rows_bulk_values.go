// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

func fileColumn(d core.TableDescription, name string) (int, error) {
	for i, c := range d.Columns {
		if c.Name == name {
			kind := strings.ToUpper(strings.TrimSpace(strings.SplitN(c.DatabaseType, "(", 2)[0]))
			binaryType := kind == "BLOB" || kind == "TINYBLOB" || kind == "MEDIUMBLOB" || kind == "LONGBLOB" || kind == "BINARY" || kind == "VARBINARY" || kind == "BYTEA"
			if c.Generated == "none" && c.PKPosition == 0 && binaryType {
				return i, nil
			}
			break
		}
	}
	return -1, public("unsupported_file_column", "Dateiwerte benötigen eine beschreibbare Binärspalte außerhalb des Primärschlüssels.")
}
func (e *Engine) FileValueUpload(ctx context.Context, r core.FileValueRequest, src io.Reader) (core.MutationResult, error) {
	if !r.Confirm || r.Preview || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return core.MutationResult{}, bulkInvalid()
	}
	timeout := r.TimeoutMS
	if timeout == 0 {
		timeout = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	// Drivers bind complete values. Read the HTTP stream with a strict upper bound.
	data, err := io.ReadAll(io.LimitReader(src, core.MaxFileValueBytes+1))
	if err != nil {
		return core.MutationResult{}, public("invalid_file", "Datei konnte nicht gelesen werden.")
	}
	if len(data) > core.MaxFileValueBytes {
		return core.MutationResult{}, public("file_too_large", "Dateiwert überschreitet das Größenlimit.")
	}
	request := core.MutationRequest{Connection: r.Connection, Table: r.Table, Expected: r.Expected, Changes: []core.RowChange{{Column: r.Column, Value: core.Cell{Kind: "binary", Value: base64.StdEncoding.EncodeToString(data)}}}, Confirm: true, TimeoutMS: timeout}
	if err = bulkValidateLimit(core.RowsBulkRequest{Connection: r.Connection, Table: r.Table, Action: "update", Rows: []core.BulkRow{{Expected: r.Expected}}, Changes: request.Changes}, 1<<20+base64.StdEncoding.EncodedLen(core.MaxFileValueBytes)); err != nil {
		return core.MutationResult{}, err
	}
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return core.MutationResult{}, err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return core.MutationResult{}, safe(ctx, err)
	}
	defer conn.Close()
	session := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = session.begin(ctx); err != nil {
		return core.MutationResult{}, safe(ctx, err)
	}
	defer session.rollback()
	d, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return core.MutationResult{}, safe(ctx, err)
	}
	if _, err = fileColumn(d, r.Column); err != nil {
		return core.MutationResult{}, err
	}
	// Lock the row before the second type check; a concurrent ALTER must not turn a file upload into text.
	current, _, err := bulkRead(ctx, session, d, r.Expected)
	if err != nil {
		return core.MutationResult{}, err
	}
	if !reflect.DeepEqual(current, r.Expected) {
		return core.MutationResult{}, rowConflict()
	}
	fresh, err := describe(ctx, session, r.Connection, r.Table)
	if err != nil {
		return core.MutationResult{}, safe(ctx, err)
	}
	if !reflect.DeepEqual(d, fresh) {
		return core.MutationResult{}, rowConflict()
	}
	if _, err = fileColumn(fresh, r.Column); err != nil {
		return core.MutationResult{}, err
	}
	result, err := rowsBulkMutate(ctx, session, request, false)
	if err != nil {
		return core.MutationResult{}, err
	}
	if err = session.commit(ctx); err != nil {
		return core.MutationResult{}, public("outcome_unknown", "Commit-Ergebnis ist unbekannt. Zustand neu laden; nicht blind wiederholen.")
	}
	return result, nil
}
func (e *Engine) FileValueDownload(ctx context.Context, r core.FileValueRequest, start func(string, int64) (io.Writer, error)) error {
	if !validTableRef(r.Table) || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return bulkInvalid()
	}
	timeout := r.TimeoutMS
	if timeout == 0 {
		timeout = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return err
	}
	defer cleanup()
	conn, err := db.Conn(ctx)
	if err != nil {
		return safe(ctx, err)
	}
	defer conn.Close()
	s := &mutationSession{conn: conn, driver: r.Connection.Driver}
	if err = s.begin(ctx); err != nil {
		return safe(ctx, err)
	}
	defer s.rollback()
	d, err := describe(ctx, s, r.Connection, r.Table)
	if err != nil {
		return safe(ctx, err)
	}
	ip, err := insertPlan(ctx, s, r.Connection, d)
	if err != nil {
		return safe(ctx, err)
	}
	if !ip.Eligible {
		return public("mutation_not_supported", "Tabelle ist für Dateiwerte nicht freigegeben.")
	}
	pos, err := fileColumn(d, r.Column)
	if err != nil {
		return err
	}
	current, _, err := bulkRead(ctx, s, d, r.Expected)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, r.Expected) {
		return rowConflict()
	}
	cell := current.Values[pos]
	if cell.Kind != "binary" {
		return public("unsupported_file_value", "Zelle enthält keinen Binärwert.")
	}
	if base64.StdEncoding.DecodedLen(len(cell.Value)) > core.MaxFileValueBytes+2 {
		return public("file_too_large", "Dateiwert überschreitet das Größenlimit.")
	}
	// Decode directly to the response stream; the DB snapshot remains bounded by B2 limits.
	if !r.Preview {
		size := len(cell.Value) / 4 * 3
		if strings.HasSuffix(cell.Value, "==") {
			size -= 2
		} else if strings.HasSuffix(cell.Value, "=") {
			size--
		}
		if size > core.MaxFileValueBytes {
			return public("file_too_large", "Dateiwert überschreitet das Größenlimit.")
		}
		dst, err := start("application/octet-stream", int64(size))
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, base64.NewDecoder(base64.StdEncoding, strings.NewReader(cell.Value)))
		return err
	}
	data, err := base64.StdEncoding.DecodeString(cell.Value)
	if err != nil {
		return public("unsupported_file_value", "Binärwert ist nicht lesbar.")
	}
	preview, err := safeFilePreview(data)
	if err != nil {
		return err
	}
	dst, err := start("image/png", int64(len(preview)))
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, bytes.NewReader(preview))
	return err
}
func safeFilePreview(data []byte) ([]byte, error) {
	invalid := func() ([]byte, error) {
		return nil, public("unsupported_preview", "Vorschau ist nur für begrenzte PNG-, JPEG- und GIF-Bilder verfügbar.")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg" && format != "gif") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 4_000_000 {
		return invalid()
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return invalid()
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil || out.Len() > core.MaxFileValueBytes {
		return invalid()
	}
	return out.Bytes(), nil
}
func (e *Engine) ForeignKeyLookup(ctx context.Context, r core.ForeignKeyLookupRequest) (core.ForeignKeyLookupResult, error) {
	empty := core.ForeignKeyLookupResult{}
	if !validTableRef(r.Table) || r.PageSize < 0 || r.PageSize > 100 || r.Offset < 0 || r.Offset > 1000000 || len(r.Search) > 256 || r.TimeoutMS < 0 || r.TimeoutMS > 60000 {
		return empty, bulkInvalid()
	}
	timeout := r.TimeoutMS
	if timeout == 0 {
		timeout = 10000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	structure, err := e.Structure(ctx, core.DescribeRequest{Connection: r.Connection, Table: r.Table})
	if err != nil {
		return empty, err
	}
	var target *core.TableRef
	valueColumn := ""
	for _, fk := range structure.Constraints {
		if fk.Kind == "foreign_key" && fk.ColumnsKnown && len(fk.Columns) == 1 && fk.Columns[0] == r.Column && fk.ReferencedTable != nil && fk.ReferencedColumnsKnown && len(fk.ReferencedColumns) == 1 {
			if target != nil {
				return empty, public("unsupported_lookup", "Spalte besitzt mehrere Fremdschlüsselziele.")
			}
			t := *fk.ReferencedTable
			target = &t
			valueColumn = fk.ReferencedColumns[0]
		}
	}
	if target == nil {
		return empty, public("unsupported_lookup", "Kein eindeutiger einfacher Fremdschlüssel mit bekanntem Ziel vorhanden.")
	}
	db, cleanup, err := e.open(ctx, r.Connection)
	if err != nil {
		return empty, err
	}
	defer cleanup()
	d, err := describe(ctx, db, r.Connection, *target)
	if err != nil {
		return empty, safe(ctx, err)
	}
	if !d.StableOrder || len(d.PrimaryKey) == 0 || d.Kind != "table" {
		return empty, public("unsupported_lookup", "Fremdschlüsselziel benötigt eine stabile Primärschlüsselordnung.")
	}
	foundValue, foundDisplay := false, false
	for _, c := range d.Columns {
		if c.Name == valueColumn {
			foundValue = true
		}
		if c.Name == r.DisplayColumn {
			foundDisplay = true
		}
	}
	if !foundValue || !foundDisplay {
		return empty, bulkInvalid()
	}
	table, err := dbplan.QualifiedName(r.Connection.Driver, d.Table.Schema, d.Table.Name)
	if err != nil {
		return empty, bulkInvalid()
	}
	v, err := dbplan.QuoteIdentifier(r.Connection.Driver, valueColumn)
	if err != nil {
		return empty, bulkInvalid()
	}
	label, err := dbplan.QuoteIdentifier(r.Connection.Driver, r.DisplayColumn)
	if err != nil {
		return empty, bulkInvalid()
	}
	args := []any{}
	parameter := func(value any) string {
		args = append(args, value)
		if r.Connection.Driver == "postgres" {
			return "$" + strconv.Itoa(len(args))
		}
		return "?"
	}
	query := "SELECT " + v + "," + label + " FROM " + table
	if r.Search != "" {
		cast := "CAST(" + label + " AS TEXT)"
		if r.Connection.Driver == "mysql" {
			cast = "CAST(" + label + " AS CHAR)"
		}
		search := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(r.Search)
		query += " WHERE " + cast + " LIKE " + parameter("%"+search+"%") + " ESCAPE '!'"
	}
	order := []string{}
	for _, key := range d.PrimaryKey {
		q, err := dbplan.QuoteIdentifier(r.Connection.Driver, key)
		if err != nil {
			return empty, bulkInvalid()
		}
		if r.Connection.Driver == "sqlite" {
			q += " COLLATE BINARY"
		}
		order = append(order, q)
	}
	size := r.PageSize
	if size == 0 {
		size = 50
	}
	query += " ORDER BY " + strings.Join(order, ",") + " LIMIT " + parameter(int64(size+1)) + " OFFSET " + parameter(int64(r.Offset))
	var set core.ResultSet
	if r.Connection.Driver == "sqlite" {
		connection, openErr := db.Conn(ctx)
		if openErr != nil {
			return empty, safe(ctx, openErr)
		}
		defer connection.Close()
		set, err = sqliteRead(ctx, connection, query, size+1, args...)
	} else {
		rows, queryErr := db.QueryContext(ctx, query, args...)
		if queryErr != nil {
			return empty, safe(ctx, queryErr)
		}
		defer rows.Close()
		set, _, err = readSet(ctx, rows, size+1, MaxResponseBytes)
	}
	if err != nil {
		return empty, safe(ctx, err)
	}
	if set.Truncated {
		return empty, public("result_too_large", "Auswahlwerte sind zu groß.")
	}
	result := core.ForeignKeyLookupResult{Table: d.Table, ValueColumn: valueColumn, DisplayColumn: r.DisplayColumn, Choices: []core.ForeignKeyChoice{}, HasMore: len(set.Rows) > size}
	for i, row := range set.Rows {
		if i >= size {
			break
		}
		result.Choices = append(result.Choices, core.ForeignKeyChoice{Value: row[0], Display: row[1]})
	}
	return result, nil
}
