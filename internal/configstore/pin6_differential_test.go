// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPin6DifferentialCorpus writes the strings the BACKEND rejects in ScanSQL mode (literals from every test in this
// package, the config-store contract fixtures and a seeded mutation corpus) to $DBM_DIFF_OUT as a JSON array. The web
// repository (src/lib/features/config-store/pin6.differential.test.ts) reads it and asserts that the browser rules
// reject at least that set (backend-rejected is a subset of client-rejected). Skipped unless DBM_DIFF_OUT is set.
func TestPin6DifferentialCorpus(t *testing.T) {
	out := os.Getenv("DBM_DIFF_OUT")
	if out == "" {
		t.Skip("DBM_DIFF_OUT not set")
	}
	seeds := map[string]bool{}
	files, _ := filepath.Glob("*_test.go")
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && len(s) > 3 && len(s) < 4000 {
					seeds[s] = true
				}
			}
			return true
		})
	}
	fixtures, _ := filepath.Glob("../../contracts/fixtures/config-store/*")
	for _, name := range fixtures {
		raw, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var doc any
		if json.Unmarshal(raw, &doc) == nil {
			var walk func(v any)
			walk = func(v any) {
				switch x := v.(type) {
				case string:
					seeds[x] = true
				case []any:
					for _, c := range x {
						walk(c)
					}
				case map[string]any:
					for _, c := range x {
						walk(c)
					}
				}
			}
			walk(doc)
		}
	}
	// Hand-written atoms: every rule shape in both a rejecting and a disguised form.
	atoms := []string{
		"password", "passwd", "passphrase", "identified by", "secret", "token", "credential", "api_key", "api-key", "apikey",
		"bearer abcdefghijklmnop1234", "private key", "encryption key", "hexkey=", "pragma key", "pragma main.hexkey", "cipher_",
		"$$", "$tag$", "user mapping", "create server", "create subscription", "dblink_connect", "uid=", "user id =", "pwd=",
		"http://user:pass@host", "postgres://admin:s3cr3t@db/app", "mysql://u:p@h", "ftp://:pw@h", "a+b.c-d://x:y@z",
		"user:pw@tcp(h:3306)/db", "u:p@unix(/tmp/s)", "-----BEGIN PRIVATE KEY-----", "-----BEGIN ENCRYPTED", "-----BEGIN RSA PRIVATE KEY",
		"eyJhbGciOiJI.eyJzdWIiOiIx.dBjft", "basic dXNlcjpwYXNz", "Basic dXNlcjpwYXNzd29yZA==", "curl -u admin:hunter2 x",
		"wget --password=a:b y", "http --user=a:b z", "password=abc", "pwd: abcd", "secret=zz", "token:abc",
		"ghp_abcdefghijklmnopqrst", "github_pat_abcdefghijklmnopqrst", "AKIAABCDEFGHIJKLMNOP", "xoxb-1234567890-abc", "sk-abcdefghijklmnopqrst",
		"AIzaabcdefghijklmnopqrstuvwxyz0123456", "0123456789abcdef0123456789abcdef", "AbCdEfGh1234AbCdEfGh1234AbCdEfGh1234",
		"abcdefghijklmnopqrstuvwxyz012345", "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=",
	}
	for _, a := range atoms {
		seeds[a] = true
	}
	// Mutations: invisible and unusual whitespace characters, combining marks, case, homoglyphs, prefixes, wrapping.
	noise := []string{"\u00a0", "\u2028", "\u2029", "\u2003", "\u1680", "\u3000", "\u200b", "\u200d", "\ufeff", "\u034f", "\u0301", "\u180e", "\u115f", "\u3164", "\x00", "\x0b", "\x0c", "\x1c", "\x85", "\u202f", "\u205f"}
	glyph := map[rune][]rune{}
	for ascii, list := range map[rune]string{
		'a': "аαａ", 'b': "вʙｂ", 'c': "сϲｃ", 'd': "ԁᴅｄ", 'e': "еεｅ", 'f': "ꜰｆ", 'g': "ɢｇ", 'h': "һʜｈ", 'i': "іɪιｉ", 'j': "јᴊｊ",
		'k': "кκᴋ\u212a", 'l': "ӏʟｌ", 'm': "мᴍｍ", 'n': "нոɴｎ", 'o': "оοօᴏｏ", 'p': "рρᴘｐ", 'q': "ԛｑ", 'r': "ʀｒ", 's': "ѕꜱ\u017fｓ",
		't': "тᴛｔ", 'u': "υսᴜｕ", 'v': "ѵνᴠｖ", 'w': "ԝᴡｗ", 'x': "хｘ", 'y': "уʏｙ", 'z': "ᴢｚ",
	} {
		glyph[ascii] = []rune(list)
	}
	rnd := rand.New(rand.NewSource(20261027))
	pool := make([]string, 0, len(seeds))
	for s := range seeds {
		pool = append(pool, s)
	}
	mutate := func(s string) string {
		r := []rune(s)
		for n := rnd.Intn(4) + 1; n > 0 && len(r) > 0; n-- {
			i := rnd.Intn(len(r))
			switch rnd.Intn(7) {
			case 0:
				r = append(r[:i], append([]rune(noise[rnd.Intn(len(noise))]), r[i:]...)...)
			case 1:
				if g, ok := glyph[r[i]|0x20]; ok {
					r[i] = g[rnd.Intn(len(g))]
				}
			case 2:
				if r[i] >= 'a' && r[i] <= 'z' {
					r[i] -= 32
				}
			case 3:
				if r[i] == ' ' {
					r[i] = []rune(noise[rnd.Intn(len(noise))])[0]
				}
			case 4:
				r = append([]rune{[]rune("0123456789+.-_ \t\n")[rnd.Intn(16)]}, r...)
			case 5:
				r = append(r, []rune(noise[rnd.Intn(len(noise))])...)
			case 6:
				r = append([]rune("SELECT '"), append(r, []rune("' FROM t")...)...)
			}
		}
		return string(r)
	}
	rejected := map[string]bool{}
	total := 0
	for _, base := range pool {
		for n := 0; n < 300; n++ {
			candidate := base
			if n > 0 {
				candidate = mutate(base)
			}
			if !strings.Contains(candidate, "\x00") || n%11 == 0 {
				total++
				if LooksSecret(candidate, ScanSQL) {
					rejected[candidate] = true
				}
			}
		}
	}
	list := make([]string, 0, len(rejected))
	for s := range rejected {
		list = append(list, s)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d seeds, %d candidates, %d rejected by the backend", len(pool), total, len(list))
}
