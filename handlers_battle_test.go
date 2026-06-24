package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"battle-bracket-wheels/internal/wheel"
)

// tieSource is a deterministic rand.Source that always returns 0 from Int63.
// This causes wheel.Spin to select the first option and ResolveBattle to
// always tie, making it useful for testing tiebreaker exhaustion.
type tieSource struct{}

func (tieSource) Int63() int64  { return 0 }
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
	//  3. battle-btn-{MatchID} — disabled battle button
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
	if !strings.Contains(body, "battle-btn-qf1") {
		t.Error("response missing battle-btn-qf1 OOB element")
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

	winnerPrefix := "<strong>Winner:</strong> Wheel "
	idx := strings.Index(body, winnerPrefix)
	if idx < 0 {
		t.Fatal("could not find winner in response body")
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
	if !strings.Contains(body, "Bicycle") && !strings.Contains(body, "Skateboard") {
		// At least one of the option texts should be present
		t.Error("slot-sf1-left fragment missing rendered option text (both Bicycle and Skateboard absent)")
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

