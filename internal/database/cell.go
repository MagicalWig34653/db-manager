// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"encoding/base64"
	"github.com/MagicalWig34653/db-manager/internal/core"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func cell(value any, databaseType string) (core.Cell, error) {
	c := core.Cell{Kind: "text"}
	typ := strings.ToUpper(databaseType)
	switch v := value.(type) {
	case nil:
		return core.Cell{Kind: "null"}, nil
	case int64:
		c.Kind = "integer"
		c.Value = strconv.FormatInt(v, 10)
	case uint64:
		c.Kind = "integer"
		c.Value = strconv.FormatUint(v, 10)
	case float64:
		c.Kind = "float"
		c.Value = strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		c.Kind = "boolean"
		c.Value = strconv.FormatBool(v)
	case time.Time:
		c.Kind = "timestamp"
		switch typ {
		case "DATE":
			c.Value = v.Format("2006-01-02")
		case "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE":
			c.Value = v.Format("2006-01-02T15:04:05.999999999")
		default:
			c.Value = v.Format(time.RFC3339Nano)
		}
	case []byte:
		if len(v) > MaxCellBytes {
			return c, public("result_too_large", "Ergebniszelle überschreitet die erlaubte Größe.")
		}
		if isBinary(typ) || typ == "" || !utf8.Valid(v) {
			c.Kind = "binary"
			c.Value = base64.StdEncoding.EncodeToString(v)
			return c, nil
		}
		c.Value = string(v)
	case string:
		c.Value = v
	default:
		return c, public("result_error", "Nicht unterstützter Ergebnisdatentyp.")
	}
	if len(c.Value) > MaxCellBytes {
		return c, public("result_too_large", "Ergebniszelle überschreitet die erlaubte Größe.")
	}
	if !utf8.ValidString(c.Value) {
		c.Kind = "binary"
		c.Value = base64.StdEncoding.EncodeToString([]byte(c.Value))
		return c, nil
	}
	if c.Kind == "text" {
		switch {
		case integerType(typ):
			c.Kind = "integer"
		case typ == "NUMERIC" || typ == "DECIMAL":
			c.Kind = "decimal"
		case strings.Contains(typ, "TIMESTAMP") || typ == "DATETIME" || typ == "DATE" || typ == "TIME" || typ == "TIMETZ":
			c.Kind = "timestamp"
		}
	}
	return c, nil
}
func isBinary(typ string) bool {
	return strings.Contains(typ, "BLOB") || strings.Contains(typ, "BINARY") || typ == "BYTEA" || typ == "BIT" || typ == "VARBIT"
}

func integerType(typ string) bool {
	switch strings.TrimPrefix(typ, "UNSIGNED ") {
	case "INT", "INT2", "INT4", "INT8", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "MEDIUMINT":
		return true
	}
	return false
}
