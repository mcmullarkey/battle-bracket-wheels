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
