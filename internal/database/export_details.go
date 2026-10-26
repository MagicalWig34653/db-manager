// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"regexp"
	"strings"

	"github.com/MagicalWig34653/db-manager/internal/core"
	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// A deliberately small catalog type allowlist. Never splice arbitrary type SQL
// (enum labels, domains, expressions) into a materialized view definition.
var exportViewType = regexp.MustCompile(`(?i)^(?:smallint|integer|int|bigint|tinyint|mediumint|boolean|bool|real|double precision|double|float|text|tinytext|mediumtext|longtext|blob|tinyblob|mediumblob|longblob|bytea|date|datetime|timestamp(?: with(?:out)? time zone)?|time(?: with(?:out)? time zone)?|numeric|decimal|character varying|character|varchar|char|binary|varbinary)(?:\([0-9]{1,4}(?:,[0-9]{1,4})?\))?(?: unsigned)?$`)

func materializedExportDefinition(c core.Connection, d core.TableDescription) (string, error) {
	name, err := dbplan.QualifiedName(c.Driver, d.Table.Schema, d.Table.Name)
	if c.Driver == "mysql" {
		name, err = dbplan.QuoteIdentifier(c.Driver, d.Table.Name)
	}
	if err != nil {
		return "", invalidExport()
	}
	cols := []string{}
	for _, col := range d.Columns {
		if col.Generated == "hidden" {
			continue
		}
		quoted, err := dbplan.QuoteIdentifier(c.Driver, col.Name)
		if err != nil {
			return "", invalidExport()
		}
		typ := col.DatabaseType
		if c.Driver == "sqlite" {
			typ = ""
		} else if !exportViewType.MatchString(typ) {
			return "", public("unsupported_export", "Viewspaltentyp wird für Tabellenmaterialisierung nicht unterstützt.")
		}
		cols = append(cols, quoted+" "+typ)
	}
	if len(cols) == 0 {
		return "", invalidExport()
	}
	return "CREATE TABLE " + name + " (" + strings.Join(cols, ",") + ")", nil
}
