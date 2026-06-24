package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"battle-bracket-wheels/internal/battle"
	"battle-bracket-wheels/internal/wheel"
)

// matchWheelIDs maps match IDs to the two wheel indices they pair.
// For AC-4, only quarter-final matches are mapped (AC-5 adds bracket progression).
var matchWheelIDs = map[string][2]int{
	"qf1": {0, 1},
	"qf2": {2, 3},
	"qf3": {4, 5},
	"qf4": {6, 7},
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

// battleHandler handles POST /battle/{matchID}.
//
// It orchestrates a battle between two wheels:
//  1. Parses the matchID to determine which two wheel indices to pair
//  2. Loads both wheels from the session
//  3. Spins both wheels (AC-3) to select a landed option
//  4. Resolves the battle via ResolveBattle (rolls + tiebreaker)
//  5. Absorbs the loser's landed option into the winner's wheel
//  6. Stores the updated winner wheel and marks the match as resolved
//  7. Sets HX-Trigger with spin-wheel data for both wheels' animations
//  8. Returns HTML fragments (match result, disabled button)
func battleHandler(store *Store, renderer Renderer, newSource func() rand.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			writeJSONError(w, http.StatusUnauthorized, "session required")
			return
		}

		matchID := r.PathValue("matchID")
		indices, ok := matchWheelIDs[matchID]
		if !ok {
			writeJSONError(w, http.StatusNotFound, "invalid match ID")
			return
		}
		idxA, idxB := indices[0], indices[1]

		// Perform ALL battle logic atomically under a single write lock:
		// check → load → spin → resolve → absorb → mark-resolved.
		// One closure, no window between check and write (closes TOCTOU race).
		var whA, whB wheel.Wheel
		var alreadyResolved bool
		var emptyWheel bool
		var resultA, resultB wheel.SpinResult
		var battleResult battle.BattleResult

		updateErr := store.Update(sessionID, func(s *Session) error {
			if s.ResolvedMatches[matchID] {
				alreadyResolved = true
				return nil
			}

			whA = s.Wheels[idxA]
			whB = s.Wheels[idxB]

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

			// Determine which wheel won and absorb the loser's landed option
			var absorbedWheel wheel.Wheel
			if battleResult.WinnerID == whA.ID {
				absorbedWheel = battle.AbsorbOption(whA, resultB.Option)
			} else {
				absorbedWheel = battle.AbsorbOption(whB, resultA.Option)
			}

			// Write result under the same lock
			if battleResult.WinnerID == whA.ID {
				s.Wheels[idxA] = absorbedWheel
			} else {
				s.Wheels[idxB] = absorbedWheel
			}
			s.ResolvedMatches[matchID] = true

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

		// Build HX-Trigger with spin data for both wheels
		triggerData := map[string]interface{}{
			"spin-wheel": []map[string]interface{}{
				{
					"wheelID":     whA.ID,
					"targetIndex": resultA.Index,
					"targetAngle": resultA.TargetAngle,
				},
				{
					"wheelID":     whB.ID,
					"targetIndex": resultB.Index,
					"targetAngle": resultB.TargetAngle,
				},
			},
		}
		triggerJSON, err := json.Marshal(triggerData)
		if err != nil {
			log.Printf("json marshal HX-Trigger: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("HX-Trigger", string(triggerJSON))

		// Render HTML fragments (match result + updated wheel with OOB)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = renderer.ExecuteTemplate(w, "matchResult", matchResultData{
			MatchID:    matchID,
			WinnerID:   battleResult.WinnerID,
			LoserID:    battleResult.LoserID,
			WinnerRoll: battleResult.WinnerRoll,
			LoserRoll:  battleResult.LoserRoll,
			Ties:       battleResult.Ties,
		})
		if err != nil {
			log.Printf("match template execution error: %v", err)
		}
	}
}
