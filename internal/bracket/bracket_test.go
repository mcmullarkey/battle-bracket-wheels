package bracket_test

import (
	"errors"
	"testing"

	"battle-bracket-wheels/internal/battle"
	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/wheel"
)

// seededWheels creates 8 wheels with distinct IDs and the given option texts.
// Each wheel gets one option from the options slice; extra wheels get no options.
func seededWheels(optionTexts ...string) [8]wheel.Wheel {
	var wh [8]wheel.Wheel
	for i := range wh {
		wh[i] = wheel.Wheel{ID: string(rune('0' + i))}
	}
	for i, text := range optionTexts {
		if i < 8 {
			wh[i] = wheel.AddOption(wh[i], text, nil)
		}
	}
	return wh
}

// makeBattleResult creates a simple BattleResult given winner/loser IDs and options.
func makeBattleResult(winnerID, loserID string, winnerLanded, loserLanded wheel.Option) battle.BattleResult {
	return battle.BattleResult{
		WinnerID:     winnerID,
		LoserID:      loserID,
		WinnerLanded: winnerLanded,
		LoserLanded:  loserLanded,
		WinnerRoll:   70,
		LoserRoll:    30,
	}
}

func TestApplyBattleResult_QF1(t *testing.T) {
	wheels := seededWheels("A", "B")
	b := bracket.New(wheels)

	br := makeBattleResult("0", "1", wheel.Option{Text: "A"}, wheel.Option{Text: "B"})
	err := b.ApplyBattleResult(bracket.MatchQF1, br, wheels[0], wheels[1])
	if err != nil {
		t.Fatalf("ApplyBattleResult QF1: %v", err)
	}

	if b.SFLeft[0] == nil {
		t.Fatal("SFLeft[0] is nil after QF1")
	}

	// SFLeft[0] should contain wheel 0's options + loser's landed text
	foundB := false
	foundA := false
	for _, opt := range b.SFLeft[0].Options {
		if opt.Text == "B" {
			foundB = true
		}
		if opt.Text == "A" {
			foundA = true
		}
	}
	if !foundA {
		t.Error("SFLeft[0] missing winner option 'A'")
	}
	if !foundB {
		t.Error("SFLeft[0] missing absorbed loser text 'B' (not original wheel)")
	}
}

func TestApplyBattleResult_AbsorbedNotOriginal(t *testing.T) {
	// Sneaky-pass guard: assertion that SFLeft[0].Options contains loserLanded.Text
	// This catches (a): copies winner.OriginalWheel not absorbed
	wheels := seededWheels("Bicycle", "Skateboard")
	b := bracket.New(wheels)

	br := makeBattleResult("0", "1", wheel.Option{Text: "Bicycle"}, wheel.Option{Text: "Skateboard"})
	err := b.ApplyBattleResult(bracket.MatchQF1, br, wheels[0], wheels[1])
	if err != nil {
		t.Fatalf("ApplyBattleResult: %v", err)
	}

	if b.SFLeft[0] == nil {
		t.Fatal("SFLeft[0] is nil")
	}

	// The winner (wheel 0, "Bicycle") — the absorbed wheel should include loser's text "Skateboard"
	hasLoserText := false
	for _, opt := range b.SFLeft[0].Options {
		if opt.Text == "Skateboard" {
			hasLoserText = true
			break
		}
	}
	if !hasLoserText {
		t.Error("SFLeft[0] does not contain loser's landed text 'Skateboard' — winner was not absorbed")
	}
}

func TestApplyBattleResult_Dedupe(t *testing.T) {
	// Sneaky-pass guard (b): overlapping texts, assert countByText==1
	wheels := seededWheels("Movie", "Movie") // same text on both wheels
	b := bracket.New(wheels)

	br := makeBattleResult("0", "1", wheel.Option{Text: "Movie"}, wheel.Option{Text: "Movie"})
	err := b.ApplyBattleResult(bracket.MatchQF1, br, wheels[0], wheels[1])
	if err != nil {
		t.Fatalf("ApplyBattleResult: %v", err)
	}

	if b.SFLeft[0] == nil {
		t.Fatal("SFLeft[0] is nil")
	}

	// "Movie" should appear exactly once (deduped)
	count := 0
	for _, opt := range b.SFLeft[0].Options {
		if opt.Text == "Movie" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("'Movie' appears %d times, want 1 (dedupe failed)", count)
	}
}

