package wheel

import (
	"math"
	"strings"
	"testing"
)

func TestComputeArcAngles(t *testing.T) {
	probs := []float64{0.2, 0.3, 0.5}
	arcs := ComputeArcAngles(probs)
	if len(arcs) != 3 {
		t.Fatalf("len(arcs) = %d, want 3", len(arcs))
	}
	// Verify arcs
	if math.Abs(arcs[0].StartDeg-0) > 0.001 || math.Abs(arcs[0].EndDeg-72) > 0.001 {
		t.Errorf("arc[0]: start=%v end=%v, want 0,72", arcs[0].StartDeg, arcs[0].EndDeg)
	}
	if math.Abs(arcs[1].StartDeg-72) > 0.001 || math.Abs(arcs[1].EndDeg-180) > 0.001 {
		t.Errorf("arc[1]: start=%v end=%v, want 72,180", arcs[1].StartDeg, arcs[1].EndDeg)
	}
	if math.Abs(arcs[2].StartDeg-180) > 0.001 || math.Abs(arcs[2].EndDeg-360) > 0.001 {
		t.Errorf("arc[2]: start=%v end=%v, want 180,360", arcs[2].StartDeg, arcs[2].EndDeg)
	}
	// Last arc clamped to 360
	if math.Abs(arcs[2].EndDeg-360.0) > 0.001 {
		t.Errorf("last arc EndDeg = %v, want 360.0", arcs[2].EndDeg)
	}
}

func TestComputeArcAnglesLargeArc(t *testing.T) {
	// Probability > 0.5 → sweep > 180 → LargeArcFlag = true
	probs := []float64{0.6, 0.4}
	arcs := ComputeArcAngles(probs)
	if len(arcs) != 2 {
		t.Fatalf("len(arcs) = %d, want 2", len(arcs))
	}
	if !arcs[0].LargeArcFlag {
		t.Error("arc[0] LargeArcFlag should be true (sweep 216° > 180°)")
	}
	if arcs[1].LargeArcFlag {
		t.Error("arc[1] LargeArcFlag should be false (sweep 144° < 180°)")
	}
}

func TestSVGArcSum(t *testing.T) {
	probs := []float64{0.1, 0.2, 0.3, 0.4}
	arcs := ComputeArcAngles(probs)
	var total float64
	for _, a := range arcs {
		total += a.EndDeg - a.StartDeg
	}
	if math.Abs(total-360.0) > 0.01 {
		t.Errorf("sum of sweeps = %v, want 360.0 ± 0.01", total)
	}
}

func TestSingleOptionFullCircle(t *testing.T) {
	arcs := ComputeArcAngles([]float64{1.0})
	// Single option should produce 2 arcs (180° each) for valid SVG
	if len(arcs) != 2 {
		t.Fatalf("len(arcs) = %d, want 2 for single option", len(arcs))
	}
	if math.Abs(arcs[0].StartDeg-0) > 0.001 || math.Abs(arcs[0].EndDeg-180) > 0.001 {
		t.Errorf("arc[0]: start=%v end=%v, want 0,180", arcs[0].StartDeg, arcs[0].EndDeg)
	}
	if math.Abs(arcs[1].StartDeg-180) > 0.001 || math.Abs(arcs[1].EndDeg-360) > 0.001 {
		t.Errorf("arc[1]: start=%v end=%v, want 180,360", arcs[1].StartDeg, arcs[1].EndDeg)
	}
	// Both arcs should have LargeArcFlag = false (180° is not > 180)
	if arcs[0].LargeArcFlag {
		t.Error("arc[0] LargeArcFlag should be false")
	}
	if arcs[1].LargeArcFlag {
		t.Error("arc[1] LargeArcFlag should be false")
	}
}

func TestArcPath(t *testing.T) {
	arc := Arc{StartDeg: 0, EndDeg: 90, LargeArcFlag: false}
	path := ArcPath(arc, 100, 100, 80)
	if !strings.HasPrefix(path, "M 100.00 100.00") {
		t.Errorf("path should start with center: %s", path)
	}
	if !strings.Contains(path, "A 80.00 80.00 0 0 1") {
		t.Errorf("path should contain arc command: %s", path)
	}
	if !strings.HasSuffix(path, "Z") {
		t.Errorf("path should end with Z: %s", path)
	}
}

func TestArcPathPoints(t *testing.T) {
	// Arc from 0° (top) to 90° (right) with center (100,100), radius 80
	// Start: x=100+80*sin(0)=100, y=100-80*cos(0)=20
	// End: x=100+80*sin(90)=180, y=100-80*cos(90)=100
	arc := Arc{StartDeg: 0, EndDeg: 90, LargeArcFlag: false}
	path := ArcPath(arc, 100, 100, 80)
	if !strings.Contains(path, "L 100.00 20.00") {
		t.Errorf("path should line to (100,20): %s", path)
	}
	if !strings.Contains(path, "A 80.00 80.00 0 0 1 180.00 100.00") {
		t.Errorf("path should arc to (180,100): %s", path)
	}
}
