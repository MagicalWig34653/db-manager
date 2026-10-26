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
