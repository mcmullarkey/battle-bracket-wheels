package main

import (
	"errors"
	"log"
	"net/http"

	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/populate"
	"battle-bracket-wheels/internal/wheel"
)

// populateStatusData holds the data for rendering the populate status fragment.
// This is the non-OOB main swap target required by HTMX 2.x for HX-Trigger
// processing (at least one non-OOB element must be present in the response).
type populateStatusData struct {
	Message string
	IsError bool
}

// populateHandler handles POST /wheels/populate.
//
// It parses a tolerant list from the "items" form field, distributes entries
// round-robin across 8 wheels via populate.ParseAndDistribute, then mutates
// the session under a single store.Update lock:
//   - s.Wheels = result.Wheels (replace, not append)
//   - s.Bracket = bracket.New(result.Wheels) (fresh bracket)
//   - s.ResolvedMatches = make(map[string]bool) (fresh empty map)
//
// On success, renders 8 OOB nextRoundSlot fragments (slot-1..slot-8) plus
// 1 non-OOB populateStatus fragment. The render data is read from the mutated
// session (inside the Update closure), not from the Result directly — this
// ensures the response reflects the actual session state, not a pre-mutation
// snapshot.
//
// Error arms render HTML fragments (not JSON) so HTMX can swap the error
// message into the #populate-status target div:
//   - GetCookie → 401 (defensive — sessionMiddleware normally ensures cookie)
//   - ParseForm → 400 (HTML error)
//   - empty items → 400 (HTML error)
//   - ErrTooFewEntries → 400 (HTML error)
//   - ErrSessionNotFound → 401 (HTML error)
//   - other → 500 (HTML error)
func populateHandler(store *Store, renderer Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			writePopulateError(w, renderer, http.StatusUnauthorized, "session required")
			return
		}

		if err := r.ParseForm(); err != nil {
			writePopulateError(w, renderer, http.StatusBadRequest, "invalid form data")
			return
		}

		items := r.FormValue("items")
		if items == "" {
			writePopulateError(w, renderer, http.StatusBadRequest, "items must not be empty")
			return
		}

		result, err := populate.ParseAndDistribute(items)
		if err != nil {
			if errors.Is(err, populate.ErrTooFewEntries) {
				writePopulateError(w, renderer, http.StatusBadRequest, "at least 8 entries required")
				return
			}
			log.Printf("populate parse error: %v", err)
			writePopulateError(w, renderer, http.StatusInternalServerError, "populate error")
			return
		}

		// Mutate session atomically under single write lock.
		// Read back the mutated wheels from the session (not from result)
		// so the rendered response reflects actual session state.
		var sessionWheels [8]wheel.Wheel
		updateErr := store.Update(sessionID, func(s *Session) error {
			s.Wheels = result.Wheels
			s.Bracket = bracket.New(result.Wheels)
			s.ResolvedMatches = make(map[string]bool)
			sessionWheels = s.Wheels
			return nil
		})
		if updateErr != nil {
			if errors.Is(updateErr, ErrSessionNotFound) {
				writePopulateError(w, renderer, http.StatusUnauthorized, "session not found")
				return
			}
			log.Printf("populate store update error: %v", updateErr)
			writePopulateError(w, renderer, http.StatusInternalServerError, "internal error")
			return
		}

		// Render 8 OOB nextRoundSlot fragments (slot-1..slot-8).
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		for i := range sessionWheels {
			slotID := slotIDFromWheelIdx(i)
			whView := wheelViewFromWheel(sessionWheels[i], slotID)
			if err := renderer.ExecuteTemplate(w, "nextRoundSlot", nextRoundSlotData{
				SlotID: slotID,
				Wheel:  whView,
			}); err != nil {
				log.Printf("nextRoundSlot template execution error (wheel %d): %v", i, err)
			}
		}

		// Non-OOB fragment — required by HTMX 2.x for main swap target.
		// Without a non-OOB element, HTMX 2.x skips HX-Trigger event processing.
		if err := renderer.ExecuteTemplate(w, "populateStatus", populateStatusData{
			Message: "Wheels populated successfully",
		}); err != nil {
			log.Printf("populateStatus template execution error: %v", err)
		}
	}
}

// writePopulateError renders an error as an HTML fragment (populateStatus
// template) so HTMX can swap it into the #populate-status target div.
// This ensures errors are user-visible (not hidden JSON) and the form
// remains interactive after an error.
func writePopulateError(w http.ResponseWriter, renderer Renderer, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := renderer.ExecuteTemplate(w, "populateStatus", populateStatusData{
		Message: message,
		IsError: true,
	}); err != nil {
		log.Printf("populateStatus error template execution: %v", err)
	}
}
