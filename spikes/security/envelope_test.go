// SPDX-License-Identifier: AGPL-3.0-only
// This experiment is not a production vault or a stable serialization contract.
package securityspike

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
)

type context struct {
	Version  uint32 `json:"version"`
	Vault    string `json:"vault"`
	Record   string `json:"record"`
	Revision uint64 `json:"revision"`
	Purpose  string `json:"purpose"`
}

type envelope struct {
	WrappedKey []byte
	Payload    []byte
}

func randomKey() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key) // Go 1.24+ crypto/rand.Read fails fatally on RNG failure.
	return key
}

func aead(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 key required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCMWithRandomNonce(block)
}

func associatedData(c context, purpose string) ([]byte, error) {
	if c.Version != 1 || c.Vault == "" || c.Record == "" || c.Revision == 0 {
		return nil, errors.New("invalid context")
	}
	c.Purpose = purpose
	return json.Marshal(c)
}

func seal(root, plaintext []byte, c context) (envelope, error) {
	wrapAD, err := associatedData(c, "wrap-data-key")
	if err != nil {
		return envelope{}, err
	}
	payloadAD, _ := associatedData(c, "encrypt-record")
	rootAEAD, err := aead(root)
	if err != nil {
		return envelope{}, err
	}
	dataKey := randomKey()
	payloadAEAD, err := aead(dataKey)
	if err != nil {
		return envelope{}, err
	}
	return envelope{
		WrappedKey: rootAEAD.Seal(nil, nil, dataKey, wrapAD),
		Payload:    payloadAEAD.Seal(nil, nil, plaintext, payloadAD),
	}, nil
}

func open(root []byte, e envelope, c context) ([]byte, error) {
	wrapAD, err := associatedData(c, "wrap-data-key")
	if err != nil {
		return nil, err
	}
	payloadAD, _ := associatedData(c, "encrypt-record")
	rootAEAD, err := aead(root)
	if err != nil {
		return nil, err
	}
	dataKey, err := rootAEAD.Open(nil, nil, e.WrappedKey, wrapAD)
	if err != nil {
		return nil, err
	}
	payloadAEAD, err := aead(dataKey)
	if err != nil {
		return nil, err
	}
	return payloadAEAD.Open(nil, nil, e.Payload, payloadAD)
}

func TestEnvelopeRoundTripAndFreshKeys(t *testing.T) {
	root := randomKey()
	c := context{Version: 1, Vault: "synthetic-vault", Record: "synthetic-record", Revision: 1}
	plaintext := []byte("synthetic credential; never connect")
	e, err := seal(root, plaintext, c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := open(root, e, c)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip failed: %v", err)
	}
	again, err := seal(root, plaintext, c)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(e.WrappedKey, again.WrappedKey) || bytes.Equal(e.Payload, again.Payload) {
		t.Fatal("independent encryptions reused output")
	}
	if bytes.Contains(e.Payload, plaintext) {
		t.Fatal("payload contains plaintext")
	}
}

func TestEnvelopeRejectsTamperingAndContextSubstitution(t *testing.T) {
	root := randomKey()
	c := context{Version: 1, Vault: "synthetic-vault", Record: "synthetic-record", Revision: 1}
	e, err := seal(root, []byte("synthetic credential"), c)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		key  []byte
		data envelope
		ctx  context
	}{
		{"wrong-root", randomKey(), e, c},
		{"short-key", []byte("short"), e, c},
		{"wrong-vault", root, e, context{1, "other-vault", c.Record, 1, ""}},
		{"wrong-record", root, e, context{1, c.Vault, "other-record", 1, ""}},
		{"wrong-revision", root, e, context{1, c.Vault, c.Record, 2, ""}},
		{"unknown-version", root, e, context{2, c.Vault, c.Record, 1, ""}},
		{"missing-vault", root, e, context{1, "", c.Record, 1, ""}},
		{"swapped-purpose", root, envelope{e.Payload, e.WrappedKey}, c},
		{"missing-key", root, envelope{nil, e.Payload}, c},
		{"missing-payload", root, envelope{e.WrappedKey, nil}, c},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if plaintext, err := open(tc.key, tc.data, tc.ctx); err == nil || len(plaintext) != 0 {
				t.Fatal("invalid envelope was accepted")
			}
		})
	}
	for partIndex, part := range [][]byte{e.WrappedKey, e.Payload} {
		for i := range part {
			modified := envelope{bytes.Clone(e.WrappedKey), bytes.Clone(e.Payload)}
			if partIndex == 0 {
				modified.WrappedKey[i] ^= 1
			} else {
				modified.Payload[i] ^= 1
			}
			if _, err := open(root, modified, c); err == nil {
				t.Fatalf("modified byte accepted: part %d, offset %d", partIndex, i)
			}
		}
		for size := 0; size < len(part); size++ {
			truncated := envelope{e.WrappedKey, e.Payload}
			if partIndex == 0 {
				truncated.WrappedKey = part[:size]
			} else {
				truncated.Payload = part[:size]
			}
			if _, err := open(root, truncated, c); err == nil {
				t.Fatalf("truncation accepted: part %d, size %d", partIndex, size)
			}
		}
	}
}

func TestValidReplayRequiresExternalFreshnessState(t *testing.T) {
	root := randomKey()
	c := context{Version: 1, Vault: "synthetic-vault", Record: "synthetic-record", Revision: 1}
	e, err := seal(root, []byte("old synthetic credential"), c)
	if err != nil {
		t.Fatal(err)
	}
	// Authentic old envelopes remain valid if the attacker also replays their context.
	if _, err := open(root, e, c); err != nil {
		t.Fatal(err)
	}
	c.Revision = 2 // A trusted local high-water mark rejects the older record.
	if _, err := open(root, e, c); err == nil {
		t.Fatal("old envelope accepted with trusted newer revision")
	}
}
