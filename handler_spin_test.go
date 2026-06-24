package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"battle-bracket-wheels/internal/wheel"
)

// spinTestServer creates a test server with a deterministic rand source for spin.
func spinTestServer(t *testing.T) (*httptest.Server, *template.Template) {
	t.Helper()
	store := NewStore()
	tmpl := testTemplate(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.Handle("POST /wheel/{id}/option", sessionMiddleware(store, addOptionHandler(store, tmpl)))
	mux.Handle("POST /wheel/{id}/spin", sessionMiddleware(store, spinHandler(store, tmpl, func() rand.Source {
		return rand.NewSource(42)
	})))
	// Home for session creation
	mux.Handle("/", sessionMiddleware(store, homeHandler(store, tmpl)))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, tmpl
}

func TestSpinHandler(t *testing.T) {
	ts, _ := spinTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add an option first so the wheel has selectable options
	form := fmt.Sprintf("text=%s", "A")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form))
	if err != nil {
		t.Fatalf("creating add option request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	// Now spin
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/spin", nil)
	if err != nil {
		t.Fatalf("creating spin request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/spin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Should have HTML body (wheel fragment)
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, "<svg") {
		t.Error("response missing <svg>")
	}

	// Should have HX-Trigger header
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

	sw := spinWheel.(map[string]interface{})
	if sw["wheelID"] != "0" {
		t.Errorf("wheelID = %v, want 0", sw["wheelID"])
	}
	if _, ok := sw["targetIndex"]; !ok {
		t.Error("spin-wheel missing targetIndex")
	}
	if _, ok := sw["targetAngle"]; !ok {
		t.Error("spin-wheel missing targetAngle")
	}
}

func TestSpinHandlerHXTriggerShape(t *testing.T) {
	ts, _ := spinTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add an option
	form := fmt.Sprintf("text=%s&weight=%s", "Opt", "1")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form))
	if err != nil {
		t.Fatalf("creating add option request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	// Spin with deterministic source to predict result
	wh := wheel.Wheel{ID: "0", Options: []wheel.Option{{Text: "Opt", Weight: ptr(1.0)}}}
	expectedResult, err := wheel.Spin(wh, rand.NewSource(42))
	if err != nil {
		t.Fatalf("Spin: %v", err)
	}

	req, err = http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/spin", nil)
	if err != nil {
		t.Fatalf("creating spin request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/spin: %v", err)
	}
	defer resp.Body.Close()

	hxTrigger := resp.Header.Get("HX-Trigger")
	if hxTrigger == "" {
		t.Fatal("missing HX-Trigger header")
	}

	// Unmarshal and verify the shape matches SpinResult
	var triggerData struct {
		SpinWheel struct {
			WheelID     string  `json:"wheelID"`
			TargetIndex int     `json:"targetIndex"`
			TargetAngle float64 `json:"targetAngle"`
		} `json:"spin-wheel"`
	}
	if err := json.Unmarshal([]byte(hxTrigger), &triggerData); err != nil {
		t.Fatalf("unmarshal HX-Trigger: %v", err)
	}

	if triggerData.SpinWheel.WheelID != "0" {
		t.Errorf("wheelID = %q, want %q", triggerData.SpinWheel.WheelID, "0")
	}
	if triggerData.SpinWheel.TargetIndex != expectedResult.Index {
		t.Errorf("targetIndex = %d, want %d", triggerData.SpinWheel.TargetIndex, expectedResult.Index)
	}
	if triggerData.SpinWheel.TargetAngle != expectedResult.TargetAngle {
		t.Errorf("targetAngle = %f, want %f", triggerData.SpinWheel.TargetAngle, expectedResult.TargetAngle)
	}
}

func TestSpinHandlerEmptyWheel(t *testing.T) {
	ts, _ := spinTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Spin on an empty wheel (no options added)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/spin", nil)
	if err != nil {
		t.Fatalf("creating spin request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/spin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func TestSpinHandlerInvalidWheelID(t *testing.T) {
	ts, _ := spinTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// POST to non-existent wheel ID (valid range is 0-7)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/999/spin", nil)
	if err != nil {
		t.Fatalf("creating spin request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/999/spin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSpinHandlerAllZeroWeight(t *testing.T) {
	ts, _ := spinTestServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add a zero-weight option
	form := fmt.Sprintf("text=%s&weight=%s", "Zero", "0")
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form))
	if err != nil {
		t.Fatalf("creating add option request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	// Spin — all-zero weight should return 4xx
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/spin", nil)
	if err != nil {
		t.Fatalf("creating spin request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/spin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func ptr(f float64) *float64 { return &f }
