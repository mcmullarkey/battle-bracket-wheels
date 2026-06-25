---
type: Function
title: SVG Arc Geometry
description: Compute arc angles and generate SVG path data for wheel slices
resource: internal/wheel/svg.go
tags: [go, svg, geometry, pure, §2-pure-core]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Convert probability distribution into SVG arc segments. Handle edge cases: single option (two 180° arcs), last arc clamped to exactly 360.0. Generate SVG `d` attribute path strings.

# Interface

- `ComputeArcAngles(probs []float64) []Arc` — probability→arc mapping; single option → two 180° arcs; last arc EndDeg clamped to 360.0
- `ArcPath(arc Arc, cx, cy, r float64) string` — SVG `d` attribute; sweep-flag=1 (clockwise, y-down); angles measured from top

# Dependencies

- Consumes probabilities from [wheel-spin-algorithm](/functions/wheel-spin-algorithm.md)
- Produces arcs consumed by [view-models](/modules/view-models.md) for template rendering

# Invariants

- SVG cannot draw a 360° arc — single option splits into two 180° arcs
- Last arc EndDeg = 360.0 exactly (floating-point clamp prevents gap)
- sweep-flag=1 (clockwise) because SVG y-axis points down
- Angles measured clockwise from top (12 o'clock position)

# Pure/Effectful

Pure. No I/O, no mutation.

# Citations

- [internal/wheel/svg.go:24 ComputeArcAngles](internal/wheel/svg.go)
- [internal/wheel/svg.go:65 ArcPath](internal/wheel/svg.go)
