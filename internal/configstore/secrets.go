// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ScanMode selects how aggressive secret rejection is.
type ScanMode int

const (
	// ScanSQL applies the word-based rules of the browser-local W-P16 store plus
	// all structural rules. Used for SQL text and everything stored beside it.
	ScanSQL ScanMode = iota
	// ScanStructural applies only structural rules (credential URLs, private
	// keys, token formats, long opaque runs). Used for preference documents,
	// layouts and identifiers, where words such as "password_hash" are
	// legitimate column names.
	ScanStructural
	// ScanIdentifier applies the structural rules without the long-run rule, for
	// scopes and keys that are opaque identifiers (for example SHA-256 hex).
	ScanIdentifier
)

// wordRules are ported one to one from the browser store (storage.svelte.ts):
// false positives only cost a stored entry, a miss would persist a credential.
var wordRules = []*regexp.Regexp{
	regexp.MustCompile(`(?i)pass(?:word|wd|phrase)`),
	regexp.MustCompile(`(?i)\bidentified\b`),
	regexp.MustCompile(`(?i)secret|token|credential|api[_-]?key|bearer|private\s+key`),
	regexp.MustCompile(`(?i)\bencryption\s+key|\b(?:hex|re)?key\s*(?:=|['"]|\bx')|\bkey\s+['"]|pragma\s+(?:\w+\s*\.\s*)?(?:hex|re)?key\b|cipher_`),
	regexp.MustCompile(`\$[A-Za-z_\x{80}-\x{ffff}]*\$`),
	regexp.MustCompile(`(?i)\b(?:user\s+mapping|create\s+server|create\s+subscription|dblink_connect\w*)\b`),
	regexp.MustCompile(`(?i)\b(?:uid|user\s*id|pwd)\s*=`),
}

