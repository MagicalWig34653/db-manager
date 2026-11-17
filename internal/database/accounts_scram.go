// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/xdg/stringprep"

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// scramVerifier derives a PostgreSQL SCRAM-SHA-256 verifier (4096 iterations, 16-byte
// random salt). Passing the verifier instead of the clear text keeps the password out of
// server logs. The caller applies PostgreSQL-compatible SASLprep before deriving it.
func scramVerifier(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return scramVerifierWithSalt(password, salt, 4096)
}

func scramVerifierWithSalt(password string, salt []byte, iterations int) (string, error) {
	salted, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", err
	}
	mac := func(key []byte, label string) []byte {
		h := hmac.New(sha256.New, key)
		h.Write([]byte(label))
		return h.Sum(nil)
	}
	stored := sha256.Sum256(mac(salted, "Client Key"))
	server := mac(salted, "Server Key")
	enc := base64.StdEncoding
	return "SCRAM-SHA-256$" + strconv.Itoa(iterations) + ":" + enc.EncodeToString(salt) + "$" + enc.EncodeToString(stored[:]) + ":" + enc.EncodeToString(server), nil
}

// verifyPostgresGrant checks inside the open transaction that the granted privilege is
// now effective. PostgreSQL only emits a warning when the grantor may not grant it.
func verifyPostgresGrant(ctx context.Context, env *opEnv, role string, g core.AccountGrant) error {
	privilege := g.Privilege
	if g.GrantOption {
		privilege += " WITH GRANT OPTION"
	}
	var ok bool
	var err error
	switch g.Scope {
	case "database":
		err = env.sess.QueryRowContext(ctx, "SELECT has_database_privilege($1, $2, $3)", role, g.Database, privilege).Scan(&ok)
	case "table":
		err = env.sess.QueryRowContext(ctx, "SELECT has_table_privilege($1, (quote_ident($2)||'.'||quote_ident($3))::regclass, $4)", role, g.Schema, g.Object, privilege).Scan(&ok)
	case "column":
		err = env.sess.QueryRowContext(ctx, "SELECT has_column_privilege($1, (quote_ident($2)||'.'||quote_ident($3))::regclass, $4, $5)", role, g.Schema, g.Object, g.Column, privilege).Scan(&ok)
	case "routine":
		err = env.sess.QueryRowContext(ctx, "SELECT has_function_privilege($1, $2::oid, $3)", role, g.RoutineID, privilege).Scan(&ok)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("grant without effect")
	}
	return nil
}

// postgresSASLprep uses RFC 3454's Unicode 3.2 tables, including unassigned,
// prohibited and bidi checks. PostgreSQL maps non-ASCII spaces before B.1
// removal (U+200B belongs to both tables).
var postgresSASLprep = func() stringprep.Profile {
	profile := stringprep.SASLprep
	profile.Mappings = []stringprep.Mapping{profile.Mappings[1], profile.Mappings[0]}
	return profile
}()

// saslPrep falls back to the original UTF-8 bytes if preparation fails, as
// PostgreSQL does for passwords that do not satisfy RFC 4013.
func saslPrep(password string) string {
	prepared, err := postgresSASLprep.Prepare(password)
	if err != nil {
		return password
	}
	return prepared
}

// verifyPostgresRevoke checks stored, direct ACLs, not effective privileges:
// PUBLIC, ownership and inherited roles may still provide access after a valid revoke.
func verifyPostgresRevoke(ctx context.Context, env *opEnv, role core.AccountRef, requested core.AccountGrant) error {
	if requested.Scope == "routine" {
		oid, err := strconv.ParseUint(requested.RoutineID, 10, 32)
		if err != nil {
			return public("revoke_verification_failed", "Die Routinenkennung konnte nicht geprüft werden. Die Transaktion wird zurückgerollt.")
		}
		requested.RoutineID = strconv.FormatUint(oid, 10)
	}
	var details core.AccountDetails
	if err := readPostgresAccountGrants(ctx, env, role, &details); err != nil {
		return public("revoke_verification_failed", "Der Rechteentzug konnte nicht geprüft werden. Die Transaktion wird zurückgerollt.")
	}
	for _, actual := range details.Grants {
		// GrantOption describes only the option for REVOKE GRANT OPTION FOR;
		// ordinary REVOKE must remove both forms of the direct privilege.
		if requested.GrantOption && !actual.GrantOption {
			continue
		}
		actual.GrantOption = requested.GrantOption
		if actual == requested {
			return public("revoke_without_effect", "Das direkte Recht wurde nicht entzogen. Die Transaktion wird zurückgerollt; es wurde nichts geändert.")
		}
	}
	return nil
}
