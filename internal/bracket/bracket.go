// Package bracket provides the bracket progression state machine: quarter-final
// through final, with winner propagation, loser-slice absorption, dependency
// gating, and idempotency gating.
//
// This package is pure — no I/O, no randomness, no HTTP concerns.
// All exported functions are deterministic: same inputs → same outputs.
//
// Responsibilities:
//   - MatchID typed constants (QF1–QF4, SF1, SF2, Final)
//   - Bracket struct with positional progression slots
//   - ApplyBattleResult for state transitions with gating
//
// NOT responsible for:
//   - Spinning wheels or resolving battles (see internal/battle)
//   - Persistent storage, serialization, or HTTP rendering
//   - Randomness generation or seeding
package bracket

import (
	"errors"
	"fmt"

	"battle-bracket-wheels/internal/battle"
	"battle-bracket-wheels/internal/wheel"
)

// MatchID uniquely identifies a match position within a bracket.
type MatchID string

const (
	MatchQF1   MatchID = "qf1"
	MatchQF2   MatchID = "qf2"
	MatchQF3   MatchID = "qf3"
	MatchQF4   MatchID = "qf4"
	MatchSF1   MatchID = "sf1"
	MatchSF2   MatchID = "sf2"
	MatchFinal MatchID = "final"
)

// ErrDependencyNotMet is returned when ApplyBattleResult is called before
// the required dependency slots are filled.
var ErrDependencyNotMet = errors.New("dependency slots not filled")

// ErrAlreadyResolved is returned when ApplyBattleResult is called on a
// match that has already been resolved (target next-round slot is filled).
var ErrAlreadyResolved = errors.New("match already resolved")

// ErrUnknownMatch is returned when ApplyBattleResult receives a MatchID
// that does not correspond to any known match.
var ErrUnknownMatch = errors.New("unknown match ID")

// FinalResult holds the outcome of the final match: the absorbed winner wheel
// and the text of the landed slice that determined the movie.
type FinalResult struct {
	Wheel        wheel.Wheel
	LandedOption wheel.Option
}

// Bracket encodes the full bracket progression state: initial QF wheel entries,
// semi-final left/right fields, final left/right fields, and the final winner.
//
// Pointer fields are nil when the corresponding slot has not yet been filled.
// Illegal states (e.g., FillLeft set but FinalLeft occupying the same semantic
// position) are unrepresentable — each pointer field maps to exactly one
// match position.
type Bracket struct {
	Slots      [8]wheel.Wheel
	SFLeft     [2]*wheel.Wheel // [0]←QF1 winner, [1]←QF3 winner
	SFRight    [2]*wheel.Wheel // [0]←QF2 winner, [1]←QF4 winner
	FinalLeft  *wheel.Wheel    // ←SF1 winner
	FinalRight *wheel.Wheel    // ←SF2 winner
	Winner     *FinalResult    // ←Final winner
}

// New creates a Bracket initialized with the given 8 wheels as the QF entries.
// The wheels are copied into the Bracket's Slots field. SF/Final slots are nil.
func New(wheels [8]wheel.Wheel) *Bracket {
	return &Bracket{
		Slots: wheels,
	}
}

// ApplyBattleResult advances the bracket state for the given match, given the
// battle result and the two participating wheels. It performs dependency gating
// (required input slots must be filled), idempotency gating (target slot must
// be empty), absorbs the loser's landed option into the winner's wheel via
// battle.AbsorbOption, and places the result in the correct next-round slot.
//
// The whA/whB parameters are the two wheels participating in the match. Their
// IDs should match the WinnerID in the BattleResult.
//
// Returns ErrDependencyNotMet, ErrAlreadyResolved, or ErrUnknownMatch on
// failure. On success, returns nil and the Bracket state is mutated in place.
func (b *Bracket) ApplyBattleResult(mid MatchID, br battle.BattleResult, whA, whB wheel.Wheel) error {
	// Dependency gate: validate required slots are filled
	if err := b.ValidateDependencies(mid); err != nil {
		return err
	}

	// Determine which wheel is the winner
	var winnerWheel wheel.Wheel
	if br.WinnerID == whA.ID {
		winnerWheel = whA
	} else {
		winnerWheel = whB
	}

	absorbed := battle.AbsorbOption(winnerWheel, br.LoserLanded)

	switch mid {
	case MatchQF1:
		if b.SFLeft[0] != nil {
			return fmt.Errorf("%w: QF1 already resolved, SFLeft[0] filled", ErrAlreadyResolved)
		}
		b.SFLeft[0] = &absorbed
	case MatchQF2:
		if b.SFRight[0] != nil {
			return fmt.Errorf("%w: QF2 already resolved, SFRight[0] filled", ErrAlreadyResolved)
		}
		b.SFRight[0] = &absorbed
	case MatchQF3:
		if b.SFLeft[1] != nil {
			return fmt.Errorf("%w: QF3 already resolved, SFLeft[1] filled", ErrAlreadyResolved)
		}
		b.SFLeft[1] = &absorbed
	case MatchQF4:
		if b.SFRight[1] != nil {
			return fmt.Errorf("%w: QF4 already resolved, SFRight[1] filled", ErrAlreadyResolved)
		}
		b.SFRight[1] = &absorbed
	case MatchSF1:
		if b.FinalLeft != nil {
			return fmt.Errorf("%w: SF1 already resolved, FinalLeft filled", ErrAlreadyResolved)
		}
		b.FinalLeft = &absorbed
	case MatchSF2:
		if b.FinalRight != nil {
			return fmt.Errorf("%w: SF2 already resolved, FinalRight filled", ErrAlreadyResolved)
		}
		b.FinalRight = &absorbed
	case MatchFinal:
		if b.Winner != nil {
			return fmt.Errorf("%w: Final already resolved, Winner filled", ErrAlreadyResolved)
		}
		b.Winner = &FinalResult{Wheel: absorbed, LandedOption: br.WinnerLanded}
	default:
		return fmt.Errorf("%w: %s", ErrUnknownMatch, mid)
	}

	return nil
}