func TestApplyBattleResult_PositionalMapping(t *testing.T) {
	// Sneaky-pass guard (c): after QF3, assert SFLeft[1] filled not SFLeft[0]
	wheels := seededWheels("A", "B", "C", "D", "E", "F")
	b := bracket.New(wheels)

	// QF3 uses Slots[4] and Slots[5] — winner goes to SFLeft[1] (not SFLeft[0])
	br := makeBattleResult("4", "5", wheel.Option{Text: "E"}, wheel.Option{Text: "F"})
	err := b.ApplyBattleResult(bracket.MatchQF3, br, wheels[4], wheels[5])
	if err != nil {
		t.Fatalf("ApplyBattleResult QF3: %v", err)
	}

	if b.SFLeft[1] == nil {
		t.Fatal("SFLeft[1] is nil after QF3 — wrong slot mapping")
	}
	if b.SFLeft[0] != nil {
		t.Error("SFLeft[0] is non-nil after QF3 — result went to wrong slot (should be SFLeft[1])")
	}

	// QF2 uses Slots[2] and Slots[3] — winner goes to SFRight[0] (not SFLeft[0])
	br2 := makeBattleResult("2", "3", wheel.Option{Text: "C"}, wheel.Option{Text: "D"})
	err = b.ApplyBattleResult(bracket.MatchQF2, br2, wheels[2], wheels[3])
	if err != nil {
		t.Fatalf("ApplyBattleResult QF2: %v", err)
	}

	if b.SFRight[0] == nil {
		t.Fatal("SFRight[0] is nil after QF2 — wrong slot mapping")
	}
	if b.SFLeft[0] != nil {
		t.Error("SFLeft[0] should remain nil — QF2 goes to SFRight[0], not SFLeft[0]"+
			"\nWarning: SFLeft[0] was set — this is because QF1 hasn't run yet but QF2 mapped incorrectly")
		// Actually after QF3, SFLeft[1] is set, SFLeft[0] should still be nil.
	}
}

func TestApplyBattleResult_MovieIsLandedSlice(t *testing.T) {
	// Sneaky-pass guard (d): Final winner's movie = WinnerLanded.Text, not Options[0]
	wheels := seededWheels("A", "B", "C", "D", "E", "F", "G", "H")
	// Full bracket progression
	b := bracket.New(wheels)

	// QF1: wheel 0 beats wheel 1
	b.ApplyBattleResult(bracket.MatchQF1,
		makeBattleResult("0", "1", wheel.Option{Text: "A"}, wheel.Option{Text: "B"}),
		wheels[0], wheels[1])
	// QF2: wheel 2 beats wheel 3
	b.ApplyBattleResult(bracket.MatchQF2,
		makeBattleResult("2", "3", wheel.Option{Text: "C"}, wheel.Option{Text: "D"}),
		wheels[2], wheels[3])
	// QF3: wheel 5 beats wheel 4
	b.ApplyBattleResult(bracket.MatchQF3,
		makeBattleResult("5", "4", wheel.Option{Text: "F"}, wheel.Option{Text: "E"}),
		wheels[5], wheels[4])
	// QF4: wheel 6 beats wheel 7
	b.ApplyBattleResult(bracket.MatchQF4,
		makeBattleResult("6", "7", wheel.Option{Text: "G"}, wheel.Option{Text: "H"}),
		wheels[6], wheels[7])

	// SF1: QF1 winner (wheel 0) beats QF2 winner (wheel 2)
	// The absorbed wheels have been stored in SFLeft[0] and SFRight[0]
	sf1Left := *b.SFLeft[0]   // absorbed wheel 0
	sf1Right := *b.SFRight[0] // absorbed wheel 2
	b.ApplyBattleResult(bracket.MatchSF1,
		makeBattleResult("0", "2", wheel.Option{Text: "A"}, wheel.Option{Text: "C"}),
		sf1Left, sf1Right)

	// SF2: QF3 winner (wheel 5) beats QF4 winner (wheel 6)
	sf2Left := *b.SFLeft[1]   // absorbed wheel 5
	sf2Right := *b.SFRight[1] // absorbed wheel 6
	b.ApplyBattleResult(bracket.MatchSF2,
		makeBattleResult("5", "6", wheel.Option{Text: "F"}, wheel.Option{Text: "G"}),
		sf2Left, sf2Right)

	// Final: SF1 winner (wheel 0) beats SF2 winner (wheel 5)
	finalLeft := *b.FinalLeft
	finalRight := *b.FinalRight
	b.ApplyBattleResult(bracket.MatchFinal,
		makeBattleResult("0", "5", wheel.Option{Text: "A-landed"}, wheel.Option{Text: "F-landed"}),
		finalLeft, finalRight)

	if b.Winner == nil {
		t.Fatal("Winner is nil after final")
	}

	// The movie should be WinnerLanded.Text ("A-landed"), NOT Options[0] of the winner wheel
	if b.Winner.LandedOption.Text != "A-landed" {
		t.Errorf("Winner.LandedOption.Text = %q, want %q (should be the spun slice, not Options[0])",
			b.Winner.LandedOption.Text, "A-landed")
	}
}

