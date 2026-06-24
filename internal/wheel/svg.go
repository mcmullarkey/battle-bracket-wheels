package wheel

import (
	"fmt"
	"math"
)

// Arc describes a single SVG arc segment of the wheel.
type Arc struct {
	StartDeg     float64 `json:"start_deg"`
	EndDeg       float64 `json:"end_deg"`
	LargeArcFlag bool    `json:"large_arc_flag"`
}

// ComputeArcAngles converts a slice of probabilities into Arc segments
// suitable for SVG rendering.
//
// Rules:
//   - sweep = p * 360°
//   - LargeArcFlag = true when sweep > 180°
//   - Last arc EndDeg is clamped to exactly 360.0 to close the circle
//   - For a single option (probability 1.0), returns two 180° arcs
//     since SVG cannot render a single 360° arc (start == end point).
func ComputeArcAngles(probs []float64) []Arc {
	n := len(probs)
	if n == 0 {
		return nil
	}

	// Single option → two 180° arcs for valid SVG full circle
	if n == 1 {
		return []Arc{
			{StartDeg: 0, EndDeg: 180, LargeArcFlag: false},
			{StartDeg: 180, EndDeg: 360, LargeArcFlag: false},
		}
	}

	arcs := make([]Arc, n)
	start := 0.0
	for i, p := range probs {
		sweep := p * 360.0
		end := start + sweep
		largeArc := sweep > 180

		// Last arc: clamp EndDeg to exactly 360.0
		if i == n-1 {
			end = 360.0
		}

		arcs[i] = Arc{StartDeg: start, EndDeg: end, LargeArcFlag: largeArc}
		start = end
	}
	return arcs
}

// ArcPath returns the SVG path "d" attribute for a single pie-slice arc.
//
// The path draws from the center to the arc start point, then arcs to the
// end point, then closes back to center.
//
// Angles are measured clockwise from the top (12 o'clock position).
//
// The SVG arc command uses sweep-flag=1 (clockwise in SVG coordinates
// where positive y points downward).
func ArcPath(arc Arc, cx, cy, r float64) string {
	startRad := toRadians(arc.StartDeg)
	endRad := toRadians(arc.EndDeg)

	startX := cx + r*math.Sin(startRad)
	startY := cy - r*math.Cos(startRad)
	endX := cx + r*math.Sin(endRad)
	endY := cy - r*math.Cos(endRad)

	largeFlag := 0
	if arc.LargeArcFlag {
		largeFlag = 1
	}

	// SVG arc: A rx ry x-axis-rotation large-arc-flag sweep-flag x y
	// sweep-flag = 1 (clockwise in SVG coordinates since y points down)
	return fmt.Sprintf("M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 %d 1 %.2f %.2f Z",
		cx, cy,
		startX, startY,
		r, r,
		largeFlag,
		endX, endY,
	)
}

// toRadians converts degrees to radians.
func toRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}
