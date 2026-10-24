// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// referenceQuote is a copy of internal/database.quoteIdentifier (the dbplan
// package must not import database); the fuzz target pins both to one rule.
func referenceQuote(driver, value string) string {
	if driver == "mysql" {
		return "`" + strings.ReplaceAll(value, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// unquote inverts the quoting and fails if the quoted text could end early,
// i.e. if an unescaped quote character appears inside.
func unquote(driver, quoted string) (string, bool) {
	q := `"`
	if driver == "mysql" {
		q = "`"
	}
	if len(quoted) < 2 || !strings.HasPrefix(quoted, q) || !strings.HasSuffix(quoted, q) {
		return "", false
	}
	inner := quoted[1 : len(quoted)-1]
	rest := inner
	for {
		i := strings.Index(rest, q)
		if i < 0 {
			break
		}
		if !strings.HasPrefix(rest[i:], q+q) {
			return "", false
		}
		rest = rest[i+2:]
	}
	return strings.ReplaceAll(inner, q+q, q), true
}

func FuzzQuoteIdentifier(f *testing.F) {
	for _, seed := range []string{"a", "a`b", `a"b`, "`", `"`, "``", "äé\U0001F600", "a;--", strings.Repeat("x", 70000), "a\x00b", "\xff\xfe"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		for _, driver := range []string{"mysql", "postgres", "sqlite"} {
			got, err := QuoteIdentifier(driver, value)
			if value == "" || strings.ContainsRune(value, 0) {
				if err == nil {
					t.Fatalf("%s %q must be rejected", driver, value)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s %q: %v", driver, value, err)
			}
			if want := referenceQuote(driver, value); got != want {
				t.Fatalf("%s %q: diverges from database.quoteIdentifier: %q vs %q", driver, value, got, want)
			}
			back, ok := unquote(driver, got)
			if !ok || back != value {
				t.Fatalf("%s %q: quoting not a closed single identifier: %q", driver, value, got)
			}
			if utf8.ValidString(value) && !utf8.ValidString(got) {
				t.Fatalf("%s %q: result not valid UTF-8", driver, value)
			}
		}
	})
}

func TestQuoteIdentifierEdgeTable(t *testing.T) {
	long := strings.Repeat("é", 100000)
	cases := []struct{ driver, in, want string }{
		{"mysql", "`", "````"},
		{"mysql", "``", "``````"},
		{"postgres", `"`, `""""`},
		{"sqlite", `""`, `""""""`},
		{"mysql", "tabä\U0001F600", "`tabä\U0001F600`"},
		{"postgres", " ", `" "`},
		{"mysql", long, "`" + long + "`"},
		{"mariadb", "a`b", "\"a`b\""}, // documented: only the driver id "mysql" (also MariaDB) selects backticks
	}
	for _, c := range cases {
		got, err := QuoteIdentifier(c.driver, c.in)
		if err != nil || got != c.want {
			t.Errorf("%s %.20q: got %.30q err=%v", c.driver, c.in, got, err)
		}
	}
	if _, err := QualifiedName("mysql", "s\x00", "t"); err == nil {
		t.Error("NUL in schema")
	}
	if _, err := QualifiedName("mysql", "s", ""); err == nil {
		t.Error("empty name")
	}
}

func TestDigestCollisionResistance(t *testing.T) {
	seen := map[string][]string{}
	add := func(driver string, stmts ...string) {
		sum, err := Digest(driver, stmts)
		if err != nil {
			t.Fatal(err)
		}
		if prev, dup := seen[sum]; dup {
			t.Fatalf("collision: %q vs %q/%q", prev, driver, stmts)
		}
		seen[sum] = append([]string{driver}, stmts...)
	}
	add("mysql", "ab", "c")
	add("mysql", "a", "bc")
	add("mysql", "abc")
	add("mysql", "a", "b", "c")
	add("mysqla", "bc")
	add("mysql", "abc", "d")
	add("mysql", "abcd")
	add("mysql", "a\x00b")
	add("mysql", "a", "b")
	add("mysql", strings.Repeat("a", 127), "b")
	add("mysql", strings.Repeat("a", 128), "b")
	add("mysql", strings.Repeat("a", 127)+"b")
	// A statement that embeds a varint-looking prefix cannot shift boundaries.
	add("mysql", "a\x01b")
	add("mysql", "a", "\x01b")
	// Semicolon normalization is deliberate: only one trailing semicolon collapses.
	one, _ := Digest("mysql", []string{"x;"})
	two, _ := Digest("mysql", []string{"x;;"})
	plain, _ := Digest("mysql", []string{"x"})
	if one != plain || two == plain {
		t.Error("semicolon normalization")
	}
}

func TestCheckCannotBeBypassed(t *testing.T) {
	plan, _ := NewPlan("mysql", []string{"DROP TABLE `t`"}, nil, nil)
	// Tampered plan struct: statements changed but stale digest copied by a caller must not match a fresh plan.
	fresh, _ := NewPlan("mysql", []string{"DROP TABLE `t`", "DROP TABLE `u`"}, nil, nil)
	if err := Check(fresh, plan.SHA256, Confirmation{Action: "drop", TargetName: "t"}, "drop", "t", true); err != ErrPlanMismatch {
		t.Fatalf("stale digest accepted: %v", err)
	}
	for _, bad := range []string{"", " " + plan.SHA256[1:], plan.SHA256 + "0", plan.SHA256[:63], strings.Repeat("g", 64), plan.SHA256[:63] + "é"} {
		if err := Check(plan, bad, Confirmation{Action: "drop", TargetName: "t"}, "drop", "t", true); err == nil {
			t.Errorf("digest %q accepted", bad)
		}
	}
	// Destructive requires target even when the caller passes the empty wanted name.
	if err := Check(plan, plan.SHA256, Confirmation{Action: "drop"}, "drop", "", true); err != ErrConfirmTarget {
		t.Fatal("empty wanted target must fail")
	}
	// Whitespace/case variants of the typed name are rejected.
	for _, typed := range []string{"t ", " t", "T", "t\x00"} {
		if err := Check(plan, plan.SHA256, Confirmation{Action: "drop", TargetName: typed}, "drop", "t", true); err != ErrConfirmTarget {
			t.Errorf("typed %q accepted", typed)
		}
	}
}
