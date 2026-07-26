package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"battle-bracket-wheels/internal/theme"
)

// testThemeServer creates a test server with the theme registry wired up
// (single "space" theme registered, mirroring production main()).
func testThemeServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := NewStore()
	tmpl := testTemplate(t)
	registry := testRegistry(t)
	mux := setupRouter(store, tmpl, registry)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// testThemeServerTwoThemes creates a test server with two themes registered
// (space + y2k) for sneaky-pass guards that verify href is dynamic.
func testThemeServerTwoThemes(t *testing.T) *httptest.Server {
	t.Helper()
	store := NewStore()
	tmpl := testTemplate(t)
	registry := theme.NewRegistry()
	if err := registry.Register("space", "Space", "/static/css/space.css"); err != nil {
		t.Fatalf("Register space: %v", err)
	}
	if err := registry.Register("y2k", "Y2K", "/static/css/y2k.css"); err != nil {
		t.Fatalf("Register y2k: %v", err)
	}
	mux := setupRouter(store, tmpl, registry)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// noRedirectClient returns an HTTP client that does not follow redirects,
// so tests can inspect the initial response (e.g. 303 + Set-Cookie) without
// the client transparently following to GET /.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestThemeEndpointPost303(t *testing.T) {
	ts := testThemeServer(t)
	form := url.Values{"theme": {"space"}}
	resp, err := noRedirectClient().PostForm(ts.URL+"/theme", form)
	if err != nil {
		t.Fatalf("POST /theme: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
	var themeCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "bbw_theme" {
			themeCookie = c
		}
	}
	if themeCookie == nil {
		t.Fatal("missing bbw_theme cookie")
	}
	if themeCookie.Value != "space" {
		t.Errorf("bbw_theme = %q, want space", themeCookie.Value)
	}
	if !themeCookie.HttpOnly {
		t.Error("bbw_theme cookie not HttpOnly")
	}
	if themeCookie.Path != "/" {
		t.Errorf("bbw_theme Path = %q, want /", themeCookie.Path)
	}
	if themeCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("bbw_theme SameSite = %v, want Lax", themeCookie.SameSite)
	}
	if themeCookie.Secure {
		t.Error("bbw_theme Secure = true, want false")
	}
}

func TestThemeEndpointPostEmpty400(t *testing.T) {
	ts := testThemeServer(t)
	resp, err := http.PostForm(ts.URL+"/theme", url.Values{})
	if err != nil {
		t.Fatalf("POST /theme: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "bbw_theme" {
			t.Error("unexpected bbw_theme cookie set on 400")
		}
	}
}

func TestThemeEndpointPostUnknown400(t *testing.T) {
	ts := testThemeServer(t)
	form := url.Values{"theme": {"nonexistent"}}
	resp, err := http.PostForm(ts.URL+"/theme", form)
	if err != nil {
		t.Fatalf("POST /theme: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "bbw_theme" {
			t.Error("unexpected bbw_theme cookie set on 400")
		}
	}
}

