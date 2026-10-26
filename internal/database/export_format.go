// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// exportFormat retains only one bounded row and a bounded INSERT batch.
type exportFormat struct {
	out        io.Writer
	r          core.ExportRequest
	tableName  string
	columns    []string
	firstTable bool
	firstRow   bool
	batch      string
	csv        bool
}

func newExportFormat(out io.Writer, r core.ExportRequest) *exportFormat {
	return &exportFormat{out: out, r: r, firstTable: true}
}
func (f *exportFormat) write(s string) error { _, err := io.WriteString(f.out, s); return err }
func (f *exportFormat) json(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.out.Write(b)
	return err
}
func (f *exportFormat) start() error {
	if f.r.Charset == "utf-8-bom" {
		if err := f.write("\xef\xbb\xbf"); err != nil {
			return err
		}
	}
	if isTextExport(f.r.Format) {
		return f.textStart()
	}
	switch f.r.Format {
	case "sql":
		// Fixed header: credentials, hosts, timestamps and filesystem paths never enter it.
		s := "-- DB Manager SQL export 1; same-dialect tables only\n-- Snapshot: per database; no server-wide snapshot. See EXPORT-FORMAT.\n"
		if f.r.Connection.Driver == "postgres" {
			s += "-- Catalog reconstruction: omits sequences, triggers, rules, partitions, ownership and storage.\nSET standard_conforming_strings=on;\nSET TIME ZONE 'UTC';\n"
		}
		if f.r.Connection.Driver == "mysql" {
			// NO_BACKSLASH_ESCAPES on the restoring server would misread backslashes in SHOW CREATE literals.
			s += "SET NAMES utf8mb4;\nSET time_zone='+00:00';\nSET @dbm_export_old_sql_mode=@@SQL_MODE;\nSET SESSION sql_mode=TRIM(BOTH ',' FROM REPLACE(CONCAT(',',@@SQL_MODE,','),',NO_BACKSLASH_ESCAPES,',','));\n"
		}
		if f.r.DisableFK {
			if f.r.Connection.Driver == "sqlite" {
				s += "PRAGMA foreign_keys=OFF;\n"
			} else {
				s += "SET @dbm_export_old_fk=@@FOREIGN_KEY_CHECKS;\nSET FOREIGN_KEY_CHECKS=0;\n"
			}
		}
		if f.r.Transaction {
			s += "BEGIN;\n"
		}
		return f.write(s)
	case "json":
		snapshot := "per_database"
		if f.r.Scope == "query" || f.r.ViewsAsTables {
			snapshot = "read_only_best_effort"
		}
		return f.write(`{"format":"dbmanager-export","version":1,"snapshot":"` + snapshot + `","tables":[`)
	default:
		f.csv = true
	}
	return nil
}
func (f *exportFormat) structure(c core.Connection, t exportTable) error {
	q := qualifiedTable(c.Driver, t.description.Table)
	if c.Driver == "mysql" {
		q = quoteIdentifier(c.Driver, t.description.Table.Name)
	}
	s := ""
	if f.r.Comments {
		s += "-- Catalog table: " + safeExportComment(t.description.Table.Schema+"."+t.description.Table.Name) + "\n"
	}
	if f.r.Drop {
		s += "DROP TABLE IF EXISTS " + q + ";\n"
	}
	def := strings.TrimSpace(t.definition)
	if c.Driver == "postgres" && t.description.Table.Schema != "public" && t.description.Table.Schema != "" {
		s += "CREATE SCHEMA IF NOT EXISTS " + quoteIdentifier(c.Driver, t.description.Table.Schema) + ";\n"
	}
	if f.r.IfNotExists {
		if !strings.HasPrefix(def, "CREATE TABLE ") {
			return invalidExport()
		}
		def = "CREATE TABLE IF NOT EXISTS " + strings.TrimPrefix(def, "CREATE TABLE ")
	}
	s += strings.TrimSuffix(def, ";") + ";\n"
	return f.write(s)
}
func (f *exportFormat) table(c core.Connection, d core.TableDescription) error {
	if isTextExport(f.r.Format) {
		return f.textTable(c, d)
	}
	target := d.Table

	f.tableName = qualifiedTable(c.Driver, target)
	if c.Driver == "mysql" {
		f.tableName = quoteIdentifier(c.Driver, target.Name)
	}
	f.columns = nil
	f.firstRow = true
	cols := []core.Column{}
	for _, col := range d.Columns {
		if (f.r.Format != "sql" && col.Generated != "hidden") || col.Generated == "none" || col.Generated == "identity" {
			f.columns = append(f.columns, quoteIdentifier(c.Driver, col.Name))
			cols = append(cols, core.Column{Name: col.Name, DatabaseType: col.DatabaseType})
		}
	}
	if f.r.Format == "json" {
		if !f.firstTable {
			if err := f.write(","); err != nil {
				return err
			}
		}
		f.firstTable = false
		if err := f.write(`{"database":`); err != nil {
			return err
		}
		if err := f.json(c.Database); err != nil {
			return err
		}
		if err := f.write(`,"table":`); err != nil {
			return err
		}
		if err := f.json(d.Table); err != nil {
			return err
		}
		if err := f.write(`,"columns":`); err != nil {
			return err
		}
		if err := f.json(cols); err != nil {
			return err
		}
		return f.write(`,"rows":[`)
	}
	if f.csv {
		names := make([]string, len(cols)+1)
		names[0] = f.csvValue("_record")
		for i, col := range cols {
			names[i+1] = f.csvValue(col.Name)
		}
		return f.csvRecord(names)
	}
	return nil
}
func (f *exportFormat) row(driver string, row []core.Cell) error {
	if isTextExport(f.r.Format) {
		return f.textRow(row)
	}
	if f.r.Format == "json" {
		if !f.firstRow {
			if err := f.write(","); err != nil {
				return err
			}
		}
		f.firstRow = false
		return f.json(row)
	}
	if f.csv {
		values := make([]string, len(row))
		for i, c := range row {
			switch c.Kind {
			case "null":
				values[i] = "\\N"
			case "binary":
				values[i] = "base64:" + c.Value
			default:
				values[i] = c.Value
				if strings.HasPrefix(values[i], "\\") || strings.HasPrefix(values[i], "base64:") {
					values[i] = "\\" + values[i]
				}
			}
			values[i] = f.csvValue(values[i])
		}
		return f.csvRecord(append([]string{f.csvValue("row")}, values...))
	}
	values := make([]string, len(row))
	for i, c := range row {
		v, err := exportLiteral(driver, c)
		if err != nil {
			return err
		}
		values[i] = v
	}
	tuple := "(" + strings.Join(values, ",") + ")"
	prefix := f.insertPrefix(driver)
	if len(prefix)+len(tuple)+2 > f.r.MaxStatementBytes {
		return public("export_statement_limit", "Eine INSERT-Anweisung überschreitet die gewählte Grenze; Export ist unvollständig.")
	}
	if f.r.Extended {
		if len(prefix)+len(f.batch)+len(tuple)+3 > f.r.MaxStatementBytes {
			if err := f.flushBatch(); err != nil {
				return err
			}
		}
		if f.batch != "" {
			f.batch += ","
		}
		f.batch += tuple
		return nil
	}
	return f.write(f.insertPrefix(driver) + tuple + ";\n")
}
func (f *exportFormat) insertPrefix(driver string) string {
	verb := "INSERT INTO "
	if f.r.DataStatement == "replace" {
		verb = "REPLACE INTO "
	}
	if f.r.DataStatement == "insert-ignore" {
		if driver == "mysql" {
			verb = "INSERT IGNORE INTO "
		} else {
			verb = "INSERT OR IGNORE INTO "
		}
	}
	s := verb + f.tableName + " (" + strings.Join(f.columns, ",") + ")"
	if driver == "postgres" {
		s += " OVERRIDING SYSTEM VALUE"
	}
	return s + " VALUES "
}
func (f *exportFormat) flushBatch() error {
	if f.batch == "" {
		return nil
	}
	err := f.write(f.insertPrefix(f.r.Connection.Driver) + f.batch + ";\n")
	f.batch = ""
	return err
}
func (f *exportFormat) endTable() error {
	if isTextExport(f.r.Format) {
		return f.textEndTable()
	}
	if f.r.Format == "json" {
		return f.write("]}")
	}
	return f.flushBatch()
}
func (f *exportFormat) csvRecord(values []string) error {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
	}
	ending := "\n"
	if f.r.CRLF {
		ending = "\r\n"
	}
	return f.write(strings.Join(quoted, f.r.Delimiter) + ending)
}
func (f *exportFormat) csvValue(s string) string {
	if f.r.Format != "excel-csv" {
		return s
	}
	// Replace controls that common spreadsheet importers reinterpret. Prefix every
	// field, including headers, so whitespace and numeric cells cannot bypass it.
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return '\uFFFD'
		}
		return r
	}, s)
	return "'" + s
}
func (f *exportFormat) finish() error {
	if isTextExport(f.r.Format) {
		return f.textFinish()
	}
	switch f.r.Format {
	case "json":
		return f.write(`],"complete":true}` + "\n")
	case "sql":
		s := ""
		if f.r.Transaction {
			s += "COMMIT;\n"
		}
		if f.r.DisableFK {
			if f.r.Connection.Driver == "sqlite" {
				s += "PRAGMA foreign_keys=ON;\n"
			} else {
				s += "SET FOREIGN_KEY_CHECKS=@dbm_export_old_fk;\n"
			}
		}
		if f.r.Connection.Driver == "mysql" {
			s += "SET SESSION sql_mode=@dbm_export_old_sql_mode;\n"
		}
		return f.write(s + "-- DBMANAGER EXPORT COMPLETE\n")
	}
	if f.csv {
		values := make([]string, len(f.columns)+1)
		values[0] = f.csvValue("complete")
		return f.csvRecord(values)
	}
	return nil
}
func exportLiteral(driver string, c core.Cell) (string, error) {
	switch c.Kind {
	case "null":
		return "NULL", nil
	case "integer", "decimal", "float":
		if !exportNumber.MatchString(c.Value) {
			return "", public("unsupported_export", "Nicht endliche oder unbekannte Zahlenwerte sind nicht exportierbar.")
		}
		v := c.Value
		if c.Kind == "float" && !strings.ContainsAny(v, ".eE") {
			v += ".0"
		}
		return v, nil
	case "boolean":
		if c.Value == "true" {
			return "TRUE", nil
		}
		if c.Value == "false" {
			return "FALSE", nil
		}
		return "", invalidExport()
	case "binary":
		b, err := base64.StdEncoding.DecodeString(c.Value)
		if err != nil {
			return "", invalidExport()
		}
		h := hex.EncodeToString(b)
		if driver == "postgres" {
			return "decode('" + h + "','hex')", nil
		}
		return "X'" + h + "'", nil
	case "text", "timestamp":
		if driver == "mysql" && c.Kind == "text" {
			return "CONVERT(X'" + hex.EncodeToString([]byte(c.Value)) + "' USING utf8mb4)", nil
		}
		if driver == "sqlite" && strings.ContainsRune(c.Value, 0) {
			return "CAST(X'" + hex.EncodeToString([]byte(c.Value)) + "' AS TEXT)", nil
		}
		if driver == "postgres" {
			if strings.ContainsRune(c.Value, 0) {
				return "", invalidExport()
			}
			return "E'" + strings.ReplaceAll(strings.ReplaceAll(c.Value, "\\", "\\\\"), "'", "''") + "'", nil
		}
		if driver == "mysql" { // Timestamp bytes originate in a UTC session, not user SQL.
			return "'" + strings.ReplaceAll(strings.ReplaceAll(c.Value, "\\", "\\\\"), "'", "''") + "'", nil
		}
		return "'" + strings.ReplaceAll(c.Value, "'", "''") + "'", nil
	}
	return "", fmt.Errorf("unsupported cell kind")
}
