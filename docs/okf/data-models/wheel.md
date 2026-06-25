---
type: Data Model
title: Wheel Domain Types
description: Core wheel aggregate — Wheel, Option, SpinResult, Arc
resource: internal/wheel/wheel.go
tags: [go, domain, wheel, pure, §1-types]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Wheel domain types: weighted option collection, spin result, SVG arc geometry. Foundation for all wheel operations. Does NOT perform HTTP, I/O, or mutation — all functions return new values.

# Interface

- `Wheel{ID int, Options []Option}` — aggregate root
- `Option{Text string, Weight *float64}` — nil Weight = equal share (raffle-ticket model)
- `SpinResult{Index int, Option Option, TargetAngle float64}` — spin outcome + animation target
- `Arc{StartDeg, EndDeg, LargeArcFlag float64}` — SVG arc segment

# Dependencies

- Consumed by [battle-resolution](/modules/battle-resolution.md) and [bracket-state-machine](/modules/bracket-state-machine.md)
- SVG arcs rendered by [svg-arc-geometry](/functions/svg-arc-geometry.md)
- Spin algorithm in [wheel-spin-algorithm](/functions/wheel-spin-algorithm.md)

# Invariants

- nil `Weight` → effective weight 1.0 (§1 — illegal state: negative weight not modeled, zero handled by normalization)
- All-zero weights → equal split across options
- Single option → two 180° arcs (SVG cannot render 360° arc)
- `SpinResult.TargetAngle` aligns slice midpoint under top pointer

# Pure/Effectful

Pure. No I/O, no mutation, no `math/rand` global state. Randomness injected via `rand.Source` parameter.

# Citations

- [internal/wheel/wheel.go](internal/wheel/wheel.go)
- [internal/wheel/svg.go](internal/wheel/svg.go)
