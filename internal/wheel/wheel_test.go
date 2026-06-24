package wheel

import (
	"math"
	"testing"
)

func ptr(f float64) *float64 { return &f }

func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

func TestNormalizeWeights(t *testing.T) {
	tests := []struct {
		name    string
		wheel   Wheel
		want    []float64
		wantErr bool
	}{
		{
			name:    "empty options returns error",
			wheel:   Wheel{Options: []Option{}},
			wantErr: true,
		},
		{
			name:  "single nil weight returns 1.0",
			wheel: Wheel{Options: []Option{{Text: "A"}}},
			want:  []float64{1.0},
		},
		{
			name:  "three nil weights equal split",
			wheel: Wheel{Options: []Option{{Text: "A"}, {Text: "B"}, {Text: "C"}}},
			want:  []float64{0.3333, 0.3333, 0.3333},
		},
		{
			name:  "mixed nil and set weights",
			wheel: Wheel{Options: []Option{{Text: "A", Weight: ptr(1.0)}, {Text: "B", Weight: ptr(3.0)}, {Text: "C"}}},
			want:  []float64{0.2, 0.6, 0.2},
		},
		{
			name:  "mixed nil and small weight",
			wheel: Wheel{Options: []Option{{Text: "A"}, {Text: "B", Weight: ptr(0.3)}, {Text: "C"}}},
			want:  []float64{0.4348, 0.1304, 0.4348},
		},
		{
			name:  "all set weights",
			wheel: Wheel{Options: []Option{{Text: "A", Weight: ptr(2.0)}, {Text: "B", Weight: ptr(3.0)}, {Text: "C", Weight: ptr(5.0)}}},
			want:  []float64{0.2, 0.3, 0.5},
		},
		{
			name:  "all zero weights fallback to equal split",
			wheel: Wheel{Options: []Option{{Text: "A", Weight: ptr(0.0)}, {Text: "B", Weight: ptr(0.0)}}},
			want:  []float64{0.5, 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWeights(tt.wheel)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				rounded := round4(got[i])
				wantRounded := round4(tt.want[i])
				if rounded != wantRounded {
					t.Errorf("probs[%d] = %v (rounded %v), want %v", i, got[i], rounded, tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeNoMutation(t *testing.T) {
	orig := Wheel{
		Options: []Option{
			{Text: "A", Weight: ptr(1.0)},
			{Text: "B"},
		},
	}
	wantOpts := []Option{
		{Text: "A", Weight: ptr(1.0)},
		{Text: "B"},
	}

	_, err := NormalizeWeights(orig)
	if err != nil {
		t.Fatalf("NormalizeWeights: %v", err)
	}

	if len(orig.Options) != len(wantOpts) {
		t.Fatalf("options mutated: len %d, want %d", len(orig.Options), len(wantOpts))
	}
	for i := range orig.Options {
		if orig.Options[i].Text != wantOpts[i].Text {
			t.Errorf("options[%d].Text = %q, want %q", i, orig.Options[i].Text, wantOpts[i].Text)
		}
		if (orig.Options[i].Weight == nil) != (wantOpts[i].Weight == nil) {
			t.Errorf("options[%d].Weight nil mismatch: got %v, want %v", i, orig.Options[i].Weight, wantOpts[i].Weight)
		}
		if orig.Options[i].Weight != nil && wantOpts[i].Weight != nil {
			if *orig.Options[i].Weight != *wantOpts[i].Weight {
				t.Errorf("options[%d].Weight = %v, want %v", i, *orig.Options[i].Weight, *wantOpts[i].Weight)
			}
		}
	}
}

func TestAddOption(t *testing.T) {
	w := Wheel{ID: "0", Options: []Option{{Text: "A"}}}
	w2 := AddOption(w, "B", nil)
	if len(w2.Options) != 2 {
		t.Errorf("AddOption: len = %d, want 2", len(w2.Options))
	}
	if w2.Options[1].Text != "B" {
		t.Errorf("AddOption: text = %q, want %q", w2.Options[1].Text, "B")
	}
	if w2.Options[1].Weight != nil {
		t.Error("AddOption: weight should be nil")
	}
	// Verify original unchanged
	if len(w.Options) != 1 {
		t.Error("AddOption mutated original wheel")
	}
}

func TestAddOptionWithWeight(t *testing.T) {
	w := Wheel{ID: "0"}
	w2 := AddOption(w, "X", ptr(2.5))
	if len(w2.Options) != 1 {
		t.Fatalf("AddOption: len = %d, want 1", len(w2.Options))
	}
	if w2.Options[0].Weight == nil {
		t.Fatal("AddOption: weight should not be nil")
	}
	if *w2.Options[0].Weight != 2.5 {
		t.Errorf("AddOption: weight = %v, want 2.5", *w2.Options[0].Weight)
	}
}

func TestRemoveOption(t *testing.T) {
	w := Wheel{ID: "0", Options: []Option{{Text: "A"}, {Text: "B"}, {Text: "C"}}}
	w2, err := RemoveOption(w, 1)
	if err != nil {
		t.Fatalf("RemoveOption: %v", err)
	}
	if len(w2.Options) != 2 {
		t.Fatalf("RemoveOption: len = %d, want 2", len(w2.Options))
	}
	if w2.Options[0].Text != "A" {
		t.Errorf("RemoveOption: [0].Text = %q, want %q", w2.Options[0].Text, "A")
	}
	if w2.Options[1].Text != "C" {
		t.Errorf("RemoveOption: [1].Text = %q, want %q", w2.Options[1].Text, "C")
	}
	// Verify original unchanged
	if len(w.Options) != 3 {
		t.Error("RemoveOption mutated original wheel")
	}
}

func TestRemoveOptionBadIndex(t *testing.T) {
	w := Wheel{ID: "0", Options: []Option{{Text: "A"}}}
	_, err := RemoveOption(w, -1)
	if err == nil {
		t.Error("RemoveOption(-1): expected error, got nil")
	}
	_, err = RemoveOption(w, 1)
	if err == nil {
		t.Error("RemoveOption(1): expected error, got nil")
	}
	_, err = RemoveOption(w, 5)
	if err == nil {
		t.Error("RemoveOption(5): expected error, got nil")
	}
}
