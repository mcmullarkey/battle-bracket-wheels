package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"battle-bracket-wheels/internal/wheel"
)

// tieSource is a deterministic rand.Source that always returns 0 from Int63.
// This causes wheel.Spin to select the first option and ResolveBattle to
// always tie, making it useful for testing tiebreaker exhaustion.
type tieSource struct{}

func (tieSource) Int63() int64    { return 0 }
func (tieSource) Seed(seed int64) {}

// battleTestServer creates a test server with deterministic rand and battle handler.
// Returns the server, template, and store (for state inspection in tests).
func battleTestServer(t *testing.T) (*httptest.Server, *template.Template, *Store) {
	t.Helper()
	store := NewStore()
	tmpl := testBattleTemplate(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("/battle/{matchID}", sessionMiddleware(store, battleHandler(store, tmpl, func() rand.Source {
		return rand.NewSource(42)
	})))
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, tmpl, store
}

// testBattleTemplate parses templates for battle tests, including match.html.
func testBattleTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.New("layout").Funcs(template.FuncMap{"add": func(a, b int) int { return a + b }})
	var err error
	tmpl, err = tmpl.Parse(layoutContent)
	if err != nil {
		t.Fatalf("parsing layout template: %v", err)
	}
	if _, err = tmpl.New("wheel").Parse(wheelContent); err != nil {
		t.Fatalf("parsing wheel template: %v", err)
	}
	if _, err = tmpl.New("matchResult").Parse(matchContent); err != nil {
		t.Fatalf("parsing match template: %v", err)
	}
	if _, err = tmpl.New("bracket").Parse(bracketContent); err != nil {
		t.Fatalf("parsing bracket template: %v", err)
	}
	return tmpl
}

// addOptionToWheel is a test helper that adds an option to a wheel.
func addOptionToWheel(t *testing.T, ts *httptest.Server, sessionID, wheelID, text string) {
	t.Helper()
	form := url.Values{"text": {text}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/"+wheelID+"/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating add option request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/%s/option: %v", wheelID, err)
	}
	resp.Body.Close()
}

func TestBattleHandler_Success(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels (QF1 = wheel 0 vs wheel 1)
	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	// POST /battle/qf1
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Verify HX-Trigger header
	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	var triggerData map[string]interface{}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	spinWheel, ok := triggerData["spin-wheel"]
	if !ok {
		t.Fatal("HX-Trigger missing spin-wheel key")
	}

	// spin-wheel should be an array of 2 objects
	switches, ok := spinWheel.([]interface{})
	if !ok {
		t.Fatalf("spin-wheel is not an array, got %T", spinWheel)
	}
	if len(switches) != 2 {
		t.Errorf("spin-wheel array length = %d, want 2", len(switches))
	}

	// Check response body contains HTML
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, "match-") && !strings.Contains(body, "winner") && !strings.Contains(body, "Battle") {
		// Just verify there's some HTML content (fragments)
		if len(body) == 0 {
			t.Error("response body is empty")
		}
	}
}

