// SPDX-License-Identifier: AGPL-3.0-only
// Package vault is a synthetic interoperability experiment, not production storage.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const MaxFileBytes = 8 << 20
const MaxEntries = 64
const MaxPayloadBytes = 64 << 10

var ErrInvalid = errors.New("invalid vault")
var ErrUnlock = errors.New("vault unlock failed")

type KDF struct {
	Name        string `json:"name"`
	Version     uint32 `json:"version"`
	Memory      uint32 `json:"memory_kib"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint32 `json:"parallelism"`
	Salt        string `json:"salt"`
}
type PasswordSlot struct {
	KDF         KDF    `json:"kdf"`
	Nonce       string `json:"nonce"`
	WrappedRoot string `json:"wrapped_root"`
}
type RecoverySlot struct {
	Nonce       string `json:"nonce"`
	WrappedRoot string `json:"wrapped_root"`
}
type Entry struct {
	ID           string `json:"id"`
	Revision     uint32 `json:"revision"`
	KeyNonce     string `json:"key_nonce"`
	WrappedKey   string `json:"wrapped_key"`
	PayloadNonce string `json:"payload_nonce"`
	Payload      string `json:"payload"`
}
type Vault struct {
	Format     string       `json:"format"`
	Version    uint32       `json:"version"`
	ID         string       `json:"vault_id"`
	Epoch      uint32       `json:"epoch"`
	Generation uint32       `json:"generation"`
	Password   PasswordSlot `json:"password"`
	Recovery   RecoverySlot `json:"recovery"`
	Entries    []Entry      `json:"entries"`
	MAC        string       `json:"mac"`
}

func random(n int) []byte    { out := make([]byte, n); _, _ = rand.Read(out); return out }
func b64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }
func decode(s string, size int) ([]byte, error) {
	b, err := base64.StdEncoding.Strict().DecodeString(s)
	if err != nil || len(b) != size || b64(b) != s {
		return nil, ErrInvalid
	}
	return b, nil
}
func identifier(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 || hex.EncodeToString(b) != s {
		return nil, ErrInvalid
	}
	return b, nil
}

// Canonical is the sole accepted wire representation: sorted ASCII keys, no whitespace.
func Canonical(v Vault) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, ErrInvalid
	}
	var object any
	if json.Unmarshal(raw, &object) != nil {
		return nil, ErrInvalid
	}
	return json.Marshal(object)
}
func Parse(data []byte) (Vault, error) {
	var v Vault
	if len(data) == 0 || len(data) > MaxFileBytes {
		return v, ErrInvalid
	}
	inString := false
	depth := 0
	for _, b := range data {
		if b >= 128 || b == '\\' {
			return v, ErrInvalid
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if !inString {
			switch b {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
			if depth < 0 || depth > 4 {
				return v, ErrInvalid
			}
		}
	}
	if inString || depth != 0 {
		return v, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return v, ErrInvalid
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return v, ErrInvalid
	}
	canonical, err := Canonical(v)
	if err != nil || !bytes.Equal(data, canonical) {
		return v, ErrInvalid
	}
	if err = v.validate(); err != nil {
		return v, err
	}
	return v, nil
}
func (v Vault) validate() error {
	if v.Format != "db-manager-vault" || v.Version != 1 || v.Epoch == 0 || v.Generation == 0 || v.Entries == nil || len(v.Entries) > MaxEntries {
		return ErrInvalid
	}
	if _, err := identifier(v.ID); err != nil {
		return err
	}
	k := v.Password.KDF
	if k.Name != "argon2id" || k.Version != 19 || k.Memory != 65536 || k.Iterations != 3 || k.Parallelism != 4 {
		return ErrInvalid
	}
	for _, item := range []struct {
		s string
		n int
	}{{k.Salt, 16}, {v.Password.Nonce, 12}, {v.Password.WrappedRoot, 48}, {v.Recovery.Nonce, 12}, {v.Recovery.WrappedRoot, 48}, {v.MAC, 32}} {
		if _, err := decode(item.s, item.n); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	nonces := map[string]bool{}
	for _, e := range v.Entries {
		if _, err := identifier(e.ID); err != nil {
			return err
		}
		if e.Revision == 0 || seen[e.ID] {
			return ErrInvalid
		}
		seen[e.ID] = true
		if nonces[e.KeyNonce] {
			return ErrInvalid
		}
		nonces[e.KeyNonce] = true
		for _, item := range []struct {
			s string
			n int
		}{{e.KeyNonce, 12}, {e.WrappedKey, 48}, {e.PayloadNonce, 12}} {
			if _, err := decode(item.s, item.n); err != nil {
				return err
			}
		}
		if len(e.Payload) > 87404 {
			return ErrInvalid
		}
		p, err := base64.StdEncoding.Strict().DecodeString(e.Payload)
		if err != nil || len(p) < 16 || len(p) > MaxPayloadBytes+16 || b64(p) != e.Payload {
			return ErrInvalid
		}
	}
	return nil
}
func append32(dst []byte, n uint32) []byte { return binary.BigEndian.AppendUint32(dst, n) }
func (v Vault) aad(purpose byte, entry *Entry) []byte {
	id, _ := identifier(v.ID)
	out := append([]byte("DBMV1\x00"), purpose)
	out = append(out, id...)
	out = append32(out, v.Epoch)
	if purpose == 1 {
		k := v.Password.KDF
		out = append32(out, k.Memory)
		out = append32(out, k.Iterations)
		out = append32(out, k.Parallelism)
		salt, _ := decode(k.Salt, 16)
		out = append(out, salt...)
	}
	if entry != nil {
		id, _ := identifier(entry.ID)
		out = append(out, id...)
		out = append32(out, entry.Revision)
	}
	return out
}
func derived(secret []byte, v Vault, purpose string) []byte {
	id, _ := identifier(v.ID)
	info := append32([]byte("DBMV1\x00"+purpose+"\x00"), v.Epoch)
	key, err := hkdf.Key(sha256.New, secret, id, string(info), 32)
	if err != nil {
		panic(err)
	}
	return key
}
func seal(key, aad, plaintext []byte) (string, string, error) {
	if len(key) != 32 {
		return "", "", ErrInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", ErrInvalid
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", ErrInvalid
	}
	nonce := random(12)
	return b64(nonce), b64(a.Seal(nil, nonce, plaintext, aad)), nil
}
func unseal(key, aad []byte, nonce, ciphertext string) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrUnlock
	}
	n, nonceErr := decode(nonce, 12)
	if nonceErr != nil {
		return nil, ErrUnlock
	}
	c, err := base64.StdEncoding.Strict().DecodeString(ciphertext)
	if err != nil {
		return nil, ErrUnlock
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrUnlock
	}
	a, err := cipher.NewGCM(b)
	if err != nil {
		return nil, ErrUnlock
	}
	p, err := a.Open(nil, n, c, aad)
	if err != nil {
		return nil, ErrUnlock
	}
	return p, nil
}
func passwordKey(password string, k KDF) ([]byte, error) {
	if len(password) == 0 || len(password) > 1024 || !utf8.ValidString(password) {
		return nil, ErrUnlock
	}
	salt, err := decode(k.Salt, 16)
	if err != nil {
		return nil, ErrUnlock
	}
	return argon2.IDKey([]byte(password), salt, k.Iterations, k.Memory, uint8(k.Parallelism), 32), nil
}
func (v Vault) manifest() []byte {
	out := append32(v.aad(5, nil), v.Generation)
	out = append(out, v.aad(1, nil)[27:]...)
	for _, s := range []string{v.Password.Nonce, v.Password.WrappedRoot, v.Recovery.Nonce, v.Recovery.WrappedRoot} {
		b, _ := base64.StdEncoding.DecodeString(s)
		out = append(out, b...)
	}
	out = append32(out, uint32(len(v.Entries)))
	for _, e := range v.Entries {
		id, _ := identifier(e.ID)
		out = append(out, id...)
		out = append32(out, e.Revision)
		for _, s := range []string{e.KeyNonce, e.WrappedKey, e.PayloadNonce} {
			b, _ := base64.StdEncoding.DecodeString(s)
			out = append(out, b...)
		}
		payload, _ := base64.StdEncoding.DecodeString(e.Payload)
		out = append32(out, uint32(len(payload)))
		out = append(out, payload...)
	}
	return out
}
func (v Vault) mac(root []byte) []byte {
	m := hmac.New(sha256.New, derived(root, v, "manifest"))
	m.Write(v.manifest())
	return m.Sum(nil)
}

// Create uses fresh roots, salts, entry keys and nonces. Only synthetic data belongs here.
func Create(password string, payloads [][]byte) (Vault, []byte, error) {
	v := Vault{Format: "db-manager-vault", Version: 1, ID: hex.EncodeToString(random(16)), Epoch: 1, Generation: 1, Entries: []Entry{}}
	v.Password.KDF = KDF{"argon2id", 19, 65536, 3, 4, b64(random(16))}
	if len(payloads) > MaxEntries {
		return v, nil, ErrInvalid
	}
	for _, payload := range payloads {
		if len(payload) > MaxPayloadBytes {
			return v, nil, ErrInvalid
		}
	}
	root, recovery := random(32), random(32)
	key, err := passwordKey(password, v.Password.KDF)
	if err != nil {
		return v, nil, err
	}
	v.Password.Nonce, v.Password.WrappedRoot, err = seal(key, v.aad(1, nil), root)
	if err != nil {
		return v, nil, err
	}
	v.Recovery.Nonce, v.Recovery.WrappedRoot, err = seal(derived(recovery, v, "recovery"), v.aad(2, nil), root)
	if err != nil {
		return v, nil, err
	}
	for _, payload := range payloads {
		if len(payload) > MaxPayloadBytes {
			return v, nil, ErrInvalid
		}
		e := Entry{ID: hex.EncodeToString(random(16)), Revision: 1}
		key := random(32)
		e.KeyNonce, e.WrappedKey, err = seal(root, v.aad(3, &e), key)
		if err != nil {
			return v, nil, err
		}
		e.PayloadNonce, e.Payload, err = seal(key, v.aad(4, &e), payload)
		if err != nil {
			return v, nil, err
		}
		v.Entries = append(v.Entries, e)
	}
	v.MAC = b64(v.mac(root))
	return v, recovery, v.validate()
}

// Unlock checks authenticated contents before returning any entry plaintext.
// minimumGeneration must come from independently trusted state; zero accepts valid old files.
func Unlock(data []byte, password string, recovery []byte, minimumGeneration uint32) ([][]byte, error) {
	v, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if v.Generation < minimumGeneration {
		return nil, ErrInvalid
	}
	var root []byte
	if recovery != nil {
		if len(recovery) != 32 {
			return nil, ErrUnlock
		}
		root, err = unseal(derived(recovery, v, "recovery"), v.aad(2, nil), v.Recovery.Nonce, v.Recovery.WrappedRoot)
	} else {
		key, keyErr := passwordKey(password, v.Password.KDF)
		if keyErr != nil {
			return nil, keyErr
		}
		root, err = unseal(key, v.aad(1, nil), v.Password.Nonce, v.Password.WrappedRoot)
	}
	if err != nil {
		return nil, ErrUnlock
	}
	expected, _ := decode(v.MAC, 32)
	if !hmac.Equal(expected, v.mac(root)) {
		return nil, ErrUnlock
	}
	result := [][]byte{}
	for _, e := range v.Entries {
		key, err := unseal(root, v.aad(3, &e), e.KeyNonce, e.WrappedKey)
		if err != nil {
			return nil, ErrUnlock
		}
		payload, err := unseal(key, v.aad(4, &e), e.PayloadNonce, e.Payload)
		if err != nil {
			return nil, ErrUnlock
		}
		result = append(result, payload)
	}
	return result, nil
}
