// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func isTextExport(format string) bool {
	switch format {
	case "xml", "yaml", "php-array", "latex", "latex-longtable", "mediawiki", "texy", "codegen":
		return true
	}
	return false
}
func (f *exportFormat) textStart() error {
	switch f.r.Format {
	case "xml":
		return f.write("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<export version=\"1\">\n")
	case "yaml":
		return f.write("{\"format\":\"dbmanager-export\",\"version\":1,\"tables\":[")
	case "php-array":
		return f.write("<?php\nreturn array('tables'=>array(\n")
	case "codegen":
		return f.write("{\"format\":\"dbmanager-codegen-text\",\"version\":1}\n")
	default:
		return nil
	}
}

// XML 1.0 cannot represent every valid UTF-8 string. Only those fields use
// base64; invalid UTF-8 is rejected before encoding rather than replaced.
func xmlExportField(tag, value string) string {
	for _, r := range value {
		if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
			return "<" + tag + " encoding=\"base64\">" + base64.StdEncoding.EncodeToString([]byte(value)) + "</" + tag + ">"
		}
	}
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return "<" + tag + ">" + b.String() + "</" + tag + ">"
}
func phpExportString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	// Hex escapes cannot interpolate PHP variables, terminate a literal or emit
	// raw controls. They preserve UTF-8 bytes without depending on PHP's charset.
	for i := 0; i < len(s); i++ {
		fmt.Fprintf(&b, "\\x%02X", s[i])
	}
	b.WriteByte('"')
	return b.String()
}
func latexExportString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\textbackslash{}`)
		case '{', '}', '$', '&', '#', '%', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '^':
			b.WriteString(`\textasciicircum{}`)
		case '~':
			b.WriteString(`\textasciitilde{}`)
		default:
			if unicode.IsControl(r) {
				fmt.Fprintf(&b, `\textbackslash{}u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
