package main

import (
	"encoding/json"
	"errors"
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

		// Load both wheels and check resolved state under read lock
		var whA, whB wheel.Wheel
		var resolved bool
		err := store.View(sessionID, func(s *Session) error {
			whA = s.Wheels[idxA]
			whB = s.Wheels[idxB]
			resolved = s.ResolvedMatches[matchID]
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "session not found")
			} else {
				writeJSONError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		if resolved {
			writeJSONError(w, http.StatusConflict, "match already resolved")
			return
		}

		// Validate both wheels have options
		if len(whA.Options) == 0 || len(whB.Options) == 0 {
			writeJSONError(w, http.StatusUnprocessableEntity, "both wheels must have at least one option")
			return
		}

		// Spin both wheels independently
		resultA, err := wheel.Spin(whA, newSource())
		if err != nil {
			log.Printf("spin error on wheel %s: %v", whA.ID, err)
			writeJSONError(w, http.StatusInternalServerError, "spin error")
			return
		}
		resultB, err := wheel.Spin(whB, newSource())
		if err != nil {
			log.Printf("spin error on wheel %s: %v", whB.ID, err)
			writeJSONError(w, http.StatusInternalServerError, "spin error")
			return
		}

		// Create RollFunc backed by the injected crypto source
		rng := rand.New(newSource())
		rollFunc := func() int {
			return rng.Intn(100) + 1
		}

		// Resolve the battle
		battleResult, err := battle.ResolveBattle(
			resultA.Option, resultB.Option,
			whA.ID, whB.ID,
			rollFunc, 100,
		)
		if err != nil {
			if errors.Is(err, battle.ErrTiebreakerExhausted) {
				writeJSONError(w, http.StatusInternalServerError, "tiebreaker exhausted")
			} else {
				log.Printf("battle resolution error: %v", err)
				writeJSONError(w, http.StatusInternalServerError, "battle resolution error")
			}
			return
		}

		// Determine which wheel won and absorb the loser's landed option
		var absorbedWheel wheel.Wheel
		var absorbedIdx int
		if battleResult.WinnerID == whA.ID {
			absorbedWheel = battle.AbsorbOption(whA, resultB.Option)
			absorbedIdx = idxA
		} else {
			absorbedWheel = battle.AbsorbOption(whB, resultA.Option)
			absorbedIdx = idxB
		}

		// Save the updated wheel and mark match as resolved
		err = store.Update(sessionID, func(s *Session) error {
			s.Wheels[absorbedIdx] = absorbedWheel
			s.ResolvedMatches[matchID] = true
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "session not found")
			} else {
				log.Printf("session update error: %v", err)
				writeJSONError(w, http.StatusInternalServerError, "internal error")
			}
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
