// Package main provides the Battle Bracket Wheels web application.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/wheel"
)

// ErrSessionNotFound is returned by Store.Update when the session ID
// does not exist in the store.
var ErrSessionNotFound = errors.New("session not found")

// cookieName is the name of the session cookie.
const cookieName = "bbw_session"

// Session represents a user session with a unique ID, creation timestamp,
// 8 configurable wheels, bracket progression state, and resolved match tracking.
type Session struct {
	ID              string           `json:"id"`
	CreatedAt       time.Time        `json:"created_at"`
	Wheels          [8]wheel.Wheel   `json:"wheels"`
	Bracket         *bracket.Bracket `json:"bracket"`
	ResolvedMatches map[string]bool  `json:"resolved_matches"`
}

// newWheels initializes 8 empty wheels with IDs "0" through "7".
func newWheels() [8]wheel.Wheel {
	var wh [8]wheel.Wheel
	for i := range wh {
		wh[i] = wheel.Wheel{ID: fmt.Sprint(i)}
	}
	return wh
}

// Store is a concurrency-safe, in-memory session store.
// Use NewStore to create an instance — a nil Store cannot Create sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates and returns a new empty Store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

// Create generates a new session with a cryptographically random hex ID
// of at least 32 characters, records the creation time, and stores it.
func (s *Store) Create() (*Session, error) {
	// Generate 16 bytes → 32 hex chars (>= 32 required)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(bytes)

	wheels := newWheels()
	session := &Session{
		ID:              id,
		CreatedAt:       time.Now(),
		Wheels:          wheels,
		Bracket:         bracket.New(wheels),
		ResolvedMatches: make(map[string]bool),
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

// Get retrieves a session by ID. Returns the session and true if found,
// or nil and false if not.
func (s *Store) Get(id string) (*Session, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	return session, ok
}

// View provides read-only access to a session under the read lock.
// It calls fn with the session pointer. If the session does not exist,
// ErrSessionNotFound is returned and fn is not called.
// The caller's fn must not capture the session pointer past the function
// boundary (no storing it, no passing to goroutines).
func (s *Store) View(id string, fn func(*Session) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return fn(session)
}

// Update atomically reads and mutates a session under the write lock.
// It calls fn with the session pointer. If the session does not exist,
// ErrSessionNotFound is returned and fn is not called.
// The caller's fn must not capture the session pointer past the function
// boundary (no storing it, no passing to goroutines).
func (s *Store) Update(id string, fn func(*Session) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, id)
	}
	return fn(session)
}

// SetCookie writes the bbw_session cookie to the response writer
// with HttpOnly, Path=/, SameSite=Lax, and NOT Secure.
func SetCookie(w http.ResponseWriter, session *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    session.ID,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
}

// GetCookie extracts the bbw_session cookie value from the request.
// Returns empty string if no cookie is present.
func GetCookie(r *http.Request) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
