package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"battle-bracket-wheels/internal/bracket"
)

// Renderer is the contract for template execution.
// It decouples handlers.go from html/template (per §3 design intent).
// The boot/wiring layer (main.go) provides a *template.Template which
// satisfies this interface automatically.
type Renderer interface {
	Execute(w io.Writer, data any) error
	ExecuteTemplate(w io.Writer, name string, data any) error
}

// BracketViewData holds the view data for the bracket layout template.
// Each field is a *WheelViewData or nil if the slot is empty.
type BracketViewData struct {
	SFLeft0    *WheelViewData
	SFRight0   *WheelViewData
	SFLeft1    *WheelViewData
	SFRight1   *WheelViewData
	FinalLeft  *WheelViewData
	FinalRight *WheelViewData
	MovieText  string
}

// bracketViewFromBracket builds BracketViewData from a bracket.Bracket model.
func bracketViewFromBracket(b *bracket.Bracket) BracketViewData {
	v := BracketViewData{}
	if b == nil {
		return v
	}
	if b.SFLeft[0] != nil {
		wh := wheelViewFromWheel(*b.SFLeft[0], "slot-sf1-left")
		v.SFLeft0 = &wh
	}
	if b.SFRight[0] != nil {
		wh := wheelViewFromWheel(*b.SFRight[0], "slot-sf1-right")
		v.SFRight0 = &wh
	}
	if b.SFLeft[1] != nil {
		wh := wheelViewFromWheel(*b.SFLeft[1], "slot-sf2-left")
		v.SFLeft1 = &wh
	}
	if b.SFRight[1] != nil {
		wh := wheelViewFromWheel(*b.SFRight[1], "slot-sf2-right")
		v.SFRight1 = &wh
	}
	if b.FinalLeft != nil {
		wh := wheelViewFromWheel(*b.FinalLeft, "slot-final-left")
		v.FinalLeft = &wh
	}
	if b.FinalRight != nil {
		wh := wheelViewFromWheel(*b.FinalRight, "slot-final-right")
		v.FinalRight = &wh
	}
	if b.Winner != nil {
		v.MovieText = b.Winner.LandedOption.Text
	}
	return v
}

// healthHandler returns {"status":"ok"} as JSON.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// homeHandler executes the layout template with session data and wheel views.
func homeHandler(store *Store, renderer Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			// Should not happen if middleware ran first
			http.Error(w, "session required", http.StatusInternalServerError)
			return
		}

		// Build wheel views and bracket view from session data under read lock
		wheelsView := make([]WheelViewData, 0, 8)
		var bracketView BracketViewData
		err := store.View(sessionID, func(session *Session) error {
			for i, wh := range session.Wheels {
				wheelsView = append(wheelsView, wheelViewFromWheel(wh, slotIDFromWheelIdx(i)))
			}
			bracketView = bracketViewFromBracket(session.Bracket)
			return nil
		})
		if err != nil {
			http.Error(w, "session not found", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"SessionID": sessionID,
			"Wheels":    wheelsView,
			"Bracket":   bracketView,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := renderer.Execute(w, data); err != nil {
			log.Printf("template execution error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
}

// sessionMiddleware ensures every request has a session.
// It wraps an http.Handler and injects the session into the request context
// via a cookie. If no cookie exists, it creates a new session and sets the cookie.
func sessionMiddleware(store *Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieValue := GetCookie(r)
		if cookieValue == "" {
			// No session cookie — create new session
			session, err := store.Create()
			if err != nil {
				log.Printf("session creation error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			SetCookie(w, session)
			cookieValue = session.ID
		} else {
			// Check if session exists in store
			_, ok := store.Get(cookieValue)
			if !ok {
				// Session not found — create new one
				session, err := store.Create()
				if err != nil {
					log.Printf("session creation error: %v", err)
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}
				SetCookie(w, session)
				cookieValue = session.ID
			}
		}

		// Add cookie to request for downstream handlers.
		// Use r.Header.Set to replace the Cookie header entirely, so that
		// a stale cookie (e.g. from before server restart) does not win
		// over the fresh session ID we just Set-Cookie'd on the response.
		r.Header.Set("Cookie", cookieName+"="+cookieValue)

		next.ServeHTTP(w, r)
	})
}
