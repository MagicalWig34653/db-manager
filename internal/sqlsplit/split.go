// SPDX-License-Identifier: AGPL-3.0-only
// Package sqlsplit splits user SQL, without claiming to validate SQL grammar.
package sqlsplit

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 1 << 20
const MaxStatements = 100

var ErrSyntax = errors.New("SQL-Trennung ist ungültig oder überschreitet die Grenzen.")

// Split recognizes dialect-specific quotes, comments and line-based DELIMITER
// directives. A custom delimiter must be punctuation, at most 16 bytes long.
// Returned statements retain comments; delimiter directives are never sent.
func Split(input, delimiter, dialect string) ([]string, error) {
	return SplitLimited(input, delimiter, dialect, MaxBytes, MaxStatements)
}

// SplitLimited uses the same lexer for bounded uploaded SQL files.
func SplitLimited(input, delimiter, dialect string, maxBytes, maxStatements int) ([]string, error) {
	return SplitLimitedContext(context.Background(), input, delimiter, dialect, maxBytes, maxStatements)
}

// SplitLimitedContext also bounds CPU work after an import cancellation.
func SplitLimitedContext(ctx context.Context, input, delimiter, dialect string, maxBytes, maxStatements int) ([]string, error) {
	if maxBytes <= 0 || maxStatements <= 0 || len(input) > maxBytes || !utf8.ValidString(input) || strings.ContainsRune(input, 0) {
		return nil, ErrSyntax
	}
	if dialect != "sqlite" && dialect != "mysql" && dialect != "postgres" {
		return nil, ErrSyntax
	}
	if delimiter == "" {
		delimiter = ";"
	}
	if !validDelimiter(delimiter) {
		return nil, ErrSyntax
	}
	result := []string{}
	var b strings.Builder
	hasCode := false
	lineStart := true
	emit := func() error {
		if hasCode && strings.TrimSpace(b.String()) != "" {
			if len(result) == maxStatements {
				return ErrSyntax
			}
			result = append(result, strings.TrimSpace(b.String()))
		}
		b.Reset()
		hasCode = false
		return nil
	}
	for i := 0; i < len(input); {
		if i%4096 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if lineStart {
			end := strings.IndexByte(input[i:], '\n')
			if end < 0 {
				end = len(input) - i
			}
			line := strings.TrimSpace(input[i : i+end])
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.EqualFold(fields[0], "DELIMITER") {
				if hasCode || len(fields) != 2 || !validDelimiter(fields[1]) {
					return nil, ErrSyntax
				}
				b.Reset()
				delimiter = fields[1]
				i += end
				if i < len(input) {
					i++
				}
				lineStart = true
				continue
			}
		}
		lineStart = false
		if strings.HasPrefix(input[i:], delimiter) {
			if err := emit(); err != nil {
				return nil, err
			}
			i += len(delimiter)
			continue
		}
		c := input[i]
		if c == '\n' {
			lineStart = true
			b.WriteByte(c)
			i++
			continue
		}
		if c == ' ' || c == '\r' || c == '\t' {
			b.WriteByte(c)
			i++
			continue
		}
		// PostgreSQL permits nested block comments. Other dialects do not.
		if strings.HasPrefix(input[i:], "/*") {
			start := i
			i += 2
			depth := 1
			if i < len(input) && (input[i] == '!' || input[i] == '+' || strings.HasPrefix(input[i:], "M!")) && dialect == "mysql" {
				hasCode = true
			}
			for i < len(input) && depth > 0 {
				if i%4096 == 0 && ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if dialect == "postgres" && strings.HasPrefix(input[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(input[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, ErrSyntax
			}
			b.WriteString(input[start:i])
			continue
		}
		dash := strings.HasPrefix(input[i:], "--") && (dialect != "mysql" || i+2 == len(input) || input[i+2] <= ' ')
		if dash || (c == '#' && dialect == "mysql") {
			end := strings.IndexByte(input[i:], '\n')
			if end < 0 {
				end = len(input) - i
			}
			b.WriteString(input[i : i+end])
			i += end
			continue
		}
		hasCode = true
		if c == '\'' || c == '"' || (c == '`' && dialect != "postgres") || (c == '[' && dialect == "sqlite") {
			start := i
			quote := c
			close := c
			if c == '[' {
				close = ']'
			}
			i++
			closed := false
			escape := dialect == "mysql" || (dialect == "postgres" && quote == '\'' && start > 0 && (input[start-1] == 'e' || input[start-1] == 'E') && (start == 1 || !word(input[start-2])))
			for i < len(input) {
				if i%4096 == 0 && ctx.Err() != nil {
					return nil, ctx.Err()
				}
				if escape && input[i] == '\\' {
					i += 2
					continue
				}
				if input[i] == close {
					i++
					if quote != '[' && i < len(input) && input[i] == close {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed || i > len(input) {
				return nil, ErrSyntax
			}
			b.WriteString(input[start:i])
			continue
		}
		if c == '$' && dialect == "postgres" && (i == 0 || !word(input[i-1])) {
			end := dollarEnd(input, i)
			if end < len(input) && input[end] == '$' {
				tag := input[i : end+1]
				n := strings.Index(input[end+1:], tag)
				if n < 0 {
					return nil, ErrSyntax
				}
				stop := end + 1 + n + len(tag)
				b.WriteString(input[i:stop])
				i = stop
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := emit(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, ErrSyntax
	}
	return result, nil
}
func letter(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func word(c byte) bool   { return letter(c) || c >= '0' && c <= '9' || c == '$' || c >= 128 }
func validDelimiter(s string) bool {
	if len(s) == 0 || len(s) > 16 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(";!%&*+,-./:<=>?@^|~$", c) {
			return false
		}
	}
	return !strings.HasPrefix(s, "--") && !strings.HasPrefix(s, "/*")
}

// Keyword skips ordinary leading comments. Executable MySQL comments are not
// interpreted as keywords, so restricted modes reject them conservatively.
func Keyword(s string, dialects ...string) string {
	dialect := "postgres"
	if len(dialects) > 0 {
		dialect = dialects[0]
	}
	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "--") && (dialect != "mysql" || len(s) == 2 || s[2] <= ' ') || (dialect == "mysql" && strings.HasPrefix(s, "#")) {
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return ""
			}
			s = s[i+1:]
			continue
		}
		if strings.HasPrefix(s, "/*") {
			if strings.HasPrefix(s, "/*!") || strings.HasPrefix(s, "/*+") || (dialect == "mysql" && strings.HasPrefix(s, "/*M!")) {
				return ""
			}
			i, depth := 2, 1
			for i < len(s) && depth > 0 {
				if dialect == "postgres" && strings.HasPrefix(s[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(s[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return ""
			}
			s = s[i:]
			continue
		}
		break
	}
	i := 0
	for i < len(s) && letter(s[i]) {
		i++
	}
	return strings.ToUpper(s[:i])
}

// dollarEnd accepts PostgreSQL's non-ASCII identifier bytes in dollar tags.
func dollarEnd(s string, start int) int {
	end := start + 1
	for end < len(s) && (letter(s[end]) || s[end] >= 128 || (end > start+1 && s[end] >= '0' && s[end] <= '9')) {
		end++
	}
	return end
}

// HasKeyword ignores quoted values, identifiers and comments. It is lexical,
// not a grammar classifier; a matching identifier may be conservative.
func HasKeyword(s, keyword, dialect string) bool {
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "/*") {
			depth := 1
			i += 2
			for i < len(s) && depth > 0 {
				if dialect == "postgres" && strings.HasPrefix(s[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(s[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(s[i:], "--") && (dialect != "mysql" || i+2 == len(s) || s[i+2] <= ' ') || (s[i] == '#' && dialect == "mysql") {
			n := strings.IndexByte(s[i:], '\n')
			if n < 0 {
				return false
			}
			i += n + 1
			continue
		}
		c := s[i]
		if c == '\'' || c == '"' || c == '`' && dialect != "postgres" || c == '[' && dialect == "sqlite" {
			quote := c
			close := c
			if c == '[' {
				close = ']'
			}
			escape := dialect == "mysql" || (dialect == "postgres" && c == '\'' && i > 0 && (s[i-1] == 'e' || s[i-1] == 'E') && (i == 1 || !word(s[i-2])))
			i++
			for i < len(s) {
				if escape && s[i] == '\\' {
					i += 2
					continue
				}
				if s[i] == close {
					i++
					if quote != '[' && i < len(s) && s[i] == close {
						i++
						continue
					}
					break
				}
				i++
			}
			continue
		}
		if c == '$' && dialect == "postgres" && (i == 0 || !word(s[i-1])) {
			end := dollarEnd(s, i)
			if end < len(s) && s[end] == '$' {
				tag := s[i : end+1]
				n := strings.Index(s[end+1:], tag)
				if n < 0 {
					return false
				}
				i = end + 1 + n + len(tag)
				continue
			}
		}
		if letter(c) || c >= 128 {
			start := i
			i++
			for i < len(s) && word(s[i]) {
				i++
			}
			if strings.EqualFold(s[start:i], keyword) {
				return true
			}
			continue
		}
		i++
	}
	return false
}
