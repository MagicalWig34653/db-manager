// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	vault "example.invalid/db-manager/vault-interop"
)

type secrets struct {
	Password   string   `json:"password"`
	Recovery   string   `json:"recovery"`
	Plaintexts []string `json:"plaintexts"`
}

func read(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit+1))
}
func run() bool {
	args := os.Args[1:]
	if len(args) < 2 {
		return false
	}
	start := time.Now()
	if args[0] == "emit" {
		password := "synthetisch – Masterpasswort \x00 😀"
		plain := [][]byte{[]byte("synthetisch: Grüße \x00 😀"), {}, {0, 255, 128}}
		v, recovery, err := vault.Create(password, plain)
		if err != nil {
			return false
		}
		data, err := vault.Canonical(v)
		if err != nil {
			return false
		}
		s := secrets{Password: password, Recovery: base64.StdEncoding.EncodeToString(recovery), Plaintexts: []string{}}
		for _, p := range plain {
			s.Plaintexts = append(s.Plaintexts, base64.StdEncoding.EncodeToString(p))
		}
		secret, _ := json.Marshal(s)
		if os.MkdirAll(args[1], 0700) != nil || os.WriteFile(filepath.Join(args[1], "vault.json"), data, 0600) != nil || os.WriteFile(filepath.Join(args[1], "synthetic-secrets.json"), secret, 0600) != nil {
			return false
		}
	} else if args[0] == "verify" && len(args) >= 4 {
		data, err := read(args[1], vault.MaxFileBytes)
		if err != nil {
			return false
		}
		secret, err := read(args[2], 65536)
		if err != nil || len(secret) > 65536 {
			return false
		}
		var s secrets
		if json.Unmarshal(secret, &s) != nil {
			return false
		}
		var recovery []byte
		if args[3] == "recovery" {
			recovery, err = base64.StdEncoding.Strict().DecodeString(s.Recovery)
			if err != nil {
				return false
			}
		} else if args[3] != "password" {
			return false
		}
		var minimum uint64
		if len(args) > 4 {
			minimum, err = strconv.ParseUint(args[4], 10, 32)
			if err != nil {
				return false
			}
		}
		plain, err := vault.Unlock(data, s.Password, recovery, uint32(minimum))
		if err != nil || len(plain) != len(s.Plaintexts) {
			return false
		}
		for i, p := range plain {
			expected, err := base64.StdEncoding.DecodeString(s.Plaintexts[i])
			if err != nil || !bytes.Equal(p, expected) {
				return false
			}
		}
	} else {
		return false
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "duration_ms": float64(time.Since(start).Microseconds()) / 1000})
	return true
}
func main() {
	if !run() {
		os.Stdout.WriteString("{\"ok\":false,\"code\":\"rejected\"}\n")
		os.Exit(1)
	}
}
