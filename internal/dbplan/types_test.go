// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"strings"
	"testing"
)

func TestColumnTypeCatalog(t *testing.T) {
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		catalog := TypeCatalog(driver)
		catalog[0] = "injected"
		if TypeCatalog(driver)[0] != "int64" {
			t.Fatal("mutable catalog")
		}
		for _, typ := range []ColumnType{{Kind: "int64"}, {Kind: "text", Length: 13}, {Kind: "decimal", Precision: 9, Scale: 2}, {Kind: "binary"}, {Kind: "boolean"}, {Kind: "date"}, {Kind: "timestamp"}} {
			if _, err := TypeSQL(driver, typ); err != nil {
				t.Fatal(driver, typ, err)
			}
		}
	}
	for _, typ := range []ColumnType{{Kind: "integer); DROP TABLE x"}, {Kind: "text"}, {Kind: "int64", Length: 1}, {Kind: "decimal", Precision: 2, Scale: 3}, {Kind: "enum", Values: []string{"x", "x"}}, {Kind: "set", Values: []string{"a,b"}}, {Kind: "enum", Values: []string{"x\x00"}}} {
		if _, err := TypeSQL("mysql", typ); err == nil {
			t.Fatalf("accepted %#v", typ)
		}
	}
	typ := ColumnType{Kind: "enum", Values: []string{"a,b", "q'\\;DROP TABLE x"}}
	got, err := TypeSQL("mysql", typ)
	if err != nil || !strings.Contains(got, "q''\\;DROP TABLE x") {
		t.Fatal(got, err)
	}
	if _, err := TypeSQL("postgres", typ); err == nil {
		t.Fatal("postgres enum accepted")
	}
}
