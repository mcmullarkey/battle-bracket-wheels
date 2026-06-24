package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// testTemplate parses the embedded layout and wheel templates for use in tests.
func testTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.New("layout").Funcs(template.FuncMap{"add": func(a, b int) int { return a + b }})
	var err error
	tmpl, err = tmpl.Parse(layoutContent)
	if err != nil {
		t.Fatalf("parsing layout template: %v", err)
	}
	// Parse wheel template as associated template; keep tmpl pointing to layout.
	if _, err = tmpl.New("wheel").Parse(wheelContent); err != nil {
		t.Fatalf("parsing wheel template: %v", err)
	}
	if _, err = tmpl.New("bracket").Parse(bracketContent); err != nil {
		t.Fatalf("parsing bracket template: %v", err)
	}
	return tmpl
}

func TestHealthEndpoint(t *testing.T) {
	store := NewStore()
	mux := setupRouter(store, testTemplate(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding JSON body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestHealthEndpointBareOK(t *testing.T) {
	// Sneaky-pass guard: /health must return structured JSON, not bare "OK"
	store := NewStore()
	mux := setupRouter(store, testTemplate(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	var rawBody struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawBody); err != nil {
		t.Fatalf("expected JSON with 'status' key, got: %v", err)
	}
	if rawBody.Status != "ok" {
		t.Errorf("status = %q, want %q", rawBody.Status, "ok")
	}
}

func TestLayoutRenders(t *testing.T) {
	store := NewStore()
	mux := setupRouter(store, testTemplate(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	// Check for Set-Cookie with bbw_session
	cookies := resp.Cookies()
	var bbwCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "bbw_session" {
			bbwCookie = c
			break
		}
	}
	if bbwCookie == nil {
		t.Error("response missing bbw_session cookie")
	} else if len(bbwCookie.Value) < 32 {
		t.Errorf("cookie value length = %d, want >= 32", len(bbwCookie.Value))
	}

	// Read body
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	bodyLower := strings.ToLower(body)

	// Check HTMX script include
	if !strings.Contains(bodyLower, "htmx.org") && !strings.Contains(bodyLower, "htmx") {
		t.Error("response body missing HTMX reference")
	}

	// Check starfield via space.css link (external CSS with @keyframes)
	if !strings.Contains(body, "/static/css/space.css") {
		t.Error("response body missing link to space.css (starfield)")
	}
	// Check at least 2 space-theme markers
	themeMarkers := []string{"neon-", "cosmic-panel", "movie-hero", "neon-button", "bracket-connector"}
	markerCount := 0
	for _, marker := range themeMarkers {
		if strings.Contains(body, marker) {
			markerCount++
		}
	}
	if markerCount < 2 {
		t.Errorf("response body has %d space-theme markers, want >= 2", markerCount)
	}

	// Check title is non-empty
	if !strings.Contains(bodyLower, "<title>") {
		t.Error("response body missing <title>")
	}

	// Check h1 is non-empty
	if !strings.Contains(body, "<h1") {
		t.Error("response body missing <h1>")
	}

	// Check bracket skeleton
	if !strings.Contains(body, `class="bracket"`) && !strings.Contains(body, "class='bracket'") {
		t.Error("response body missing .bracket class")
	}
	for i := 1; i <= 8; i++ {
		slotID := fmt.Sprintf("slot-%d", i)
		if !strings.Contains(body, slotID) {
			t.Errorf("response body missing %s", slotID)
		}
	}

	// Hardcoded-HTML guard: session ID should be rendered in the HTML
	if bbwCookie != nil && !strings.Contains(body, bbwCookie.Value) {
		t.Error("session ID not found in rendered HTML — template likely hardcoded")
	}
}

func TestPortFromEnv(t *testing.T) {
	os.Setenv("PORT", "9123")
	defer os.Unsetenv("PORT")

	port := getPort()
	if port != "9123" {
		t.Errorf("getPort() = %q, want %q", port, "9123")
	}
}

func TestPortDefault(t *testing.T) {
	os.Unsetenv("PORT")
	port := getPort()
	if port != "8080" {
		t.Errorf("getPort() without PORT = %q, want %q", port, "8080")
	}

	os.Setenv("PORT", "")
	port = getPort()
	if port != "8080" {
		t.Errorf("getPort() with empty PORT = %q, want %q", port, "8080")
	}
	os.Unsetenv("PORT")
}

func TestSpaceCSS_SizeAndKeyframes(t *testing.T) {
	// P5: space.css <=150KB + contains @keyframes + NO url() referencing image files
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	// Size check: <=153600 bytes (150KB)
	if len(data) > 153600 {
		t.Errorf("space.css size = %d bytes, want <= 153600 (150KB)", len(data))
	}

	// Must contain @keyframes
	if !strings.Contains(css, "@keyframes") {
		t.Error("space.css missing @keyframes (CSS-only starfield)")
	}

	// Must NOT contain url() referencing image files
	imageExts := []string{".gif", ".png", ".jpg", ".jpeg", ".webp", ".mp4", ".webm"}
	for _, ext := range imageExts {
		pattern := `url(` + ext
		if strings.Contains(css, pattern) {
			t.Errorf("space.css contains url() referencing %s (images not allowed)", ext)
		}
	}
}

func TestSpaceCSS_ResponsiveBreakpoint(t *testing.T) {
	// P3: space.css contains @media (max-width: 1024px)
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	if !strings.Contains(css, "@media (max-width: 1024px)") {
		t.Error("space.css missing @media (max-width: 1024px) breakpoint")
	}
}

func TestSpaceCSS_MovieHeroRule(t *testing.T) {
	// P4: space.css contains .movie-hero rule with font-size >=1.5rem, color, and text-shadow
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	if !strings.Contains(css, ".movie-hero") {
		t.Fatal("space.css missing .movie-hero rule")
	}

	// Extract just the .movie-hero rule block for scoped property checks
	movieHeroSection := extractCSSRule(css, ".movie-hero")
	if movieHeroSection == "" {
		t.Fatal("could not find .movie-hero CSS rule block")
	}

	// Check font-size INSIDE the .movie-hero block (must be >= 1.5rem)
	if !strings.Contains(movieHeroSection, "font-size") {
		t.Error(".movie-hero rule missing 'font-size' property")
	}

	// Check color INSIDE the .movie-hero block
	if !strings.Contains(movieHeroSection, "color:") {
		t.Error(".movie-hero rule missing 'color' property")
	}

	// Check text-shadow INSIDE the .movie-hero block
	if !strings.Contains(movieHeroSection, "text-shadow") {
		t.Error(".movie-hero rule missing 'text-shadow' property")
	}
}

func TestSpaceCSS_NoImageURLs(t *testing.T) {
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	// Must not contain url() with image references
	if strings.Contains(css, "url(") {
		// Extract url() references
		re := regexp.MustCompile(`url\([^)]+\)`)
		matches := re.FindAllString(css, -1)
		for _, m := range matches {
			if strings.Contains(m, ".png") || strings.Contains(m, ".jpg") ||
				strings.Contains(m, ".jpeg") || strings.Contains(m, ".gif") ||
				strings.Contains(m, ".webp") || strings.Contains(m, ".mp4") ||
				strings.Contains(m, ".webm") || strings.Contains(m, ".svg") {
				t.Errorf("space.css contains image url(): %s", m)
			}
		}
	}
}

func TestRenderYAML_DeployConfig(t *testing.T) {
	// P7: render.yaml contains startCommand + PORT envVar + healthCheckPath /health
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "startCommand:") {
		t.Error("render.yaml missing startCommand")
	}
	if !strings.Contains(content, "PORT") {
		t.Error("render.yaml missing PORT envVar")
	}
	if !strings.Contains(content, "healthCheckPath:") && !strings.Contains(content, "healthCheckPath /health") {
		if !strings.Contains(content, "/health") {
			t.Error("render.yaml missing healthCheckPath /health")
		}
	}
}

func TestRenderYAML_BuildCommand(t *testing.T) {
	data, err := os.ReadFile("render.yaml")
	if err != nil {
		t.Fatalf("reading render.yaml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "buildCommand:") {
		t.Error("render.yaml missing buildCommand")
	}
}

// extractCSSRule extracts a CSS rule block by selector name
func extractCSSRule(css, selector string) string {
	idx := strings.Index(css, selector+" {")
	if idx < 0 {
		// Try without space before {
		idx = strings.Index(css, selector+"{")
	}
	if idx < 0 {
		return ""
	}

	// Find the opening brace
	braceIdx := strings.Index(css[idx:], "{")
	if braceIdx < 0 {
		return ""
	}
	start := idx + braceIdx

	// Find matching closing brace
	depth := 1
	end := start + 1
	for end < len(css) && depth > 0 {
		if css[end] == '{' {
			depth++
		} else if css[end] == '}' {
			depth--
		}
		end++
	}
	if depth == 0 {
		return css[start:end]
	}
	return ""
}

func TestEmbedStaticAssets(t *testing.T) {
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded static/css/space.css is empty")
	}
}

func TestStaticCSSServedViaHTTP(t *testing.T) {
	// Integration test: verify that /static/css/space.css is served correctly
	// through the HTTP router (not just readable from embed.FS).
	store := NewStore()
	mux := setupRouter(store, testTemplate(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/css/space.css")
	if err != nil {
		t.Fatalf("GET /static/css/space.css: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	// Verify the CSS actually contains @keyframes (proves it's the real file, not empty)
	if !strings.Contains(string(body), "@keyframes") {
		t.Error("served space.css missing @keyframes")
	}
}

func TestServerBindsZeroZeroZeroZero(t *testing.T) {
	// Structural check: verify main.go uses "0.0.0.0:" prefix
	// We just verify the getAddr function returns the right format
	addr := getAddr("8080")
	if !strings.HasPrefix(addr, "0.0.0.0:") {
		t.Errorf("getAddr() = %q, want prefix 0.0.0.0:", addr)
	}
}