func TestApplyBattleResult_DependencyGate(t *testing.T) {
	// Sneaky-pass guard (e): dependency-gate skipped → assert err != nil + state unmodified
	wheels := seededWheels("A", "B", "C", "D") // only first 4 wheels have options
	b := bracket.New(wheels)

	// Try SF1 before QF1+QF2 — dependency error
	br := makeBattleResult("0", "2", wheel.Option{Text: "A"}, wheel.Option{Text: "C"})
	err := b.ApplyBattleResult(bracket.MatchSF1, br, wheel.Wheel{}, wheel.Wheel{})
	if err == nil {
		t.Fatal("expected error for SF1 before QF1+QF2, got nil")
	}

	// State should be unmodified
	if b.FinalLeft != nil {
		t.Error("FinalLeft was modified despite dependency error")
	}
	if b.FinalRight != nil {
		t.Error("FinalRight was modified despite dependency error")
	}
}

func TestApplyBattleResult_IdempotencyGate(t *testing.T) {
	// Sneaky-pass guard (f): QF1 twice → second call err != nil + state unchanged
	wheels := seededWheels("A", "B")
	b := bracket.New(wheels)

	// First call: should succeed
	br1 := makeBattleResult("0", "1", wheel.Option{Text: "A"}, wheel.Option{Text: "B"})
	err1 := b.ApplyBattleResult(bracket.MatchQF1, br1, wheels[0], wheels[1])
	if err1 != nil {
		t.Fatalf("first ApplyBattleResult QF1: %v", err1)
	}

	// Second call: should error (already resolved)
	br2 := makeBattleResult("0", "1", wheel.Option{Text: "A"}, wheel.Option{Text: "B"})
	err2 := b.ApplyBattleResult(bracket.MatchQF1, br2, wheels[0], wheels[1])
	if err2 == nil {
		t.Fatal("expected error for second QF1, got nil")
	}
	if !errors.Is(err2, bracket.ErrAlreadyResolved) {
		t.Errorf("error = %v, want ErrAlreadyResolved", err2)
	}
}

func TestApplyBattleResult_FinalBeforeSFs(t *testing.T) {
	// Sneaky-pass guard (g): Final before SFs done → err != nil
	wheels := seededWheels("A", "B", "C", "D")
	b := bracket.New(wheels)

	// Try Final directly
	err := b.ApplyBattleResult(bracket.MatchFinal, battle.BattleResult{}, wheel.Wheel{}, wheel.Wheel{})
	if err == nil {
		t.Fatal("expected error for Final without SFs, got nil")
	}
	if !errors.Is(err, bracket.ErrDependencyNotMet) {
		t.Errorf("error = %v, want ErrDependencyNotMet (ValidateDependencies was called)", err)
	}
}