func TestThemeEndpointGet405(t *testing.T) {
	ts := testThemeServer(t)
	resp, err := http.Get(ts.URL + "/theme")
	if err != nil {
		t.Fatalf("GET /theme: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestThemeRenderingNoCookie(t *testing.T) {
	ts := testThemeServer(t)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(string(body), `href="/static/css/space.css"`) {
		t.Errorf("body missing link href=/static/css/space.css (default theme)\nbody: %s", body)
	}
}

func TestThemeRenderingWithCookie(t *testing.T) {
	ts := testThemeServer(t)
	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "space"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(string(body), `href="/static/css/space.css"`) {
		t.Errorf("body missing link href=/static/css/space.css\nbody: %s", body)
	}
}

func TestThemeRenderingUnknownCookieFallback(t *testing.T) {
	// Unknown cookie value → fallback to Default().CSSPath (closed set).
	ts := testThemeServer(t)
	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "nonexistent"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(string(body), `href="/static/css/space.css"`) {
		t.Errorf("unknown cookie should fallback to Default().CSSPath\nbody: %s", body)
	}
}

func TestThemeRenderingCookieSelectsTheme(t *testing.T) {
	// Sneaky-pass guard: href must come from resolved Theme.CSSPath, not hardcoded.
	// Register a 2nd theme (y2k), set cookie to y2k, verify href changes.
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
	if !strings.Contains(string(body), `href="/static/css/y2k.css"`) {
		t.Errorf("body missing href=/static/css/y2k.css (cookie-selected theme)\nbody: %s", body)
	}
	// space.css must NOT be the active stylesheet link when y2k is selected.
	// Dropdown options use Key/Name, not CSSPath, so space.css should not appear at all.
	if strings.Contains(string(body), `/static/css/space.css`) {
		t.Errorf("body should not contain space.css when y2k theme is active\nbody: %s", body)
	}
}

func TestThemeDropdownRendered(t *testing.T) {
	ts := testThemeServer(t)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `class="theme-selector"`) {
		t.Error("body missing form.theme-selector")
	}
	if !strings.Contains(bodyStr, `action="/theme"`) {
		t.Error("body missing form action=/theme")
	}
	if !strings.Contains(bodyStr, `method="post"`) {
		t.Error("body missing form method=post")
	}
	if !strings.Contains(bodyStr, `name="theme"`) {
		t.Error("body missing select[name=theme]")
	}
	if !strings.Contains(bodyStr, `value="space"`) {
		t.Error("body missing option value=space")
	}
	// Current theme (space, default when no cookie) should carry selected attribute.
	if !strings.Contains(bodyStr, "selected") {
		t.Error("body missing selected attribute on current theme option")
	}
}

func TestThemeDropdownTwoThemes(t *testing.T) {
	// Dropdown must render one option per registered theme.
	ts := testThemeServerTwoThemes(t)
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, `value="space"`) {
		t.Error("body missing option value=space")
	}
	if !strings.Contains(bodyStr, `value="y2k"`) {
		t.Error("body missing option value=y2k")
	}
	// Default theme (space) should be selected when no cookie.
	if !strings.Contains(bodyStr, "selected") {
		t.Error("body missing selected attribute on default theme option")
	}
}

func TestThemeDropdownSelectedMatchesCookie(t *testing.T) {
	// When bbw_theme=y2k cookie is set, the y2k option should carry selected.
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
	// The y2k option should have selected; the space option should not.
	// Find the option tags and check which has selected.
	y2kOptionIdx := strings.Index(bodyStr, `value="y2k"`)
	spaceOptionIdx := strings.Index(bodyStr, `value="space"`)
	if y2kOptionIdx < 0 {
		t.Fatal("body missing option value=y2k")
	}
	if spaceOptionIdx < 0 {
		t.Fatal("body missing option value=space")
	}
	// Check that selected appears near the y2k option (within the same <option> tag).
	// Find the end of the y2k option tag.
	y2kOptionEnd := strings.Index(bodyStr[y2kOptionIdx:], ">")
	if y2kOptionEnd < 0 {
		t.Fatal("malformed y2k option tag")
	}
	y2kOptionTag := bodyStr[y2kOptionIdx : y2kOptionIdx+y2kOptionEnd+1]
	if !strings.Contains(y2kOptionTag, "selected") {
		t.Errorf("y2k option tag should contain 'selected' when cookie=y2k\ntag: %s", y2kOptionTag)
	}
	// The space option should NOT have selected.
	spaceOptionEnd := strings.Index(bodyStr[spaceOptionIdx:], ">")
	if spaceOptionEnd < 0 {
		t.Fatal("malformed space option tag")
	}
	spaceOptionTag := bodyStr[spaceOptionIdx : spaceOptionIdx+spaceOptionEnd+1]
	if strings.Contains(spaceOptionTag, "selected") {
		t.Errorf("space option tag should NOT contain 'selected' when cookie=y2k\ntag: %s", spaceOptionTag)
	}
}

