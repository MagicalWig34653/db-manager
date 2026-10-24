// SPDX-License-Identifier: AGPL-3.0-only
package dbplan

import (
	"strings"
	"testing"
)

func TestDigestStableAndSensitive(t *testing.T) {
	base, err := Digest("mysql", []string{"DROP TABLE `a`;", "SELECT 1"})
	if err != nil || len(base) != 64 {
		t.Fatalf("digest=%q err=%v", base, err)
	}
	same, _ := Digest("mysql", []string{"  DROP TABLE `a`  ", "SELECT 1;"})
	if same != base {
		t.Fatal("normalization must ignore surrounding whitespace and one trailing semicolon")
	}
	for name, other := range map[string]struct {
		driver string
		stmts  []string
	}{
		"driver": {"postgres", []string{"DROP TABLE `a`", "SELECT 1"}},
		"order":  {"mysql", []string{"SELECT 1", "DROP TABLE `a`"}},
		"split":  {"mysql", []string{"DROP TABLE `a`SELECT 1"}},
		"moved":  {"mysql", []string{"DROP TABLE `a`", "SELEC", "T 1"}},
		"inner":  {"mysql", []string{"DROP TABLE `a` ", "SELECT  1"}},
		"fewer":  {"mysql", []string{"DROP TABLE `a`"}},
	} {
		got, err := Digest(other.driver, other.stmts)
		if err != nil || got == base {
			t.Errorf("%s: digest must differ (err=%v)", name, err)
		}
	}
}

func TestDigestRejectsInvalidInput(t *testing.T) {
	if _, err := Digest("", []string{"SELECT 1"}); err == nil {
		t.Error("empty driver")
	}
	if _, err := Digest("sqlite", nil); err == nil {
		t.Error("empty list")
	}
	if _, err := Digest("sqlite", []string{"SELECT 1", " ; "}); err == nil {
		t.Error("empty statement")
	}
}

func TestNewPlanCopiesAndDefaultsSlices(t *testing.T) {
	stmts := []string{"SELECT 1"}
	plan, err := NewPlan("sqlite", stmts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stmts[0] = "changed"
	if plan.Statements[0] != "SELECT 1" || plan.Warnings == nil || plan.Affected == nil {
		t.Fatalf("%+v", plan)
	}
}

func TestCheck(t *testing.T) {
	plan, _ := NewPlan("postgres", []string{`DROP TABLE "t"`}, nil, nil)
	ok := Confirmation{Action: "drop_table", TargetName: "t"}
	if err := Check(plan, plan.SHA256, ok, "drop_table", "t", true); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		sha     string
		confirm Confirmation
		want    error
	}{
		"plan":    {strings.Repeat("0", 64), ok, ErrPlanMismatch},
		"digest":  {"ABC", ok, ErrInvalidDigest},
		"upper":   {strings.ToUpper(plan.SHA256), ok, ErrInvalidDigest},
		"action":  {plan.SHA256, Confirmation{Action: "truncate", TargetName: "t"}, ErrConfirmAction},
		"target":  {plan.SHA256, Confirmation{Action: "drop_table", TargetName: "T"}, ErrConfirmTarget},
		"missing": {plan.SHA256, Confirmation{Action: "drop_table"}, ErrConfirmTarget},
	}
	for name, c := range cases {
		if err := Check(plan, c.sha, c.confirm, "drop_table", "t", true); err != c.want {
			t.Errorf("%s: got %v want %v", name, err, c.want)
		}
	}
	if err := Check(plan, plan.SHA256, Confirmation{Action: "add_column"}, "add_column", "", false); err != nil {
		t.Errorf("non-destructive needs no target: %v", err)
	}
	if err := Check(plan, plan.SHA256, Confirmation{Action: "drop_table", TargetName: ""}, "drop_table", "", true); err != ErrConfirmTarget {
		t.Errorf("destructive with empty wanted target must fail: %v", err)
	}
}

func TestStatus(t *testing.T) {
	for _, s := range []Status{StatusCompleted, StatusRejected, StatusOutcomeUnknown} {
		if !s.Valid() {
			t.Error(s)
		}
	}
	if Status("ok").Valid() {
		t.Error("unknown status")
	}
}

func TestCapabilities(t *testing.T) {
	c := Capabilities{}
	c.Set("mysql", "rename", Capability{Support: Supported})
	c.Set("sqlite", "rename", Capability{Support: Limited, Reason: "nur Tabellen"})
	c.Set("sqlite", "partition", Capability{Support: Unsupported, Reason: "SQLite kennt keine Partitionen"})
	if !c.Allows("mysql", "rename") || !c.Allows("sqlite", "rename") || c.Allows("sqlite", "partition") {
		t.Fatal("allows")
	}
	if got := c.Get("oracle", "rename"); got.Support != Unsupported || got.Reason == "" {
		t.Fatalf("unknown driver: %+v", got)
	}
	if d := c.Drivers(); len(d) != 2 || d[0] != "mysql" || d[1] != "sqlite" {
		t.Fatalf("%v", d)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("limited without reason must panic")
		}
	}()
	c.Set("mysql", "x", Capability{Support: Limited})
}

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct{ driver, in, want string }{
		{"mysql", "a`b", "`a``b`"},
		{"mysql", `a"b`, "`a\"b`"},
		{"postgres", `a"b`, `"a""b"`},
		{"postgres", "a`b", "\"a`b\""},
		{"sqlite", "my table", `"my table"`},
		{"postgres", "x; DROP TABLE y", `"x; DROP TABLE y"`},
	}
	for _, c := range cases {
		got, err := QuoteIdentifier(c.driver, c.in)
		if err != nil || got != c.want {
			t.Errorf("%v: got %q err=%v", c, got, err)
		}
	}
	if _, err := QuoteIdentifier("mysql", ""); err == nil {
		t.Error("empty")
	}
	if _, err := QuoteIdentifier("mysql", "a\x00b"); err == nil {
		t.Error("NUL")
	}
	if got, _ := QualifiedName("postgres", "public", "t"); got != `"public"."t"` {
		t.Error(got)
	}
	if got, _ := QualifiedName("mysql", "", "t"); got != "`t`" {
		t.Error(got)
	}
}
