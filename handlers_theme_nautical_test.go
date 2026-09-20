package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// Integration slice: the Nautical Nightmares theme is selectable alongside
// existing themes and serves its stylesheet plus its sea/lighthouse artwork.

func TestNauticalThemeInDropdown(t *testing.T) {
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

	if !strings.Contains(bodyStr, `value="nautical"`) {
		t.Error("body missing option value=nautical in theme dropdown")
	}
	if !strings.Contains(bodyStr, ">Nautical Nightmares<") {
		t.Error("body missing Nautical Nightmares option label in theme dropdown")
	}
	// Existing themes must remain selectable.
	if !strings.Contains(bodyStr, `value="space"`) {
		t.Error("body missing option value=space (existing theme removed)")
	}
	if !strings.Contains(bodyStr, `value="y2k"`) {
		t.Error("body missing option value=y2k (existing theme removed)")
	}
}

func TestNauticalThemeCookieSelectsStylesheet(t *testing.T) {
	ts := testThemeServer(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "bbw_theme", Value: "nautical"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET / with nautical cookie: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	bodyStr := string(body)

	if !strings.Contains(bodyStr, `href="/static/css/nautical.css"`) {
		t.Errorf("body missing href=/static/css/nautical.css (cookie-selected theme)\nbody: %s", bodyStr)
	}
	if strings.Contains(bodyStr, `href="/static/css/space.css"`) {
		t.Errorf("body should not link space.css when nautical theme is active")
	}
}

func TestNauticalThemeAssetsServed(t *testing.T) {
	ts := testThemeServer(t)

	assets := []string{
		"/static/img/nautical-sea.jpg",
		"/static/img/nautical-lighthouse.jpg",
	}

	// Stylesheet exists and references both background layers.
	resp, err := http.Get(ts.URL + "/static/css/nautical.css")
	if err != nil {
		t.Fatalf("GET /static/css/nautical.css: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("nautical.css status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	css, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading nautical.css: %v", err)
	}
	for _, asset := range assets {
		if !strings.Contains(string(css), asset) {
			t.Errorf("nautical.css does not reference %s background layer", asset)
		}
	}

	// Artwork is served from the embedded static filesystem.
	for _, asset := range assets {
		imgResp, err := http.Get(ts.URL + asset)
		if err != nil {
			t.Fatalf("GET %s: %v", asset, err)
		}
		img, err := io.ReadAll(imgResp.Body)
		imgResp.Body.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", asset, err)
		}
		if imgResp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", asset, imgResp.StatusCode, http.StatusOK)
		}
		// JPEG magic bytes — the asset is a real image, not a placeholder.
		if len(img) < 3 || img[0] != 0xFF || img[1] != 0xD8 || img[2] != 0xFF {
			t.Errorf("%s is not a valid JPEG", asset)
		}
	}
}
