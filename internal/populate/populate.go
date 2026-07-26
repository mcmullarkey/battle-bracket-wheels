// Package populate parses tolerant list input (Google-Docs-style) into entries
// and distributes them evenly across 8 wheels via round-robin.
//
// This package is pure — no I/O, no mutation, no HTTP, no session access.
// It imports only internal/wheel for the Wheel and Option types.
// All functions are deterministic: same inputs → same outputs.
//
// Responsibilities:
//   - Parse tolerant text input into ordered entries
//   - Distribute entries round-robin across 8 wheels (entry[i] → wheel[i%8])
//
// NOT responsible for:
//   - HTTP handling or endpoint wiring (see AC-5 handler)
//   - Session mutation or bracket reset (see AC-5 handler)
//   - UI rendering or template selection (see AC-6)
package populate

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"battle-bracket-wheels/internal/wheel"
)

// ErrTooFewEntries is returned when ParseAndDistribute parses fewer than 8
// entries. The error message contains "at least 8 entries" so callers can
// surface a user-friendly message. The returned Result.Entries is still
// populated (non-nil) so callers can inspect what was parsed.
var ErrTooFewEntries = errors.New("need at least 8 entries to populate 8 wheels")

// Result is the output of ParseAndDistribute: 8 wheels populated round-robin
// and the flat list of parsed entries (blanks skipped, no dedup).
//
// The fixed-size [8]wheel.Wheel array enforces exactly 8 wheels at the type
// level — callers cannot accidentally produce a 7- or 9-wheel result.
type Result struct {
	Wheels  [8]wheel.Wheel
	Entries []string
}

// ParseAndDistribute parses tolerant text input into entries and distributes
// them evenly across 8 wheels via round-robin (entry[i] → wheel[i%8]).
//
// Parser tolerance:
//   - Delimiters: newlines (\n, \r), commas, tabs
//   - Bullets (•, -, *) stripped when followed by whitespace or sole char
//   - Numbering (N. or (N)) stripped when followed by whitespace
//   - Blank lines skipped (do not count as entries)
//   - Leading/trailing whitespace trimmed
//   - Internal spaces preserved
//   - Duplicates preserved (no dedup)
//
// Returns ErrTooFewEntries if fewer than 8 entries are parsed. On error,
// Result.Entries is still populated (non-nil) so callers can inspect what was
// parsed. Result.Wheels is zero-valued (empty options) on error.
//
// Pure: same input → same output. No I/O, no side effects, no patches needed
// for testing.
func ParseAndDistribute(input string) (Result, error) {
	entries := parse(input)
	if entries == nil {
		entries = []string{}
	}

	if len(entries) < 8 {
		return Result{Entries: entries}, ErrTooFewEntries
	}

	var wheels [8]wheel.Wheel
	for i := range wheels {
		wheels[i] = wheel.Wheel{ID: fmt.Sprint(i)}
	}
	for i, entry := range entries {
		idx := i % 8
		wheels[idx].Options = append(
			wheels[idx].Options,
			wheel.Option{Text: entry},
		)
	}

	return Result{Wheels: wheels, Entries: entries}, nil
}

// parse splits tolerant list input into ordered entries.
//
// Delimiters: newlines (\n, \r), commas, tabs. Consecutive delimiters
// collapse (no empty tokens). Each token is cleaned via cleanToken; tokens
// that clean to empty are skipped. Returns nil if no entries are found.
func parse(input string) []string {
	tokens := strings.FieldsFunc(input, isDelimiter)

	var entries []string
	for _, tok := range tokens {
		if cleaned := cleanToken(tok); cleaned != "" {
			entries = append(entries, cleaned)
		}
	}
	return entries
}

// isDelimiter reports whether r is a list delimiter: newline, carriage
// return, comma, or tab. Spaces are NOT delimiters (internal spaces within
// an entry are preserved).
func isDelimiter(r rune) bool {
	return r == '\n' || r == '\r' || r == ',' || r == '\t'
}

// cleanToken trims whitespace, strips a leading bullet or numbering prefix,
// and trims again. Returns the cleaned token or "" if nothing remains.
func cleanToken(tok string) string {
	s := strings.TrimSpace(tok)
	s = stripBullet(s)
	s = strings.TrimSpace(s)
	s = stripNumbering(s)
	s = strings.TrimSpace(s)
	return s
}

// stripBullet removes a leading bullet character (•, -, *) when it is
// followed by whitespace or is the sole character. This avoids mangling
// tokens like "-5" or "-negative" where the dash is part of the text.
//
// The unicode bullet U+2022 (•) is always stripped (unambiguous marker).
// Dash (-) and star (*) are stripped only when followed by whitespace,
// preserving negative numbers and dash-leading words.
func stripBullet(s string) string {
	if s == "" {
		return s
	}

	// Unicode bullet U+2022 (•) — always a bullet marker.
	if strings.HasPrefix(s, "•") {
		return strings.TrimPrefix(s, "•")
	}

	// ASCII bullets: dash (-) and star (*).
	// Strip only when followed by whitespace or sole character.
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "*") {
		rest := s[1:]
		if rest == "" || startsWithSpace(rest) {
			return rest
		}
	}
	return s
}

// stripNumbering removes a leading numbering prefix: "N." or "(N)" when
// followed by whitespace or at end of token. This avoids mangling tokens
// like "1.5" or "2.0" where the period is a decimal point, not a list marker.
func stripNumbering(s string) string {
	if s == "" {
		return s
	}

	// Pattern: (N) followed by whitespace or end.
	if s[0] == '(' {
		if close := strings.IndexByte(s, ')'); close > 1 {
			if isAllDigits(s[1:close]) {
				rest := s[close+1:]
				if rest == "" || startsWithSpace(rest) {
					return rest
				}
			}
		}
	}

	// Pattern: N. followed by whitespace or end.
	if dot := strings.IndexByte(s, '.'); dot > 0 {
		if isAllDigits(s[:dot]) {
			rest := s[dot+1:]
			if rest == "" || startsWithSpace(rest) {
				return rest
			}
		}
	}

	return s
}

// startsWithSpace reports whether the first rune of s is Unicode whitespace.
func startsWithSpace(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsSpace(r)
}

// isAllDigits reports whether s is non-empty and every byte is an ASCII digit.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
