// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestMigrationListIsOrderedAndComplete(t *testing.T) {
	if len(migrations) == 0 {
		t.Fatal("no migrations")
	}
	for i, m := range migrations {
		if m.Version != i+1 || m.Name == "" || len(m.Statements) == 0 || len(m.Checksum()) != 64 {
			t.Fatalf("migration %d malformed", i)
		}
	}
	if LatestVersion() != len(migrations) {
		t.Fatal("latest version")
	}
}

func TestPlanDetectsPendingMismatchAndNewerSchema(t *testing.T) {
	report, err := plan(map[int]string{})
	if err != nil || len(report.Applied) != len(migrations) || report.Current != 0 {
		t.Fatalf("empty plan %+v %v", report, err)
	}
	full := map[int]string{}
	for _, m := range migrations {
		full[m.Version] = m.Checksum()
	}
	if report, err := plan(full); err != nil || len(report.Applied) != 0 || report.Current != LatestVersion() {
		t.Fatalf("full plan %+v %v", report, err)
	}
	tampered := map[int]string{1: "deadbeef"}
	if _, err := plan(tampered); !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("checksum drift: %v", err)
	}
	newer := map[int]string{LatestVersion() + 1: "x"}
	if _, err := plan(newer); !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("newer schema: %v", err)
	}
}

func TestClassifyMapsDriverErrorsToFixedMessages(t *testing.T) {
	secretHost := "secret-host.internal"
	cases := []struct {
		err  error
		want error
	}{
		{nil, nil},
		{fmt.Errorf("dial tcp %s: connection refused", secretHost), ErrUnavailable},
		{&mysql.MySQLError{Number: 1045, Message: "Access denied for user 'u'@'h' (using password: YES)"}, ErrAuth},
		{&mysql.MySQLError{Number: 1142, Message: "CREATE command denied"}, ErrAuth},
		{&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, ErrTLS},
		{x509.HostnameError{Host: secretHost}, ErrTLS},
		{mysql.ErrNoTLS, ErrTLS},
		{&mysql.MySQLError{Number: 2000, Message: "boom"}, ErrUnavailable},
	}
	for _, tc := range cases {
		got := classify(tc.err)
		if got != tc.want {
			t.Fatalf("classify(%v) = %v want %v", tc.err, got, tc.want)
		}
		if got != nil && (got.Error() == "" || containsAny(got.Error(), secretHost, "password", "Access denied")) {
			t.Fatal("classified error leaks driver text")
		}
	}
	if err := classify(context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("context error not preserved")
	}
}

func containsAny(text string, parts ...string) bool {
	for _, part := range parts {
		for i := 0; i+len(part) <= len(text); i++ {
			if text[i:i+len(part)] == part {
				return true
			}
		}
	}
	return false
}

func TestLockSecondsRoundsUp(t *testing.T) {
	for in, want := range map[time.Duration]int{time.Millisecond: 1, time.Second: 1, 1500 * time.Millisecond: 2, 60 * time.Second: 60} {
		if got := lockSeconds(in); got != want {
			t.Fatalf("lockSeconds(%s) = %d want %d", in, got, want)
		}
	}
}

func TestConfigSecretsAreNotJSONSerializable(t *testing.T) {
	cfg := Config{Host: "h", Password: "sentinel-pw", MigrationPassword: "sentinel-mpw", ServerKey: make([]byte, KeySize)}
	data, err := json.Marshal(cfg)
	if err != nil || strings.Contains(string(data), "sentinel") || strings.Contains(string(data), "ServerKey") {
		t.Fatalf("secret serialized: %s %v", data, err)
	}
}
