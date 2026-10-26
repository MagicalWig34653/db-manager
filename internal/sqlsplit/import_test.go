// SPDX-License-Identifier: AGPL-3.0-only
package sqlsplit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestImportLimitsKeepLegacyLimits(t *testing.T) {
	input := strings.Repeat("SELECT 1;", 101)
	if _, err := Split(input, "", "sqlite"); err == nil {
		t.Fatal("legacy statement limit changed")
	}
	if out, err := SplitLimited(input, "", "sqlite", 64<<20, 10000); err != nil || len(out) != 101 {
		t.Fatal(len(out), err)
	}
	if _, err := SplitLimited(input, "", "sqlite", 64<<20, 100); err == nil {
		t.Fatal("import statement limit ignored")
	}
	if _, err := SplitLimited(input, "", "sqlite", 1, 10000); err == nil {
		t.Fatal("byte limit ignored")
	}
}
func TestImportLexerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SplitLimitedContext(ctx, "SELECT 1", "", "sqlite", 64<<20, 10000); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled parse", err)
	}
}