func wikiExportString(s string) string {
	var b strings.Builder
	// Encode every character, including syntax delimiters and line breaks.
	// Controls are displayed as escaped text, never structural newlines.
	for _, r := range s {
		if unicode.IsControl(r) {
			for _, c := range fmt.Sprintf(`\u%04X`, r) {
				fmt.Fprintf(&b, "&#%d;", c)
			}
		} else {
			fmt.Fprintf(&b, "&#%d;", r)
		}
	}
	return b.String()
}
func safeExportComment(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, s)
}
func (f *exportFormat) textTable(c core.Connection, d core.TableDescription) error {
	if !utf8.ValidString(c.Database) || !utf8.ValidString(d.Table.Name) || !utf8.ValidString(d.Table.Schema) {
		return invalidExport()
	}
	f.columns = nil
	f.firstRow = true
	cols := []core.Column{}
	for _, col := range d.Columns {
		if col.Generated != "hidden" {
			if !utf8.ValidString(col.Name) || !utf8.ValidString(col.DatabaseType) {
				return invalidExport()
			}
			f.columns = append(f.columns, col.Name)
			cols = append(cols, core.Column{Name: col.Name, DatabaseType: col.DatabaseType})
		}
	}
	meta := struct {
		Database string        `json:"database"`
		Table    core.TableRef `json:"table"`
		Columns  []core.Column `json:"columns"`
	}{c.Database, d.Table, cols}
	switch f.r.Format {
	case "xml":
		s := "<table>" + xmlExportField("database", c.Database) + xmlExportField("schema", d.Table.Schema) + xmlExportField("name", d.Table.Name) + "<columns>"
		for _, col := range cols {
			s += "<column>" + xmlExportField("name", col.Name) + xmlExportField("database_type", col.DatabaseType) + "</column>"
		}
		return f.write(s + "</columns><rows>\n")
	case "yaml":
		if !f.firstTable {
			if err := f.write(","); err != nil {
				return err
			}
		}
		f.firstTable = false
		b, _ := json.Marshal(meta)
		return f.write(strings.TrimSuffix(string(b), "}") + `,"rows":[`)
	case "php-array":
		b, _ := json.Marshal(meta)
		return f.write("array('metadata'=>" + phpExportString(string(b)) + ",'rows'=>array(\n")
	case "codegen":
		if err := f.json(map[string]any{"table": meta}); err != nil {
			return err
		}
		return f.write("\n")
	default:
		names := append([]string{c.Database + ":" + d.Table.Schema + "." + d.Table.Name}, f.columns...)
		if f.r.Format == "latex" || f.r.Format == "latex-longtable" {
			env := "tabular"
			if f.r.Format == "latex-longtable" {
				env = "longtable"
			}
			if err := f.write("% Table: " + safeExportComment(latexExportString(names[0])) + "\n\\begin{" + env + "}{" + strings.Repeat("l", len(f.columns)) + "}\n"); err != nil {
				return err
			}
			return f.textDisplayRow(f.columns)
		}
		if f.r.Format == "mediawiki" {
			if err := f.write("{| class=\"wikitable\"\n|+ " + wikiExportString(names[0]) + "\n"); err != nil {
				return err
			}
		}
		return f.textDisplayRow(f.columns)
	}
}
func (f *exportFormat) textDisplayRow(values []string) error {
	encoded := make([]string, len(values))
	for i, v := range values {
		if strings.HasPrefix(f.r.Format, "latex") {
			encoded[i] = latexExportString(v)
		} else {
			encoded[i] = wikiExportString(v)
		}
	}
	switch f.r.Format {
	case "latex", "latex-longtable":
		return f.write(strings.Join(encoded, " & ") + " \\\\\n")
	case "mediawiki":
		return f.write("|-\n| " + strings.Join(encoded, " || ") + "\n")
	default:
		return f.write("| " + strings.Join(encoded, " | ") + " |\n")
	}
}
func (f *exportFormat) textRow(row []core.Cell) error {
	for _, c := range row {
		if !utf8.ValidString(c.Value) {
			return public("unsupported_export", "Ungültiges UTF-8 wird nicht umgewandelt.")
		}
	}
	switch f.r.Format {
	case "xml":
		s := "<row>"
		for _, c := range row {
			s += "<cell>" + xmlExportField("kind", c.Kind) + xmlExportField("value", c.Value) + "</cell>"
		}
		return f.write(s + "</row>\n")
	case "yaml":
		if !f.firstRow {
			if err := f.write(","); err != nil {
				return err
			}
		}
		f.firstRow = false
		return f.json(row)
	case "php-array":
		s := "array("
		for _, c := range row {
			s += "array('kind'=>" + phpExportString(c.Kind) + ",'value'=>" + phpExportString(c.Value) + "),"
		}
		return f.write(s + "),\n")
	case "codegen":
		if err := f.json(map[string]any{"row": row}); err != nil {
			return err
		}
		return f.write("\n")
	default:
		values := make([]string, len(row))
		for i, c := range row {
			b, _ := json.Marshal(c)
			values[i] = string(b)
		}
		return f.textDisplayRow(values)
	}
}
func (f *exportFormat) textEndTable() error {
	switch f.r.Format {
	case "xml":
		return f.write("</rows></table>\n")
	case "yaml":
		return f.write("]}")
	case "php-array":
		return f.write(")),\n")
	case "latex":
		return f.write("\\end{tabular}\n")
	case "latex-longtable":
		return f.write("\\end{longtable}\n")
	case "mediawiki":
		return f.write("|}\n")
	case "texy":
		return f.write("\n")
	default:
		return nil
	}
}
func (f *exportFormat) textFinish() error {
	switch f.r.Format {
	case "xml":
		return f.write("<complete>true</complete></export>\n")
	case "yaml":
		return f.write("],\"complete\":true}\n")
	case "php-array":
		return f.write("),'complete'=>true);\n")
	case "latex", "latex-longtable":
		return f.write("% DBMANAGER EXPORT COMPLETE\n")
	case "mediawiki", "texy":
		return f.write("<!-- DBMANAGER EXPORT COMPLETE -->\n")
	default:
		return f.write("{\"complete\":true}\n")
	}
}
