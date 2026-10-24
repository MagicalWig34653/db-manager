// SPDX-License-Identifier: AGPL-3.0-only

// Package dbplan holds the shared building blocks of the plan/execute action
// pattern (PWA-PLAN 2.1): statement lists with a stable digest, typed
// confirmation, result status and per-driver capabilities. It performs no I/O.
package dbplan

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
)

// DigestDomain is mixed into every plan digest so a plan hash can never be
// confused with another hash in the system. Changing it invalidates all plans.
const DigestDomain = "dbmanager/plan/v1"

// Status is the outcome of an executed action.
type Status string

const (
	// StatusCompleted means the action was committed.
	StatusCompleted Status = "completed"
	// StatusRejected means nothing was changed.
	StatusRejected Status = "rejected"
	// StatusOutcomeUnknown means the commit result is unknown; never retried automatically.
	StatusOutcomeUnknown Status = "outcome_unknown"
)

// Valid reports whether s is one of the defined statuses.
func (s Status) Valid() bool {
	return s == StatusCompleted || s == StatusRejected || s == StatusOutcomeUnknown
}

// Plan is the dry-run result of a structural change: the exact statements for
// one driver plus warnings and affected objects.
type Plan struct {
	Driver     string   `json:"driver"`
	Statements []string `json:"statements"`
	Warnings   []string `json:"warnings"`
	Affected   []string `json:"affected_objects"`
	SHA256     string   `json:"plan_sha256"`
}

// NewPlan builds a plan and computes its digest. Empty statement lists are
// rejected because an empty plan cannot be confirmed meaningfully.
func NewPlan(driver string, statements, warnings, affected []string) (Plan, error) {
	sum, err := Digest(driver, statements)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Driver: driver, Statements: append([]string{}, statements...), Warnings: nonNil(warnings), Affected: nonNil(affected), SHA256: sum}, nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string{}, values...)
}

// Normalize trims surrounding whitespace and one trailing semicolon. Content
// inside a statement is never altered because it may contain literals.
func Normalize(statement string) string {
	statement = strings.TrimSpace(statement)
	statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	return statement
}

// Digest returns the lowercase hex SHA-256 over the domain, the driver and the
// normalized statements. Every part is length-prefixed so that moving text
// between statements always changes the digest.
func Digest(driver string, statements []string) (string, error) {
	if driver == "" {
		return "", errors.New("dbplan: driver is required")
	}
	if len(statements) == 0 {
		return "", errors.New("dbplan: at least one statement is required")
	}
	hash := sha256.New()
	write := func(value string) {
		var length [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(length[:], uint64(len(value)))
		hash.Write(length[:n])
		hash.Write([]byte(value))
	}
	write(DigestDomain)
	write(driver)
	var count [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(count[:], uint64(len(statements)))
	hash.Write(count[:n])
	for _, statement := range statements {
		normalized := Normalize(statement)
		if normalized == "" {
			return "", errors.New("dbplan: empty statement")
		}
		write(normalized)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Confirmation is the explicit user confirmation of an execute request.
// TargetName is the typed object name for destructive actions.
type Confirmation struct {
	Action     string `json:"action"`
	TargetName string `json:"target_name,omitempty"`
}

// Errors returned by Check.
var (
	ErrPlanMismatch  = errors.New("dbplan: plan differs from the confirmed plan")
	ErrConfirmAction = errors.New("dbplan: confirmation action does not match")
	ErrConfirmTarget = errors.New("dbplan: typed target name does not match")
	ErrInvalidDigest = errors.New("dbplan: expected plan digest is not a sha256 hex string")
)

// Check verifies an execute request against the freshly computed plan.
// expectedSHA is the client's expected_plan_sha256. wantAction is the action
// the route performs. For destructive actions wantTarget is the object name the
// user must have typed; for non-destructive actions pass destructive=false.
// A mismatch maps to HTTP 409 (plan) or 400 (confirmation) in the caller.
func Check(fresh Plan, expectedSHA string, confirm Confirmation, wantAction, wantTarget string, destructive bool) error {
	if !validHexDigest(expectedSHA) {
		return ErrInvalidDigest
	}
	if subtle.ConstantTimeCompare([]byte(fresh.SHA256), []byte(expectedSHA)) != 1 {
		return ErrPlanMismatch
	}
	if confirm.Action != wantAction {
		return ErrConfirmAction
	}
	if destructive && (wantTarget == "" || confirm.TargetName != wantTarget) {
		return ErrConfirmTarget
	}
	return nil
}

func validHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
