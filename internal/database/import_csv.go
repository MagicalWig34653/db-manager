// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// csvImportReader parses one bounded record at a time, preserving quoted newlines.
// Quote escaping is either doubled quotes or an explicit backslash escape.
type csvImportReader struct {
	text             string
	pos              int
	separator, quote rune
	backslash        bool
}

func newCSVImportReader(text string, o core.CSVImportOptions) (*csvImportReader, error) {
	sep, quote := ',', '"'
	if o.Separator != "" {
		if utf8.RuneCountInString(o.Separator) != 1 {
			return nil, invalidImport()
		}
		sep, _ = utf8.DecodeRuneInString(o.Separator)
	}
	if o.Quote != "" {
		if utf8.RuneCountInString(o.Quote) != 1 {
			return nil, invalidImport()
		}
		quote, _ = utf8.DecodeRuneInString(o.Quote)
	}
	if sep == quote || sep == '\n' || sep == '\r' || sep == 0 || sep == '\\' || quote == '\n' || quote == '\r' || quote == 0 || quote == '\\' || (o.Escape != "" && o.Escape != "double" && o.Escape != "backslash") {
		return nil, invalidImport()
	}
	return &csvImportReader{text: text, separator: sep, quote: quote, backslash: o.Escape == "backslash"}, nil
}
func (r *csvImportReader) read(ctx context.Context) ([]string, error) {
	if r.pos == len(r.text) {
		return nil, io.EOF
	}
	values := []string{}
	rowBytes := 0
	var b strings.Builder
	quoted, closed, started := false, false, false
	appendValue := func() error {
		if len(values) >= 1024 || b.Len() > MaxCellBytes {
			return invalidImport()
		}
		rowBytes += b.Len()
		if rowBytes > 4<<20 {
			return invalidImport()
		}
		values = append(values, b.String())
		b.Reset()
		closed = false
		started = false
		return nil
	}
	for r.pos < len(r.text) {
		if r.pos%4096 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c, n := utf8.DecodeRuneInString(r.text[r.pos:])
		r.pos += n
		if quoted {
			if c == r.quote {
				next, m := utf8.DecodeRuneInString(r.text[r.pos:])
				if !r.backslash && next == r.quote {
					b.WriteRune(next)
					r.pos += m
				} else {
					quoted = false
					closed = true
				}
			} else if c == '\\' && r.backslash {
				if r.pos == len(r.text) {
					return nil, invalidImport()
				}
				next, m := utf8.DecodeRuneInString(r.text[r.pos:])
				r.pos += m
				if next != r.quote && next != '\\' {
					b.WriteRune('\\')
				}
				b.WriteRune(next)
			} else {
				b.WriteRune(c)
			}
		} else {
			switch {
			case c == r.separator:
				if err := appendValue(); err != nil {
					return nil, err
				}
			case c == '\n' || c == '\r':
				if c == '\r' && r.pos < len(r.text) && r.text[r.pos] == '\n' {
					r.pos++
				}
				if err := appendValue(); err != nil {
					return nil, err
				}
				return values, nil
			case closed:
				return nil, invalidImport()
			case c == r.quote:
				if started {
					return nil, invalidImport()
				}
				quoted = true
				started = true
			default:
				b.WriteRune(c)
				started = true
			}
		}
		if b.Len() > MaxCellBytes {
			return nil, invalidImport()
		}
	}
	if quoted {
		return nil, invalidImport()
	}
	if err := appendValue(); err != nil {
		return nil, err
	}
	return values, nil
}

