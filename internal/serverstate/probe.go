// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
)

var probeID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func probeContext(id string) []byte { return []byte("state_key_probe|sealed|" + id) }

// PutProbe stores a sealed field in the key-identifier test table. It exists
// to prove application-level encryption end to end; it is not a user feature.
func (s *Store) PutProbe(ctx context.Context, id string, plaintext []byte) error {
	if !probeID.MatchString(id) {
		return ErrConfig
	}
	sealed, err := s.cipher.Seal(plaintext, probeContext(id))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO state_key_probe (id, key_id, sealed) VALUES (?, ?, ?) "+
		"ON DUPLICATE KEY UPDATE key_id = VALUES(key_id), sealed = VALUES(sealed)", id, s.cipher.ActiveKeyID(), sealed)
	return classify(err)
}

// GetProbe loads and decrypts a probe field.
func (s *Store) GetProbe(ctx context.Context, id string) ([]byte, error) {
	if !probeID.MatchString(id) {
		return nil, ErrConfig
	}
	var sealed []byte
	err := s.db.QueryRowContext(ctx, "SELECT sealed FROM state_key_probe WHERE id = ?", id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, classify(err)
	}
	return s.cipher.Open(sealed, probeContext(id))
}
