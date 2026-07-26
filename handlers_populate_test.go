package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"battle-bracket-wheels/internal/bracket"
	"battle-bracket-wheels/internal/populate"
	"battle-bracket-wheels/internal/wheel"
)

// populateTestServer creates a test server with the populate handler,
// add-option handler, and home handler (for session cookie bootstrap).
// Returns the server and store (for state inspection in tests).
func populateTestServer(t *testing.T) (*httptest.Server, *Store) {
	t.Helper()
	store := NewStore()
	tmpl := testBattleTemplate(t)
	mux := http.NewServeMux()
	mux.Handle("POST /wheels/populate", sessionMiddleware(store, populateHandler(store, tmpl)))
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl, testRegistry(t))))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, store
}

// sixteenEntries returns 16 newline-separated entries for testing.
func sixteenEntries() string {
	entries := []string{
		"Movie 01", "Movie 02", "Movie 03", "Movie 04",
		"Movie 05", "Movie 06", "Movie 07", "Movie 08",
		"Movie 09", "Movie 10", "Movie 11", "Movie 12",
		"Movie 13", "Movie 14", "Movie 15", "Movie 16",
	}
	return strings.Join(entries, "\n")
}

// sixteenEntriesV2 returns a different set of 16 entries for idempotent-replace tests.
func sixteenEntriesV2() string {
	entries := []string{
		"Alpha", "Bravo", "Charlie", "Delta",
		"Echo", "Foxtrot", "Golf", "Hotel",
		"India", "Juliet", "Kilo", "Lima",
		"Mike", "November", "Oscar", "Papa",
	}
	return strings.Join(entries, "\n")
}

func TestPopulateHandler_Success(t *testing.T) {
	ts, _ := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"items": {sixteenEntries()}}
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

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	buf := make([]byte, 1<<20)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Exactly 8 OOB elements (slot-1..slot-8 via nextRoundSlot template)
	oobCount := strings.Count(body, "hx-swap-oob")
	if oobCount != 8 {
		t.Errorf("response has %d hx-swap-oob elements, want 8", oobCount)
	}

	// Verify each expected OOB slot ID is present (slot-1 through slot-8)
	for i := 1; i <= 8; i++ {
		slotID := "slot-" + string(rune('0'+i))
		if !strings.Contains(body, `id="`+slotID+`"`) {
			t.Errorf("response missing %s OOB element", slotID)
		}
	}

	// Verify non-OOB populate-status element is present
	if !strings.Contains(body, `id="populate-status"`) {
		t.Error("response missing populate-status non-OOB element")
	}
}

func TestPopulateHandler_MutationGate(t *testing.T) {
	ts, store := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Pre-populate: add "OLD" to wheel 0 and mark a match as resolved
	addOptionToWheel(t, ts, sessionID, "0", "OLD")
	err := store.Update(sessionID, func(s *Session) error {
		s.ResolvedMatches["qf1"] = true
		return nil
	})
	if err != nil {
		t.Fatalf("pre-populate store.Update: %v", err)
	}

	// Capture pre-populate bracket pointer and resolvedMatches map reference
	var oldBracket *bracket.Bracket
	var oldResolved map[string]bool
	err = store.View(sessionID, func(s *Session) error {
		oldBracket = s.Bracket
		oldResolved = s.ResolvedMatches
		return nil
	})
	if err != nil {
		t.Fatalf("pre-populate store.View: %v", err)
	}

	// Compute expected result for comparison
	expected, err := populate.ParseAndDistribute(sixteenEntries())
	if err != nil {
		t.Fatalf("populate.ParseAndDistribute: %v", err)
	}

	// POST /wheels/populate
	form := url.Values{"items": {sixteenEntries()}}
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
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Mutation gate: store.View read-back
	var wheels [8]wheel.Wheel
	var newBracket *bracket.Bracket
	var newResolved map[string]bool
	err = store.View(sessionID, func(s *Session) error {
		wheels = s.Wheels
		newBracket = s.Bracket
		newResolved = s.ResolvedMatches
		return nil
	})
	if err != nil {
		t.Fatalf("post-populate store.View: %v", err)
	}

	// 1. s.Wheels matches result.Wheels for all 8 wheels (replace, not append)
	for i := range wheels {
		if !reflect.DeepEqual(wheels[i], expected.Wheels[i]) {
			t.Errorf("wheel %d: store wheels != result wheels\nstore: %+v\nresult: %+v",
				i, wheels[i], expected.Wheels[i])
		}
	}

	// 2. Pre-existing "OLD" option absent from wheel 0
	for _, opt := range wheels[0].Options {
		if opt.Text == "OLD" {
			t.Error("wheel 0 still contains 'OLD' option — replace mode not working")
		}
	}

	// 3. s.Bracket is fresh (pointer != pre-populate)
	if newBracket == oldBracket {
		t.Error("bracket pointer unchanged — expected fresh bracket.New()")
	}

	// 4. s.Bracket.Slots matches result.Wheels
	if !reflect.DeepEqual(newBracket.Slots, expected.Wheels) {
		t.Error("bracket.Slots != result.Wheels — bracket not rebuilt from new wheels")
	}

	// 5. s.ResolvedMatches is fresh make(map[string]bool)
	if newResolved == nil {
		t.Error("ResolvedMatches is nil — expected fresh make(map[string]bool)")
	}
	if len(newResolved) != 0 {
		t.Errorf("ResolvedMatches len = %d, want 0 (fresh map)", len(newResolved))
	}
	// Verify map reference changed (fresh make, not cleared existing map)
	if reflect.ValueOf(newResolved).Pointer() == reflect.ValueOf(oldResolved).Pointer() {
		t.Error("ResolvedMatches map reference unchanged — expected fresh make()")
	}
}

