// SPDX-License-Identifier: AGPL-3.0-only
package serverstate

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func testKey(b byte) []byte { return bytes.Repeat([]byte{b}, KeySize) }

func TestCipherRoundTripAndNoPlaintext(t *testing.T) {
	c, err := NewCipher(testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("highly-sensitive-value")
	sealed, err := c.Seal(plain, []byte("ctx"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, plain) || !bytes.Contains(sealed, []byte(c.ActiveKeyID())) {
		t.Fatal("plaintext visible or key id missing")
	}
	opened, err := c.Open(sealed, []byte("ctx"))
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatalf("round trip: %v", err)
	}
	again, _ := c.Seal(plain, []byte("ctx"))
	if bytes.Equal(sealed, again) {
		t.Fatal("nonce reused")
	}
	empty, err := c.Seal(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := c.Open(empty, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty value: %v", err)
	}
}

func TestCipherFailureModes(t *testing.T) {
	c, _ := NewCipher(testKey(1))
	other, _ := NewCipher(testKey(2))
	sealed, _ := c.Seal([]byte("value"), []byte("ctx"))

	var missing *Cipher
	if _, err := missing.Open(sealed, nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("missing key: %v", err)
	}
	if _, err := missing.Seal([]byte("x"), nil); !errors.Is(err, ErrNoKey) {
		t.Fatalf("missing key seal: %v", err)
	}
	if _, err := other.Open(sealed, []byte("ctx")); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := c.Open(sealed, []byte("other")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("wrong context: %v", err)
	}
	tampered := append([]byte{}, sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, err := c.Open(tampered, []byte("ctx")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("tampered: %v", err)
	}
	for _, garbage := range [][]byte{nil, {1}, {9, 3, 'a', 'b', 'c'}, {1, 0}, bytes.Repeat([]byte{1}, 5), {1, 200, 'x'}} {
		if _, err := c.Open(garbage, nil); err == nil {
			t.Fatalf("garbage accepted %v", garbage)
		}
	}
	// A key with the same id but modified bytes in the header is detected by the AAD.
	flipped := append([]byte{}, sealed...)
	flipped[3] ^= 1
	if _, err := c.Open(flipped, []byte("ctx")); err == nil {
		t.Fatal("header tamper accepted")
	}
	if _, err := NewCipher(testKey(1)[:16]); err == nil {
		t.Fatal("short key accepted")
	}
	for _, err := range []error{ErrNoKey, ErrUnknownKey, ErrDecrypt, ErrMalformed} {
		if strings.Contains(err.Error(), "value") {
			t.Fatal("error leaks content")
		}
	}
}

func TestKeyRotationKeepsOldValuesReadable(t *testing.T) {
	old, _ := NewCipher(testKey(1))
	sealed, _ := old.Seal([]byte("legacy"), nil)
	rotated, err := NewCipher(testKey(2), testKey(1))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := rotated.Open(sealed, nil); err != nil || string(got) != "legacy" {
		t.Fatalf("old value after rotation: %v", err)
	}
	fresh, _ := rotated.Seal([]byte("new"), nil)
	if _, err := old.Open(fresh, nil); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("old key opened new value: %v", err)
	}
	if KeyID(testKey(1)) == KeyID(testKey(2)) || strings.Contains(KeyID(testKey(1)), "\x01") {
		t.Fatal("key ids")
	}
}
