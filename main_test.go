package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
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

	// Check starfield (@keyframes or canvas)
	if !strings.Contains(body, "@keyframes") && !strings.Contains(body, "canvas") {
		t.Error("response body missing starfield (@keyframes or canvas)")
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

func TestEmbedStaticAssets(t *testing.T) {
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded static/css/space.css is empty")
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
