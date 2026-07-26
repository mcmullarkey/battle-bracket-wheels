// Package populate tests cover parser tolerance and round-robin distribution.
package populate

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"battle-bracket-wheels/internal/wheel"
)

// alphaIotaInput exercises every delimiter/prefix the spec names:
// commas, newlines, tabs, blank lines, bullets (•/-/*), numbering (N.),
// and leading/trailing whitespace. Parses to exactly 9 entries Alpha–Iota.
const alphaIotaInput = "Alpha, Beta\nGamma\tDelta\n\n• Epsilon\n1. Zeta\n2. Eta\n- Theta\n* Iota"

// wantAlphaIota is the expected parse order for alphaIotaInput.
var wantAlphaIota = []string{
	"Alpha", "Beta", "Gamma", "Delta",
	"Epsilon", "Zeta", "Eta", "Theta", "Iota",
}

func TestParse_Tolerance(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "tabs as delimiters",
			input: "x\ty\tz",
			want:  []string{"x", "y", "z"},
		},
		{
			name:  "bullets stripped (unicode bullet)",
			input: "• Apple\n• Banana",
			want:  []string{"Apple", "Banana"},
		},
		{
			name:  "dash bullets stripped",
			input: "- Cherry\n- Date",
			want:  []string{"Cherry", "Date"},
		},
		{
			name:  "star bullets stripped",
			input: "* Elder\n* Fig",
			want:  []string{"Elder", "Fig"},
		},
		{
			name:  "numbering N. stripped",
			input: "1. Foo\n2. Bar",
			want:  []string{"Foo", "Bar"},
		},
		{
			name:  "parenthesized numbering (N) stripped",
			input: "(1) Foo\n(2) Bar",
			want:  []string{"Foo", "Bar"},
		},
		{
			name:  "mixed delimiters in one input",
			input: "a,b\nc\n- d",
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "blank line between entries skipped",
			input: "Alpha\n\nBeta\n",
			want:  []string{"Alpha", "Beta"},
		},
		{
			name:  "multiple consecutive blank lines skipped",
			input: "Alpha\n\n\n\nBeta",
			want:  []string{"Alpha", "Beta"},
		},
		{
			name:  "duplicates preserved (no dedup)",
			input: "A\nA\nB",
			want:  []string{"A", "A", "B"},
		},
		{
			name:  "leading and trailing whitespace trimmed",
			input: "   Spaced   \n  Indented  ",
			want:  []string{"Spaced", "Indented"},
		},
		{
			name:  "internal spaces preserved",
			input: "New York\nLos Angeles",
			want:  []string{"New York", "Los Angeles"},
		},
		{
			name:  "carriage return newline normalized",
			input: "Alpha\r\nBeta\r\nGamma",
			want:  []string{"Alpha", "Beta", "Gamma"},
		},
		{
			name:  "trailing delimiters produce no empty entry",
			input: "Alpha,Beta,",
			want:  []string{"Alpha", "Beta"},
		},
		{
			name:  "empty string yields no entries",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace-only yields no entries",
			input: "   \n\t  \n",
			want:  nil,
		},
		{
			name:  "emoji entry preserved",
			input: "🚀 Rocket\n⭐ Star",
			want:  []string{"🚀 Rocket", "⭐ Star"},
		},
		{
			name:  "long entry preserved",
			input: strings.Repeat("A", 500),
			want:  []string{strings.Repeat("A", 500)},
		},
		{
			name:  "decimal number not mangled by numbering strip",
			input: "1.5\n2.0",
			want:  []string{"1.5", "2.0"},
		},
		{
			name:  "dash-leading text not mangled by bullet strip",
			input: "-5\n-negative",
			want:  []string{"-5", "-negative"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parse(tc.input)
			if !equalStrings(got, tc.want) {
				t.Fatalf("parse(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseAndDistribute_AlphaToIota is the primary spec predicate: 9 mixed-
// delimiter items distributed round-robin across 8 wheels with all 7
// acceptance criteria asserted.
func TestParseAndDistribute_AlphaToIota(t *testing.T) {
	res, err := ParseAndDistribute(alphaIotaInput)
	if err != nil {
		t.Fatalf("ParseAndDistribute returned unexpected error: %v", err)
	}

	// AC-1: result.Wheels[i].ID == fmt.Sprint(i) for all i in [0,7].
	for i := 0; i < 8; i++ {
		if res.Wheels[i].ID != fmt.Sprint(i) {
			t.Errorf("Wheels[%d].ID = %q, want %q", i, res.Wheels[i].ID, fmt.Sprint(i))
		}
	}

	// AC-2: result.Wheels[0].Options[0].Text == "Alpha" (round-robin start).
	if len(res.Wheels[0].Options) < 1 || res.Wheels[0].Options[0].Text != "Alpha" {
		t.Errorf("Wheels[0].Options[0] = %+v, want Text %q", firstOrZero(res.Wheels[0]), "Alpha")
	}

	// AC-3: result.Wheels[0].Options[1].Text == "Iota" (9th item overflows to wheel[0]).
	if len(res.Wheels[0].Options) < 2 || res.Wheels[0].Options[1].Text != "Iota" {
		t.Errorf("Wheels[0].Options[1] = %+v, want Text %q", secondOrZero(res.Wheels[0]), "Iota")
	}

	// AC-4: max(len) - min(len) <= 1 (even distribution).
	maxLen, minLen := optionLenExtremes(res.Wheels)
	if maxLen-minLen > 1 {
		t.Errorf("distribution uneven: max=%d, min=%d, diff=%d (want <= 1)", maxLen, minLen, maxLen-minLen)
	}

	// AC-5: result.Wheels[i].Options[j].Weight == nil for all i,j.
	for i, w := range res.Wheels {
		for j, opt := range w.Options {
			if opt.Weight != nil {
				t.Errorf("Wheels[%d].Options[%d].Weight = %v, want nil", i, j, *opt.Weight)
			}
		}
	}

	// AC-6: len(result.Entries) == 9 (blanks skipped, no dedup).
	if len(res.Entries) != 9 {
		t.Errorf("len(Entries) = %d, want 9", len(res.Entries))
	}

	// AC-7: result.Entries[0] == "Alpha" && result.Entries[8] == "Iota".
	if len(res.Entries) != 9 || res.Entries[0] != "Alpha" || res.Entries[8] != "Iota" {
		t.Errorf("Entries = %v, want [Alpha ... Iota] (9 items, order preserved)", res.Entries)
	}

	// Verify full parse order.
	if !equalStrings(res.Entries, wantAlphaIota) {
		t.Errorf("Entries = %v, want %v", res.Entries, wantAlphaIota)
	}
}

func TestParseAndDistribute_TooFewEntries(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "whitespace-only", input: "   \n\t  \n"},
		{name: "seven items", input: "one\ntwo\nthree\nfour\nfive\nsix\nseven"},
		{name: "single item", input: "solo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseAndDistribute(tc.input)
			if !errors.Is(err, ErrTooFewEntries) {
				t.Fatalf("ParseAndDistribute(%q) err = %v, want ErrTooFewEntries", tc.input, err)
			}
			if !strings.Contains(err.Error(), "at least 8 entries") {
				t.Errorf("err message %q must contain %q", err.Error(), "at least 8 entries")
			}
			// Entries must still be populated so callers can inspect what was parsed.
			if res.Entries == nil {
				t.Errorf("Entries should be populated even on error, got nil")
			}
		})
	}
}

// TestParseAndDistribute_SneakyBlankLine is the spec's negative case (b):
// "Alpha\n\nBeta\n" (1 blank line between 2 real entries) must yield
// len(Entries) == 2, NOT 3. If the parser counts blanks as entries, this fails.
func TestParseAndDistribute_SneakyBlankLine(t *testing.T) {
	res, err := ParseAndDistribute("Alpha\n\nBeta\n")
	if !errors.Is(err, ErrTooFewEntries) {
		t.Fatalf("expected ErrTooFewEntries for 2 entries, got %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2 (blank line must NOT count as entry); entries=%v",
			len(res.Entries), res.Entries)
	}
	if res.Entries[0] != "Alpha" || res.Entries[1] != "Beta" {
		t.Errorf("Entries = %v, want [Alpha Beta]", res.Entries)
	}
}

func TestParseAndDistribute_DuplicatesPreserved(t *testing.T) {
	// 8 entries with a duplicate A — no dedup, so 8 entries survive.
	res, err := ParseAndDistribute("A\nA\nB\nC\nD\nE\nF\nG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Entries) != 8 {
		t.Fatalf("len(Entries) = %d, want 8 (duplicates preserved)", len(res.Entries))
	}
	if res.Entries[0] != "A" || res.Entries[1] != "A" {
		t.Errorf("Entries[0:2] = %v, want [A A] (duplicates preserved)", res.Entries[:2])
	}
}

func TestParseAndDistribute_ExactlyEight(t *testing.T) {
	input := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight"
	res, err := ParseAndDistribute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, w := range res.Wheels {
		if len(w.Options) != 1 {
			t.Errorf("Wheels[%d] has %d options, want exactly 1", i, len(w.Options))
		}
	}
	if len(res.Entries) != 8 {
		t.Errorf("len(Entries) = %d, want 8", len(res.Entries))
	}
}

// TestParseAndDistribute_CyclicOverflow verifies >16 entries still distribute
// cyclically: 17 entries → wheel[0] gets 3 (indices 0,8,16), wheels 1-7 get 2.
func TestParseAndDistribute_CyclicOverflow(t *testing.T) {
	entries := make([]string, 17)
	for i := range entries {
		entries[i] = fmt.Sprintf("E%d", i)
	}
	input := strings.Join(entries, "\n")
	res, err := ParseAndDistribute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Wheels[0].Options) != 3 {
		t.Errorf("Wheels[0] has %d options, want 3 (indices 0,8,16)", len(res.Wheels[0].Options))
	}
	for i := 1; i < 8; i++ {
		if len(res.Wheels[i].Options) != 2 {
			t.Errorf("Wheels[%d] has %d options, want 2", i, len(res.Wheels[i].Options))
		}
	}
	// Verify round-robin placement: E0, E8, E16 on wheel[0].
	want0 := []string{"E0", "E8", "E16"}
	for j, want := range want0 {
		if res.Wheels[0].Options[j].Text != want {
			t.Errorf("Wheels[0].Options[%d].Text = %q, want %q", j, res.Wheels[0].Options[j].Text, want)
		}
	}
}

