// Package battle_test contains pure unit tests for the battle resolution logic.
package battle_test

import (
	"errors"
	"testing"

	"battle-bracket-wheels/internal/battle"
	"battle-bracket-wheels/internal/wheel"
)

// sequencedRoller returns a RollFunc that returns values from vals in sequence.
// It panics if called more times than len(vals).
func sequencedRoller(vals ...int) battle.RollFunc {
	i := 0
	return func() int {
		if i >= len(vals) {
			panic("sequencedRoller exhausted")
		}
		v := vals[i]
		i++
		return v
	}
}

// fixedRoller returns a RollFunc that always returns v.
func fixedRoller(v int) battle.RollFunc {
	return func() int { return v }
}

// countingRoller returns a RollFunc that records the number of calls
// and alternates high/low so the first round produces a winner (not a tie).
func countingRoller() (battle.RollFunc, *int) {
	var count int
	return func() int {
		count++
		// Alternate so roll A > roll B: 70 vs 30, then subsequent rounds if needed
		if count%2 == 1 {
			return 70
		}
		return 30
	}, &count
}

func TestResolveBattle_AWins(t *testing.T) {
	optA := wheel.Option{Text: "A", Weight: ptr(1.0)}
	optB := wheel.Option{Text: "B", Weight: ptr(1.0)}

	result, err := battle.ResolveBattle(optA, optB, "idA", "idB", sequencedRoller(70, 30), 100)
	if err != nil {
		t.Fatalf("ResolveBattle returned error: %v", err)
	}
	if result.WinnerID != "idA" {
		t.Errorf("WinnerID = %q, want %q", result.WinnerID, "idA")
	}
	if result.LoserID != "idB" {
		t.Errorf("LoserID = %q, want %q", result.LoserID, "idB")
	}
	if result.WinnerRoll != 70 {
		t.Errorf("WinnerRoll = %d, want %d", result.WinnerRoll, 70)
	}
	if result.LoserRoll != 30 {
		t.Errorf("LoserRoll = %d, want %d", result.LoserRoll, 30)
	}
	if result.WinnerLanded != optA {
		t.Errorf("WinnerLanded = %v, want %v", result.WinnerLanded, optA)
	}
	if result.LoserLanded != optB {
		t.Errorf("LoserLanded = %v, want %v", result.LoserLanded, optB)
	}
	if result.Ties != 0 {
		t.Errorf("Ties = %d, want %d", result.Ties, 0)
	}
}

func TestResolveBattle_BWins(t *testing.T) {
	optA := wheel.Option{Text: "A", Weight: ptr(1.0)}
	optB := wheel.Option{Text: "B", Weight: ptr(1.0)}

	result, err := battle.ResolveBattle(optA, optB, "idA", "idB", sequencedRoller(30, 70), 100)
	if err != nil {
		t.Fatalf("ResolveBattle returned error: %v", err)
	}
	if result.WinnerID != "idB" {
		t.Errorf("WinnerID = %q, want %q (catches hardcoded-A)", result.WinnerID, "idB")
	}
	if result.LoserID != "idA" {
		t.Errorf("LoserID = %q, want %q", result.LoserID, "idA")
	}
	if result.WinnerRoll != 70 {
		t.Errorf("WinnerRoll = %d, want %d", result.WinnerRoll, 70)
	}
	if result.LoserRoll != 30 {
		t.Errorf("LoserRoll = %d, want %d", result.LoserRoll, 30)
	}
	if result.WinnerLanded != optB {
		t.Errorf("WinnerLanded = %v, want %v (should be idB's landed)", result.WinnerLanded, optB)
	}
	if result.LoserLanded != optA {
		t.Errorf("LoserLanded = %v, want %v (should be idA's landed)", result.LoserLanded, optA)
	}
}

func TestResolveBattle_Tiebreaker(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	// 50,50 tie → re-roll; then 60,40 → A wins
	result, err := battle.ResolveBattle(optA, optB, "idA", "idB", sequencedRoller(50, 50, 60, 40), 100)
	if err != nil {
		t.Fatalf("ResolveBattle returned error: %v", err)
	}
	if result.WinnerRoll == result.LoserRoll {
		t.Error("WinnerRoll == LoserRoll, expected different rolls after tiebreaker")
	}
	if result.Ties != 1 {
		t.Errorf("Ties = %d, want %d", result.Ties, 1)
	}
}

