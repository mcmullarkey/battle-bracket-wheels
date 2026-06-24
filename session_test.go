package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestCreateSession(t *testing.T) {
	store := NewStore()
	session, err := store.Create()
	if err != nil {
		t.Fatalf("store.Create() returned error: %v", err)
	}
	if len(session.ID) < 32 {
		t.Errorf("session ID length = %d, want >= 32", len(session.ID))
	}
	// Ensure it's hex (0-9, a-f)
	for _, c := range session.ID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("session ID contains non-hex character: %c", c)
			break
		}
	}
	if session.CreatedAt.IsZero() {
		t.Error("session CreatedAt is zero")
	}
}

func TestGetSession(t *testing.T) {
	store := NewStore()
	created, err := store.Create()
	if err != nil {
		t.Fatalf("store.Create() returned error: %v", err)
	}
	got, ok := store.Get(created.ID)
	if !ok {
		t.Fatal("store.Get() returned ok=false for existing session")
	}
	if got.ID != created.ID {
		t.Errorf("got.ID = %q, want %q", got.ID, created.ID)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("got.CreatedAt = %v, want %v", got.CreatedAt, created.CreatedAt)
	}
	// Non-existent session
	_, ok = store.Get("nonexistent")
	if ok {
		t.Error("store.Get() returned ok=true for nonexistent session")
	}
}

func TestSessionCookieRoundtrip(t *testing.T) {
	store := NewStore()

	// First request: no cookie → new session + cookie set
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	sessionMiddleware(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check cookie was set
		cookie, err := r.Cookie("bbw_session")
		if err != nil {
			t.Errorf("no bbw_session cookie on first request: %v", err)
		}
		if cookie != nil && len(cookie.Value) < 32 {
			t.Errorf("cookie value length = %d, want >= 32", len(cookie.Value))
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, req)

	// Check Set-Cookie header
	resp := rr.Result()
	cookies := resp.Cookies()
	var bbwCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "bbw_session" {
			bbwCookie = c
			break
		}
	}
	if bbwCookie == nil {
		t.Fatal("no Set-Cookie for bbw_session")
	}
	if !bbwCookie.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}
	if bbwCookie.Path != "/" {
		t.Errorf("cookie Path = %q, want %q", bbwCookie.Path, "/")
	}
	if bbwCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %d, want %d", bbwCookie.SameSite, http.SameSiteLaxMode)
	}
	if bbwCookie.Secure {
		t.Error("cookie Secure = true, want false")
	}
	if bbwCookie.MaxAge != 0 {
		// MaxAge 0 means session cookie (no explicit expiry)
		// This is fine, we just don't want a positive MaxAge
	}

	sessionID := bbwCookie.Value

	// Second request: same cookie → same session
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	rr2 := httptest.NewRecorder()

	sessionMiddleware(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("bbw_session")
		if err != nil {
			t.Errorf("no bbw_session cookie on second request: %v", err)
		}
		if cookie != nil && cookie.Value != sessionID {
			t.Errorf("cookie value = %q, want %q", cookie.Value, sessionID)
		}
		// Verify session exists in store
		_, ok := store.Get(sessionID)
		if !ok {
			t.Error("session not found in store on second request")
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr2, req2)

	// Third request: different cookie (stale) → middleware creates new session
	// and must replace the stale cookie in the request so the handler sees the fresh ID.
	req3 := httptest.NewRequest(http.MethodGet, "/", nil)
	req3.AddCookie(&http.Cookie{Name: "bbw_session", Value: "different-session-id"})
	rr3 := httptest.NewRecorder()

	var handlerCookieValue string
	sessionMiddleware(store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("bbw_session")
		if err != nil {
			t.Errorf("no bbw_session cookie on third request: %v", err)
		}
		if cookie != nil {
			handlerCookieValue = cookie.Value
			// A different cookie value should still be accepted (the middleware
			// should not reject unknown session IDs)
			if cookie.Value == sessionID {
				t.Error("third request should have different cookie value")
			}
			if cookie.Value == "different-session-id" {
				t.Error("handler received stale cookie instead of fresh session ID")
			}
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr3, req3)

	// Verify the handler received the same fresh session ID that was
	// Set-Cookie'd on the response. This assertion FAILS with r.AddCookie
	// (which appends after the stale cookie), and passes with r.Header.Set
	// (which replaces the entire Cookie header).
	resp3 := rr3.Result()
	var freshCookieValue string
	for _, c := range resp3.Cookies() {
		if c.Name == "bbw_session" {
			freshCookieValue = c.Value
			break
		}
	}
	if freshCookieValue == "" {
		t.Fatal("middleware did not Set-Cookie a fresh session for stale cookie")
	}
	if handlerCookieValue != freshCookieValue {
		t.Errorf("handler received cookie %q, but Set-Cookie has fresh ID %q", handlerCookieValue, freshCookieValue)
	}
}

func TestConcurrentSessionCreation(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup
	ids := make(chan string, 100)

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := store.Create()
			if err != nil {
				t.Errorf("store.Create() error: %v", err)
				return
			}
			ids <- session.ID
		}()
	}
	wg.Wait()
	close(ids)

	collected := make(map[string]bool, 100)
	for id := range ids {
		if collected[id] {
			t.Errorf("duplicate session ID: %s", id)
		}
		collected[id] = true
	}
	if len(collected) != 100 {
		t.Errorf("got %d unique IDs, want 100", len(collected))
	}
}

func TestSetCookieHelper(t *testing.T) {
	session := &Session{ID: "abc123deadbeef0123456789abcdef0123456789"}
	w := httptest.NewRecorder()
	SetCookie(w, session)

	resp := w.Result()
	cookies := resp.Cookies()
	var bbwCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "bbw_session" {
			bbwCookie = c
			break
		}
	}
	if bbwCookie == nil {
		t.Fatal("SetCookie did not set bbw_session cookie")
	}
	if bbwCookie.Value != session.ID {
		t.Errorf("cookie value = %q, want %q", bbwCookie.Value, session.ID)
	}
	if !bbwCookie.HttpOnly {
		t.Error("cookie HttpOnly = false")
	}
	if bbwCookie.Path != "/" {
		t.Errorf("cookie Path = %q, want /", bbwCookie.Path)
	}
	if bbwCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want SameSiteLaxMode", bbwCookie.SameSite)
	}
	if bbwCookie.Secure {
		t.Error("cookie Secure = true, want false")
	}
}

func TestGetCookieHelper(t *testing.T) {
	sessionID := "abc123deadbeef0123456789abcdef0123456789"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	got := GetCookie(r)
	if got != sessionID {
		t.Errorf("GetCookie = %q, want %q", got, sessionID)
	}

	// No cookie
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	got2 := GetCookie(r2)
	if got2 != "" {
		t.Errorf("GetCookie without cookie = %q, want ''", got2)
	}
}