func TestBattleHandler_EmptyWheel(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Only add option to one wheel (wheel 0 has options, wheel 1 is empty)
	addOptionToWheel(t, ts, sessionID, "0", "A")

	// POST /battle/qf1
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestBattleHandler_InvalidMatchID(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// POST /battle/invalid
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/invalid", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/invalid: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestBattleHandler_AlreadyResolved(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels
	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	// First POST resolves the match
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating first battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("first POST /battle/qf1: %v", err)
	}
	resp.Body.Close()

	// Second POST should get 409
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating second battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("second POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

func TestBattleHandler_HXTriggerBothWheels(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options
	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	// Verify shape: {"spin-wheel": [{"wheelID": "...", ...}, {"wheelID": "...", ...}]}
	var triggerData struct {
		SpinWheel []struct {
			WheelID     string  `json:"wheelID"`
			TargetIndex int     `json:"targetIndex"`
			TargetAngle float64 `json:"targetAngle"`
		} `json:"spin-wheel"`
	}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	if len(triggerData.SpinWheel) != 2 {
		t.Fatalf("spin-wheel array length = %d, want 2", len(triggerData.SpinWheel))
	}

	// Both wheel IDs should be present
	wheelIDs := map[string]bool{}
	for _, sw := range triggerData.SpinWheel {
		wheelIDs[sw.WheelID] = true
	}
	if !wheelIDs["0"] {
		t.Error("HX-Trigger missing wheel ID 0")
	}
	if !wheelIDs["1"] {
		t.Error("HX-Trigger missing wheel ID 1")
	}
}

func TestBattleHandler_BothWheelsEmpty(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Both wheels are empty (no options added)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestBattleHandler_MatchQF2(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// QF2 = wheel 2 vs wheel 3
	addOptionToWheel(t, ts, sessionID, "2", "C")
	addOptionToWheel(t, ts, sessionID, "3", "D")

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf2", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf2: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Verify HX-Trigger has both wheel IDs 2 and 3
	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	var triggerData struct {
		SpinWheel []struct {
			WheelID string `json:"wheelID"`
		} `json:"spin-wheel"`
	}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	wheelIDs := map[string]bool{}
	for _, sw := range triggerData.SpinWheel {
		wheelIDs[sw.WheelID] = true
	}
	if !wheelIDs["2"] {
		t.Error("HX-Trigger missing wheel ID 2")
	}
	if !wheelIDs["3"] {
		t.Error("HX-Trigger missing wheel ID 3")
	}
}

func TestBattleHandler_OOBFragmentCount(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels (QF1 = wheel 0 vs wheel 1)
	addOptionToWheel(t, ts, sessionID, "0", "Bicycle")
	addOptionToWheel(t, ts, sessionID, "1", "Skateboard")

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Should have exactly 3 elements with hx-swap-oob="true":
	//  1. match-{MatchID} — match result fragment
	//  2. slot-{nextRound}-{side} — next-round slot with absorbed winner wheel
	//  3. center-display — advancing text for the next round
	// The disabled button is the non-OOB main swap target (so HTMX 2.x
	// processes the HX-Trigger header for the spin-wheel animation).
	count := strings.Count(body, "hx-swap-oob")
	if count != 3 {
		t.Errorf("response has %d hx-swap-oob elements, want 3", count)
	}

	// Verify each expected OOB ID is present
	if !strings.Contains(body, "match-qf1") {
		t.Error("response missing match-qf1 OOB element")
	}
	if !strings.Contains(body, "slot-sf1-left") {
		t.Error("response missing slot-sf1-left OOB element (should be next-round slot for QF1)")
	}
	if !strings.Contains(body, "center-display") {
		t.Error("response missing center-display OOB element")
	}

	// Verify the disabled button is present as non-OOB content
	if !strings.Contains(body, `disabled class="battle-btn"`) {
		t.Error("response missing disabled button as non-OOB content")
	}
	if !strings.Contains(body, "Battle Complete") {
		t.Error("response missing 'Battle Complete' text on disabled button")
	}
}

func TestBattleHandler_PostBattleStoreState(t *testing.T) {
	ts, _, store := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add distinct options to both wheels
	addOptionToWheel(t, ts, sessionID, "0", "Bicycle")
	addOptionToWheel(t, ts, sessionID, "1", "Skateboard")

	// Execute battle
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Extract winner ID from response body
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	winnerPrefix := `class="winner-text">Wheel `
	idx := strings.Index(body, winnerPrefix)
	if idx < 0 {
		t.Fatal("could not find winner in response body (no winner-text pattern)")
	}
	winnerID := string(body[idx+len(winnerPrefix)])

	// Verify store state: winner wheel absorbed the loser's option
	var wh0, wh1 wheel.Wheel
	err = store.View(sessionID, func(s *Session) error {
		wh0 = s.Wheels[0]
		wh1 = s.Wheels[1]
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}

	if winnerID == "0" {
		if len(wh0.Options) != 2 {
			t.Errorf("winner wheel 0 has %d options, want 2", len(wh0.Options))
		}
		if len(wh1.Options) != 1 {
			t.Errorf("loser wheel 1 has %d options, want 1", len(wh1.Options))
		}
	} else {
		if len(wh1.Options) != 2 {
			t.Errorf("winner wheel 1 has %d options, want 2", len(wh1.Options))
		}
		if len(wh0.Options) != 1 {
			t.Errorf("loser wheel 0 has %d options, want 1", len(wh0.Options))
		}
	}

	// Verify match is marked as resolved in the store
	var resolved bool
	err = store.View(sessionID, func(s *Session) error {
		resolved = s.ResolvedMatches["qf1"]
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}
	if !resolved {
		t.Error("match qf1 not marked as resolved in store")
	}
}

func TestBattleHandler_ConcurrentResolve(t *testing.T) {
	ts, _, store := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels (QF1 = wheel 0 vs wheel 1)
	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	// Fire N goroutines simultaneously at the same matchID
	const goroutines = 10
	statusCodes := make(chan int, goroutines)
	var ready sync.WaitGroup
	start := make(chan struct{})

	for range goroutines {
		ready.Add(1)
		go func() {
			ready.Done()
			<-start
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
			if err != nil {
				return
			}
			req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
			statusCodes <- resp.StatusCode
		}()
	}

	// Wait for all goroutines to be ready, then release them simultaneously
	ready.Wait()
	close(start)

	// Collect all responses
	results := make([]int, 0, goroutines)
	for range goroutines {
		results = append(results, <-statusCodes)
	}

	// Count results: exactly one 200, N-1 409s
	var ok200, conflict409 int
	for _, code := range results {
		switch code {
		case http.StatusOK:
			ok200++
		case http.StatusConflict:
			conflict409++
		default:
			t.Errorf("unexpected status %d", code)
		}
	}

	if ok200 != 1 {
		t.Errorf("got %d 200 responses, want 1", ok200)
	}
	if conflict409 != goroutines-1 {
		t.Errorf("got %d 409 responses, want %d", conflict409, goroutines-1)
	}

	// Verify store has exactly one resolved match (winner absorbed loser's option)
	var wh0, wh1 wheel.Wheel
	err := store.View(sessionID, func(s *Session) error {
		wh0 = s.Wheels[0]
		wh1 = s.Wheels[1]
		if !s.ResolvedMatches["qf1"] {
			t.Error("match qf1 not marked as resolved in store")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}

	// One wheel should have absorbed the other's option (2 options),
	// the other wheel should still have 1 option
	if len(wh0.Options) != 2 && len(wh1.Options) != 2 {
		t.Errorf("neither wheel has 2 options: wh0=%d, wh1=%d", len(wh0.Options), len(wh1.Options))
	}
	if len(wh0.Options) != 1 && len(wh1.Options) != 1 {
		t.Errorf("neither wheel has 1 option: wh0=%d, wh1=%d", len(wh0.Options), len(wh1.Options))
	}
}

// TestBattleHandler_TiebreakerExhaustion verifies that when the RollFunc
// always ties, the handler returns 500 with the correct error message.
func TestBattleHandler_TiebreakerExhaustion(t *testing.T) {
	store := NewStore()
	tmpl := testBattleTemplate(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("/battle/{matchID}", sessionMiddleware(store, battleHandler(store, tmpl, func() rand.Source {
		return tieSource{}
	})))
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels
	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	// POST /battle/qf1 — tieSource causes constant ties
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// battleRequest is a test helper that performs a single battle POST and returns the response.
func battleRequest(t *testing.T, ts *httptest.Server, sessionID, matchID string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/"+matchID, nil)
	if err != nil {
		t.Fatalf("creating battle request for %s: %v", matchID, err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/%s: %v", matchID, err)
	}
	return resp
}

// readResponseBody reads the full response body into a string.
func readResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	resp.Body.Close()
	return string(buf[:n])
}

func TestPostBattle_OOBTargets(t *testing.T) {
	// Sneaky-pass guard (i): POST /battle/qf1 → response contains id="slot-sf1-left"
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to wheels 0 and 1 (QF1)
	addOptionToWheel(t, ts, sessionID, "0", "Bicycle")
	addOptionToWheel(t, ts, sessionID, "1", "Skateboard")

	resp := battleRequest(t, ts, sessionID, "qf1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// Must contain slot-sf1-left OOB target
	if !strings.Contains(body, "slot-sf1-left") {
		t.Error("response missing slot-sf1-left OOB element")
	}
	if !strings.Contains(body, "hx-swap-oob") {
		t.Error("response missing hx-swap-oob attributes")
	}
}

func TestPostBattle_QF3Target(t *testing.T) {
	// Sneaky-pass guard (i): POST /battle/qf3 → response contains id="slot-sf2-left" (not slot-sf1-left)
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to wheels 4 and 5 (QF3)
	addOptionToWheel(t, ts, sessionID, "4", "E")
	addOptionToWheel(t, ts, sessionID, "5", "F")

	resp := battleRequest(t, ts, sessionID, "qf3")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// Must contain slot-sf2-left (not slot-sf1-left)
	if !strings.Contains(body, "slot-sf2-left") {
		t.Error("response missing slot-sf2-left OOB element (QF3 should target sf2-left)")
	}
	if strings.Contains(body, "slot-sf1-left") {
		t.Error("response contains slot-sf1-left, but QF3 should target slot-sf2-left")
	}
}

func TestPostBattle_NoReTyping(t *testing.T) {
	// Sneaky-pass guard (h): next-round slot fragment contains rendered option text, NOT empty <input> elements
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	addOptionToWheel(t, ts, sessionID, "0", "Bicycle")
	addOptionToWheel(t, ts, sessionID, "1", "Skateboard")

	resp := battleRequest(t, ts, sessionID, "qf1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// The next-round slot (slot-sf1-left) should contain the wheel template with option text rendered
	// Check that the absorbed wheel's options appear as rendered text in the slot fragment
	if !strings.Contains(body, "Bicycle") || !strings.Contains(body, "Skateboard") {
		// Both option texts must be present (absorbed wheel's options + winner's original options)
		t.Error("slot-sf1-left fragment missing absorbed option text (need BOTH Bicycle and Skateboard)")
	}

	// Check that option text is rendered as visible text, not inside an empty <input>
	// The wheel template renders option text in <li> elements, not in empty inputs
	if strings.Contains(body, `<input type="text" name="text" placeholder="Option text" value="">`) {
		t.Error("slot fragment contains empty input instead of pre-filled rendered options")
	}
}

func TestPostBattle_FinalMovie(t *testing.T) {
	// Full bracket progression to reach the final match
	ts, _, store := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to all 8 wheels
	for i := 0; i < 8; i++ {
		addOptionToWheel(t, ts, sessionID, fmt.Sprintf("%d", i), fmt.Sprintf("Opt%d", i))
	}

	// Run QF matches (all need to succeed)
	matches := []string{"qf1", "qf2", "qf3", "qf4", "sf1", "sf2"}
	for _, mid := range matches {
		resp := battleRequest(t, ts, sessionID, mid)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s returned status %d, want 200", mid, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Now run the Final match
	resp := battleRequest(t, ts, sessionID, "final")
	defer resp.Body.Close()

	body := readResponseBody(t, resp)

	// Response must contain movie-result with "You're watching:"
	if !strings.Contains(body, "movie-result") {
		t.Error("response missing movie-result element")
	}
	if !strings.Contains(body, "You're watching:") {
		t.Error("response missing 'You're watching:' in movie result")
	}

	// Check HX-Trigger has bracketComplete
	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	var triggerData map[string]interface{}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	bracketComplete, ok := triggerData["bracketComplete"]
	if !ok {
		t.Error("HX-Trigger missing bracketComplete for final match")
	} else {
		// Should be true (or truthy)
		if bc, ok := bracketComplete.(bool); !ok || !bc {
			t.Errorf("bracketComplete = %v, want true", bracketComplete)
		}
	}

	// Verify store has the winner populated
	err := store.View(sessionID, func(s *Session) error {
		if s.Bracket == nil {
			t.Fatal("Bracket is nil")
		}
		if s.Bracket.Winner == nil {
			t.Fatal("Winner is nil after final")
		}
		if s.Bracket.Winner.LandedOption.Text == "" {
			t.Error("Winner.LandedOption.Text is empty")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("store.View: %v", err)
	}
}

func TestPostBattle_InvalidMatchID(t *testing.T) {
	// POST /battle/invalid → 404
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	resp := battleRequest(t, ts, sessionID, "invalid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPostBattle_GETMethod(t *testing.T) {
	// GET /battle/qf1 → 405
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating GET request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPostBattle_NonExistentWheel(t *testing.T) {
	// SF1 without QF1+QF2 should fail (dependency gate)
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to all 8 wheels
	for i := 0; i < 8; i++ {
		addOptionToWheel(t, ts, sessionID, fmt.Sprintf("%d", i), fmt.Sprintf("Opt%d", i))
	}

	// Try SF1 directly without running QF1+QF2 first
	resp := battleRequest(t, ts, sessionID, "sf1")
	defer resp.Body.Close()

	// Should fail with conflict (dependency not met)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409 (conflict - dependency not met)", resp.StatusCode)
	}
}

func TestPostBattle_BracketIdempotency(t *testing.T) {
	// Verify that resolving a QF match twice fails the second time
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	// First battle should succeed
	resp1 := battleRequest(t, ts, sessionID, "qf1")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first battle status = %d, want 200", resp1.StatusCode)
	}
	resp1.Body.Close()

	// Second battle for same match should fail
	resp2 := battleRequest(t, ts, sessionID, "qf1")
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("second battle status = %d, want 409", resp2.StatusCode)
	}
}

func TestPostBattle_FinalOOBCount(t *testing.T) {
	// Run full bracket to Final, then assert the Final response has exactly 3 OOB elements
	// (matchResult: match-final + battle-btn-final; movieResult: movie-result).
	// NOT 4 — there should be no nextRoundSlot fragment for the Final.
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to all 8 wheels
	for i := 0; i < 8; i++ {
		addOptionToWheel(t, ts, sessionID, fmt.Sprintf("%d", i), fmt.Sprintf("Opt%d", i))
	}

	// Run QF + SF matches to reach the Final
	matches := []string{"qf1", "qf2", "qf3", "qf4", "sf1", "sf2"}
	for _, mid := range matches {
		resp := battleRequest(t, ts, sessionID, mid)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s returned status %d, want 200", mid, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Now run the Final match
	resp := battleRequest(t, ts, sessionID, "final")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("final returned status %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// Should have exactly 3 hx-swap-oob elements:
	//  1. match-final — match result fragment
	//  2. movie-result — movie result (NOT a duplicate nextRoundSlot with id=movie-result)
	//  3. center-display — champion text with winning movie
	// The disabled button is the non-OOB main swap target.
	oobCount := strings.Count(body, "hx-swap-oob")
	if oobCount != 3 {
		t.Errorf("Final response has %d hx-swap-oob elements, want 3", oobCount)
	}

	// Verify the expected OOB IDs are present
	if !strings.Contains(body, "match-final") {
		t.Error("response missing match-final OOB element")
	}
	if !strings.Contains(body, "movie-result") {
		t.Error("response missing movie-result OOB element")
	}
	if !strings.Contains(body, "center-display") {
		t.Error("response missing center-display OOB element")
	}

	// Verify the disabled button is present as non-OOB content
	if !strings.Contains(body, `disabled class="battle-btn"`) {
		t.Error("response missing disabled button as non-OOB content")
	}
	if !strings.Contains(body, "Battle Complete") {
		t.Error("response missing 'Battle Complete' text on disabled button")
	}

	// Verify no duplicate id="movie-result" (which would happen if SlotMapping
	// still returned "movie-result" AND the movieResult template also used it)
	if strings.Count(body, `id="movie-result"`) != 1 {
		t.Error("response has duplicate id=\"movie-result\" — SlotMapping for Final should return empty string")
	}

	// Verify Final center-display has NO list markup (single champion text only)
	// Note: use <li> or <li with a tag-terminating char to avoid false-positive
	// matching SVG <line> elements in the match-result template.
	if strings.Contains(body, `<ul`) {
		t.Error("Final center-display contains <ul> — expected no list markup for Final")
	}
	if regexp.MustCompile(`<li[\s>]`).MatchString(body) {
		t.Error("Final center-display contains <li> — expected no list markup for Final")
	}
}

func TestPostBattle_QF2Target(t *testing.T) {
	// POST /battle/qf2 → response contains id="slot-sf1-right" (not slot-sf1-left)
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// QF2 uses wheels 2 and 3
	addOptionToWheel(t, ts, sessionID, "2", "C")
	addOptionToWheel(t, ts, sessionID, "3", "D")

	resp := battleRequest(t, ts, sessionID, "qf2")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// Must contain slot-sf1-right OOB target (QF2 winner goes to SFRight[0])
	if !strings.Contains(body, "slot-sf1-right") {
		t.Error("response missing slot-sf1-right OOB element (QF2 should target sf1-right)")
	}
	if strings.Contains(body, "slot-sf1-left") {
		t.Error("response contains slot-sf1-left, but QF2 should target slot-sf1-right, not sf1-left")
	}
}

func TestPostBattle_SF2Target(t *testing.T) {
	// Run QF3+QF4 first, then POST /battle/sf2 → response contains id="slot-final-right"
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to wheels 4,5,6,7 (QF3+QF4)
	addOptionToWheel(t, ts, sessionID, "4", "E")
	addOptionToWheel(t, ts, sessionID, "5", "F")
	addOptionToWheel(t, ts, sessionID, "6", "G")
	addOptionToWheel(t, ts, sessionID, "7", "H")

	// Run QF3 and QF4 first (SF2 depends on them)
	respQF3 := battleRequest(t, ts, sessionID, "qf3")
	if respQF3.StatusCode != http.StatusOK {
		t.Fatalf("qf3 returned status %d, want 200", respQF3.StatusCode)
	}
	respQF3.Body.Close()

	respQF4 := battleRequest(t, ts, sessionID, "qf4")
	if respQF4.StatusCode != http.StatusOK {
		t.Fatalf("qf4 returned status %d, want 200", respQF4.StatusCode)
	}
	respQF4.Body.Close()

	// Now run SF2
	resp := battleRequest(t, ts, sessionID, "sf2")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sf2 returned status %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// Must contain slot-final-right OOB target (SF2 winner goes to FinalRight)
	if !strings.Contains(body, "slot-final-right") {
		t.Error("response missing slot-final-right OOB element (SF2 should target final-right)")
	}
	if strings.Contains(body, "slot-final-left") {
		t.Error("response contains slot-final-left, but SF2 should target slot-final-right, not final-left")
	}
}

// extractOOBFragment extracts the inner content of an hx-swap-oob="true" element by its id.
// Returns empty string if not found. Handles simple elements with no nested divs.
func extractOOBFragment(body, id string) string {
	startTag := fmt.Sprintf(`id="%s" hx-swap-oob="true"`, id)
	idx := strings.Index(body, startTag)
	if idx < 0 {
		return ""
	}
	// Find the closing > of the opening tag
	gtIdx := strings.Index(body[idx:], ">")
	if gtIdx < 0 {
		return ""
	}
	contentStart := idx + gtIdx + 1

	// Find closing </div>
	closeTag := "</div>"
	closeIdx := strings.Index(body[contentStart:], closeTag)
	if closeIdx < 0 {
		return ""
	}
	return body[contentStart : contentStart+closeIdx]
}

func TestPostBattle_CenterDisplayAdvancingOptions(t *testing.T) {
	ts, _, store := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	t.Run("QF1 list structure with all advancing options", func(t *testing.T) {
		// Wheel 0 has 3 options, wheel 1 has 1 option
		// After absorption (regardless of winner), the absorbed wheel contains
		// the winner's originals + the loser's landed option.
		addOptionToWheel(t, ts, sessionID, "0", "A")
		addOptionToWheel(t, ts, sessionID, "0", "B")
		addOptionToWheel(t, ts, sessionID, "0", "C")
		addOptionToWheel(t, ts, sessionID, "1", "D")

		resp := battleRequest(t, ts, sessionID, "qf1")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body := readResponseBody(t, resp)
		centerDisplay := extractOOBFragment(body, "center-display")
		if centerDisplay == "" {
			t.Fatal("center-display OOB fragment not found")
		}

		// Read the absorbed wheel from bracket state after the battle
		var absorbedWheel *wheel.Wheel
		err := store.View(sessionID, func(s *Session) error {
			absorbedWheel = s.Bracket.SFLeft[0]
			return nil
		})
		if err != nil {
			t.Fatalf("store.View: %v", err)
		}
		if absorbedWheel == nil {
			t.Fatal("absorbed wheel is nil — SFLeft[0] not populated")
		}

		// Every option text from the absorbed wheel must appear in center-display
		for _, opt := range absorbedWheel.Options {
			if !strings.Contains(centerDisplay, opt.Text) {
				t.Errorf("center-display missing absorbed option %q (absorbed wheel has %d options: %v)",
					opt.Text, len(absorbedWheel.Options), optionTexts(absorbedWheel.Options))
			}
		}

		// Must render as list structure, not single <p>
		if !strings.Contains(centerDisplay, "<li") {
			t.Error("center-display missing <li> list markup — expected list structure for QF/SF")
		}

		// Must NOT be the old single-paragraph format
		if strings.Contains(centerDisplay, `<p class="advancing-option">`) {
			t.Error("center-display contains old single-paragraph format — expected list for QF/SF")
		}
	})

	t.Run("QF1 center-display round label", func(t *testing.T) {
		defer func() {
			// Create fresh session for this subtest
		}()

		ts2, _, _ := battleTestServer(t)
		sessionID2 := getSessionCookie(t, ts2)

		addOptionToWheel(t, ts2, sessionID2, "0", "A")
		addOptionToWheel(t, ts2, sessionID2, "1", "B")

		resp := battleRequest(t, ts2, sessionID2, "qf1")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body := readResponseBody(t, resp)
		centerDisplay := extractOOBFragment(body, "center-display")
		if centerDisplay == "" {
			t.Fatal("center-display OOB fragment not found")
		}

		if !strings.Contains(centerDisplay, "Advancing to Semifinal") {
			t.Error("center-display missing round label 'Advancing to Semifinal' for QF1")
		}
	})

	t.Run("QF1 dedupe shared option text", func(t *testing.T) {
		ts2, _, _ := battleTestServer(t)
		sessionID2 := getSessionCookie(t, ts2)

		// Both wheels share "Movie" — after absorption it should appear exactly once
		addOptionToWheel(t, ts2, sessionID2, "0", "Movie")
		addOptionToWheel(t, ts2, sessionID2, "0", "UniqueA")
		addOptionToWheel(t, ts2, sessionID2, "1", "Movie")

		resp := battleRequest(t, ts2, sessionID2, "qf1")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body := readResponseBody(t, resp)
		centerDisplay := extractOOBFragment(body, "center-display")
		if centerDisplay == "" {
			t.Fatal("center-display OOB fragment not found")
		}

		// "Movie" should appear exactly once in center-display
		if strings.Count(centerDisplay, "Movie") != 1 {
			t.Errorf("center-display contains 'Movie' %d times, want exactly 1 (deduped)",
				strings.Count(centerDisplay, "Movie"))
		}
	})

	t.Run("QF1 edge case 1+1 distinct", func(t *testing.T) {
		ts2, _, _ := battleTestServer(t)
		sessionID2 := getSessionCookie(t, ts2)

		// Both wheels have 1 distinct option each
		addOptionToWheel(t, ts2, sessionID2, "0", "Alpha")
		addOptionToWheel(t, ts2, sessionID2, "1", "Beta")

		resp := battleRequest(t, ts2, sessionID2, "qf1")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body := readResponseBody(t, resp)
		centerDisplay := extractOOBFragment(body, "center-display")
		if centerDisplay == "" {
			t.Fatal("center-display OOB fragment not found")
		}

		// Both options must appear
		if !strings.Contains(centerDisplay, "Alpha") {
			t.Error("center-display missing 'Alpha'")
		}
		if !strings.Contains(centerDisplay, "Beta") {
			t.Error("center-display missing 'Beta'")
		}

		// Must have list structure
		if !strings.Contains(centerDisplay, "<li") {
			t.Error("center-display missing <li> list markup")
		}
	})

	t.Run("Final no list markup", func(t *testing.T) {
		ts2, _, _ := battleTestServer(t)
		sessionID2 := getSessionCookie(t, ts2)

		// Run full bracket to Final
		for i := 0; i < 8; i++ {
			addOptionToWheel(t, ts2, sessionID2, fmt.Sprintf("%d", i), fmt.Sprintf("Opt%d", i))
		}

		matches := []string{"qf1", "qf2", "qf3", "qf4", "sf1", "sf2"}
		for _, mid := range matches {
			resp := battleRequest(t, ts2, sessionID2, mid)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s returned status %d, want 200", mid, resp.StatusCode)
			}
			resp.Body.Close()
		}

		// Final battle
		resp := battleRequest(t, ts2, sessionID2, "final")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("final returned status %d, want 200", resp.StatusCode)
		}

		body := readResponseBody(t, resp)

		// Extract center-display
		centerDisplay := extractOOBFragment(body, "center-display")
		if centerDisplay == "" {
			t.Fatal("center-display OOB fragment not found in Final response")
		}

		// Final MUST NOT have list markup
		if strings.Contains(centerDisplay, "<ul") {
			t.Error("Final center-display contains <ul> — expected no list markup")
		}
		if strings.Contains(centerDisplay, "<li") {
			t.Error("Final center-display contains <li> — expected no list markup")
		}

		// Final must still show the single champion movie
		if !strings.Contains(body, "movie-result") {
			t.Error("Final response missing movie-result")
		}
		if !strings.Contains(body, "You're watching:") {
			t.Error("Final response missing 'You're watching:' in movie result")
		}
	})
}

// optionTexts extracts a []string of option texts for test assertions.
func optionTexts(opts []wheel.Option) []string {
	texts := make([]string, len(opts))
	for i, o := range opts {
		texts[i] = o.Text
	}
	return texts
}

func TestMatchResult_SpaceThemeVisual(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add options to both wheels (QF1 = wheel 0 vs wheel 1)
	addOptionToWheel(t, ts, sessionID, "0", "Bicycle")
	addOptionToWheel(t, ts, sessionID, "1", "Skateboard")

	resp := battleRequest(t, ts, sessionID, "qf1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readResponseBody(t, resp)

	// 1. Exactly 3 hx-swap-oob="true" elements (existing contract preserved)
	oobCount := strings.Count(body, "hx-swap-oob")
	if oobCount != 3 {
		t.Errorf("response has %d hx-swap-oob elements, want 3", oobCount)
	}

	// 2. #match-qf1 OOB div carries class="pending-reveal" (reveal contract preserved)
	if !strings.Contains(body, `id="match-qf1" hx-swap-oob="true" class="pending-reveal"`) {
		t.Error("match-qf1 OOB div missing pending-reveal class or correct attribute order")
	}

	// 3. #match-qf1 contains distinct structural CSS classes for winner vs loser
	if !strings.Contains(body, "match-winner") {
		t.Error("response missing match-winner class for winner visual")
	}
	if !strings.Contains(body, "match-loser") {
		t.Error("response missing match-loser class for loser visual")
	}

	// 4. #match-qf1 contains an inline <svg> element (space-themed visual marker)
	if !strings.Contains(body, "<svg") {
		t.Error("response missing inline SVG element")
	}

	// 5. Response body no longer contains the old <strong>Winner:</strong> label markup
	if strings.Contains(body, "<strong>Winner:</strong>") {
		t.Error("response still contains old <strong>Winner:</strong> label markup as primary indicator")
	}

	// 6. Response body still contains the text "Winner" and "Loser" (data integrity)
	if !strings.Contains(body, "Winner") {
		t.Error("response missing text 'Winner' (data integrity)")
	}
	if !strings.Contains(body, "Loser") {
		t.Error("response missing text 'Loser' (data integrity)")
	}

	// 7. space.css contains animation referencing meteor in a rule scoped to .match-result or descendant
	cssData, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(cssData)

	// Parse CSS rules by splitting on closing braces to find selector-content pairs
	meteorScopedToMatchResult := false
	rules := strings.Split(css, "}")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		// Split on first { to get selector and content
		braceIdx := strings.Index(rule, "{")
		if braceIdx < 0 {
			continue
		}
		selector := strings.TrimSpace(rule[:braceIdx])
		content := rule[braceIdx+1:]
		if strings.Contains(selector, ".match-result") &&
			strings.Contains(content, "animation") &&
			strings.Contains(content, "meteor") {
			meteorScopedToMatchResult = true
			break
		}
	}
	if !meteorScopedToMatchResult {
		t.Error("space.css missing animation:meteor in a rule scoped to .match-result or descendant")
	}

	// 8. .match-result CSS rule has at least one of: background, border, box-shadow, backdrop-filter, or --neon-*
	matchResultRule := extractCSSRule(css, ".match-result")
	if matchResultRule == "" {
		t.Fatal(".match-result CSS rule not found in space.css")
	}
	hasVisualToken := strings.Contains(matchResultRule, "background") ||
		strings.Contains(matchResultRule, "border") ||
		strings.Contains(matchResultRule, "box-shadow") ||
		strings.Contains(matchResultRule, "backdrop-filter") ||
		strings.Contains(matchResultRule, "--neon-")
	if !hasVisualToken {
		t.Error(".match-result CSS rule missing visual tokens (background, border, box-shadow, backdrop-filter, or --neon-*)")
	}

	// 9. .match-result uses flexbox centering (AC: centered horizontally + vertically)
	if !strings.Contains(matchResultRule, "display: flex") {
		t.Error(".match-result missing display: flex for centering")
	}
	if !strings.Contains(matchResultRule, "align-items: center") {
		t.Error(".match-result missing align-items: center (horizontal centering)")
	}
	if !strings.Contains(matchResultRule, "justify-content: center") {
		t.Error(".match-result missing justify-content: center (vertical centering)")
	}
}

// TestBattlePointer_SpaceTheme verifies the decorative .battle-pointer element:
//   - Present in POST /battle/qf1 response inside matchResult OOB fragment
//   - Uses inline SVG (no url() images)
//   - space.css has .battle-pointer rule with >=2 space-theme custom properties
//   - NOT hidden by @media (max-width: 640px)
func TestBattlePointer_SpaceTheme(t *testing.T) {
	// ---- Part 1: Battle response contains .battle-pointer with inline SVG ----
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// The .battle-pointer must appear inside the match-{matchID} OOB fragment
	if !strings.Contains(body, `class="battle-pointer"`) {
		t.Error("battle response missing .battle-pointer element")
	}

	// Must contain an inline SVG inside the pointer
	pointerIdx := strings.Index(body, "battle-pointer")
	if pointerIdx >= 0 {
		fragmentAfter := body[pointerIdx:]
		if !strings.Contains(fragmentAfter, "<svg") {
			t.Error(".battle-pointer missing inline SVG")
		}
	}

	// Must NOT contain url() referencing images in the battle response
	if strings.Contains(body, `url(`) {
		// Check for image url() references
		imageExts := []string{".gif", ".png", ".jpg", ".jpeg", ".webp", ".svg"}
		for _, ext := range imageExts {
			if strings.Contains(body, "url("+ext) {
				t.Errorf("battle response contains url() referencing %s", ext)
			}
		}
	}

	// ---- Part 2: space.css has .battle-pointer rule with >=2 theme tokens ----
	data, err := fs.ReadFile(staticFS, "css/space.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/space.css: %v", err)
	}
	css := string(data)

	pointerRule := extractCSSRule(css, ".battle-pointer")
	if pointerRule == "" {
		t.Fatal("space.css missing .battle-pointer CSS rule")
	}

	// Count space-theme custom properties referenced
	themeTokens := []string{"--neon-", "--cosmic-", "--star-", "--nebula-"}
	tokenCount := 0
	for _, token := range themeTokens {
		if strings.Contains(pointerRule, token) {
			tokenCount++
		}
	}
	if tokenCount < 2 {
		t.Errorf(".battle-pointer rule references %d space-theme tokens, want >= 2", tokenCount)
	}

	// Must NOT contain url() referencing images
	for _, ext := range []string{".gif", ".png", ".jpg", ".jpeg", ".webp", ".svg"} {
		if strings.Contains(pointerRule, "url("+ext) {
			t.Errorf(".battle-pointer rule contains url() referencing %s", ext)
		}
	}

	// Must have at least one property for visual substance beyond display/visibility
	hasVisualSubstance := strings.Contains(pointerRule, "color") ||
		strings.Contains(pointerRule, "background") ||
		strings.Contains(pointerRule, "border") ||
		strings.Contains(pointerRule, "box-shadow") ||
		strings.Contains(pointerRule, "width") ||
		strings.Contains(pointerRule, "height") ||
		strings.Contains(pointerRule, "font-size")
	if !hasVisualSubstance {
		t.Error(".battle-pointer rule lacks visual substance (no color/background/border/size properties)")
	}

	// ---- Part 3: NOT hidden by @media (max-width: 640px) ----
	// Parse mobile media query block
	mobileMediaIdx := strings.Index(css, "@media (max-width: 640px)")
	if mobileMediaIdx < 0 {
		t.Fatal("space.css missing @media (max-width: 640px)")
	}

	// Find the mobile query's closing brace
	mobileDepth := 1
	mobileEnd := mobileMediaIdx + len("@media (max-width: 640px)")
	for mobileEnd < len(css) && mobileDepth > 0 {
		if css[mobileEnd] == '{' {
			mobileDepth++
		} else if css[mobileEnd] == '}' {
			mobileDepth--
		}
		mobileEnd++
	}
	mobileBlock := css[mobileMediaIdx:mobileEnd]

	// Check that .battle-pointer is NOT mentioned inside the mobile block
	if strings.Contains(mobileBlock, ".battle-pointer") {
		t.Error(".battle-pointer referenced inside @media (max-width: 640px) — would be hidden on mobile")
	}
}

// TestBattleHandler_HXTrigger_WinnerInTrigger verifies that:
//   - HX-Trigger spin-wheel array has exactly one entry with "winner":true
//   - The winner:true entry's wheelID matches the battle result WinnerID
//   - The non-winner entry has winner:false (or absent, defaulting to false)
func TestBattleHandler_HXTrigger_WinnerInTrigger(t *testing.T) {
	ts, _, _ := battleTestServer(t)
	sessionID := getSessionCookie(t, ts)

	addOptionToWheel(t, ts, sessionID, "0", "A")
	addOptionToWheel(t, ts, sessionID, "1", "B")

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/battle/qf1", nil)
	if err != nil {
		t.Fatalf("creating battle request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /battle/qf1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	// Parse HX-Trigger with winner field
	var triggerData struct {
		SpinWheel []struct {
			WheelID     string  `json:"wheelID"`
			SlotID      string  `json:"slotID"`
			TargetIndex int     `json:"targetIndex"`
			TargetAngle float64 `json:"targetAngle"`
			Winner      bool    `json:"winner"`
		} `json:"spin-wheel"`
	}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	if len(triggerData.SpinWheel) != 2 {
		t.Fatalf("spin-wheel array length = %d, want 2", len(triggerData.SpinWheel))
	}

	// Count winner:true entries — must be exactly 1
	winnerCount := 0
	var winnerWheelID string
	for _, sw := range triggerData.SpinWheel {
		if sw.Winner {
			winnerCount++
			winnerWheelID = sw.WheelID
		}
	}
	if winnerCount != 1 {
		t.Errorf("got %d winner:true entries, want exactly 1", winnerCount)
	}

	// Extract actual WinnerID from the response body (matchResult fragment)
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// The template renders: <span class="winner-text">Wheel {ID} (roll: {N})</span>
	winnerMatch := regexp.MustCompile(`winner-text">Wheel\s+(\S+)`).FindStringSubmatch(body)
	if len(winnerMatch) < 2 {
		t.Fatal("could not extract WinnerID from response body")
	}
	expectedWinnerID := winnerMatch[1]

	if winnerWheelID != expectedWinnerID {
		t.Errorf("winner:true entry has wheelID=%q, want %q (from matchResult)", winnerWheelID, expectedWinnerID)
	}
}
