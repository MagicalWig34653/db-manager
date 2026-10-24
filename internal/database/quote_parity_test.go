// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"testing"

	"github.com/MagicalWig34653/db-manager/internal/dbplan"
)

// TestQuoteIdentifierParityWithDbplan keeps the two implementations identical.
func TestQuoteIdentifierParityWithDbplan(t *testing.T) {
	for _, driver := range []string{"mysql", "postgres", "sqlite", "other"} {
		for _, name := range []string{"a", "a`b", `a"b`, "`", `"`, "``", `""`, "ä😀", " ", "x; DROP TABLE y", "line\nbreak"} {
			got, err := dbplan.QuoteIdentifier(driver, name)
			if err != nil || got != quoteIdentifier(driver, name) {
				t.Errorf("%s %q: dbplan %q (%v) vs database %q", driver, name, got, err, quoteIdentifier(driver, name))
			}
		}
	}
}
