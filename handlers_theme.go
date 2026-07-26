package main

import (
	"net/http"
	"strings"

	"battle-bracket-wheels/internal/theme"
)

// themeCookieName is the name of the theme cookie.
// It mirrors bbw_session attributes (HttpOnly, Path=/, SameSite=Lax, Secure=false)
// but is independent of the session — theme is per-browser, not session-scoped.
const themeCookieName = "bbw_theme"

// themeHandler handles POST /theme.
//
// It validates the submitted theme key against the registry (closed set),
// sets the bbw_theme cookie, and redirects to "/" via 303 See Other.
// Non-POST methods get 405 (route is registered without method prefix so
// Go 1.22+ ServeMux routes all methods here for proper 405 handling,
// mirroring the battleHandler pattern).
//
// This route is NOT wrapped in sessionMiddleware — theme selection is
// per-browser and session-independent.
func themeHandler(registry *theme.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid form data")
			return
		}
		key := strings.TrimSpace(r.FormValue("theme"))
		if key == "" {
			writeJSONError(w, http.StatusBadRequest, "theme must not be empty")
			return
		}
		if _, ok := registry.Resolve(key); !ok {
			writeJSONError(w, http.StatusBadRequest, "unknown theme")
			return
		}
		setThemeCookie(w, key)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// setThemeCookie writes the bbw_theme cookie to the response writer.
// Attributes mirror bbw_session: HttpOnly, Path=/, SameSite=Lax, Secure=false.
func setThemeCookie(w http.ResponseWriter, key string) {
	http.SetCookie(w, &http.Cookie{
		Name:     themeCookieName,
		Value:    key,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
}

// resolveTheme reads the bbw_theme cookie from the request and resolves it
// against the registry. If the cookie is absent, empty, or holds an
// unregistered key, it falls back to the registry Default.
//
// This is a pure wrapper — it reads the request but produces no side effects.
func resolveTheme(r *http.Request, registry *theme.Registry) theme.Theme {
	cookie, err := r.Cookie(themeCookieName)
	if err != nil || cookie.Value == "" {
		return registry.Default()
	}
	t, ok := registry.Resolve(cookie.Value)
	if !ok {
		return registry.Default()
	}
	return t
}
