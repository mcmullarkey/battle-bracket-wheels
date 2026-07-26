package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestPopulateUI_FormStructure verifies AC-1:
// GET / response body contains .populate-form with textarea[name="items"],
// button[type="submit"], hx-post="/wheels/populate", hx-target="#populate-status".
// Also verifies #populate-status exists and has NO inline display:none.
func TestPopulateUI_FormStructure(t *testing.T) {
	ts, _ := populateTestServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	// .populate-form class present
	if !strings.Contains(html, `class="populate-form"`) {
		t.Error("GET / response missing .populate-form class")
	}

	// textarea[name="items"]
	if !strings.Contains(html, `<textarea name="items"`) {
		t.Error("GET / response missing textarea[name='items']")
	}

	// textarea has rows >= 3
	if !strings.Contains(html, `rows="3"`) &&
		!strings.Contains(html, `rows="4"`) &&
		!strings.Contains(html, `rows="5"`) &&
		!strings.Contains(html, `rows="6"`) &&
		!strings.Contains(html, `rows="8"`) &&
		!strings.Contains(html, `rows="10"`) {
		t.Error("textarea missing rows attribute >= 3")
	}

	// button[type="submit"]
	if !strings.Contains(html, `type="submit"`) {
		t.Error("GET / response missing button[type='submit']")
	}

	// hx-post="/wheels/populate"
	if !strings.Contains(html, `hx-post="/wheels/populate"`) {
		t.Error("GET / response missing hx-post='/wheels/populate'")
	}

	// hx-target="#populate-status"
	if !strings.Contains(html, `hx-target="#populate-status"`) {
		t.Error("GET / response missing hx-target='#populate-status'")
	}

	// #populate-status exists
	if !strings.Contains(html, `id="populate-status"`) {
		t.Error("GET / response missing #populate-status element")
	}

	// #populate-status must NOT have inline display:none (negative case from spec)
	if strings.Contains(html, `id="populate-status" style="display:none`) ||
		strings.Contains(html, `id="populate-status" style="display: none`) ||
		strings.Contains(html, `id="populate-status" hidden`) {
		t.Error("#populate-status has inline display:none or hidden — must be visible")
	}
}

// TestPopulateUI_Success verifies AC-2:
// POST 8 valid items → 200, body contains 8 hx-swap-oob fragments targeting
// #slot-1..#slot-8 (exact spelling), fragment option text, AND 1 non-OOB swap.
func TestPopulateUI_Success(t *testing.T) {
	ts, _ := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// 8 valid items (minimum required by populate.ParseAndDistribute)
	items := "Movie A\nMovie B\nMovie C\nMovie D\nMovie E\nMovie F\nMovie G\nMovie H"
	form := url.Values{"items": {items}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheels/populate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	// Exactly 8 OOB elements
	oobCount := strings.Count(html, "hx-swap-oob")
	if oobCount != 8 {
		t.Errorf("response has %d hx-swap-oob elements, want 8", oobCount)
	}

	// Verify each expected OOB slot ID (exact spelling slot-1..slot-8, NOT slot-01)
	for i := 1; i <= 8; i++ {
		slotID := "slot-" + string(rune('0'+i))
		if !strings.Contains(html, `id="`+slotID+`"`) {
			t.Errorf("response missing %s OOB element", slotID)
		}
	}

	// Verify slot-01 is NOT present (negative case from spec)
	if strings.Contains(html, `id="slot-01"`) {
		t.Error("response contains slot-01 — should be slot-1 (exact spelling)")
	}

	// Verify non-OOB populate-status element is present (HTMX 2.x requirement)
	if !strings.Contains(html, `id="populate-status"`) {
		t.Error("response missing populate-status non-OOB element")
	}

	// Verify option text is present in fragments
	if !strings.Contains(html, "Movie A") {
		t.Error("response missing option text 'Movie A'")
	}
}

// TestPopulateUI_ErrorTooFew verifies AC-3:
// POST < 8 items → 400, error text rendered inside #populate-status,
// element visible (no inline display:none), Content-Type is text/html.
func TestPopulateUI_ErrorTooFew(t *testing.T) {
	ts, _ := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Only 3 items (below minimum of 8)
	form := url.Values{"items": {"a\nb\nc"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheels/populate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	// Content-Type must be text/html (not JSON) — error rendered as HTML fragment
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	// Error text rendered inside #populate-status
	if !strings.Contains(html, `id="populate-status"`) {
		t.Error("error response missing #populate-status element")
	}

	// Error text must be present (not empty)
	if !strings.Contains(html, "at least 8") {
		t.Errorf("error response missing error text 'at least 8', got: %s", html)
	}

	// #populate-status must NOT have inline display:none (negative case)
	if strings.Contains(html, `id="populate-status" style="display:none`) ||
		strings.Contains(html, `id="populate-status" style="display: none`) ||
		strings.Contains(html, `id="populate-status" hidden`) {
		t.Error("#populate-status has inline display:none or hidden — error must be visible")
	}
}

// TestPopulateUI_ErrorEmpty verifies AC-3:
// POST empty items → 400, error text rendered inside #populate-status as HTML.
func TestPopulateUI_ErrorEmpty(t *testing.T) {
	ts, _ := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"items": {""}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheels/populate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	if !strings.Contains(html, `id="populate-status"`) {
		t.Error("error response missing #populate-status element")
	}
}

// TestPopulateUI_HTMXErrorSwapConfig verifies that the layout registers an
// HTMX beforeSwap event listener that forces swapping of 4xx responses.
// HTMX 2.x does not swap 4xx/5xx responses by default — without this
// configuration, error fragments returned with a 400 status never reach
// the DOM and the user sees an empty #populate-status div.
func TestPopulateUI_HTMXErrorSwapConfig(t *testing.T) {
	ts, _ := populateTestServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	// Layout must register a beforeSwap listener that sets shouldSwap=true
	// for 4xx responses so error fragments are swapped into the DOM.
	if !strings.Contains(html, "htmx:beforeSwap") {
		t.Error("layout missing htmx:beforeSwap event listener for 4xx swap")
	}
	if !strings.Contains(html, "shouldSwap") {
		t.Error("layout missing shouldSwap property in beforeSwap listener")
	}
	if !strings.Contains(html, "400") {
		t.Error("layout missing 400 status threshold in beforeSwap listener")
	}
}

// TestPopulateUI_FormOuterHTMLSwap verifies that the populate form uses
// hx-swap="outerHTML" so the response replaces the target #populate-status
// div entirely. Without outerHTML, the default innerHTML swap inserts a
// response containing <div id="populate-status"> inside the existing
// #populate-status div, creating nested duplicate IDs.
func TestPopulateUI_FormOuterHTMLSwap(t *testing.T) {
	ts, _ := populateTestServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	// The populate form must use outerHTML swap on the #populate-status target.
	// hx-target="#populate-status" hx-swap="outerHTML" is unique to the populate
	// form (battle buttons use hx-target="this").
	if !strings.Contains(html, `hx-target="#populate-status" hx-swap="outerHTML"`) {
		t.Error("populate form missing hx-swap='outerHTML' — innerHTML swap creates nested duplicate #populate-status IDs")
	}
}

// TestPopulateUI_ErrorWhitespace verifies AC-3:
// POST whitespace-only items → 400, error text rendered inside #populate-status as HTML.
func TestPopulateUI_ErrorWhitespace(t *testing.T) {
	ts, _ := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"items": {"   \n  \n  "}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheels/populate: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	html := string(body)

	if !strings.Contains(html, `id="populate-status"`) {
		t.Error("error response missing #populate-status element")
	}
}
