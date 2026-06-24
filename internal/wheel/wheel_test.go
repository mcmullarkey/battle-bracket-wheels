package wheel

import (
	"math"
	"math/rand"
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

// ---- Spin tests ----

func TestSpinDeterminism(t *testing.T) {
	wh := Wheel{Options: []Option{{Text: "A"}, {Text: "B", Weight: ptr(2.0)}, {Text: "C", Weight: ptr(3.0)}}}
	src := rand.NewSource(42)
	r1, err1 := Spin(wh, src)
	src = rand.NewSource(42)
	r2, err2 := Spin(wh, src)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected error: %v / %v", err1, err2)
	}
	if r1.Index != r2.Index {
		t.Errorf("Index: %d != %d", r1.Index, r2.Index)
	}
	if r1.Option.Text != r2.Option.Text {
		t.Errorf("Option.Text: %q != %q", r1.Option.Text, r2.Option.Text)
	}
	if r1.TargetAngle != r2.TargetAngle {
		t.Errorf("TargetAngle: %f != %f", r1.TargetAngle, r2.TargetAngle)
	}
}

func TestSpinDistribution(t *testing.T) {
	wh := Wheel{Options: []Option{
		{Text: "A", Weight: ptr(1.0)},
		{Text: "B", Weight: ptr(2.0)},
		{Text: "C", Weight: ptr(7.0)},
	}}
	src := rand.NewSource(42)
	n := 10000
	counts := make([]int, 3)
	for range n {
		result, err := Spin(wh, src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[result.Index]++
	}
	expected := []float64{1000, 2000, 7000}
	chiSq := 0.0
	for i := range counts {
		diff := float64(counts[i]) - expected[i]
		chiSq += diff * diff / expected[i]
	}
	// df=2, α=0.01 critical ≈ 9.21
	if chiSq > 9.21 {
		t.Errorf("chi-square = %.2f, want <= 9.21 (α=0.01, df=2). Counts: %v", chiSq, counts)
	}
}

func TestSpinUniformCoverage(t *testing.T) {
	wh := Wheel{Options: []Option{
		{Text: "A", Weight: ptr(1.0)},
		{Text: "B", Weight: ptr(1.0)},
		{Text: "C", Weight: ptr(1.0)},
	}}
	src := rand.NewSource(42)
	seen := make(map[int]bool)
	for range 100 {
		result, err := Spin(wh, src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[result.Index] = true
		if len(seen) == 3 {
			break
		}
	}
	if len(seen) != 3 {
		t.Errorf("only saw %d distinct indices in first 100 spins: %v", len(seen), seen)
	}
}

func TestSpinSingleOption(t *testing.T) {
	wh := Wheel{Options: []Option{{Text: "X"}}}
	src := rand.NewSource(42)
	result, err := Spin(wh, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Index != 0 {
		t.Errorf("Index = %d, want 0", result.Index)
	}
	if result.Option.Text != "X" {
		t.Errorf("Option.Text = %q, want %q", result.Option.Text, "X")
	}
	if result.TargetAngle != 0 {
		t.Errorf("TargetAngle = %f, want 0", result.TargetAngle)
	}
}

func TestSpinAngleCorrectness(t *testing.T) {
	// 3 equal slices → 3 distinct TargetAngle values, 120° apart
	wh := Wheel{Options: []Option{
		{Text: "A", Weight: ptr(1.0)},
		{Text: "B", Weight: ptr(1.0)},
		{Text: "C", Weight: ptr(1.0)},
	}}
	src := rand.NewSource(42)
	angles := make(map[int]float64)
	for range 100 {
		result, err := Spin(wh, src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		angles[result.Index] = result.TargetAngle
		if len(angles) == 3 {
			break
		}
	}
	if len(angles) != 3 {
		t.Fatalf("expected 3 distinct angles, got %d: %v", len(angles), angles)
	}
	// Verify they're 120° apart and match expected {300, 180, 60}
	expected := map[int]float64{0: 300, 1: 180, 2: 60}
	for idx, want := range expected {
		got := angles[idx]
		if math.Abs(got-want) > 0.001 {
			t.Errorf("angle[%d] = %f, want %f", idx, got, want)
		}
	}
}

func TestSpinZeroWeightExclusion(t *testing.T) {
	wh := Wheel{Options: []Option{
		{Text: "Z1", Weight: ptr(0.0)},
		{Text: "Only", Weight: ptr(1.0)},
		{Text: "Z2", Weight: ptr(0.0)},
	}}
	src := rand.NewSource(42)
	n := 1000
	for range n {
		result, err := Spin(wh, src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Index != 1 {
			t.Errorf("Index = %d, want 1 (only non-zero weight option)", result.Index)
		}
	}
}

func TestSpinEmptyWheel(t *testing.T) {
	src := rand.NewSource(42)
	_, err := Spin(Wheel{}, src)
	if err != ErrNoSelectableOptions {
		t.Errorf("error = %v, want %v", err, ErrNoSelectableOptions)
	}
}

func TestSpinAllZeroWeight(t *testing.T) {
	wh := Wheel{Options: []Option{
		{Text: "A", Weight: ptr(0.0)},
		{Text: "B", Weight: ptr(0.0)},
	}}
	src := rand.NewSource(42)
	_, err := Spin(wh, src)
	if err != ErrNoSelectableOptions {
		t.Errorf("error = %v, want %v", err, ErrNoSelectableOptions)
	}
}

// ---- computeTargetAngle tests ----

func TestComputeTargetAngle_SingleOption(t *testing.T) {
	got := computeTargetAngle([]float64{1.0}, 0)
	if got != 0 {
		t.Errorf("single option target = %f, want 0", got)
	}
}

func TestComputeTargetAngle_ThreeEqual(t *testing.T) {
	probs := []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
	expected := []float64{300, 180, 60}
	for i := range probs {
		got := computeTargetAngle(probs, i)
		if math.Abs(got-expected[i]) > 0.001 {
			t.Errorf("idx %d: target = %f, want %f", i, got, expected[i])
		}
	}
}

func TestComputeTargetAngle_TwoEqual(t *testing.T) {
	// N=2: midpoints are 90° and 270°
	// targets: (360-90)%360 = 270°, (360-270)%360 = 90°
	probs := []float64{0.5, 0.5}
	expected := []float64{270, 90}
	for i := range probs {
		got := computeTargetAngle(probs, i)
		if math.Abs(got-expected[i]) > 0.001 {
			t.Errorf("idx %d: target = %f, want %f", i, got, expected[i])
		}
	}
}

func TestComputeTargetAngle_FourEqual(t *testing.T) {
	// N=4: midpoints are 45°, 135°, 225°, 315°
	// targets: (360-45)=315°, (360-135)=225°, (360-225)=135°, (360-315)=45°
	probs := []float64{0.25, 0.25, 0.25, 0.25}
	expected := []float64{315, 225, 135, 45}
	for i := range probs {
		got := computeTargetAngle(probs, i)
		if math.Abs(got-expected[i]) > 0.001 {
			t.Errorf("idx %d: target = %f, want %f", i, got, expected[i])
		}
	}
}
