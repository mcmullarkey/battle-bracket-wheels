package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/theme"
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
// Propagated (SF/Final) wheels are set as ReadOnly since battle-resolved wheels
// should not be editable via spin/add/delete forms.
func bracketViewFromBracket(b *bracket.Bracket) BracketViewData {
	v := BracketViewData{}
	if b == nil {
		return v
	}
	if b.SFLeft[0] != nil {
		wh := wheelViewFromWheel(*b.SFLeft[0], "slot-sf1-left")
		wh.ReadOnly = true
		v.SFLeft0 = &wh
	}
	if b.SFRight[0] != nil {
		wh := wheelViewFromWheel(*b.SFRight[0], "slot-sf1-right")
		wh.ReadOnly = true
		v.SFRight0 = &wh
	}
	if b.SFLeft[1] != nil {
		wh := wheelViewFromWheel(*b.SFLeft[1], "slot-sf2-left")
		wh.ReadOnly = true
		v.SFLeft1 = &wh
	}
	if b.SFRight[1] != nil {
		wh := wheelViewFromWheel(*b.SFRight[1], "slot-sf2-right")
		wh.ReadOnly = true
		v.SFRight1 = &wh
	}
	if b.FinalLeft != nil {
		wh := wheelViewFromWheel(*b.FinalLeft, "slot-final-left")
		wh.ReadOnly = true
		v.FinalLeft = &wh
	}
	if b.FinalRight != nil {
		wh := wheelViewFromWheel(*b.FinalRight, "slot-final-right")
		wh.ReadOnly = true
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

// homeHandler executes the layout template with session data, wheel views,
// and theme information. It reads the bbw_theme cookie, resolves it against
// the registry (falling back to Default for absent/unknown values), and
// injects ThemeCSS (the active stylesheet path), Themes (all registered
// themes for the dropdown), and CurrentTheme (the active key for the
// selected attribute) into the template data.
func homeHandler(store *Store, renderer Renderer, registry *theme.Registry) http.HandlerFunc {
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

		// Resolve theme from cookie (pure — no side effects).
		currentTheme := resolveTheme(r, registry)

		data := map[string]interface{}{
			"SessionID":    sessionID,
			"Wheels":       wheelsView,
			"Bracket":      bracketView,
			"ThemeCSS":     currentTheme.CSSPath,
			"Themes":       registry.Themes(),
			"CurrentTheme": currentTheme.Key,
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

		// Update the session cookie on the request for downstream handlers.
		// Use setRequestCookie to replace only bbw_session, preserving other
		// cookies (e.g. bbw_theme) so downstream handlers can read them.
		setRequestCookie(r, cookieName, cookieValue)

		next.ServeHTTP(w, r)
	})
}
