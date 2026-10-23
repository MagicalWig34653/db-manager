// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"

	mysql "github.com/go-sql-driver/mysql"
)

// Error is a fixed public error. It never carries driver text, DSNs, hosts,
// user names or secrets, so it is safe to log and to return.
type Error struct{ message string }

func (e *Error) Error() string { return e.message }

var (
	ErrUnavailable     = &Error{"Zustandsspeicher nicht erreichbar."}
	ErrTLS             = &Error{"Zustandsspeicher: TLS-Zertifikatsprüfung fehlgeschlagen."}
	ErrAuth            = &Error{"Zustandsspeicher: Anmeldung oder Berechtigung abgelehnt."}
	ErrSchemaOutdated  = &Error{"Zustandsspeicher: Schema ist veraltet; Migration ausführen."}
	ErrSchemaMismatch  = &Error{"Zustandsspeicher: Schema passt nicht zu dieser Programmversion."}
	ErrMigrationLocked = &Error{"Zustandsspeicher: Migrationssperre nicht erhalten."}
	ErrMigrationFailed = &Error{"Zustandsspeicher: Migration fehlgeschlagen."}
	ErrNoKey           = &Error{"Zustandsspeicher: Serverschlüssel fehlt."}
	ErrUnknownKey      = &Error{"Zustandsspeicher: Schlüsselkennung unbekannt."}
	ErrDecrypt         = &Error{"Zustandsspeicher: Entschlüsselung fehlgeschlagen."}
	ErrMalformed       = &Error{"Zustandsspeicher: Chiffretext ungültig."}
	ErrNotFound        = &Error{"Zustandsspeicher: Eintrag nicht gefunden."}
	ErrConfig          = &Error{"Zustandsspeicher: Konfiguration ungültig."}
)

// configError returns a fixed configuration error with a reason that never contains values.
func configError(reason string) error { return &Error{"Zustandsspeicher: " + reason} }

// classify maps arbitrary driver errors to a fixed public error. Context
// cancellation is preserved so callers can distinguish it.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var public *Error
	if errors.As(err, &public) {
		return public
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var verification *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &verification) || errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &invalid) || errors.Is(err, mysql.ErrNoTLS) {
		return ErrTLS
	}
	var server *mysql.MySQLError
	if errors.As(err, &server) {
		switch server.Number {
		case 1044, 1045, 1142, 1143, 1227, 1370, 1698:
			return ErrAuth
		}
	}
	return ErrUnavailable
}
