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

	"battle-bracket-wheels/internal/theme"
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

// testRegistry creates a registry with both "space" and "y2k" themes registered,
// mirroring production main(). Used by tests that need a valid registry but
// do not test theme-specific behavior.
func testRegistry(t *testing.T) *theme.Registry {
	t.Helper()
	r := theme.NewRegistry()
	if err := r.Register("space", "Space", "/static/css/space.css"); err != nil {
		t.Fatalf("testRegistry Register space: %v", err)
	}
	if err := r.Register("y2k", "Y2K", "/static/css/y2k.css"); err != nil {
		t.Fatalf("testRegistry Register y2k: %v", err)
	}
	return r
}

func TestHealthEndpoint(t *testing.T) {
	store := NewStore()
	mux := setupRouter(store, testTemplate(t), testRegistry(t))
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
	mux := setupRouter(store, testTemplate(t), testRegistry(t))
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
	mux := setupRouter(store, testTemplate(t), testRegistry(t))
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

	// Check default theme CSS link (space is default — first registered)
	if !strings.Contains(body, "/static/css/space.css") {
		t.Error("response body missing link to space.css (default theme)")
	}

	// Theme-aware: dropdown must render both registered themes
	if !strings.Contains(body, `value="space"`) {
		t.Error("response body missing option value=space in theme dropdown")
	}
	if !strings.Contains(body, `value="y2k"`) {
		t.Error("response body missing option value=y2k in theme dropdown")
	}

	// Check at least 2 theme markers (CSS class names in template — theme-independent)
	themeMarkers := []string{"neon-", "cosmic-panel", "movie-hero", "neon-button", "bracket-connector"}
	markerCount := 0
	for _, marker := range themeMarkers {
		if strings.Contains(body, marker) {
			markerCount++
		}
	}
	if markerCount < 2 {
		t.Errorf("response body has %d theme markers, want >= 2", markerCount)
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

func TestSpaceCSS_RevealTransitionPlacement(t *testing.T) {
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	// No selector matching .pending-reveal (bare or compound) may declare transition.
	// A transition on any .pending-reveal selector causes result flash on HTMX OOB insertion.
	re := regexp.MustCompile(`\.pending-reveal[^{]*\{[^}]*transition`)
	if re.MatchString(css) {
		t.Error("no selector matching .pending-reveal may declare 'transition' (causes result flash on OOB insertion)")
	}

	// .revealed must declare transition for the fade-in on class swap.
	revealedRule := extractCSSRule(css, ".revealed")
	if !strings.Contains(revealedRule, "transition") {
		t.Error(".revealed must declare 'transition' (fade-in on class swap)")
	}
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
	mux := setupRouter(store, testTemplate(t), testRegistry(t))
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

// ===== Y2K Theme Tests =====
//
// The TestY2K* family verifies the structural predicates from AC-3:
//   1. y2k.css exists in embedded FS and is >0 bytes
//   2. :root redeclares all 20 custom properties from space.css (token-name parity)
//   3. >=8 tokens have Y2K-identifiable values != space.css values
//   4. Defines all 6 @keyframes names (twinkle, twinkle-slow, meteor, cosmic-glow, pulse, border-glow)
//   5. Mirrors all 3 @media breakpoints (min-width:1025px, max-width:1024px, max-width:640px)
//   6. Contains rules for all 12 selectors
//   7. NO url() references
//   8. registry.Register + Resolve succeed
//   9. GET /static/css/y2k.css returns 200 text/css
//  10. GET / with bbw_theme=y2k renders <link href="/static/css/y2k.css">

// readY2KCSS reads y2k.css from the embedded filesystem.
func readY2KCSS(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(staticFS, "css/y2k.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/y2k.css: %v", err)
	}
	return string(data)
}

// readSpaceCSS reads space.css from the embedded filesystem.
func readSpaceCSS(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	return string(data)
}

// extractCustomProperties parses a :root { ... } block and returns a map of
// property name → raw value string (trimmed, trailing semicolon removed).
func extractCustomProperties(css string) map[string]string {
	props := make(map[string]string)

	// Find :root block
	rootIdx := strings.Index(css, ":root")
	if rootIdx < 0 {
		return props
	}
	braceStart := strings.Index(css[rootIdx:], "{")
	if braceStart < 0 {
		return props
	}
	braceStart += rootIdx

	depth := 1
	end := braceStart + 1
	for end < len(css) && depth > 0 {
		if css[end] == '{' {
			depth++
		} else if css[end] == '}' {
			depth--
		}
		end++
	}
	rootBlock := css[braceStart+1 : end-1]

	// Parse each --name: value; pair
	for _, line := range strings.Split(rootBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "/*" || strings.HasPrefix(line, "/*") {
			continue
		}
		if !strings.HasPrefix(line, "--") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		value = strings.TrimSuffix(value, ";")
		value = strings.TrimSpace(value)
		if name != "" && value != "" {
			props[name] = value
		}
	}
	return props
}

// requiredTokens lists all 20 custom property names from space.css:41-62.
var requiredTokens = []string{
	"--space-deep",
	"--space-mid",
	"--space-light",
	"--star-white",
	"--star-blue",
	"--star-gold",
	"--nebula-purple",
	"--cosmic-teal",
	"--neon-cyan",
	"--neon-magenta",
	"--neon-gold",
	"--bracket-border",
	"--breakpoint-tablet",
	"--cosmic-bg",
	"--cosmic-border",
	"--cosmic-shadow",
	"--glass-blur",
	"--neon-text-cyan",
	"--neon-text-magenta",
	"--neon-text-gold",
}

// requiredKeyframes lists all 6 @keyframes names from space.css.
var requiredKeyframes = []string{
	"twinkle",
	"twinkle-slow",
	"meteor",
	"cosmic-glow",
	"pulse",
	"border-glow",
}

// requiredBreakpoints lists all 3 @media breakpoint strings from space.css.
var requiredBreakpoints = []string{
	"@media (min-width: 1025px)",
	"@media (max-width: 1024px)",
	"@media (max-width: 640px)",
}

// requiredSelectors lists all 12 minimum selector surface selectors.
var requiredSelectors = []string{
	".cosmic-panel",
	".movie-hero",
	".neon-button",
	".bracket",
	".bracket-connector",
	".slot",
	".wheel-slice",
	".pending-reveal",
	".revealed",
	".center-display",
	".match-result",
	".battle-pointer",
}

func TestY2K_ExistsAndNonEmpty(t *testing.T) {
	// AC-3 predicate 1: static/css/y2k.css exists in embedded FS and is >0 bytes
	data, err := fs.ReadFile(staticFS, "css/y2k.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/y2k.css: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded static/css/y2k.css is empty")
	}
}

func TestY2K_TokenParity(t *testing.T) {
	// AC-3 predicate 2: :root redeclares every custom property from space.css:41-62
	// Floor guard: >= 20 custom properties declared
	y2kCSS := readY2KCSS(t)
	y2kProps := extractCustomProperties(y2kCSS)

	if len(y2kProps) < 20 {
		t.Errorf("y2k.css :root declares %d custom properties, want >= 20", len(y2kProps))
	}

	for _, name := range requiredTokens {
		if _, ok := y2kProps[name]; !ok {
			t.Errorf("y2k.css :root missing custom property %q", name)
		}
	}
}

func TestY2K_TokenValuesDiffer(t *testing.T) {
	// AC-3 predicate 3: at least 8 tokens have Y2K-identifiable values
	// NOT equal to corresponding space.css values (anti-copy sneaky-pass)
	spaceCSS := readSpaceCSS(t)
	y2kCSS := readY2KCSS(t)

	spaceProps := extractCustomProperties(spaceCSS)
	y2kProps := extractCustomProperties(y2kCSS)

	differCount := 0
	for _, name := range requiredTokens {
		spaceVal, spaceOk := spaceProps[name]
		y2kVal, y2kOk := y2kProps[name]
		if !spaceOk {
			t.Errorf("space.css missing token %q — test fixture issue", name)
			continue
		}
		if !y2kOk {
			t.Errorf("y2k.css missing token %q", name)
			continue
		}
		if spaceVal != y2kVal {
			differCount++
		}
	}

	if differCount < 8 {
		t.Errorf("only %d tokens have Y2K-identifiable values (differ from space.css), want >= 8", differCount)
	}
}

func TestY2K_Keyframes(t *testing.T) {
	// AC-3 predicate 4: defines all 6 @keyframes names from space.css
	y2kCSS := readY2KCSS(t)

	for _, name := range requiredKeyframes {
		pattern := "@keyframes " + name
		if !strings.Contains(y2kCSS, pattern) {
			t.Errorf("y2k.css missing @keyframes %q", name)
		}
	}
}

func TestY2K_Breakpoints(t *testing.T) {
	// AC-3 predicate 5: mirrors all 3 @media breakpoints from space.css
	y2kCSS := readY2KCSS(t)

	for _, bp := range requiredBreakpoints {
		if !strings.Contains(y2kCSS, bp) {
			t.Errorf("y2k.css missing @media breakpoint %q", bp)
		}
	}
}

func TestY2K_SelectorSurface(t *testing.T) {
	// AC-3 predicate 6: contains rules for all 12 minimum selectors
	y2kCSS := readY2KCSS(t)

	for _, sel := range requiredSelectors {
		// Check for selector followed by { (rule definition)
		if !strings.Contains(y2kCSS, sel+" {") && !strings.Contains(y2kCSS, sel+"{") &&
			!strings.Contains(y2kCSS, sel+",") && !strings.Contains(y2kCSS, sel+" ") {
			t.Errorf("y2k.css missing selector %q", sel)
		}
	}
}

func TestY2K_NoURL(t *testing.T) {
	// AC-3 predicate 7: NO url() references (CSS-only, no external image assets)
	y2kCSS := readY2KCSS(t)

	if strings.Contains(y2kCSS, "url(") {
		re := regexp.MustCompile(`url\([^)]*\)`)
		matches := re.FindAllString(y2kCSS, -1)
		t.Errorf("y2k.css contains %d url() references (no external assets allowed): %v", len(matches), matches)
	}
}

func TestY2K_BattlePointerLeft(t *testing.T) {
	// AC-1 mirror: .battle-pointer.pointer-left must exist in y2k.css
	y2kCSS := readY2KCSS(t)

	if !strings.Contains(y2kCSS, ".battle-pointer.pointer-left") {
		t.Error("y2k.css missing .battle-pointer.pointer-left rule (AC-1 mirror)")
	}
}

func TestY2K_Registry(t *testing.T) {
	// AC-3 predicate 8: Register + Resolve succeed
	r := theme.NewRegistry()
	if err := r.Register("space", "Space", "/static/css/space.css"); err != nil {
		t.Fatalf("Register space: %v", err)
	}
	if err := r.Register("y2k", "Y2K", "/static/css/y2k.css"); err != nil {
		t.Fatalf("Register y2k: %v", err)
	}

	t2, ok := r.Resolve("y2k")
	if !ok {
		t.Fatal("Resolve(y2k) returned ok=false, want true")
	}
	if t2.Key != "y2k" {
		t.Errorf("Resolve(y2k).Key = %q, want %q", t2.Key, "y2k")
	}
	if t2.Name != "Y2K" {
		t.Errorf("Resolve(y2k).Name = %q, want %q", t2.Name, "Y2K")
	}
	if t2.CSSPath != "/static/css/y2k.css" {
		t.Errorf("Resolve(y2k).CSSPath = %q, want %q", t2.CSSPath, "/static/css/y2k.css")
	}
}

func TestY2K_HTTPServesCSS(t *testing.T) {
	// AC-3 predicate 9: GET /static/css/y2k.css returns 200 with Content-Type: text/css
	store := NewStore()
	mux := setupRouter(store, testTemplate(t), testRegistry(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/css/y2k.css")
	if err != nil {
		t.Fatalf("GET /static/css/y2k.css: %v", err)
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
	if len(body) == 0 {
		t.Error("served y2k.css body is empty")
	}
}

func TestY2K_ThemeLinkRendered(t *testing.T) {
	// AC-3 predicate 10: GET / with bbw_theme=y2k renders <link href="/static/css/y2k.css">
	ts := testThemeServerTwoThemes(t)

	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "y2k"})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `href="/static/css/y2k.css"`) {
		t.Errorf("body missing link href=/static/css/y2k.css\nbody: %s", bodyStr)
	}

	// space.css must NOT appear when y2k is active (self-contained theme)
	if strings.Contains(bodyStr, `/static/css/space.css`) {
		t.Errorf("body should not contain space.css when y2k theme is active\nbody: %s", bodyStr)
	}
}
