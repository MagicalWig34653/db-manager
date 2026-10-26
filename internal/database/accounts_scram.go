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
	"unicode"

	"golang.org/x/text/unicode/norm"

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

// saslPrep approximates RFC 4013 SASLprep the way PostgreSQL applies it to passwords: NFKC
// normalization, "mapped to nothing" characters removed and non-ASCII spaces mapped to a space.
// If the result contains prohibited characters PostgreSQL uses the raw password, and so do we.
func saslPrep(password string) string {
	ascii := true
	for i := 0; i < len(password); i++ {
		if password[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return password
	}
	var out []rune
	for _, r := range norm.NFKC.String(password) {
		switch {
		case r == 0x00AD || r == 0x034F || r == 0x1806 || r >= 0x180B && r <= 0x180D || r >= 0x200B && r <= 0x200D || r == 0x2060 || r >= 0xFE00 && r <= 0xFE0F || r == 0xFEFF:
			continue
		case unicode.Is(unicode.Zs, r) && r != ' ':
			out = append(out, ' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Cs, r) || r == 0xFFFD || r >= 0xFDD0 && r <= 0xFDEF || r&0xFFFE == 0xFFFE || r >= 0x2FF0 && r <= 0x2FFB || r >= 0xE0001 && r <= 0xE007F:
			return password
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
