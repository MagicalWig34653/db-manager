// SPDX-License-Identifier: AGPL-3.0-only
package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func fixture(t *testing.T) (Vault, []byte, []byte) {
	t.Helper()
	v, r, err := Create("synthetic-password", [][]byte{[]byte("synthetic payload")})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Canonical(v)
	if err != nil {
		t.Fatal(err)
	}
	return v, r, d
}
func TestPasswordRecoveryAndLostWays(t *testing.T) {
	_, recovery, data := fixture(t)
	for _, r := range [][]byte{nil, recovery} {
		if _, err := Unlock(data, "synthetic-password", r, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Unlock(data, "wrong-password", nil, 0); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := Unlock(data, "", random(32), 0); err == nil {
		t.Fatal("wrong recovery accepted")
	}
	if _, err := Unlock(data, "", recovery, 0); err != nil {
		t.Fatal("independent recovery failed", err)
	}
}
func TestBoundedCanonicalParsingAndReplayBoundary(t *testing.T) {
	v, _, data := fixture(t)
	for i := 0; i < len(data); i++ {
		if _, err := Parse(data[:i]); err == nil {
			t.Fatalf("truncation %d accepted", i)
		}
	}
	for _, bad := range [][]byte{append(bytes.Clone(data), ' '), bytes.Repeat([]byte("x"), MaxFileBytes+1), []byte(`{"x":[[[[[0]]]]]}`), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1.0`), 1), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)} {
		if _, err := Parse(bad); err == nil {
			t.Fatal("noncanonical input accepted")
		}
	}
	v.Password.KDF.Memory = 1 << 31
	bad, _ := Canonical(v)
	if _, err := Parse(bad); err == nil {
		t.Fatal("KDF DoS accepted")
	}
	if _, err := Unlock(data, "synthetic-password", nil, 0); err != nil {
		t.Fatal("valid old file must remain cryptographically valid")
	}
	if _, err := Unlock(data, "synthetic-password", nil, 2); err == nil {
		t.Fatal("trusted generation floor ignored")
	}
}
func TestManifestAndContextBinding(t *testing.T) {
	v, recovery, _ := fixture(t)
	for _, mutate := range []func(*Vault){func(v *Vault) { v.Entries = nil }, func(v *Vault) { v.Entries = []Entry{} }, func(v *Vault) { v.Generation++ }, func(v *Vault) { v.Epoch++ }, func(v *Vault) { v.ID = "00000000000000000000000000000000" }, func(v *Vault) { v.Entries[0].Revision++ }, func(v *Vault) { v.Entries[0].Payload = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 17)) }} {
		copyBytes, _ := json.Marshal(v)
		var changed Vault
		_ = json.Unmarshal(copyBytes, &changed)
		mutate(&changed)
		data, _ := Canonical(changed)
		if _, err := Unlock(data, "", recovery, 0); err == nil {
			t.Fatal("altered manifest/context accepted")
		}
	}
	key := random(32)
	aad := v.aad(3, &v.Entries[0])
	nonce, ciphertext, err := seal(key, aad, []byte("synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unseal(key, v.aad(4, &v.Entries[0]), nonce, ciphertext); err == nil {
		t.Fatal("purpose substitution accepted")
	}
	encoded, _ := base64.StdEncoding.DecodeString(ciphertext)
	for i := range encoded {
		changed := bytes.Clone(encoded)
		changed[i] ^= 1
		if _, err := unseal(key, aad, nonce, base64.StdEncoding.EncodeToString(changed)); err == nil {
			t.Fatalf("cipher byte %d accepted", i)
		}
	}
}

func TestAADFixedVectorAndValidMACCannotHideContextSwap(t *testing.T) {
	v := Vault{ID: "000102030405060708090a0b0c0d0e0f", Epoch: 0x01020304}
	e := Entry{ID: "101112131415161718191a1b1c1d1e1f", Revision: 0x0a0b0c0d}
	want := "44424d56310003000102030405060708090a0b0c0d0e0f01020304101112131415161718191a1b1c1d1e1f0a0b0c0d"
	if hex.EncodeToString(v.aad(3, &e)) != want {
		t.Fatal("binary AAD vector mismatch")
	}
	actual, recovery, _ := fixture(t)
	root, err := unseal(derived(recovery, actual, "recovery"), actual.aad(2, nil), actual.Recovery.Nonce, actual.Recovery.WrappedRoot)
	if err != nil {
		t.Fatal(err)
	}
	actual.Entries[0].Revision++
	actual.MAC = b64(actual.mac(root))
	data, _ := Canonical(actual)
	if _, err := Unlock(data, "", recovery, 0); err == nil {
		t.Fatal("valid manifest hid entry context swap")
	}
}