func TestPopulateHandler_TooFewEntries(t *testing.T) {
	ts, store := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Pre-populate: add "OLD" to wheel 0
	addOptionToWheel(t, ts, sessionID, "0", "OLD")

	// Capture pre-populate state for unchanged verification
	var oldWheel0 wheel.Wheel
	_ = store.View(sessionID, func(s *Session) error {
		oldWheel0 = s.Wheels[0]
		return nil
	})

	// POST with 7 entries (below minimum of 8)
	form := url.Values{"items": {"a\nb\nc\nd\ne\nf\ng"}}
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
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding JSON error: %v", err)
	}
	if !strings.Contains(body.Error, "at least 8 entries") {
		t.Errorf("error message = %q, want it to contain 'at least 8 entries'", body.Error)
	}

	// Session unchanged: wheel 0 still has "OLD"
	var newWheel0 wheel.Wheel
	err = store.View(sessionID, func(s *Session) error {
		newWheel0 = s.Wheels[0]
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}
	if !reflect.DeepEqual(newWheel0, oldWheel0) {
		t.Error("session changed after error — wheels should be unchanged")
	}
}

func TestPopulateHandler_IdempotentReplace(t *testing.T) {
	ts, store := populateTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// First populate with set A
	form := url.Values{"items": {sixteenEntries()}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheels/populate (1st): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("1st populate status = %d, want 200", resp.StatusCode)
	}

	// Second populate with set B (different entries)
	form2 := url.Values{"items": {sixteenEntriesV2()}}
	req2, err := http.NewRequest(http.MethodPost, ts.URL+"/wheels/populate", strings.NewReader(form2.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST /wheels/populate (2nd): %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("2nd populate status = %d, want 200", resp2.StatusCode)
	}

	// Verify store has set B, not set A
	expectedB, _ := populate.ParseAndDistribute(sixteenEntriesV2())
	var wheels [8]wheel.Wheel
	err = store.View(sessionID, func(s *Session) error {
		wheels = s.Wheels
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}
	for i := range wheels {
		if !reflect.DeepEqual(wheels[i], expectedB.Wheels[i]) {
			t.Errorf("wheel %d: store has set A, want set B (replace not working)", i)
		}
	}
}

func TestPopulateHandler_NoSession(t *testing.T) {
	store := NewStore()
	tmpl := testBattleTemplate(t)
	handler := populateHandler(store, tmpl)

	form := url.Values{"items": {sixteenEntries()}}
	req := httptest.NewRequest(http.MethodPost, "/wheels/populate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No bbw_session cookie set
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestPopulateHandler_EmptyItems(t *testing.T) {
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
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
