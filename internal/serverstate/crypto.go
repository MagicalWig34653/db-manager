// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// KeySize is the required server key length in bytes (AES-256).
const KeySize = 32

const (
	formatVersion byte = 1
	nonceSize          = 12
	maxKeyIDLen        = 40
)

// Cipher seals and opens sensitive fields with AES-256-GCM. Every sealed value
// carries the identifier of the key that produced it, so a keyring can later
// hold previous keys for rotation. A nil *Cipher models a missing server key
// and returns ErrNoKey instead of panicking.
//
// Sealed layout: version(1) | keyIDLen(1) | keyID | nonce(12) | ciphertext+tag.
// The additional authenticated data binds version, key ID and a caller context
// (for example table, column and row identity), so values cannot be moved
// between rows undetected.
type Cipher struct {
	active string
	keys   map[string]cipher.AEAD
}

// KeyID derives the public identifier of a key. It is a one-way, domain
// separated digest prefix and does not reveal the key.
func KeyID(key []byte) string {
	sum := sha256.Sum256(append([]byte("dbmanager-state-key-id-v1\x00"), key...))
	return "k-" + hex.EncodeToString(sum[:8])
}

// NewCipher creates a cipher with one active key and optional previous keys
// that may only open existing values.
func NewCipher(active []byte, previous ...[]byte) (*Cipher, error) {
	c := &Cipher{keys: map[string]cipher.AEAD{}}
	for i, key := range append([][]byte{active}, previous...) {
		if len(key) != KeySize {
			return nil, ErrConfig
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, ErrConfig
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrConfig
		}
		id := KeyID(key)
		c.keys[id] = aead
		if i == 0 {
			c.active = id
		}
	}
	return c, nil
}

// ActiveKeyID returns the identifier used for new values.
func (c *Cipher) ActiveKeyID() string {
	if c == nil {
		return ""
	}
	return c.active
}

// Seal encrypts plaintext under the active key, bound to context.
func (c *Cipher) Seal(plaintext, context []byte) ([]byte, error) {
	if c == nil || c.active == "" {
		return nil, ErrNoKey
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrDecrypt
	}
	header := append([]byte{formatVersion, byte(len(c.active))}, c.active...)
	out := append(append([]byte{}, header...), nonce...)
	return c.keys[c.active].Seal(out, nonce, plaintext, aad(header, context)), nil
}

// Open decrypts a sealed value. Wrong key, wrong context and tampering all
// produce the same ErrDecrypt without revealing which one occurred.
func (c *Cipher) Open(sealed, context []byte) ([]byte, error) {
	if c == nil || c.active == "" {
		return nil, ErrNoKey
	}
	if len(sealed) < 2 || sealed[0] != formatVersion {
		return nil, ErrMalformed
	}
	idLen := int(sealed[1])
	headerLen := 2 + idLen
	if idLen == 0 || idLen > maxKeyIDLen || len(sealed) < headerLen+nonceSize+16 {
		return nil, ErrMalformed
	}
	aead, ok := c.keys[string(sealed[2:headerLen])]
	if !ok {
		return nil, ErrUnknownKey
	}
	nonce := sealed[headerLen : headerLen+nonceSize]
	plaintext, err := aead.Open(nil, nonce, sealed[headerLen+nonceSize:], aad(sealed[:headerLen], context))
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

func aad(header, context []byte) []byte {
	return append(append([]byte{}, header...), context...)
}
