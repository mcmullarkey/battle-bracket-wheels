package main

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"battle-bracket-wheels/internal/wheel"
)

// WheelViewData holds the data needed to render a single wheel template.
type WheelViewData struct {
	ID      string
	Options []WheelOptionView
}

// WheelOptionView holds option data for template rendering, including
// the SVG arc paths for that option's slice(s).
type WheelOptionView struct {
	Text     string
	Index    int
	ArcPaths []string
}

// wheelViewFromWheel builds a WheelViewData from a wheel.Wheel model,
// computing normalized weights and SVG arc paths.
func wheelViewFromWheel(wh wheel.Wheel) WheelViewData {
	view := WheelViewData{ID: wh.ID}

	probs, err := wheel.NormalizeWeights(wh)
	if err != nil {
		// No options — empty view is fine
		return view
	}

	arcs := wheel.ComputeArcAngles(probs)
	cx, cy, r := 100.0, 100.0, 80.0

	// Build arc paths per option
	// For a single option, arcs has 2 entries (two 180° arcs).
	// For multiple options, arcs has 1 entry per option.
	if len(wh.Options) == 1 {
		// Single option: collect both arcs under one option
		paths := make([]string, len(arcs))
		for j, a := range arcs {
			paths[j] = wheel.ArcPath(a, cx, cy, r)
		}
		view.Options = append(view.Options, WheelOptionView{
			Text:     wh.Options[0].Text,
			Index:    0,
			ArcPaths: paths,
		})
	} else {
		for i, opt := range wh.Options {
			path := wheel.ArcPath(arcs[i], cx, cy, r)
			view.Options = append(view.Options, WheelOptionView{
				Text:     opt.Text,
				Index:    i,
				ArcPaths: []string{path},
			})
		}
	}

	return view
}

// writeJSONError writes a JSON error response with the given status and message.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// renderWheelFragment executes the wheel template and writes it to the response.
func renderWheelFragment(w http.ResponseWriter, tmpl Renderer, view WheelViewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "wheel", view); err != nil {
		log.Printf("wheel template execution error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// addOptionHandler handles POST /wheel/{id}/option
func addOptionHandler(store *Store, renderer Renderer) http.HandlerFunc {
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

		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form data")
			return
		}

		text := strings.TrimSpace(r.FormValue("text"))
		if text == "" {
			writeJSONError(w, http.StatusBadRequest, "option text must not be empty")
			return
		}

		var weight *float64
		weightStr := r.FormValue("weight")
		if weightStr != "" {
			wVal, err := strconv.ParseFloat(weightStr, 64)
			if err != nil || math.IsNaN(wVal) || math.IsInf(wVal, 0) {
				writeJSONError(w, http.StatusBadRequest, "invalid weight value")
				return
			}
			if wVal < 0 {
				writeJSONError(w, http.StatusBadRequest, "weight must not be negative")
				return
			}
			weight = &wVal
		}

		// Atomically add option under write lock
		var wh wheel.Wheel
		err = store.Update(sessionID, func(s *Session) error {
			wh = s.Wheels[wheelIdx]
			wh = wheel.AddOption(wh, text, weight)
			s.Wheels[wheelIdx] = wh
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

		view := wheelViewFromWheel(wh)
		renderWheelFragment(w, renderer, view)
	}
}

// deleteOptionHandler handles DELETE /wheel/{id}/option/{idx}
func deleteOptionHandler(store *Store, renderer Renderer) http.HandlerFunc {
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

		idxStr := r.PathValue("idx")
		optIdx, err := strconv.Atoi(idxStr)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid option index")
			return
		}

		var newWh wheel.Wheel
		err = store.Update(sessionID, func(s *Session) error {
			wh := s.Wheels[wheelIdx]
			var innerErr error
			newWh, innerErr = wheel.RemoveOption(wh, optIdx)
			if innerErr != nil {
				return innerErr
			}
			s.Wheels[wheelIdx] = newWh
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "session not found")
			} else {
				writeJSONError(w, http.StatusBadRequest, err.Error())
			}
			return
		}

		view := wheelViewFromWheel(newWh)
		renderWheelFragment(w, renderer, view)
	}
}

// parseWheelID validates and parses a wheel ID string (0-7).
func parseWheelID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 0 || id > 7 {
		return 0, errInvalidWheelID
	}
	return id, nil
}

var errInvalidWheelID = errors.New("invalid wheel ID")
