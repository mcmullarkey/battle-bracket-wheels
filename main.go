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
	mux := setupRouter(store, tmpl)

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
func setupRouter(store *Store, tmpl *template.Template) http.Handler {
	mux := http.NewServeMux()

	// /health registered before / per spec
	mux.HandleFunc("/health", healthHandler)

	// Static assets via embed.FS
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler))

	// Home page with session middleware
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl)))

	// Wheel option CRUD routes
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("DELETE /wheel/{id}/option/{idx}", sessionMiddleware(store, deleteOptionHandler(store, tmpl)))

	// Spin route — weighted-random slice selection with client-side animation
	mux.Handle("POST /wheel/{id}/spin", sessionMiddleware(store, spinHandler(store, tmpl, newSpinSource)))

	// Battle route — spin two wheels, resolve, absorb loser's option
	mux.Handle("POST /battle/{matchID}", sessionMiddleware(store, battleHandler(store, tmpl, newSpinSource)))

	return mux
}