// ValidateDependencies checks that the required dependency slots are filled
// for the given match. Returns ErrDependencyNotMet if a required slot is empty.
//
// Dependencies:
//   - QF: the corresponding Slots positions must have at least one option each
//   - SF1: SFLeft[0] and SFRight[0] both non-nil
//   - SF2: SFLeft[1] and SFRight[1] both non-nil
//   - Final: FinalLeft and FinalRight both non-nil
func (b *Bracket) ValidateDependencies(mid MatchID) error {
	switch mid {
	case MatchQF1:
		if len(b.Slots[0].Options) == 0 || len(b.Slots[1].Options) == 0 {
			return fmt.Errorf("%w: QF1 needs Slots[0] and Slots[1] filled", ErrDependencyNotMet)
		}
	case MatchQF2:
		if len(b.Slots[2].Options) == 0 || len(b.Slots[3].Options) == 0 {
			return fmt.Errorf("%w: QF2 needs Slots[2] and Slots[3] filled", ErrDependencyNotMet)
		}
	case MatchQF3:
		if len(b.Slots[4].Options) == 0 || len(b.Slots[5].Options) == 0 {
			return fmt.Errorf("%w: QF3 needs Slots[4] and Slots[5] filled", ErrDependencyNotMet)
		}
	case MatchQF4:
		if len(b.Slots[6].Options) == 0 || len(b.Slots[7].Options) == 0 {
			return fmt.Errorf("%w: QF4 needs Slots[6] and Slots[7] filled", ErrDependencyNotMet)
		}
	case MatchSF1:
		if b.SFLeft[0] == nil || b.SFRight[0] == nil {
			return fmt.Errorf("%w: SF1 needs SFLeft[0] and SFRight[0] filled", ErrDependencyNotMet)
		}
	case MatchSF2:
		if b.SFLeft[1] == nil || b.SFRight[1] == nil {
			return fmt.Errorf("%w: SF2 needs SFLeft[1] and SFRight[1] filled", ErrDependencyNotMet)
		}
	case MatchFinal:
		if b.FinalLeft == nil || b.FinalRight == nil {
			return fmt.Errorf("%w: Final needs FinalLeft and FinalRight filled", ErrDependencyNotMet)
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnknownMatch, mid)
	}
	return nil
}

// SlotMapping returns the target HTML slot ID for the next-round slot
// that a given match's result populates.
//
// Mapping:
//   - QF1 → slot-sf1-left
//   - QF2 → slot-sf1-right
//   - QF3 → slot-sf2-left
//   - QF4 → slot-sf2-right
//   - SF1 → slot-final-left
//   - SF2 → slot-final-right
//   - Final → "" (no next-round slot; movieResult is the OOB target)
func SlotMapping(mid MatchID) string {
	switch mid {
	case MatchQF1:
		return "slot-sf1-left"
	case MatchQF2:
		return "slot-sf1-right"
	case MatchQF3:
		return "slot-sf2-left"
	case MatchQF4:
		return "slot-sf2-right"
	case MatchSF1:
		return "slot-final-left"
	case MatchSF2:
		return "slot-final-right"
	case MatchFinal:
		return "" // Final has no next-round slot; movieResult is the OOB target
	default:
		return ""
	}
}
