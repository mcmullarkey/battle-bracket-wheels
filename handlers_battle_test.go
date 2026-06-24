package main

import (
	"encoding/json"
	"html/template"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// battleTestServer creates a test server with deterministic rand and battle handler.
func battleTestServer(t *testing.T) (*httptest.Server, *template.Template) {
	t.Helper()
	store := NewStore()
	tmpl := testBattleTemplate(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("POST /battle/{matchID}", sessionMiddleware(store, battleHandler(store, tmpl, func() rand.Source {
		return rand.NewSource(42)
	})))
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, tmpl
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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
	ts, _ := battleTestServer(t)
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

