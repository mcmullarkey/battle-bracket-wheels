package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/layout.html
var layoutContent string

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
	tmpl, err := template.New("layout").Parse(layoutContent)
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
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

// setupRouter creates and configures the HTTP mux with all routes.
func setupRouter(store *Store, tmpl *template.Template) http.Handler {
	mux := http.NewServeMux()

	// /health registered before / per spec
	mux.HandleFunc("/health", healthHandler)

	// Static assets via embed.FS
	staticHandler := http.FileServer(http.FS(staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler))

	// Home page with session middleware
	mux.Handle("/", sessionMiddleware(store, homeHandler(tmpl)))

	return mux
}

// toString is a template helper that converts interface{} to string.
func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// trimSpace trims whitespace from a string.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
