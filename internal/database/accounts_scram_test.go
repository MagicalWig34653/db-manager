// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"strings"
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

func TestScramVerifierFormatAndDeterminism(t *testing.T) {
	a, err := scramVerifierWithSalt("pencil", []byte("0123456789abcdef"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := scramVerifierWithSalt("pencil", []byte("0123456789abcdef"), 4096)
	if a != b || !strings.HasPrefix(a, "SCRAM-SHA-256$4096:MDEyMzQ1Njc4OWFiY2RlZg==$") || strings.Count(a, ":") != 2 {
		t.Fatalf("verifier %q", a)
	}
	c, _ := scramVerifier("pencil")
	if !strings.HasPrefix(c, "SCRAM-SHA-256$4096:") {
		t.Fatalf("iteration count changed: %q", c)
	}
	d, _ := scramVerifier("pencil")
	if c == d || strings.Contains(c, "pencil") {
		t.Fatal("salt must differ and the verifier must not contain the password")
	}
}

func TestSystemAccountsAreProtected(t *testing.T) {
	for _, test := range []struct {
		driver string
		a      core.AccountRef
		prot   bool
		second bool
	}{
		{"mysql", core.AccountRef{Name: "mysql.sys", Host: "localhost"}, true, false},
		{"mysql", core.AccountRef{Name: "MariaDB.sys", Host: "localhost"}, true, false},
		{"mysql", core.AccountRef{Name: "root", Host: "%"}, false, true},
		{"mysql", core.AccountRef{Name: "app", Host: "%"}, false, false},
		{"postgres", core.AccountRef{Name: "pg_monitor"}, true, false},
		{"postgres", core.AccountRef{Name: "postgres"}, false, true},
		{"postgres", core.AccountRef{Name: "appuser"}, false, false},
	} {
		if p, s := accountSystem(test.driver, test.a); p != test.prot || s != test.second {
			t.Fatalf("%s %v: %v %v", test.driver, test.a, p, s)
		}
	}
}

func TestPostgresSecretStatementNeverContainsClearText(t *testing.T) {
	for _, secret := range []string{"plain-pw", "ﬁx①ｗpw", "caf\u0065\u0301-pw", "it's \"quoted\""} {
		statement, err := accountSecretStatement("postgres", `CREATE ROLE "x" LOGIN PASSWORD <protected-secret>`, secret)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(statement, secret) || !strings.Contains(statement, "PASSWORD 'SCRAM-SHA-256$4096:") {
			t.Fatalf("clear text or wrong form for %q: %s", secret, statement)
		}
	}
	mysqlStatement, _ := accountSecretStatement("mysql", "CREATE USER 'x'@'%' IDENTIFIED BY <protected-secret>", "pw'x")
	if !strings.HasSuffix(mysqlStatement, "'pw''x'") {
		t.Fatalf("mysql literal %s", mysqlStatement)
	}
}

func TestSASLPrepNormalizesLikePostgres(t *testing.T) {
	for in, want := range map[string]string{"plain": "plain", "ﬁx①ｗpw": "fix1wpw", "caf\u0065\u0301-pw": "caf\u00e9-pw", "a\u00a0b": "a b", "a\u00adb": "ab", "bad\x07ctl\u00e9": "bad\x07ctl\u00e9"} {
		if got := saslPrep(in); got != want {
			t.Fatalf("saslPrep(%q) = %q want %q", in, got, want)
		}
	}
}
