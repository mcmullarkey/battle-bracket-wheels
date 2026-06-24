package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"battle-bracket-wheels/internal/wheel"
)

func testWheelServer(t *testing.T) (*httptest.Server, *template.Template) {
	t.Helper()
	store := NewStore()
	tmpl := testTemplate(t)
	mux := setupRouter(store, tmpl)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, tmpl
}

func getSessionCookie(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "bbw_session" {
			return c.Value
		}
	}
	t.Fatal("no bbw_session cookie in response")
	return ""
}

func TestWheelOption_Add(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"text": {"D"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if !strings.Contains(body, "<svg") {
		t.Error("response missing <svg>")
	}
	if !strings.Contains(body, `data-option="D"`) {
		t.Error("response missing data-option=\"D\"")
	}
	// Single option renders 2 arc paths (two 180° halves)
	if strings.Count(body, `data-option="`) != 2 {
		t.Error("expected 2 data-option attributes for single option (two arcs)")
	}
	// Count slices — at least one path with wheel-slice class
	if !strings.Contains(body, `class="wheel-slice"`) {
		t.Error("response missing wheel-slice class")
	}
}

func TestWheelOption_EmptyText(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"text": {""}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func TestWheelOption_EmptyTextAfterTrim(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"text": {"   "}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func TestWheelOption_NegativeWeight(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"text": {"X"}, "weight": {"-1"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func TestWheelOption_Remove(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add two options first
	for _, text := range []string{"A", "B"} {
		form := url.Values{"text": {text}}
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /wheel/0/option: %v", err)
		}
		resp.Body.Close()
	}

	// Now delete option at index 0
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/wheel/0/option/0", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /wheel/0/option/0: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// Response should contain SVG with one slice (the remaining option "B")
	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, "<svg") {
		t.Error("response missing <svg>")
	}
	// Single option renders 2 arc paths (two 180° halves)
	if strings.Count(body, `data-option="`) != 2 {
		t.Error("expected 2 data-option attributes for single option (two arcs)")
	}
}

func TestWheelOption_RemoveOutOfRange(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add one option
	form := url.Values{"text": {"A"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	// Delete out-of-range index
	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/wheel/0/option/999", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /wheel/0/option/999: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}

func TestWheelOption_RemoveLast(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add one option
	form := url.Values{"text": {"A"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	// Delete the last option
	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/wheel/0/option/0", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /wheel/0/option/0: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (0 options valid)", resp.StatusCode)
	}

	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if strings.Count(body, `data-option="`) != 0 {
		t.Error("expected 0 data-option attributes after removing last option")
	}
}

func TestIndex_EightSlots(t *testing.T) {
	ts, _ := testWheelServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	buf := make([]byte, 65536)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	for i := 0; i < 8; i++ {
		wheelID := fmt.Sprintf("wheel-%d", i)
		if !strings.Contains(body, `id="`+wheelID+`"`) {
			t.Errorf("response missing id=%q", wheelID)
		}
	}
}

func TestConcurrentWheelMutation(t *testing.T) {
	store := NewStore()
	tmpl := testTemplate(t)
	mux := setupRouter(store, tmpl)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Create a session via the store directly
	session, err := store.Create()
	if err != nil {
		t.Fatalf("store.Create(): %v", err)
	}

	// Fire N goroutines each adding an option to wheel 0
	n := 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			text := fmt.Sprintf("Option-%d", idx)
			form := url.Values{"text": {text}}
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
			if err != nil {
				t.Errorf("creating request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "bbw_session", Value: session.ID})
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("POST /wheel/0/option: %v", err)
				return
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()

	// Verify all options are present in the session (no lost updates)
	got, ok := store.Get(session.ID)
	if !ok {
		t.Fatal("session not found after concurrent mutations")
	}

	if optCount := len(got.Wheels[0].Options); optCount != n {
		t.Errorf("wheel 0 has %d options, want %d (lost updates)", optCount, n)
	}
}

func TestConcurrentReadWriteRace(t *testing.T) {
	store := NewStore()
	tmpl := testTemplate(t)
	mux := setupRouter(store, tmpl)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Create a session via the store directly
	session, err := store.Create()
	if err != nil {
		t.Fatalf("store.Create(): %v", err)
	}

	// Add some initial options
	for i := 0; i < 5; i++ {
		err := store.Update(session.ID, func(s *Session) error {
			s.Wheels[0] = wheel.AddOption(s.Wheels[0], fmt.Sprintf("Opt-%d", i), nil)
			return nil
		})
		if err != nil {
			t.Fatalf("store.Update(): %v", err)
		}
	}

	// Fire N goroutines concurrently:
	// - half doing GET / (homeHandler reads wheels via View)
	// - half doing POST /wheel/0/option (modifies wheels via Update)
	// Run with -race to detect unsynchronized read↔write.
	n := 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		if i%2 == 0 {
			// GET (read)
			go func() {
				defer wg.Done()
				req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
				if err != nil {
					t.Errorf("creating GET request: %v", err)
					return
				}
				req.AddCookie(&http.Cookie{Name: "bbw_session", Value: session.ID})
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}()
		} else {
			// POST (write)
			go func() {
				defer wg.Done()
				form := url.Values{"text": {"X"}}
				req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
				if err != nil {
					t.Errorf("creating POST request: %v", err)
					return
				}
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: "bbw_session", Value: session.ID})
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					resp.Body.Close()
				}
			}()
		}
	}
	wg.Wait()
}

func TestWheelOption_BadWheelID(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	tests := []struct {
		name    string
		wheelID string
		want    int
	}{
		{"out of range (999)", "999", http.StatusNotFound},
		{"non-numeric (abc)", "abc", http.StatusNotFound},
		{"negative (-1)", "-1", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{"text": {"X"}}
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/"+tt.wheelID+"/option", strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST /wheel/%s/option: %v", tt.wheelID, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestWheelOption_InvalidWeight(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	form := url.Values{"text": {"X"}, "weight": {"abc"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestWheelOption_NegativeIdx(t *testing.T) {
	ts, _ := testWheelServer(t)
	sessionID := getSessionCookie(t, ts)

	// Add one option so the wheel isn't empty
	form := url.Values{"text": {"A"}}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/wheel/0/option", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /wheel/0/option: %v", err)
	}
	resp.Body.Close()

	t.Run("negative index (-1)", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/wheel/0/option/-1", nil)
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /wheel/0/option/-1: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("non-numeric index (abc)", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/wheel/0/option/abc", nil)
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: "bbw_session", Value: sessionID})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /wheel/0/option/abc: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}
