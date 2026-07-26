// Package theme provides a closed-set theme registry that maps theme keys
// to stylesheet paths.
//
// This package is pure — no I/O, no net/http, no template concerns.
// It mirrors the internal/wheel, internal/battle, internal/bracket convention
// of a pure domain package decoupled from the HTTP layer.
//
// The Registry holds a closed set of registered themes. Unregistered keys
// (including path-traversal attempts like "../etc") resolve to (Theme{}, false),
// making path traversal impossible — the cookie value is never interpolated into
// a file path; it is only used as a lookup key against the closed set.
package theme

import (
	"errors"
	"sync"
)

// ErrDuplicateTheme is returned by Register when a theme key is already registered.
var ErrDuplicateTheme = errors.New("theme already registered")

// Theme represents a selectable UI theme with a stylesheet path.
type Theme struct {
	Key     string
	Name    string
	CSSPath string
}

// Registry holds a closed set of registered themes, preserving registration
// order for Default() and Names(). It is safe for concurrent use.
type Registry struct {
	mu     sync.RWMutex
	themes map[string]Theme
	order  []string
}

// NewRegistry creates and returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		themes: make(map[string]Theme),
	}
}

// Register adds a theme to the registry under the given key.
// Returns ErrDuplicateTheme if the key is already registered.
// The first registered theme becomes the Default.
func (r *Registry) Register(key, name, cssPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.themes[key]; exists {
		return ErrDuplicateTheme
	}
	r.themes[key] = Theme{Key: key, Name: name, CSSPath: cssPath}
	r.order = append(r.order, key)
	return nil
}

// Resolve looks up a theme by key. Returns the Theme and true if found,
// or (Theme{}, false) if the key is not registered.
// Unregistered keys — including path-traversal strings like "../etc" —
// always return (Theme{}, false) because the registry is a closed set.
func (r *Registry) Resolve(key string) (Theme, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.themes[key]
	return t, ok
}

// Default returns the first registered theme, or Theme{} if the registry
// is empty.
func (r *Registry) Default() Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.order) == 0 {
		return Theme{}
	}
	return r.themes[r.order[0]]
}

// Names returns the keys of all registered themes in registration order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Themes returns all registered themes in registration order.
// Used by the home handler to render the dropdown options.
func (r *Registry) Themes() []Theme {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Theme, len(r.order))
	for i, key := range r.order {
		out[i] = r.themes[key]
	}
	return out
}