func TestThemeSessionIndependence(t *testing.T) {
	// /theme route is NOT wrapped in sessionMiddleware.
	// Setting theme must NOT require or create a bbw_session cookie.
	ts := testThemeServer(t)
	form := url.Values{"theme": {"space"}}
	resp, err := noRedirectClient().PostForm(ts.URL+"/theme", form)
	if err != nil {
		t.Fatalf("POST /theme: %v", err)
	}
	defer resp.Body.Close()
	hasSession := false
	hasTheme := false
	for _, c := range resp.Cookies() {
		if c.Name == "bbw_session" {
			hasSession = true
		}
		if c.Name == "bbw_theme" {
			hasTheme = true
		}
	}
	if hasSession {
		t.Error("POST /theme set bbw_session cookie — route should NOT be wrapped in sessionMiddleware")
	}
	if !hasTheme {
		t.Error("POST /theme missing bbw_theme cookie")
	}
}

func TestThemeCookieSurvivesSessionDeletion(t *testing.T) {
	// Theme cookie is per-browser, independent of session lifecycle.
	// Setting a theme, then getting a fresh session, should preserve the theme cookie.
	ts := testThemeServer(t)

	// Set theme cookie.
	form := url.Values{"theme": {"space"}}
	resp, err := http.PostForm(ts.URL+"/theme", form)
	if err != nil {
		t.Fatalf("POST /theme: %v", err)
	}
	resp.Body.Close()

	// Now GET / with the theme cookie but NO session cookie.
	// sessionMiddleware will create a new session, but the theme cookie persists.
	req, err := http.NewRequest("GET", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "space"})
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp2.Body.Close()

	hasSession := false
	for _, c := range resp2.Cookies() {
		if c.Name == "bbw_session" {
			hasSession = true
		}
	}
	// sessionMiddleware creates a new session (sets bbw_session).
	if !hasSession {
		t.Error("GET / with theme cookie but no session should create bbw_session")
	}
	// The theme cookie was sent by the client; the server doesn't re-set it on GET /,
	// but the client still has it. We verify the request carried it and the page
	// rendered with the correct theme.
	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.Contains(string(body), `href="/static/css/space.css"`) {
		t.Error("GET / with theme cookie should render space.css (theme preserved)")
	}
}

// TestSetRequestCookiePreservesOtherCookies pins the contract that
// setRequestCookie replaces only the named cookie while preserving all others.
// Before the fix, sessionMiddleware used r.Header.Set("Cookie", ...) which
// wiped out every cookie except bbw_session — breaking bbw_theme resolution.
func TestSetRequestCookiePreservesOtherCookies(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: "stale-id"})
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "y2k"})
	req.AddCookie(&http.Cookie{Name: "other", Value: "keep"})

	setRequestCookie(req, "bbw_session", "fresh-id")

	// Session cookie must be updated.
	if got := GetCookie(req); got != "fresh-id" {
		t.Errorf("bbw_session = %q, want fresh-id", got)
	}
	// Theme cookie must survive.
	if tc, err := req.Cookie("bbw_theme"); err != nil || tc.Value != "y2k" {
		t.Errorf("bbw_theme = %q (err=%v), want y2k (preserved)", tc, err)
	}
	// Other cookies must survive.
	if oc, err := req.Cookie("other"); err != nil || oc.Value != "keep" {
		t.Errorf("other = %q (err=%v), want keep (preserved)", oc, err)
	}
}

// TestSetRequestCookieAddsWhenAbsent verifies setRequestCookie adds the cookie
// when it doesn't already exist.
func TestSetRequestCookieAddsWhenAbsent(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "y2k"})

	setRequestCookie(req, "bbw_session", "new-id")

	if got := GetCookie(req); got != "new-id" {
		t.Errorf("bbw_session = %q, want new-id", got)
	}
	if tc, err := req.Cookie("bbw_theme"); err != nil || tc.Value != "y2k" {
		t.Errorf("bbw_theme = %q (err=%v), want y2k (preserved)", tc, err)
	}
}
