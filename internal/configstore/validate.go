// SPDX-License-Identifier: AGPL-3.0-only
package configstore

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	scopePattern     = regexp.MustCompile(`^[A-Za-z0-9._:~-]{0,128}$`)
	refPattern       = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,64}$`)
	settingsKey      = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	variablePattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	idPattern        = regexp.MustCompile(`^[0-9a-f]{32}$`)
	ownerPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	privacyKey       = "privacy"
	privacyHistoryOn = "history_enabled"
)

// okText accepts valid UTF-8 without control characters (tab, newline and
// carriage return allowed), without NUL, format characters or U+FFFD.
func okText(text string, multiline bool) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, r := range text {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			if !multiline {
				return false
			}
		case r < 0x20 || r == 0x7f || r == utf8.RuneError || (r >= 0x80 && r < 0xa0):
			return false
		case unicode.Is(unicode.Cf, r) && !multiline:
			// Invisible format characters in identifiers can disguise names.
			return false
		}
	}
	return true
}

// jsonCheck validates one JSON value structurally and collects all strings.
type jsonCheck struct {
	nodes     int
	maxString int
	strs      []string
}

func (c *jsonCheck) value(dec *json.Decoder, depth int) error {
	c.nodes++
	if c.nodes > MaxJSONNodes {
		return ErrInvalidData
	}
	tok, err := dec.Token()
	if err != nil {
		return ErrInvalid
	}
	switch t := tok.(type) {
	case json.Delim:
		if depth >= MaxJSONDepth {
			return ErrInvalidData
		}
		if t == '{' {
			seen := map[string]bool{}
			for dec.More() {
				keyToken, err := dec.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok || key == "" || len(key) > 128 || !okText(key, false) || seen[key] {
					return ErrInvalidData
				}
				if len(seen) >= MaxJSONObjectKeys {
					return ErrInvalidData
				}
				seen[key] = true
				c.strs = append(c.strs, key)
				if err := c.value(dec, depth+1); err != nil {
					return err
				}
			}
		} else {
			count := 0
			for dec.More() {
				if count++; count > MaxJSONArray {
					return ErrInvalidData
				}
				if err := c.value(dec, depth+1); err != nil {
					return err
				}
			}
		}
		if _, err := dec.Token(); err != nil {
			return ErrInvalid
		}
	case string:
		if len(t) > c.maxString || !okText(t, true) {
			return ErrInvalidData
		}
		c.strs = append(c.strs, t)
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return ErrInvalidData
		}
	case bool:
	default: // null
		return ErrInvalidData
	}
	return nil
}

// checkJSON validates raw as a single JSON object and returns its compact form
// plus every key and string value. Duplicate keys, nulls, invalid text, deep
// nesting and oversized values are rejected.
func checkJSON(raw []byte, maxString int) ([]byte, []string, error) {
	if !utf8.Valid(raw) {
		return nil, nil, ErrInvalid
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, nil, ErrInvalidData
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	check := &jsonCheck{maxString: maxString}
	if err := check.value(dec, 0); err != nil {
		return nil, nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, ErrInvalid
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return nil, nil, ErrInvalid
	}
	return compact.Bytes(), check.strs, nil
}

// macHex is HMAC-SHA-256 over the NUL-separated parts, hex encoded.
func macHex(key []byte, parts ...string) string {
	sum := hmac.New(sha256.New, key)
	for _, part := range parts {
		sum.Write([]byte(part))
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func runes(text string) int { return utf8.RuneCountInString(text) }

// typed payloads; unknown fields are rejected by strict decoding.
type historyData struct {
	SQL string `json:"sql"`
}
type bookmarkData struct {
	Name      string   `json:"name"`
	SQL       string   `json:"sql"`
	Variables []string `json:"variables"`
}
type searchData struct {
	Name       string          `json:"name"`
	Definition json.RawMessage `json:"definition"`
}
type tableData struct {
	Database string `json:"database"`
	Table    string `json:"table"`
}

func strictInto(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return ErrInvalidData
	}
	return nil
}

func checkSQL(sql string) error {
	if strings.TrimSpace(sql) == "" || runes(sql) > MaxSQLChars || len(sql) > MaxSQLBytes {
		if len(sql) > MaxSQLBytes || runes(sql) > MaxSQLChars {
			return ErrItemTooLarge
		}
		return ErrInvalidData
	}
	return nil
}

func checkName(name string) error {
	if strings.TrimSpace(name) == "" || runes(name) > MaxNameLen || name != strings.TrimSpace(name) {
		return ErrInvalidData
	}
	return nil
}

// validateData performs the per-kind checks and returns the derived natural key
// ("" if the kind has none).
func validateData(spec Spec, key string, data []byte) (derivedKey string, err error) {
	switch spec.Kind {
	case KindHistory:
		var v historyData
		if err := strictInto(data, &v); err != nil {
			return "", err
		}
		return "", checkSQL(v.SQL)
	case KindBookmarks:
		var v bookmarkData
		if err := strictInto(data, &v); err != nil {
			return "", err
		}
		if err := checkName(v.Name); err != nil {
			return "", err
		}
		if len(v.Variables) > MaxVariables {
			return "", ErrInvalidData
		}
		seen := map[string]bool{}
		for _, name := range v.Variables {
			if !variablePattern.MatchString(name) || seen[name] {
				return "", ErrInvalidData
			}
			seen[name] = true
		}
		return "", checkSQL(v.SQL)
	case KindSearches:
		var v searchData
		if err := strictInto(data, &v); err != nil {
			return "", err
		}
		if err := checkName(v.Name); err != nil {
			return "", err
		}
		if len(bytes.TrimSpace(v.Definition)) == 0 || bytes.TrimSpace(v.Definition)[0] != '{' {
			return "", ErrInvalidData
		}
		return "", nil
	case KindFavorites, KindRecents:
		var v tableData
		if err := strictInto(data, &v); err != nil {
			return "", err
		}
		if v.Table == "" || runes(v.Table) > MaxIdentifier || runes(v.Database) > MaxIdentifier || !okText(v.Table, false) || !okText(v.Database, false) {
			return "", ErrInvalidData
		}
		return v.Database + "\x1f" + v.Table, nil
	case KindSettings:
		if key == privacyKey {
			var v map[string]json.RawMessage
			if err := strictInto(data, &v); err != nil || len(v) != 1 {
				return "", ErrInvalidData
			}
			var on bool
			if raw, ok := v[privacyHistoryOn]; !ok || json.Unmarshal(raw, &on) != nil {
				return "", ErrInvalidData
			}
		}
		return "", nil
	}
	return "", nil
}

func validOwner(owner string) bool { return ownerPattern.MatchString(owner) }
