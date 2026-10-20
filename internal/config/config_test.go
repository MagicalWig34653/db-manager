// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenFileRequiredAndValidated(t *testing.T) {
	t.Setenv("DBM_TOKEN_FILE", "")
	if _, err := Load(); err == nil {
		t.Fatal("missing token allowed")
	}
	path := filepath.Join(t.TempDir(), "token")
	t.Setenv("DBM_TOKEN_FILE", path)
	if err := os.WriteFile(path, []byte("short"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("weak token allowed")
	}
	token := strings.Repeat("x", 32)
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil || cfg.Token != token {
		t.Fatalf("token newline handling failed: %v", err)
	}
}
