package main

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"

	"battle-bracket-wheels/internal/wheel"
)

// spinHandler handles POST /wheel/{id}/spin.
//
// It reads the wheel from the session, performs a weighted-random selection
// via wheel.Spin, sets an HX-Trigger header with spin result data for
// client-side animation, and returns the wheel fragment HTML.
//
// The newSource parameter allows injecting a deterministic rand.Source in
// tests. In production, it's seeded from crypto/rand via newSpinSource.
func spinHandler(store *Store, renderer Renderer, newSource func() rand.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			writeJSONError(w, http.StatusUnauthorized, "session required")
			return
		}

		idStr := r.PathValue("id")
		wheelIdx, err := parseWheelID(idStr)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "invalid wheel ID")
			return
		}

		// Read the wheel under read lock (spin is read-only on the store)
		var wh wheel.Wheel
		err = store.View(sessionID, func(s *Session) error {
			wh = s.Wheels[wheelIdx]
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

		// Perform weighted-random selection
		result, err := wheel.Spin(wh, newSource())
		if err != nil {
			if errors.Is(err, wheel.ErrNoSelectableOptions) {
				writeJSONError(w, http.StatusBadRequest, "no selectable options")
			} else {
				log.Printf("spin error: %v", err)
				writeJSONError(w, http.StatusInternalServerError, "spin error")
			}
			return
		}

		// Set HX-Trigger header for client-side spin animation.
		// HTMX will fire a "spin-wheel" custom event with this detail
		// on the target element after the swap.
		slotID := slotIDFromWheelIdx(wheelIdx)
		triggerData := map[string]interface{}{
			"spin-wheel": map[string]interface{}{
				"wheelID":     wh.ID,
				"slotID":      slotID,
				"targetIndex": result.Index,
				"targetAngle": result.TargetAngle,
			},
		}
		triggerJSON, err := json.Marshal(triggerData)
		if err != nil {
			log.Printf("json marshal HX-Trigger: %v", err)
			writeJSONError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("HX-Trigger", string(triggerJSON))

		// Render the wheel fragment (same as option CRUD handlers)
		view := wheelViewFromWheel(wh, slotID)
		renderWheelFragment(w, renderer, view)
	}
}