func TestBracketProgression_FullFlow(t *testing.T) {
	// Complete 7-match bracket lifecycle with deterministic results
	wheels := seededWheels("A", "B", "C", "D", "E", "F", "G", "H")
	b := bracket.New(wheels)

	// QF1: wheel 0 beats wheel 1
	err := b.ApplyBattleResult(bracket.MatchQF1,
		makeBattleResult("0", "1", wheel.Option{Text: "A"}, wheel.Option{Text: "B"}),
		wheels[0], wheels[1])
	if err != nil {
		t.Fatalf("QF1: %v", err)
	}
	if b.SFLeft[0] == nil {
		t.Fatal("QF1: SFLeft[0] not filled")
	}

	// QF2: wheel 3 beats wheel 2
	err = b.ApplyBattleResult(bracket.MatchQF2,
		makeBattleResult("3", "2", wheel.Option{Text: "D"}, wheel.Option{Text: "C"}),
		wheels[3], wheels[2])
	if err != nil {
		t.Fatalf("QF2: %v", err)
	}
	if b.SFRight[0] == nil {
		t.Fatal("QF2: SFRight[0] not filled")
	}

	// QF3: wheel 4 beats wheel 5
	err = b.ApplyBattleResult(bracket.MatchQF3,
		makeBattleResult("4", "5", wheel.Option{Text: "E"}, wheel.Option{Text: "F"}),
		wheels[4], wheels[5])
	if err != nil {
		t.Fatalf("QF3: %v", err)
	}
	if b.SFLeft[1] == nil {
		t.Fatal("QF3: SFLeft[1] not filled")
	}

	// QF4: wheel 7 beats wheel 6
	err = b.ApplyBattleResult(bracket.MatchQF4,
		makeBattleResult("7", "6", wheel.Option{Text: "H"}, wheel.Option{Text: "G"}),
		wheels[7], wheels[6])
	if err != nil {
		t.Fatalf("QF4: %v", err)
	}
	if b.SFRight[1] == nil {
		t.Fatal("QF4: SFRight[1] not filled")
	}

	// SF1: QF1 winner (wheel 0) beats QF2 winner (wheel 3)
	sf1Left := *b.SFLeft[0]
	sf1Right := *b.SFRight[0]
	err = b.ApplyBattleResult(bracket.MatchSF1,
		makeBattleResult("0", "3", wheel.Option{Text: "A"}, wheel.Option{Text: "D"}),
		sf1Left, sf1Right)
	if err != nil {
		t.Fatalf("SF1: %v", err)
	}
	if b.FinalLeft == nil {
		t.Fatal("SF1: FinalLeft not filled")
	}

	// SF2: QF3 winner (wheel 4) beats QF4 winner (wheel 7)
	sf2Left := *b.SFLeft[1]
	sf2Right := *b.SFRight[1]
	err = b.ApplyBattleResult(bracket.MatchSF2,
		makeBattleResult("4", "7", wheel.Option{Text: "E"}, wheel.Option{Text: "H"}),
		sf2Left, sf2Right)
	if err != nil {
		t.Fatalf("SF2: %v", err)
	}
	if b.FinalRight == nil {
		t.Fatal("SF2: FinalRight not filled")
	}

	// Final: SF1 winner (wheel 0) beats SF2 winner (wheel 4)
	finalLeft := *b.FinalLeft
	finalRight := *b.FinalRight
	err = b.ApplyBattleResult(bracket.MatchFinal,
		makeBattleResult("0", "4", wheel.Option{Text: "A-final"}, wheel.Option{Text: "E-final"}),
		finalLeft, finalRight)
	if err != nil {
		t.Fatalf("Final: %v", err)
	}
	if b.Winner == nil {
		t.Fatal("Final: Winner not filled")
	}

	// Final movie = final winner's landed text ("A-final")
	if b.Winner.LandedOption.Text != "A-final" {
		t.Errorf("movie = %q, want %q", b.Winner.LandedOption.Text, "A-final")
	}

	// Verify absorption: SFLeft[0] (winner wheel 0) should contain loser text "B"
	hasB := false
	for _, opt := range b.SFLeft[0].Options {
		if opt.Text == "B" {
			hasB = true
			break
		}
	}
	if !hasB {
		t.Error("SFLeft[0] missing absorbed text 'B'")
	}
}

func TestSlotMapping(t *testing.T) {
	tests := []struct {
		mid  bracket.MatchID
		want string
	}{
		{bracket.MatchQF1, "slot-sf1-left"},
		{bracket.MatchQF2, "slot-sf1-right"},
		{bracket.MatchQF3, "slot-sf2-left"},
		{bracket.MatchQF4, "slot-sf2-right"},
		{bracket.MatchSF1, "slot-final-left"},
		{bracket.MatchSF2, "slot-final-right"},
		{bracket.MatchFinal, ""}, // Final has no next-round slot; movieResult is the OOB target
	}

	for _, tt := range tests {
		got := bracket.SlotMapping(tt.mid)
		if got != tt.want {
			t.Errorf("SlotMapping(%s) = %q, want %q", tt.mid, got, tt.want)
		}
	}
}

func TestApplyBattleResult_UnknownMatch(t *testing.T) {
	b := bracket.New([8]wheel.Wheel{})
	err := b.ApplyBattleResult("invalid", battle.BattleResult{}, wheel.Wheel{}, wheel.Wheel{})
	if err == nil {
		t.Fatal("expected error for unknown match ID, got nil")
	}
	if !errors.Is(err, bracket.ErrUnknownMatch) {
		t.Errorf("error = %v, want ErrUnknownMatch", err)
	}
}
