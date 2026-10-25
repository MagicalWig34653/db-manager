// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"strings"
	"testing"
)

// parseEnum inverts the literal list of ENUM('a','b”c') and fails when a quote
// could end a literal early or when text remains outside literals.
func parseEnum(sql, kind string) ([]string, bool) {
	prefix := strings.ToUpper(kind) + "("
	if !strings.HasPrefix(sql, prefix) || !strings.HasSuffix(sql, ")") {
		return nil, false
	}
	body := sql[len(prefix) : len(sql)-1]
	var out []string
	for i := 0; i < len(body); {
		if body[i] != '\'' {
			return nil, false
		}
		i++
		var cur strings.Builder
		closed := false
		for i < len(body) {
			if body[i] == '\'' {
				if i+1 < len(body) && body[i+1] == '\'' {
					cur.WriteByte('\'')
					i += 2
					continue
				}
				closed = true
				i++
				break
			}
			cur.WriteByte(body[i])
			i++
		}
		if !closed {
			return nil, false
		}
		out = append(out, cur.String())
		if i < len(body) {
			if body[i] != ',' {
				return nil, false
			}
			i++
		}
	}
	return out, true
}

func FuzzTypeSQLEnum(f *testing.F) {
	for _, s := range []string{"a", "a'b", "'", "''", "a,b", "\\", "x';DROP TABLE t;--", "ünï", " lead", "tab\t", "`", "\"", "a'", "'a"} {
		f.Add(s, "t2")
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		for _, kind := range []string{"enum", "set"} {
			sql, err := TypeSQL("mysql", ColumnType{Kind: kind, Values: []string{a, b}})
			if err != nil {
				continue
			}
			got, ok := parseEnum(sql, kind)
			if !ok || len(got) != 2 || got[0] != a || got[1] != b {
				t.Fatalf("%s %q %q -> %q parsed %v ok=%v", kind, a, b, sql, got, ok)
			}
		}
	})
}

func TestTypeSQLBoundaries(t *testing.T) {
	bad := []ColumnType{
		{Kind: "text", Length: 0}, {Kind: "text", Length: 1025}, {Kind: "text", Length: -1},
		{Kind: "decimal", Precision: 0}, {Kind: "decimal", Precision: 39}, {Kind: "decimal", Precision: 5, Scale: 6}, {Kind: "decimal", Precision: 5, Scale: -1},
		{Kind: "int64", Length: 3}, {Kind: "boolean", Precision: 1}, {Kind: "date", Values: []string{"a"}},
		{Kind: "enum"}, {Kind: "enum", Values: []string{"a", "a"}}, {Kind: "enum", Values: []string{"a "}}, {Kind: "enum", Values: []string{"a\x00"}},
		{Kind: "set", Values: []string{"a,b"}}, {Kind: "enum", Values: []string{string(make([]byte, 256))}}, {Kind: "text VARCHAR(1)"}, {Kind: "BIGINT"},
		{Kind: "enum", Values: []string{"\xff"}},
	}
	for _, c := range bad {
		for _, d := range []string{"mysql", "postgres", "sqlite"} {
			if s, err := TypeSQL(d, c); err == nil {
				t.Errorf("%s %#v accepted: %s", d, c, s)
			}
		}
	}
	for _, d := range []string{"postgres", "sqlite"} {
		for _, k := range []string{"enum", "set"} {
			if _, err := TypeSQL(d, ColumnType{Kind: k, Values: []string{"a"}}); err == nil {
				t.Errorf("%s accepts %s", d, k)
			}
		}
	}
	if s, err := TypeSQL("mysql", ColumnType{Kind: "text", Length: 1024}); err != nil || s != "VARCHAR(1024)" {
		t.Error(s, err)
	}
	if s, err := TypeSQL("postgres", ColumnType{Kind: "decimal", Precision: 38, Scale: 38}); err != nil || s != "DECIMAL(38,38)" {
		t.Error(s, err)
	}
	if s, _ := TypeSQL("postgres", ColumnType{Kind: "binary"}); s != "BYTEA" {
		t.Error(s)
	}
	if TypeCatalog("oracle") == nil || len(TypeCatalog("oracle")) != 0 {
		t.Error("unknown driver must have an empty catalog")
	}
}