// TestParseAndDistribute_EvennessProperty is the property-based evenness
// assertion: for any entry count >= 8, max(len)-min(len) <= 1.
func TestParseAndDistribute_EvennessProperty(t *testing.T) {
	for n := 8; n <= 25; n++ {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			entries := make([]string, n)
			for i := range entries {
				entries[i] = fmt.Sprintf("item%d", i)
			}
			res, err := ParseAndDistribute(strings.Join(entries, "\n"))
			if err != nil {
				t.Fatalf("unexpected error for n=%d: %v", n, err)
			}
			maxLen, minLen := optionLenExtremes(res.Wheels)
			if maxLen-minLen > 1 {
				t.Errorf("n=%d: max=%d min=%d diff=%d, want <= 1", n, maxLen, minLen, maxLen-minLen)
			}
			// Total options must equal total entries (nothing dropped).
			total := 0
			for _, w := range res.Wheels {
				total += len(w.Options)
			}
			if total != n {
				t.Errorf("n=%d: total options=%d, want %d (entries dropped)", n, total, n)
			}
		})
	}
}

func TestParseAndDistribute_AllWeightsNil(t *testing.T) {
	input := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj"
	res, err := ParseAndDistribute(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, w := range res.Wheels {
		for j, opt := range w.Options {
			if opt.Weight != nil {
				t.Errorf("Wheels[%d].Options[%d].Weight = %v, want nil", i, j, *opt.Weight)
			}
		}
	}
}

func TestParseAndDistribute_WheelIDs(t *testing.T) {
	res, err := ParseAndDistribute("a\nb\nc\nd\ne\nf\ng\nh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 8; i++ {
		if res.Wheels[i].ID != fmt.Sprint(i) {
			t.Errorf("Wheels[%d].ID = %q, want %q", i, res.Wheels[i].ID, fmt.Sprint(i))
		}
	}
}

// TestParseAndDistribute_PureNoMutation ensures calling ParseAndDistribute
// twice on the same input yields equal, independent results (no shared state).
func TestParseAndDistribute_PureNoMutation(t *testing.T) {
	res1, err1 := ParseAndDistribute(alphaIotaInput)
	res2, err2 := ParseAndDistribute(alphaIotaInput)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	for i := 0; i < 8; i++ {
		if res1.Wheels[i].ID != res2.Wheels[i].ID {
			t.Errorf("Wheels[%d].ID differs across calls: %q vs %q", i, res1.Wheels[i].ID, res2.Wheels[i].ID)
		}
		if len(res1.Wheels[i].Options) != len(res2.Wheels[i].Options) {
			t.Errorf("Wheels[%d] option count differs: %d vs %d", i, len(res1.Wheels[i].Options), len(res2.Wheels[i].Options))
		}
	}
}

// --- helpers ---

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Treat nil and empty as equal for comparison purposes.
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func optionLenExtremes(wheels [8]wheel.Wheel) (maxLen, minLen int) {
	maxLen = 0
	minLen = len(wheels[0].Options)
	for _, w := range wheels {
		n := len(w.Options)
		if n > maxLen {
			maxLen = n
		}
		if n < minLen {
			minLen = n
		}
	}
	return maxLen, minLen
}

func firstOrZero(w wheel.Wheel) wheel.Option {
	if len(w.Options) > 0 {
		return w.Options[0]
	}
	return wheel.Option{}
}

func secondOrZero(w wheel.Wheel) wheel.Option {
	if len(w.Options) > 1 {
		return w.Options[1]
	}
	return wheel.Option{}
}
