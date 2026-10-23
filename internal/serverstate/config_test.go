// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSecret(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validEnv(t *testing.T) map[string]string {
	dir := t.TempDir()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", KeySize)))
	return map[string]string{
		EnvHost:          "db.internal.example",
		EnvName:          "dbmanager_state",
		EnvUser:          "dbm_runtime",
		EnvPasswordFile:  writeSecret(t, dir, "pw", "sentinel-runtime-password\n"),
		EnvServerKeyFile: writeSecret(t, dir, "key", key+"\n"),
	}
}

func getter(env map[string]string) func(string) string {
	return func(name string) string { return env[name] }
}

func TestLoadConfigDisabledWithoutVariables(t *testing.T) {
	cfg, err := LoadConfigFrom(getter(nil))
	if err != nil || cfg.Enabled() {
		t.Fatalf("disabled config: %v enabled=%v", err, cfg.Enabled())
	}
}

func TestLoadConfigPartialIsError(t *testing.T) {
	for _, name := range allStateEnvs[1:] {
		if _, err := LoadConfigFrom(getter(map[string]string{name: "x"})); err == nil {
			t.Fatalf("%s without host accepted", name)
		}
	}
}

func TestLoadConfigValidAndRedacted(t *testing.T) {
	cfg, err := LoadConfigFrom(getter(validEnv(t)))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled() || cfg.Port != 3306 || cfg.TLSDisabled || cfg.Password != "sentinel-runtime-password" || len(cfg.ServerKey) != KeySize {
		t.Fatalf("unexpected config %+v", cfg.String())
	}
	for _, text := range []string{fmt.Sprint(cfg), fmt.Sprintf("%v %+v %#v %s", cfg, cfg, cfg, cfg)} {
		if strings.Contains(text, "sentinel-runtime-password") || strings.Contains(text, "kkkk") {
			t.Fatal("secret leaked through formatting")
		}
	}
}

func TestLoadConfigRejectsInvalid(t *testing.T) {
	mutations := map[string]func(map[string]string){
		"bad host":          func(e map[string]string) { e[EnvHost] = "db/../x" },
		"unix socket":       func(e map[string]string) { e[EnvHost] = "/var/run/mysqld.sock" },
		"bad port":          func(e map[string]string) { e[EnvPort] = "70000" },
		"bad database":      func(e map[string]string) { e[EnvName] = "a;drop" },
		"missing user":      func(e map[string]string) { delete(e, EnvUser) },
		"missing password":  func(e map[string]string) { delete(e, EnvPasswordFile) },
		"unreadable secret": func(e map[string]string) { e[EnvPasswordFile] = "/nonexistent/secret" },
		"missing key":       func(e map[string]string) { delete(e, EnvServerKeyFile) },
		"bad tls":           func(e map[string]string) { e[EnvTLS] = "prefer" },
		"ca with disable":   func(e map[string]string) { e[EnvTLS] = TLSDisable; e[EnvCAFile] = e[EnvPasswordFile] },
		"invalid ca":        func(e map[string]string) { e[EnvCAFile] = e[EnvPasswordFile] },
		"migrate half":      func(e map[string]string) { e[EnvMigrateUser] = "dbm_migrate" },
		"migrate same user": func(e map[string]string) {
			e[EnvMigrateUser] = "dbm_runtime"
			e[EnvMigratePasswdFile] = e[EnvPasswordFile]
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			env := validEnv(t)
			mutate(env)
			_, err := LoadConfigFrom(getter(env))
			if err == nil {
				t.Fatal("accepted")
			}
			if strings.Contains(err.Error(), "sentinel-runtime-password") {
				t.Fatal("secret in error")
			}
		})
	}
}

func TestKeyEncodings(t *testing.T) {
	raw := []byte(strings.Repeat("\xfb", KeySize))
	for name, encoded := range map[string]string{
		"std":    base64.StdEncoding.EncodeToString(raw),
		"url":    base64.URLEncoding.EncodeToString(raw),
		"rawstd": base64.RawStdEncoding.EncodeToString(raw),
		"hex":    hex.EncodeToString(raw),
	} {
		key, err := decodeKey(encoded)
		if err != nil || string(key) != string(raw) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, bad := range []string{"", "short", base64.StdEncoding.EncodeToString(raw[:16]), hex.EncodeToString(raw[:31])} {
		if _, err := decodeKey(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestOptionalMigrationAccountAndTLSDisable(t *testing.T) {
	env := validEnv(t)
	dir := t.TempDir()
	env[EnvMigrateUser] = "dbm_migrate"
	env[EnvMigratePasswdFile] = writeSecret(t, dir, "mpw", "sentinel-migration-password")
	env[EnvTLS] = TLSDisable
	env[EnvPort] = "3307"
	cfg, err := LoadConfigFrom(getter(env))
	if err != nil || cfg.MigrationUser != "dbm_migrate" || cfg.MigrationPassword != "sentinel-migration-password" || !cfg.TLSDisabled || cfg.Port != 3307 {
		t.Fatalf("migration/tls config: %v", err)
	}
	user, password := cfg.migrationAccount()
	if user != "dbm_migrate" || password != "sentinel-migration-password" {
		t.Fatal("migration account not selected")
	}
}
