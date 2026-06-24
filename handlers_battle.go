package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"battle-bracket-wheels/internal/battle"
	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/wheel"
)

// matchWheelIDs maps quarter-final match IDs to the two wheel indices they pair.
var matchWheelIDs = map[bracket.MatchID][2]int{
	bracket.MatchQF1: {0, 1},
	bracket.MatchQF2: {2, 3},
	bracket.MatchQF3: {4, 5},
	bracket.MatchQF4: {6, 7},
}

// bracketMatchIDs lists match IDs that use bracket state (SF and Final).
// Uses typed MatchID constants per §1.1 (encode invariants in types).
var bracketMatchIDs = map[bracket.MatchID]bool{
	bracket.MatchSF1:   true,
	bracket.MatchSF2:   true,
	bracket.MatchFinal: true,
}

// matchResultData holds the data for rendering the match result template.
type matchResultData struct {
	MatchID    string
	WinnerID   string
	LoserID    string
	WinnerRoll int
	LoserRoll  int
	Ties       int
}

// nextRoundSlotData holds the data for rendering the next-round slot OOB fragment.
type nextRoundSlotData struct {
	SlotID string
	Wheel  WheelViewData
}

// movieResultData holds the data for rendering the final movie result OOB fragment.
type movieResultData struct {
	Text string
}