func TestResolveBattle_TiebreakerNoBias(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	// 50,50 tie → re-roll; then 40,60 → B wins (no bias toward idA)
	result, err := battle.ResolveBattle(optA, optB, "idA", "idB", sequencedRoller(50, 50, 40, 60), 100)
	if err != nil {
		t.Fatalf("ResolveBattle returned error: %v", err)
	}
	if result.WinnerID != "idB" {
		t.Errorf("WinnerID = %q, want %q (no tiebreaker bias toward idA)", result.WinnerID, "idB")
	}
}

func TestResolveBattle_RollRangeZero(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	// Roll returning 0 should be rejected
	_, err := battle.ResolveBattle(optA, optB, "idA", "idB", fixedRoller(0), 100)
	if err == nil {
		t.Error("expected error for roll=0, got nil")
	}
}

func TestResolveBattle_RollRange101(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	// Roll returning 101 should be rejected
	_, err := battle.ResolveBattle(optA, optB, "idA", "idB", fixedRoller(101), 100)
	if err == nil {
		t.Error("expected error for roll=101, got nil")
	}
}

func TestResolveBattle_Exhausted(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	// fixedRoller(50) always returns 50 → always ties
	result, err := battle.ResolveBattle(optA, optB, "idA", "idB", fixedRoller(50), 3)
	if !errors.Is(err, battle.ErrTiebreakerExhausted) {
		t.Fatalf("expected ErrTiebreakerExhausted, got %v", err)
	}
	if result.Ties != 3 {
		t.Errorf("Ties = %d, want %d", result.Ties, 3)
	}
}

func TestResolveBattle_UsesInjectedRollFunc(t *testing.T) {
	optA := wheel.Option{Text: "A"}
	optB := wheel.Option{Text: "B"}

	rollFn, count := countingRoller()
	_, err := battle.ResolveBattle(optA, optB, "idA", "idB", rollFn, 100)
	if err != nil {
		t.Fatalf("ResolveBattle returned error: %v", err)
	}
	if *count == 0 {
		t.Error("injected RollFunc was never called")
	}
}

func TestAbsorbOption_NewOption(t *testing.T) {
	winner := wheel.Wheel{ID: "0", Options: []wheel.Option{{Text: "Existing"}}}
	loserLanded := wheel.Option{Text: "NewOption"}

	result := battle.AbsorbOption(winner, loserLanded)

	if len(result.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(result.Options))
	}
	if result.Options[1].Text != "NewOption" {
		t.Errorf("option[1].Text = %q, want %q", result.Options[1].Text, "NewOption")
	}
	if result.Options[1].Weight != nil {
		t.Error("absorbed option should have nil weight")
	}
	// Original should not be mutated
	if len(winner.Options) != 1 {
		t.Error("original wheel was mutated")
	}
}

func TestAbsorbOption_Dedupe(t *testing.T) {
	winner := wheel.Wheel{ID: "0", Options: []wheel.Option{{Text: "Existing"}}}
	loserLanded := wheel.Option{Text: "Existing"}

	result := battle.AbsorbOption(winner, loserLanded)

	if len(result.Options) != 1 {
		t.Errorf("expected 1 option (deduped), got %d", len(result.Options))
	}
	// Original should not be mutated
	if len(winner.Options) != 1 {
		t.Error("original wheel was mutated")
	}
}

func TestAbsorbOption_WeightPreserved(t *testing.T) {
	w := ptr(2.0)
	winner := wheel.Wheel{ID: "0", Options: []wheel.Option{{Text: "Existing", Weight: w}}}
	loserLanded := wheel.Option{Text: "NewOption"}

	result := battle.AbsorbOption(winner, loserLanded)

	if result.Options[0].Weight == nil || *result.Options[0].Weight != 2.0 {
		t.Error("existing option's weight should be preserved")
	}
}

func ptr(f float64) *float64 { return &f }
