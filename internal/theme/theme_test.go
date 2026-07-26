package theme

import (
	"testing"
)

func TestThemeRegistryRegisterAndResolve(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("space", "Space", "/static/css/space.css"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := r.Resolve("space")
	if !ok {
		t.Fatal("Resolve(space) = false, want true")
	}
	if got.CSSPath != "/static/css/space.css" {
		t.Errorf("Resolve(space).CSSPath = %q, want /static/css/space.css", got.CSSPath)
	}
	if got.Key != "space" {
		t.Errorf("Resolve(space).Key = %q, want space", got.Key)
	}
	if got.Name != "Space" {
		t.Errorf("Resolve(space).Name = %q, want Space", got.Name)
	}
}

func TestThemeRegistryUnknownKey(t *testing.T) {
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	got, ok := r.Resolve("unregistered")
	if ok {
		t.Error("Resolve(unregistered) = true, want false")
	}
	if got != (Theme{}) {
		t.Errorf("Resolve(unregistered) = %+v, want zero Theme{}", got)
	}
}

func TestThemeRegistryPathTraversal(t *testing.T) {
	// Closed-set registry: path traversal keys are never registered,
	// so Resolve returns (Theme{}, false). No interpolation of cookie value.
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	cases := []string{"../etc", "../../etc/passwd", "..%2fetc", "space/../../etc"}
	for _, key := range cases {
		got, ok := r.Resolve(key)
		if ok {
			t.Errorf("Resolve(%q) = true, want false (path traversal rejected)", key)
		}
		if got != (Theme{}) {
			t.Errorf("Resolve(%q) = %+v, want zero Theme{}", key, got)
		}
	}
}

func TestThemeRegistryDefault(t *testing.T) {
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	d := r.Default()
	if d.CSSPath != "/static/css/space.css" {
		t.Errorf("Default().CSSPath = %q, want /static/css/space.css", d.CSSPath)
	}
	if d.Key != "space" {
		t.Errorf("Default().Key = %q, want space", d.Key)
	}
}

func TestThemeRegistryDefaultEmpty(t *testing.T) {
	// Empty registry → Default returns zero Theme (no panic).
	r := NewRegistry()
	d := r.Default()
	if d != (Theme{}) {
		t.Errorf("Default() on empty registry = %+v, want zero Theme{}", d)
	}
}

func TestThemeRegistryNames(t *testing.T) {
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	r.Register("y2k", "Y2K", "/static/css/y2k.css")
	names := r.Names()
	found := false
	for _, n := range names {
		if n == "space" {
			found = true
		}
	}
	if !found {
		t.Errorf("Names() = %v, want to contain space", names)
	}
	if len(names) != 2 {
		t.Errorf("len(Names()) = %d, want 2", len(names))
	}
}

func TestThemeRegistryNamesPreservesOrder(t *testing.T) {
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	r.Register("y2k", "Y2K", "/static/css/y2k.css")
	r.Register("neon", "Neon", "/static/css/neon.css")
	names := r.Names()
	want := []string{"space", "y2k", "neon"}
	if len(names) != len(want) {
		t.Fatalf("len(Names()) = %d, want %d", len(names), len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("Names()[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestThemeRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("space", "Space", "/static/css/space.css"); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := r.Register("space", "Space2", "/static/css/space2.css")
	if err == nil {
		t.Error("duplicate Register returned nil error, want error")
	}
	// Original registration must be preserved (not overwritten).
	got, ok := r.Resolve("space")
	if !ok {
		t.Fatal("Resolve(space) after duplicate = false, want true")
	}
	if got.Name != "Space" {
		t.Errorf("Resolve(space).Name after duplicate = %q, want Space (original preserved)", got.Name)
	}
}

func TestThemeRegistryThemes(t *testing.T) {
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	r.Register("y2k", "Y2K", "/static/css/y2k.css")
	themes := r.Themes()
	if len(themes) != 2 {
		t.Fatalf("len(Themes()) = %d, want 2", len(themes))
	}
	if themes[0].Key != "space" || themes[1].Key != "y2k" {
		t.Errorf("Themes() order = [%s, %s], want [space, y2k]", themes[0].Key, themes[1].Key)
	}
}

func TestThemeRegistryConcurrentResolve(t *testing.T) {
	// Race detector guard: concurrent Resolve calls must be safe.
	r := NewRegistry()
	r.Register("space", "Space", "/static/css/space.css")
	r.Register("y2k", "Y2K", "/static/css/y2k.css")

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			r.Resolve("space")
			r.Resolve("y2k")
			r.Default()
			r.Names()
			r.Themes()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