// battleHandler handles POST /battle/{matchID}.
//
// It orchestrates a battle between two wheels:
//  1. Parses the matchID to determine which two wheels to pair
//  2. Loads both wheels (from Session.Wheels for QF, from Bracket for SF/Final)
//  3. Spins both wheels (AC-3) to select a landed option
//  4. Resolves the battle via ResolveBattle (rolls + tiebreaker)
//  5. Applies bracket progression which absorbs loser option and returns absorbed wheel
//  6. Stores the updated winner wheel and applies bracket progression
//  7. Sets HX-Trigger with spin-wheel data for both wheels' animations
//  8. Returns HTML fragments (match result, next-round slot, disabled button)
func battleHandler(store *Store, renderer Renderer, newSource func() rand.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			writeJSONError(w, http.StatusUnauthorized, "session required")
			return
		}

		matchIDStr := r.PathValue("matchID")

		// Validate match ID — check both QF and bracket match sets
		bid := bracket.MatchID(matchIDStr)
		indices, isQF := matchWheelIDs[bid]
		if !isQF && !bracketMatchIDs[bid] {
			writeJSONError(w, http.StatusNotFound, "invalid match ID")
			return
		}

		// Check method — only POST is allowed
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Perform ALL battle logic atomically under a single write lock:
		// check → load → spin → resolve → bracket-progression → mark-resolved.
		var whA, whB wheel.Wheel
		var wheelASlotID, wheelBSlotID string
		var alreadyResolved bool
		var emptyWheel bool
		var resultA, resultB wheel.SpinResult
		var battleResult battle.BattleResult
		var absorbedWheel wheel.Wheel

		updateErr := store.Update(sessionID, func(s *Session) error {
			if s.ResolvedMatches[matchIDStr] {
				alreadyResolved = true
				return nil
			}

			// Load the two wheels based on match type
			if isQF {
				idxA, idxB := indices[0], indices[1]
				whA = s.Wheels[idxA]
				whB = s.Wheels[idxB]
				wheelASlotID = slotIDFromWheelIdx(idxA)
				wheelBSlotID = slotIDFromWheelIdx(idxB)
			} else {
				// SF or Final — load from bracket pointers
				if err := s.Bracket.ValidateDependencies(bid); err != nil {
					return fmt.Errorf("bracket dependency: %w", err)
				}
				switch bid {
				case bracket.MatchSF1:
					whA = *s.Bracket.SFLeft[0]
					whB = *s.Bracket.SFRight[0]
					wheelASlotID = "slot-sf1-left"
					wheelBSlotID = "slot-sf1-right"
				case bracket.MatchSF2:
					whA = *s.Bracket.SFLeft[1]
					whB = *s.Bracket.SFRight[1]
					wheelASlotID = "slot-sf2-left"
					wheelBSlotID = "slot-sf2-right"
				case bracket.MatchFinal:
					whA = *s.Bracket.FinalLeft
					whB = *s.Bracket.FinalRight
					wheelASlotID = "slot-final-left"
					wheelBSlotID = "slot-final-right"
				}
			}

			if len(whA.Options) == 0 || len(whB.Options) == 0 {
				emptyWheel = true
				return nil
			}

			var spinErr error
			resultA, spinErr = wheel.Spin(whA, newSource())
			if spinErr != nil {
				return fmt.Errorf("spin error on wheel %s: %w", whA.ID, spinErr)
			}
			resultB, spinErr = wheel.Spin(whB, newSource())
			if spinErr != nil {
				return fmt.Errorf("spin error on wheel %s: %w", whB.ID, spinErr)
			}

			rng := rand.New(newSource())
			rollFunc := func() int {
				return rng.Intn(100) + 1
			}

			var resolveErr error
			battleResult, resolveErr = battle.ResolveBattle(
				resultA.Option, resultB.Option,
				whA.ID, whB.ID,
				rollFunc, 100,
			)
			if resolveErr != nil {
				return resolveErr
			}

			// Sync bracket slots from session wheels for QF matches
			// before ApplyBattleResult validates dependencies.
			if isQF {
				s.Bracket.Slots = s.Wheels
			}

			// Apply bracket progression (handles absorption internally)
			// and returns the absorbed winner wheel.
			var absorbErr error
			absorbedWheel, absorbErr = s.Bracket.ApplyBattleResult(bid, battleResult, whA, whB)
			if absorbErr != nil {
				return fmt.Errorf("bracket progression: %w", absorbErr)
			}

			// For QF matches, update session wheels with the absorbed wheel
			if isQF {
				if battleResult.WinnerID == whA.ID {
					s.Wheels[indices[0]] = absorbedWheel
				} else {
					s.Wheels[indices[1]] = absorbedWheel
				}
				s.Bracket.Slots = s.Wheels
			}

			s.ResolvedMatches[matchIDStr] = true
			return nil
		})

		if updateErr != nil {
			if errors.Is(updateErr, ErrSessionNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "session not found")
				return
			}
			if errors.Is(updateErr, battle.ErrTiebreakerExhausted) {
				writeJSONError(w, http.StatusInternalServerError, "tiebreaker exhausted")
				return
			}
			if errors.Is(updateErr, bracket.ErrDependencyNotMet) {
				writeJSONError(w, http.StatusConflict, "dependency not met: bracket slots not filled")
				return
			}
			if errors.Is(updateErr, bracket.ErrAlreadyResolved) {
				writeJSONError(w, http.StatusConflict, "match already resolved")
				return
			}
			log.Printf("battle error: %v", updateErr)
			writeJSONError(w, http.StatusInternalServerError, "battle error")
			return
		}
		if alreadyResolved {
			writeJSONError(w, http.StatusConflict, "match already resolved")
			return
		}
		if emptyWheel {
			writeJSONError(w, http.StatusUnprocessableEntity, "both wheels must have at least one option")
			return
		}

		// Build HX-Trigger with spin data for both wheels, including slotID
		// for scoped wheel-group ID lookup in the JS animation.
		triggerData := map[string]interface{}{
			"spin-wheel": []map[string]interface{}{
				{
					"wheelID":     whA.ID,
					"slotID":      wheelASlotID,
					"targetIndex": resultA.Index,
					"targetAngle": resultA.TargetAngle,
				},
				{
					"wheelID":     whB.ID,
					"slotID":      wheelBSlotID,
					"targetIndex": resultB.Index,
					"targetAngle": resultB.TargetAngle,
				},
			},
		}

		// For Final match, add bracketComplete trigger
		if bid == bracket.MatchFinal {
			triggerData["bracketComplete"] = true
		}

		triggerJSON, err := json.Marshal(triggerData)
		if err != nil {
			log.Printf("json marshal HX-Trigger: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("HX-Trigger", string(triggerJSON))

		// Render HTML fragments
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// 1. Match result fragment
		err = renderer.ExecuteTemplate(w, "matchResult", matchResultData{
			MatchID:    matchIDStr,
			WinnerID:   battleResult.WinnerID,
			LoserID:    battleResult.LoserID,
			WinnerRoll: battleResult.WinnerRoll,
			LoserRoll:  battleResult.LoserRoll,
			Ties:       battleResult.Ties,
		})
		if err != nil {
			log.Printf("match template execution error: %v", err)
		}

		// 2. Next-round slot fragment with absorbed winner wheel
		// Absorbed wheels in next-round slots are read-only — they are propagated
		// from a previous battle and should not be editable.
		nextRoundSlotID := bracket.SlotMapping(bid)
		if nextRoundSlotID != "" {
			whView := wheelViewFromWheel(absorbedWheel, nextRoundSlotID)
			whView.ReadOnly = true
			err = renderer.ExecuteTemplate(w, "nextRoundSlot", nextRoundSlotData{
				SlotID: nextRoundSlotID,
				Wheel:  whView,
			})
			if err != nil {
				log.Printf("nextRoundSlot template execution error: %v", err)
			}
		}

		// 3. For Final match, render movie result fragment
		if bid == bracket.MatchFinal {
			err = renderer.ExecuteTemplate(w, "movieResult", movieResultData{
				Text: battleResult.WinnerLanded.Text,
			})
			if err != nil {
				log.Printf("movieResult template execution error: %v", err)
			}
		}

		// 4. Disabled button as non-OOB response for the main swap target.
		// This element is NOT hx-swap-oob, so HTMX uses it for the main
		// swap (replacing the clicked battle button).  Without a non-OOB
		// element, HTMX 2.x skips HX-Trigger event processing entirely,
		// and the spin-wheel animation never fires.
		err = renderer.ExecuteTemplate(w, "disabledButton", nil)
		if err != nil {
			log.Printf("disabledButton template execution error: %v", err)
		}
	}
}
