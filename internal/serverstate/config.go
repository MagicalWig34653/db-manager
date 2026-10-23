// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Environment variable names. Secrets are only ever read from files.
const (
	EnvHost              = "DBM_STATE_DB_HOST"
	EnvPort              = "DBM_STATE_DB_PORT"
	EnvName              = "DBM_STATE_DB_NAME"
	EnvUser              = "DBM_STATE_DB_USER"
	EnvPasswordFile      = "DBM_STATE_DB_PASSWORD_FILE"
	EnvCAFile            = "DBM_STATE_DB_CA_FILE"
	EnvTLS               = "DBM_STATE_DB_TLS"
	EnvMigrateUser       = "DBM_STATE_DB_MIGRATE_USER"
	EnvMigratePasswdFile = "DBM_STATE_DB_MIGRATE_PASSWORD_FILE"
	EnvServerKeyFile     = "DBM_STATE_SERVER_KEY_FILE"

	// TLSDisable is the only non-default value of EnvTLS. It is a local
	// development switch; the default is certificate-verified TLS.
	TLSDisable = "disable"

	maxSecretFileBytes = 4096
)

var (
	dnsName      = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)
	identifier   = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	accountName  = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,32}$`)
	allStateEnvs = []string{EnvHost, EnvPort, EnvName, EnvUser, EnvPasswordFile, EnvCAFile, EnvTLS, EnvMigrateUser, EnvMigratePasswdFile, EnvServerKeyFile}
)

// Config describes the optional MariaDB connection for the server state.
// It is administrator controlled and never accepted from API callers.
type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string `json:"-"`
	// CAFile adds trust roots for the state database only (separate from DBM_DB_CA_FILE).
	CAFile string
	// TLSDisabled is an explicit development-only opt-out.
	TLSDisabled bool
	// MigrationUser is optional; when set, only the migration step uses it for DDL.
	MigrationUser     string
	MigrationPassword string `json:"-"`
	// ServerKey is the 32-byte root key for application-level field encryption.
	ServerKey []byte `json:"-"`

	roots *x509.CertPool
}

// String redacts every secret so accidental formatting cannot leak it.
func (c Config) String() string {
	return "serverstate.Config{host=" + c.Host + ", database=" + c.Database + ", secrets=redacted}"
}

// GoString redacts every secret for %#v formatting.
func (c Config) GoString() string { return c.String() }

// Enabled reports whether a state store is configured.
func (c Config) Enabled() bool { return c.Host != "" }

// LoadConfig reads the configuration from the process environment.
func LoadConfig() (Config, error) { return LoadConfigFrom(os.Getenv) }

// LoadConfigFrom reads the configuration through getenv. Without any state
// variable the result is the zero Config (state store disabled). Partial
// configuration is an error so that a typo never silently disables the store.
func LoadConfigFrom(getenv func(string) string) (Config, error) {
	var cfg Config
	cfg.Host = strings.TrimSpace(getenv(EnvHost))
	if cfg.Host == "" {
		for _, name := range allStateEnvs {
			if getenv(name) != "" {
				return Config{}, configError(EnvHost + " fehlt, andere DBM_STATE_*-Variablen sind gesetzt.")
			}
		}
		return Config{}, nil
	}
	if !validHost(cfg.Host) {
		return Config{}, configError(EnvHost + " ist kein gültiger Hostname oder keine gültige IP.")
	}
	cfg.Port = 3306
	if raw := getenv(EnvPort); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, configError(EnvPort + " ist ungültig.")
		}
		cfg.Port = port
	}
	cfg.Database = getenv(EnvName)
	if !identifier.MatchString(cfg.Database) {
		return Config{}, configError(EnvName + " fehlt oder ist ungültig.")
	}
	cfg.User = getenv(EnvUser)
	if !accountName.MatchString(cfg.User) {
		return Config{}, configError(EnvUser + " fehlt oder ist ungültig.")
	}
	var err error
	if cfg.Password, err = readSecret(getenv(EnvPasswordFile), EnvPasswordFile); err != nil {
		return Config{}, err
	}
	switch getenv(EnvTLS) {
	case "", "verify":
	case TLSDisable:
		cfg.TLSDisabled = true
	default:
		return Config{}, configError(EnvTLS + " erlaubt nur leer, verify oder disable.")
	}
	if cfg.CAFile = getenv(EnvCAFile); cfg.CAFile != "" {
		if cfg.TLSDisabled {
			return Config{}, configError(EnvCAFile + " und " + EnvTLS + "=disable schließen sich aus.")
		}
		data, err := os.ReadFile(cfg.CAFile)
		if err != nil || len(data) > 1<<20 {
			return Config{}, configError("CA-Datei konnte nicht gelesen werden.")
		}
		roots, rootsErr := x509.SystemCertPool()
		if rootsErr != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return Config{}, configError("CA-Datei enthält keine gültigen Zertifikate.")
		}
		cfg.roots = roots
	}
	cfg.MigrationUser = getenv(EnvMigrateUser)
	migrationFile := getenv(EnvMigratePasswdFile)
	if (cfg.MigrationUser == "") != (migrationFile == "") {
		return Config{}, configError(EnvMigrateUser + " und " + EnvMigratePasswdFile + " nur gemeinsam setzen.")
	}
	if cfg.MigrationUser != "" {
		if !accountName.MatchString(cfg.MigrationUser) || cfg.MigrationUser == cfg.User {
			return Config{}, configError("Migrationsbenutzer ist ungültig oder gleich dem Laufzeitbenutzer.")
		}
		if cfg.MigrationPassword, err = readSecret(migrationFile, EnvMigratePasswdFile); err != nil {
			return Config{}, err
		}
	}
	keyFile := getenv(EnvServerKeyFile)
	raw, err := readSecret(keyFile, EnvServerKeyFile)
	if err != nil {
		return Config{}, err
	}
	if cfg.ServerKey, err = decodeKey(raw); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validHost(host string) bool {
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Zone() == ""
	}
	return dnsName.MatchString(host) && !strings.Contains(host, "..")
}

// readSecret reads one secret from a file. Only trailing line breaks are removed.
func readSecret(path, name string) (string, error) {
	if path == "" {
		return "", configError(name + " fehlt.")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxSecretFileBytes {
		return "", configError(name + ": Datei konnte nicht gelesen werden.")
	}
	data = bytes.TrimRight(data, "\r\n")
	if len(data) == 0 || bytes.ContainsRune(data, 0) {
		return "", configError(name + ": Datei ist leer oder ungültig.")
	}
	return string(data), nil
}

// decodeKey accepts a base64 (standard or URL, padded or not) or 64-digit hex encoding of 32 bytes.
func decodeKey(raw string) ([]byte, error) {
	key, err := decodeKeyBytes(raw)
	if err == nil && bytes.Equal(key, make([]byte, KeySize)) {
		return nil, configError(EnvServerKeyFile + " darf nicht nur aus Nullbytes bestehen.")
	}
	return key, err
}

func decodeKeyBytes(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if key, err := hex.DecodeString(raw); err == nil && len(key) == KeySize {
		return key, nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if key, err := encoding.DecodeString(raw); err == nil && len(key) == KeySize {
			return key, nil
		}
	}
	return nil, configError(EnvServerKeyFile + " muss 32 Byte als Base64 oder Hex enthalten.")
}
