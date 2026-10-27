// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
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
	regexp.MustCompile(`(?i)[a-z][a-z0-9+.\-]*://[^/\s:@]+:[^/\s@]+@`),
	regexp.MustCompile(`(?i)-----BEGIN[ A-Z]*(?:PRIVATE KEY|ENCRYPTED)`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]*`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[abprs]-[A-Za-z0-9-]{10,}|sk-[A-Za-z0-9_-]{20,}|AIza[0-9A-Za-z_-]{30,})`),
}

var longRun = regexp.MustCompile(`[A-Za-z0-9+/=_-]{40,}`)

// fold maps compatibility forms (full-width letters, ligatures) to plain text and
// removes invisible format characters, so keywords cannot be split or disguised.
func fold(text string) string {
	text = norm.NFKC.String(text)
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
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
	return (digit && upper && lower) || (hexOnly && digit && (upper || lower))
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