// csvImportCell deliberately supports a conservative portable type subset.
// Unsupported types fail before writes instead of relying on lossy coercions.
func csvImportCell(value string, col core.TableColumn, o core.CSVImportOptions, driver string) (core.Cell, error) {
	if o.Null != nil && value == *o.Null {
		if !col.Nullable {
			return core.Cell{}, fmt.Errorf("null_not_allowed")
		}
		return core.Cell{Kind: "null"}, nil
	}
	t := strings.ToUpper(col.DatabaseType)
	base := strings.Fields(strings.Split(t, "(")[0])
	if len(base) == 0 {
		base = []string{"TEXT"}
	}
	kind := base[0]
	switch {
	case kind == "INT" || kind == "INTEGER" || kind == "TINYINT" || kind == "SMALLINT" || kind == "MEDIUMINT" || kind == "BIGINT" || kind == "INT2" || kind == "INT4" || kind == "INT8":
		bits := 64
		if driver != "sqlite" {
			switch kind {
			case "TINYINT":
				bits = 8
			case "SMALLINT", "INT2":
				bits = 16
			case "MEDIUMINT":
				bits = 24
			case "INT", "INTEGER", "INT4":
				bits = 32
			}
		}
		if strings.Contains(t, "UNSIGNED") {
			if _, err := strconv.ParseUint(value, 10, bits); err != nil {
				return core.Cell{}, fmt.Errorf("invalid_integer")
			}
		} else if _, err := strconv.ParseInt(value, 10, bits); err != nil {
			return core.Cell{}, fmt.Errorf("invalid_integer")
		}
		return core.Cell{Kind: "integer", Value: value}, nil
	case kind == "REAL" || kind == "DOUBLE" || kind == "FLOAT":
		bits := 64
		if driver != "sqlite" && (kind == "REAL" || kind == "FLOAT") {
			bits = 32
		}
		f, err := strconv.ParseFloat(value, bits)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return core.Cell{}, fmt.Errorf("invalid_real")
		}
		return core.Cell{Kind: "float", Value: value}, nil
	case kind == "NUMERIC" || kind == "DECIMAL":
		if !decimalValue.MatchString(value) || !importDecimalFits(value, t) {
			return core.Cell{}, fmt.Errorf("invalid_decimal")
		}
		return core.Cell{Kind: "decimal", Value: value}, nil
	case kind == "BOOL" || kind == "BOOLEAN":
		if value != "true" && value != "false" {
			return core.Cell{}, fmt.Errorf("invalid_boolean")
		}
		return core.Cell{Kind: "boolean", Value: value}, nil
	case kind == "TEXT" || kind == "VARCHAR" || kind == "CHAR" || kind == "CHARACTER" || kind == "CLOB" || kind == "TINYTEXT" || kind == "MEDIUMTEXT" || kind == "LONGTEXT":
		if n, _, ok := importTypeSize(t); ok && utf8.RuneCountInString(value) > n {
			return core.Cell{}, fmt.Errorf("text_too_long")
		}
		return core.Cell{Kind: "text", Value: value}, nil
	default:
		return core.Cell{}, fmt.Errorf("unsupported_type")
	}
}

func importTypeSize(t string) (int, int, bool) {
	start := strings.IndexByte(t, '(')
	if start < 0 {
		return 0, 0, false
	}
	end := strings.IndexByte(t[start:], ')')
	if end < 0 {
		return 0, 0, false
	}
	parts := strings.Split(t[start+1:start+end], ",")
	if len(parts) > 2 {
		return 0, 0, false
	}
	precision, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || precision < 1 {
		return 0, 0, false
	}
	scale := 0
	if len(parts) == 2 {
		scale, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || scale < 0 || scale > precision {
			return 0, 0, false
		}
	}
	return precision, scale, true
}
func importDecimalFits(value, t string) bool {
	precision, scale, ok := importTypeSize(t)
	if !ok {
		return !strings.Contains(t, "(")
	}
	parts := strings.Split(strings.TrimPrefix(value, "-"), ".")
	integerDigits := len(strings.TrimLeft(parts[0], "0"))
	fractionDigits := 0
	if len(parts) == 2 {
		fractionDigits = len(parts[1])
	}
	return integerDigits <= precision-scale && fractionDigits <= scale
}
