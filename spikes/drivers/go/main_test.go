// SPDX-License-Identifier: AGPL-3.0-only
package driverspike

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
	"os"
	"testing"
	"time"
)

func TestDrivers(t *testing.T) {
	for _, tc := range []struct{ name, driver, dsn, query, sleep string }{
		{"sqlite", "sqlite", ":memory:", "SELECT 'Grüße', NULL, 9223372036854775807", ""},
		{"mysql", "mysql", os.Getenv("SPIKE_MYSQL8_DSN"), "SELECT 'Grüße', NULL, CAST(18446744073709551615 AS UNSIGNED)", "SELECT SLEEP(10)"},
		{"mariadb", "mysql", os.Getenv("SPIKE_MYSQL_DSN"), "SELECT 'Grüße', NULL, CAST(18446744073709551615 AS UNSIGNED)", "SELECT SLEEP(10)"},
		{"postgres", "pgx", os.Getenv("SPIKE_POSTGRES_DSN"), "SELECT 'Grüße', NULL, 9223372036854775807::bigint", "SELECT pg_sleep(10)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dsn == "" {
				t.Skip("explicit disposable database DSN required")
			}
			db, err := sql.Open(tc.driver, tc.dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				t.Fatal(err)
			}
			var text string
			var nullable any
			var integer string
			if err := db.QueryRowContext(ctx, tc.query).Scan(&text, &nullable, &integer); err != nil {
				t.Fatal(err)
			}
			expectedInteger := "9223372036854775807"
			if tc.driver == "mysql" {
				expectedInteger = "18446744073709551615"
			}
			if text != "Grüße" || nullable != nil || integer != expectedInteger {
				t.Fatalf("invalid values: %q %v %q", text, nullable, integer)
			}
			if tc.sleep != "" {
				deadline, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer stop()
				start := time.Now()
				_, err = db.ExecContext(deadline, tc.sleep)
				if err == nil || time.Since(start) > 2*time.Second {
					t.Fatalf("cancellation failed: %v", err)
				}
				if err := db.PingContext(ctx); err != nil {
					t.Fatalf("pool unavailable after cancellation: %v", err)
				}
			}
		})
	}
}
