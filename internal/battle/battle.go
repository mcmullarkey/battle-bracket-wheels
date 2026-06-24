// Package battle provides pure match resolution logic: rolling, tiebreaking,
// and absorbing the loser's landed option into the winner's wheel.
//
// This package does NOT import math/rand or any I/O — it is a pure computation
// core. Randomness is injected via RollFunc (typically backed by crypto/rand in
// the handler layer).
package battle

import (
	"errors"
	"fmt"

	"battle-bracket-wheels/internal/wheel"
)

// RollFunc is a function that returns a pseudo-random integer in [1, 100].
// In production, the handler uses math/rand seeded from crypto/rand.
// Tests inject sequenced or fixed rollers for determinism.
type RollFunc func() int

// BattleResult bundles the outcome of a single match between two wheels.
type BattleResult struct {
	WinnerID, LoserID     string
	WinnerRoll, LoserRoll int
	WinnerLanded, LoserLanded wheel.Option
	Ties                  int
}

// ErrTiebreakerExhausted is returned when a match cannot be decided within
// the allowed number of tiebreaker re-rolls (maxTies).
var ErrTiebreakerExhausted = errors.New("tiebreaker exhausted")

// ResolveBattle resolves a single match between two landed options.
//
// It calls roll() twice per round: once for contestant A and once for B.
// The higher roll wins. On a tie, it re-rolls up to maxTies times.
// If all attempts tie, ErrTiebreakerExhausted is returned.
//
// Rolls outside [1, 100] produce a non-nil error.
//
// This function is pure: same inputs → same outputs. No I/O, no mutation.
func ResolveBattle(landedA, landedB wheel.Option, idA, idB string, roll RollFunc, maxTies int) (BattleResult, error) {
	ties := 0
	for ties < maxTies {
		rollA := roll()
		rollB := roll()

		if rollA < 1 || rollA > 100 || rollB < 1 || rollB > 100 {
			return BattleResult{}, fmt.Errorf("roll out of valid range 1-100: got %d, %d", rollA, rollB)
		}

		if rollA > rollB {
			return BattleResult{
				WinnerID:     idA,
				LoserID:      idB,
				WinnerRoll:   rollA,
				LoserRoll:    rollB,
				WinnerLanded: landedA,
				LoserLanded:  landedB,
				Ties:         ties,
			}, nil
		}
		if rollB > rollA {
			return BattleResult{
				WinnerID:     idB,
				LoserID:      idA,
				WinnerRoll:   rollB,
				LoserRoll:    rollA,
				WinnerLanded: landedB,
				LoserLanded:  landedA,
				Ties:         ties,
			}, nil
		}

		ties++
	}

	return BattleResult{Ties: ties}, ErrTiebreakerExhausted
}

// AbsorbOption appends the loser's landed option to the winner's wheel if its
// text is not already present (deduplication by text). The appended option
// has a nil Weight, meaning it gets an equal share in future spins.
//
// Returns a new Wheel without mutating the input.
func AbsorbOption(winner wheel.Wheel, loserLanded wheel.Option) wheel.Wheel {
	// Deduplicate by text: skip if text already exists on winner
	for _, opt := range winner.Options {
		if opt.Text == loserLanded.Text {
			// Already present — return unchanged
			return winner
		}
	}

	// Append with nil weight
	newOpts := make([]wheel.Option, len(winner.Options)+1)
	copy(newOpts, winner.Options)
	newOpts[len(newOpts)-1] = wheel.Option{Text: loserLanded.Text, Weight: nil}
	return wheel.Wheel{ID: winner.ID, Options: newOpts}
}
