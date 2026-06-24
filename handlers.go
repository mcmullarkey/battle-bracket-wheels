package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

// Renderer is the contract for template execution.
// It decouples handlers.go from html/template (per §3 design intent).
// The boot/wiring layer (main.go) provides a *template.Template which
// satisfies this interface automatically.
type Renderer interface {
	Execute(w io.Writer, data any) error
	ExecuteTemplate(w io.Writer, name string, data any) error
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

		// Build wheel views from session data
		wheelsView := make([]WheelViewData, 0, 8)
		if session, ok := store.Get(sessionID); ok {
			for _, wh := range session.Wheels {
				wheelsView = append(wheelsView, wheelViewFromWheel(wh))
			}
		}

		data := map[string]interface{}{
			"SessionID": sessionID,
			"Wheels":    wheelsView,
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
