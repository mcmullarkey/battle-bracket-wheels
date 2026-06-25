---
type: Function
title: Weighted-Random Spin Algorithm
description: Raffle-ticket weight normalization + cumulative-distribution spin selection + target-angle computation
resource: internal/wheel/wheel.go
tags: [go, algorithm, wheel, pure, §2-pure-core]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Convert weighted options into a probability distribution, select one via injected randomness, compute the SVG rotation target angle. Non-obvious because nil-weight semantics and angle alignment are subtle.

# Interface

- `NormalizeWeights(Wheel) ([]float64, error)` — raffle-ticket model: nil→1.0, all-zero→equal split
- `Spin(Wheel, rand.Source) (SpinResult, error)` — cumulative-distribution selection; returns `ErrNoSelectableOptions` for empty/all-zero
- `computeTargetAngle(probs []float64, index int) float64` — `(360 - midpoint) mod 360` to align slice midpoint under top pointer

# Dependencies

- Operates on [wheel domain types](/data-models/wheel.md)
- `rand.Source` injected by [app-architecture](/modules/app-architecture.md) via `newSpinSource` (crypto/rand seed)

# Invariants

- Weight 0 explicitly set → treated as 0 probability (distinct from nil = 1.0)
- Target angle = 360° minus the midpoint of the selected slice's angular range
- Pure given `rand.Source` — same source + same wheel = same result

# Pure/Effectful

Pure core. Randomness injected, not imported. No side effects.

# Examples

```go
src := rand.NewSource(42)
result, err := wheel.Spin(w, src)
// result.TargetAngle used by client JS to rotate SVG
```

# Citations

- [internal/wheel/wheel.go:44 NormalizeWeights](internal/wheel/wheel.go)
- [internal/wheel/wheel.go:116 Spin](internal/wheel/wheel.go)
- [internal/wheel/wheel.go:175 computeTargetAngle](internal/wheel/wheel.go)
