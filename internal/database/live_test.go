// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// TestLiveDrivers requires exclusively owned disposable fixtures from test-engine.py.
func TestLiveDrivers(t *testing.T) {
	for _, test := range []struct{ name, driver, port string }{{"mysql", "mysql", "DBM_TEST_MYSQL_PORT"}, {"maria", "mysql", "DBM_TEST_MARIA_PORT"}, {"postgres", "postgres", "DBM_TEST_POSTGRES_PORT"}} {
		t.Run(test.name, func(t *testing.T) {
			raw := os.Getenv(test.port)
			if raw == "" {
				t.Skip("disposable fixture port required")
			}
			port, err := strconv.Atoi(raw)
			if err != nil {
				t.Fatal(err)
			}
			e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
			if err != nil {
				t.Fatal(err)
			}
			c := core.Connection{Driver: test.driver, Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
			if test.driver == "postgres" {
				c.Username = "postgres"
			}
			untrusted := testEngine(t)
			if err := untrusted.Test(context.Background(), c); err == nil {
				t.Fatal("untrusted CA accepted")
			}
			if err := e.Test(context.Background(), c); err != nil {
				t.Fatalf("trusted TLS connection: %v", err)
			}
			wrongHost := c
			wrongHost.Host = "127.0.0.1"
			if err := e.Test(context.Background(), wrongHost); err == nil {
				t.Fatal("wrong TLS hostname accepted")
			}
			query := "SELECT NULL, '', X'00FF', CAST('12345678901234567890.12345678901234567890' AS DECIMAL(40,20)), CAST(18446744073709551615 AS UNSIGNED), TIMESTAMP('2026-01-02 03:04:05.123456')"
			if test.driver == "postgres" {
				query = "SELECT NULL, ''::text, decode('00ff','hex'), 12345678901234567890.12345678901234567890::numeric, 9223372036854775807::bigint, '2026-01-02 03:04:05.123456+02'::timestamptz"
			}
			r := execute(t, e, c, query)
			row := r.Results[0].Rows[0]
			if row[0].Kind != "null" || row[1].Kind != "text" || row[1].Value != "" || row[2].Kind != "binary" || row[2].Value != "AP8=" || row[3].Kind != "decimal" || row[3].Value != "12345678901234567890.12345678901234567890" || row[4].Kind != "integer" || row[5].Kind != "timestamp" {
				t.Fatalf("typed roundtrip %#v", row)
			}
			wantInteger := "18446744073709551615"
			if test.driver == "postgres" {
				wantInteger = "9223372036854775807"
			}
			if row[4].Value != wantInteger {
				t.Fatalf("integer %s", row[4].Value)
			}
			name := "`b1 odd'name`"
			if test.driver == "postgres" {
				name = `"b1 odd'name"`
			}
			execute(t, e, c, "CREATE TABLE "+name+" (value TEXT)")
			execute(t, e, c, "INSERT INTO "+name+" VALUES ('persisted')")
			objects, err := e.Catalog(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, object := range objects {
				if object.Name == "b1 odd'name" {
					found = true
				}
			}
			if !found {
				t.Fatalf("catalog %#v", objects)
			}
			if _, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "INSERT INTO " + name + " VALUES ('batch'); SELECT 1"}); err == nil {
				t.Fatal("batch accepted")
			}
			r = execute(t, e, c, "SELECT COUNT(*) FROM "+name)
			if r.Results[0].Rows[0][0].Value != "1" {
				t.Fatal("rejected batch changed database")
			}
			limited, err := e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: "SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3", MaxRows: 1})
			if err != nil || !limited.Results[0].Truncated || len(limited.Results[0].Rows) != 1 {
				t.Fatalf("row limit %#v %v", limited, err)
			}
			sleep := "SELECT SLEEP(10)"
			if test.driver == "postgres" {
				sleep = "SELECT pg_sleep(10)"
			}
			start := time.Now()
			_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: sleep, TimeoutMS: 30})
			if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
				t.Fatalf("deadline: %v", err)
			}
			execute(t, e, c, "SELECT 1")
			huge := "SELECT REPEAT('x',1048577)"
			if test.driver == "postgres" {
				huge = "SELECT repeat('x',1048577)"
			}
			_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: huge})
			var public *core.Error
			if !errors.As(err, &public) || public.Code != "result_too_large" {
				t.Fatalf("cell size: %v", err)
			}
			total := "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<9) SELECT REPEAT('x',1048576) FROM n"
			if test.driver == "postgres" {
				total = "SELECT repeat('x',1048576) FROM generate_series(1,9)"
			}
			_, err = e.Query(context.Background(), core.QueryRequest{Connection: c, SQL: total})
			if !errors.As(err, &public) || public.Code != "result_too_large" {
				t.Fatalf("response size: %v", err)
			}
			bad := c
			bad.Password = "sensitive-never-return"
			err = e.Test(context.Background(), bad)
			if err == nil || strings.Contains(err.Error(), bad.Password) {
				t.Fatalf("unsafe auth error: %v", err)
			}
		})
	}
	if raw := os.Getenv("DBM_TEST_NO_TLS_PORT"); raw != "" {
		port, _ := strconv.Atoi(raw)
		e, err := New(t.TempDir(), nil, WithCAFile(os.Getenv("DBM_TEST_CA_FILE")))
		if err != nil {
			t.Fatal(err)
		}
		c := core.Connection{Driver: "mysql", Host: "localhost", Port: port, Database: "probe", Username: "root", Password: "public-b1-fixture"}
		if err := e.Test(context.Background(), c); err == nil {
			t.Fatal("TLS downgrade accepted")
		}
		c.TLSMode = "disable"
		if err := e.Test(context.Background(), c); err != nil {
			t.Fatalf("explicit development plaintext: %v", err)
		}
	}
}
