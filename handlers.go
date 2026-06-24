package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
)

// healthHandler returns {"status":"ok"} as JSON.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// homeHandler executes the layout template with session data.
func homeHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := GetCookie(r)
		if sessionID == "" {
			// Should not happen if middleware ran first
			http.Error(w, "session required", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"SessionID": sessionID,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
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

		// Add cookie to request for downstream handlers
		r.AddCookie(&http.Cookie{
			Name:  cookieName,
			Value: cookieValue,
		})

		next.ServeHTTP(w, r)
	})
}
