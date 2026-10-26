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

	"github.com/MagicalWig34653/db-manager/internal/core"
)

// scramVerifier derives a PostgreSQL SCRAM-SHA-256 verifier (4096 iterations, 16-byte
// random salt). Passing the verifier instead of the clear text keeps the password out of
// server logs. The password is used as raw UTF-8 bytes (SASLprep is not applied; it only
// differs for unusual Unicode input).
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
