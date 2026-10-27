// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import "errors"

// Error is a fixed public error. It never carries input text, SQL, driver text,
// paths or secrets, so it is safe to return to clients and to log.
type Error struct {
	Code    string
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Code }

func fixed(status int, code, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Public errors with their HTTP status.
var (
	ErrInvalid          = fixed(400, "invalid_request", "Anfrage ist ungültig.")
	ErrInvalidData      = fixed(422, "invalid_data", "Die Daten entsprechen nicht dem Format dieser Sammlung.")
	ErrUnknownKind      = fixed(404, "unknown_kind", "Unbekannte Sammlung.")
	ErrNotFound         = fixed(404, "not_found", "Eintrag nicht gefunden.")
	ErrAlreadyExists    = fixed(409, "already_exists", "Eintrag existiert bereits.")
	ErrVersionConflict  = fixed(412, "version_conflict", "Der Eintrag wurde zwischenzeitlich geändert. Neu laden und erneut versuchen.")
	ErrPreconditionNeed = fixed(428, "precondition_required", "If-Match mit der Version des Eintrags ist erforderlich.")
	ErrItemTooLarge     = fixed(413, "item_too_large", "Der Eintrag ist zu groß.")
	ErrQuota            = fixed(409, "quota_exceeded", "Die Grenze für diese Sammlung ist erreicht. Einträge löschen und erneut versuchen.")
	ErrSecretRejected   = fixed(422, "secret_rejected", "Der Inhalt enthält ein mögliches Geheimnis (Passwort, Token oder Schlüssel) und wird nicht gespeichert.")
	ErrHistoryDisabled  = fixed(409, "history_disabled", "Der Verlauf ist nicht eingeschaltet (Einstellung privacy.history_enabled).")
	ErrPlanStale        = fixed(409, "plan_stale", "Der Bestand hat sich seit der Vorschau geändert. Vorschau neu erstellen.")
	ErrConfirmation     = fixed(422, "confirmation_invalid", "Die Bestätigung stimmt nicht mit der Vorschau überein.")
	ErrUnavailable      = fixed(503, "state_unavailable", "Serverzustand ist nicht erreichbar.")
	ErrCorrupt          = fixed(500, "state_unreadable", "Gespeicherte Daten konnten nicht gelesen werden. Serverschlüssel und Sicherung prüfen.")
)

// Sentinel errors returned by Repo implementations (mapped by the Service).
var (
	// RepoExists is returned together with the existing row on duplicate insert.
	RepoExists = errors.New("configstore: exists")
	// RepoNotFound reports a missing item.
	RepoNotFound = errors.New("configstore: not found")
	// RepoVersion reports a failed If-Match check.
	RepoVersion = errors.New("configstore: version")
	// RepoQuota reports an exceeded limit that eviction cannot resolve.
	RepoQuota = errors.New("configstore: quota")
	// RepoStale reports that a clear digest no longer matches.
	RepoStale = errors.New("configstore: stale")
	// RepoUnavailable reports an unreachable or misconfigured store.
	RepoUnavailable = errors.New("configstore: unavailable")
	// RepoCorrupt reports data that cannot be decrypted or decoded.
	RepoCorrupt = errors.New("configstore: corrupt")
)

// mapRepo converts a repository error to a public error.
func mapRepo(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, RepoExists):
		return ErrAlreadyExists
	case errors.Is(err, RepoNotFound):
		return ErrNotFound
	case errors.Is(err, RepoVersion):
		return ErrVersionConflict
	case errors.Is(err, RepoQuota):
		return ErrQuota
	case errors.Is(err, RepoStale):
		return ErrPlanStale
	case errors.Is(err, RepoCorrupt):
		return ErrCorrupt
	default:
		var public *Error
		if errors.As(err, &public) {
			return public
		}
		return ErrUnavailable
	}
}
