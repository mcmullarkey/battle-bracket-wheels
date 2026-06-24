// Package wheel provides the wheel model with relative-weight normalization,
// option CRUD operations, and SVG arc math.
//
// This package is pure — no I/O, no mutation, no HTTP concerns.
// All functions are deterministic and return new values rather than
// modifying inputs.
package wheel

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
)

// ErrNoSelectableOptions is returned when a wheel has no options to normalize.
var ErrNoSelectableOptions = errors.New("no selectable options")

// Wheel represents a spinning wheel with an ID and a list of options.
type Wheel struct {
	ID      string   `json:"id"`
	Options []Option `json:"options"`
}

// Option represents a single selectable item on the wheel.
// Weight is a pointer to allow distinguishing "not set" (nil) from "set to 0".
// A nil Weight means the option gets an equal share (effective weight 1.0
// in the raffle-ticket model).
type Option struct {
	Text   string   `json:"text"`
	Weight *float64 `json:"weight,omitempty"`
}

// NormalizeWeights converts relative weights to probabilities that sum to 1.0.
//
// The raffle-ticket algorithm:
//   - nil Weight → effective weight 1.0
//   - effective weight = *Weight otherwise
//   - p[i] = effective[i] / sum(effective)
//   - If sum is 0 (all zero weights), fall back to equal split.
//   - If no options exist, returns ErrNoSelectableOptions.
//
// The input wheel is never modified.
func NormalizeWeights(w Wheel) ([]float64, error) {
	n := len(w.Options)
	if n == 0 {
		return nil, ErrNoSelectableOptions
	}

	eff := make([]float64, n)
	sum := 0.0
	for i, opt := range w.Options {
		if opt.Weight == nil {
			eff[i] = 1.0
		} else {
			eff[i] = *opt.Weight
		}
		sum += eff[i]
	}

	// All-zero weights → equal split
	if sum == 0 {
		probs := make([]float64, n)
		for i := range probs {
			probs[i] = 1.0 / float64(n)
		}
		return probs, nil
	}

	probs := make([]float64, n)
	for i := range probs {
		probs[i] = eff[i] / sum
	}
	return probs, nil
}

// AddOption returns a new Wheel with the given option appended.
// The original wheel is not modified.
func AddOption(w Wheel, text string, weight *float64) Wheel {
	newOpts := make([]Option, len(w.Options)+1)
	copy(newOpts, w.Options)
	newOpts[len(newOpts)-1] = Option{Text: text, Weight: weight}
	return Wheel{ID: w.ID, Options: newOpts}
}

// RemoveOption returns a new Wheel with the option at index idx removed.
// Returns an error if idx is out of range.
// The original wheel is not modified.
func RemoveOption(w Wheel, idx int) (Wheel, error) {
	if idx < 0 || idx >= len(w.Options) {
		return w, fmt.Errorf("option index %d out of range [0, %d)", idx, len(w.Options))
	}
	newOpts := make([]Option, 0, len(w.Options)-1)
	newOpts = append(newOpts, w.Options[:idx]...)
	newOpts = append(newOpts, w.Options[idx+1:]...)
	return Wheel{ID: w.ID, Options: newOpts}, nil
}

// SpinResult bundles the outcome of a single spin: the selected option index,
// the option itself, and the rotation angle to align its midpoint under the
// top pointer.
type SpinResult struct {
	Index       int     `json:"index"`
	Option      Option  `json:"option"`
	TargetAngle float64 `json:"targetAngle"`
}

// Spin selects a weighted-random option from the wheel and computes the target
// rotation angle to align that slice's midpoint under the top pointer.
//
// Pure: same (w, src) → same SpinResult. Tests inject deterministic rand.Source.
// The src parameter is consumed (one Float64 draw per call).
//
// Returns ErrNoSelectableOptions if the wheel has no options or all effective
// weights are zero.
func Spin(w Wheel, src rand.Source) (SpinResult, error) {
	n := len(w.Options)
	if n == 0 {
		return SpinResult{}, ErrNoSelectableOptions
	}

	// Check if all effective weights are explicitly zero.
	// nil Weight → effective 1.0, so if any is nil we have selectable options.
	allZero := true
	for _, opt := range w.Options {
		if opt.Weight == nil {
			allZero = false
			break
		}
		if *opt.Weight != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return SpinResult{}, ErrNoSelectableOptions
	}

	probs, err := NormalizeWeights(w)
	if err != nil {
		return SpinResult{}, err
	}

	// Weighted random selection using cumulative distribution
	rng := rand.New(src)
	f := rng.Float64()
	cumulative := 0.0
	idx := 0
	for i, p := range probs {
		cumulative += p
		if f < cumulative {
			idx = i
			break
		}
	}

	targetAngle := computeTargetAngle(probs, idx)

	return SpinResult{
		Index:       idx,
		Option:      w.Options[idx],
		TargetAngle: targetAngle,
	}, nil
}

// computeTargetAngle computes the rotation angle (degrees) needed to align
// the midpoint of slice idx under the top pointer (0°).
//
// It uses ComputeArcAngles to get the actual arc boundaries (which respect
// weighted probabilities), then computes the midpoint from the real arc.
// Target rotation = (360 - midpoint) mod 360, so the midpoint lands at 0°.
//
// For a single option, returns 0 (no rotation needed — the entire wheel is
// the only slice).
func computeTargetAngle(probs []float64, idx int) float64 {
	n := len(probs)
	if n <= 1 {
		return 0
	}
	arcs := ComputeArcAngles(probs)
	arc := arcs[idx]
	midpoint := (arc.StartDeg + arc.EndDeg) / 2.0
	target := math.Mod(360.0-midpoint, 360.0)
	if target < 0 {
		target += 360.0
	}
	return target
}