var structuralRules = []*regexp.Regexp{
	// scheme://user:password@host
	regexp.MustCompile(`(?i)[a-z][a-z0-9+.\-]*://[^/\s:@]*:[^/\s@]+@`),
	// Go/MySQL DSN user:password@tcp(host:port)/db
	regexp.MustCompile(`(?i)[^\s:@/]+:[^\s@/]+@(?:tcp|unix|tcp4|tcp6)\(`),
	regexp.MustCompile(`(?i)-----BEGIN[ A-Z]*(?:PRIVATE KEY|ENCRYPTED)`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`),
	// Basic auth header, curl/wget -u user:password, password=value pairs in connection strings.
	regexp.MustCompile(`(?i)\bbasic\s+[A-Za-z0-9+/=]{8,}`),
	regexp.MustCompile(`(?i)\b(?:curl|wget|http)\b[^\n]*\s(?:-u|--user|--password|--http-password)[ =]\S*:\S+`),
	regexp.MustCompile(`(?i)\b(?:pass(?:word|wd|phrase)?|pwd|secret|token)\s*[=:]\s*[^\s,;'"]{2,}`),
	regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[abprs]-[A-Za-z0-9-]{10,}|sk-[A-Za-z0-9_-]{20,}|AIza[0-9A-Za-z_-]{30,})`),
}

var longRun = regexp.MustCompile(`[A-Za-z0-9+/=_-]{32,}`)

// confusables maps the most common look-alike letters (Cyrillic, Greek) to
// ASCII for scanning. It is a small table, not the full UTS #39 skeleton.
var confusables = map[rune]rune{
	'а': 'a', 'в': 'b', 'с': 'c', 'ԁ': 'd', 'е': 'e', 'ꜰ': 'f', 'һ': 'h', 'і': 'i', 'ј': 'j', 'к': 'k', 'ӏ': 'l', 'м': 'm', 'н': 'h',
	'о': 'o', 'р': 'p', 'ԛ': 'q', 'ѕ': 's', 'т': 't', 'υ': 'u', 'ѵ': 'v', 'ԝ': 'w', 'х': 'x', 'у': 'y', 'ᴢ': 'z',
	'А': 'A', 'В': 'B', 'С': 'C', 'Е': 'E', 'Н': 'H', 'І': 'I', 'Ј': 'J', 'К': 'K', 'М': 'M', 'О': 'O', 'Р': 'P', 'Ѕ': 'S', 'Т': 'T', 'Х': 'X', 'Υ': 'Y', 'Ζ': 'Z',
	'ᴘ': 'p', 'ᴀ': 'a', 'ꜱ': 's', 'ᴡ': 'w', 'ᴏ': 'o', 'ʀ': 'r', 'ᴅ': 'd', 'ᴇ': 'e', 'ᴋ': 'k', 'ᴛ': 't', 'ɴ': 'n', 'ɪ': 'i', 'ʟ': 'l', 'ᴍ': 'm', 'ᴄ': 'c', 'ᴜ': 'u', 'ᴠ': 'v', 'ʏ': 'y', 'ɢ': 'g', 'ʙ': 'b', 'ᴊ': 'j', 'ʜ': 'h',
	'ո': 'n', 'օ': 'o', 'ս': 'u',
	'α': 'a', 'ο': 'o', 'ρ': 'p', 'ν': 'v', 'ι': 'i', 'κ': 'k', 'ϲ': 'c', 'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Η': 'H', 'Ι': 'I', 'Κ': 'K', 'Μ': 'M', 'Ν': 'N', 'Ο': 'O', 'Ρ': 'P', 'Τ': 'T', 'Χ': 'X',
}

// fold maps compatibility forms (full-width letters, ligatures) to plain text,
// removes invisible format characters, combining marks (incl. U+034F and the
// variation selectors) and Hangul fillers, and maps common homoglyphs to ASCII,
// so keywords cannot be split or disguised.
func fold(text string) string {
	// The table runs before NFKC (which rewrites some entries, e.g. U+03F2) and after it.
	text = mapConfusables(text)
	text = norm.NFKC.String(text)
	text = strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cf, r), unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r):
			return -1
		case r == 0x115F || r == 0x1160 || r == 0x3164 || r == 0xFFA0:
			return -1
		}
		return r
	}, text)
	return mapConfusables(text)
}

func mapConfusables(text string) string {
	return strings.Map(func(r rune) rune {
		if ascii, ok := confusables[r]; ok {
			return ascii
		}
		return r
	}, text)
}

// opaqueRun reports whether a long run looks like a base64 or hex secret.
func opaqueRun(run string) bool {
	var digit, upper, lower, hexOnly = false, false, false, true
	for _, r := range run {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case r >= 'A' && r <= 'Z':
			upper = true
			if r > 'F' {
				hexOnly = false
			}
		case r >= 'a' && r <= 'z':
			lower = true
			if r > 'f' {
				hexOnly = false
			}
		default:
			hexOnly = false
		}
	}
	return (digit && upper && lower) || (hexOnly && digit && (upper || lower) && len(run) >= 32)
}

// LooksSecret reports whether text must not be stored in the given mode.
func LooksSecret(text string, mode ScanMode) bool {
	for _, candidate := range [2]string{text, fold(text)} {
		if mode == ScanSQL {
			for _, rule := range wordRules {
				if rule.MatchString(candidate) {
					return true
				}
			}
		}
		for _, rule := range structuralRules {
			if rule.MatchString(candidate) {
				return true
			}
		}
		if mode != ScanIdentifier {
			for _, run := range longRun.FindAllString(candidate, -1) {
				if opaqueRun(run) {
					return true
				}
			}
		}
		if candidate == fold(candidate) {
			break
		}
	}
	return false
}

var sensitiveKey = regexp.MustCompile(`(?i)pass|secret|token|pwd|credential|api[_-]?key|private[_-]?key|(?:^|[_.-])(?:pin|pw|auth)$`)

// sensitiveValues rejects documents that store a string under a key whose name
// says "credential" (password, token, key, ...). Booleans, numbers and nested
// objects pass; a one-character value is allowed (for example a separator).
// Arrays of names under neutral keys (column lists) are unaffected.
func sensitiveValues(raw []byte) error {
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	var walk func(v any, sensitive bool) bool
	walk = func(v any, sensitive bool) bool {
		switch t := v.(type) {
		case map[string]any:
			for k, child := range t {
				if walk(child, sensitive || sensitiveKey.MatchString(fold(k))) {
					return true
				}
			}
		case []any:
			for _, child := range t {
				if walk(child, sensitive) {
					return true
				}
			}
		case string:
			return sensitive && len(t) > 1
		}
		return false
	}
	if walk(doc, false) {
		return ErrSecretRejected
	}
	return nil
}
