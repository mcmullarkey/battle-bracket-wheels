package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// Integration slice: the Nic Cage theme is selectable alongside existing
// themes and serves its stylesheet plus layered background image.

func TestCageThemeInDropdown(t *testing.T) {
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

	if !strings.Contains(bodyStr, `value="cage"`) {
		t.Error("body missing option value=cage in theme dropdown")
	}
	if !strings.Contains(bodyStr, ">Nic Cage<") {
		t.Error("body missing Nic Cage option label in theme dropdown")
	}
	// Existing themes must remain selectable.
	if !strings.Contains(bodyStr, `value="space"`) {
		t.Error("body missing option value=space (existing theme removed)")
	}
	if !strings.Contains(bodyStr, `value="y2k"`) {
		t.Error("body missing option value=y2k (existing theme removed)")
	}
}

func TestCageThemeCookieSelectsStylesheet(t *testing.T) {
	ts := testThemeServer(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "cage"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET / with cage cookie: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `href="/static/css/cage.css"`) {
		t.Errorf("body missing href=/static/css/cage.css (cookie-selected theme)\nbody: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `href="/static/css/space.css"`) {
		t.Errorf("body should not link space.css when cage theme is active")
	}
}

func TestCageThemeAssetsServed(t *testing.T) {
	ts := testThemeServer(t)

	// Stylesheet exists and references the layered background image.
	resp, err := http.Get(ts.URL + "/static/css/cage.css")
	if err != nil {
		t.Fatalf("GET /static/css/cage.css: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cage.css status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	css, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading cage.css: %v", err)
	}
	if !strings.Contains(string(css), "/static/img/cage-declaration.jpg") {
		t.Error("cage.css does not reference /static/img/cage-declaration.jpg background")
	}

	// Composite background image (Nic Cage layered over the Declaration of
	// Independence) is served from the embedded static filesystem.
	imgResp, err := http.Get(ts.URL + "/static/img/cage-declaration.jpg")
	if err != nil {
		t.Fatalf("GET /static/img/cage-declaration.jpg: %v", err)
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		t.Fatalf("cage-declaration.jpg status = %d, want %d", imgResp.StatusCode, http.StatusOK)
	}
	img, err := io.ReadAll(imgResp.Body)
	if err != nil {
		t.Fatalf("reading cage-declaration.jpg: %v", err)
	}
	// JPEG magic bytes — the asset is a real image, not a placeholder.
	if len(img) < 3 || img[0] != 0xFF || img[1] != 0xD8 || img[2] != 0xFF {
		t.Error("cage-declaration.jpg is not a valid JPEG")
	}
}
