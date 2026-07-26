package main

import (
	"crypto/rand"
	"embed"
	"encoding/binary"
	"html/template"
	"io/fs"
	"log"
	mathrand "math/rand"
	"net/http"
	"os"

	"battle-bracket-wheels/internal/theme"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/layout.html
var layoutContent string

//go:embed templates/wheel.html
var wheelContent string

//go:embed templates/match.html
var matchContent string

//go:embed templates/bracket.html
var bracketContent string

// staticFS is the embedded filesystem for serving static assets.
var staticFS fs.FS

func init() {
	var err error
	staticFS, err = fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("failed to create static sub-filesystem: %v", err)
	}
}

func main() {
	port := getPort()
	addr := getAddr(port)

	// Parse templates from embedded content (no template.Must per spec)
	tmpl := template.New("layout").Funcs(template.FuncMap{"add": func(a, b int) int { return a + b }})
	var err error
	tmpl, err = tmpl.Parse(layoutContent)
	if err != nil {
		log.Fatalf("failed to parse layout template: %v", err)
	}
	// Parse wheel template as an associated template; keep tmpl pointing to layout.
	if _, err = tmpl.New("wheel").Parse(wheelContent); err != nil {
		log.Fatalf("failed to parse wheel template: %v", err)
	}
	// Parse match result template as an associated template.
	if _, err = tmpl.New("matchResult").Parse(matchContent); err != nil {
		log.Fatalf("failed to parse match template: %v", err)
	}
	// Parse bracket fragment templates as associated templates.
	if _, err = tmpl.New("bracket").Parse(bracketContent); err != nil {
		log.Fatalf("failed to parse bracket template: %v", err)
	}

	store := NewStore()

	// Construct theme registry — registered once at startup.
	// The first registered theme ("space") becomes the Default.
	registry := theme.NewRegistry()
	if err := registry.Register("space", "Space", "/static/css/space.css"); err != nil {
		log.Fatalf("failed to register space theme: %v", err)
	}

	mux := setupRouter(store, tmpl, registry)

	log.Printf("Battle Bracket Wheels listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// getPort reads the PORT environment variable, defaulting to "8080".
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8080"
	}
	return port
}

// getAddr returns the listen address with the given port on 0.0.0.0.
func getAddr(port string) string {
	return "0.0.0.0:" + port
}

// newSpinSource creates a math/rand.Source seeded from crypto/rand.
// This provides non-deterministic spin results in production.
func newSpinSource() mathrand.Source {
	var seed int64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &seed); err != nil {
		// Fallback: crypto/rand failure is extremely unlikely, but if it
		// happens we use a timestamp-based seed rather than a fixed value.
		seed = int64(os.Getpid()) ^ int64(os.Getppid())
	}
	return mathrand.NewSource(seed)
}

// setupRouter creates and configures the HTTP mux with all routes.
func setupRouter(store *Store, tmpl *template.Template, registry *theme.Registry) http.Handler {
	mux := http.NewServeMux()

	// /health registered before / per spec
	mux.HandleFunc("/health", healthHandler)

	// Static assets via embed.FS
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler))

	// Theme selection — NOT wrapped in sessionMiddleware.
	// Theme is per-browser, session-independent (precedent: /health, /static/).
	// Registered without method prefix so non-POST methods get 405 from the
	// handler (Go 1.22+ ServeMux method-patterns fall through to catch-all
	// instead of returning 405 — mirrors the battleHandler pattern).
	mux.Handle("/theme", themeHandler(registry))

	// Home page with session middleware
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl, registry)))

	// Wheel option CRUD routes
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("DELETE /wheel/{id}/option/{idx}", sessionMiddleware(store, deleteOptionHandler(store, tmpl)))

	// Spin route — weighted-random slice selection with client-side animation
	mux.Handle("POST /wheel/{id}/spin", sessionMiddleware(store, spinHandler(store, tmpl, newSpinSource)))

	// Battle route — spin two wheels, resolve, absorb loser's option
	// Method is validated in the handler to allow proper 405 Method Not Allowed
	// handling (Go 1.22+ ServeMux route-method-patterns fall through to catch-all
	// for unmatched methods instead of returning 405).
	mux.Handle("/battle/{matchID}", sessionMiddleware(store, battleHandler(store, tmpl, newSpinSource)))

	// Populate route — parse list input, distribute round-robin across 8 wheels,
	// reset bracket and resolved matches under single store.Update lock.
	mux.Handle("POST /wheels/populate", sessionMiddleware(store, populateHandler(store, tmpl)))

	return mux
}
