---
type: Module
title: View-Model Layer
description: Domain→template mapping — WheelViewData, BracketViewData, arc path computation for rendering
resource: handlers_wheel.go
tags: [go, view-model, §3-cut-at-joints, §5-function-discipline]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Transform domain types into template-shaped view-models. Compute SVG arc paths and text positions for rendering. Isolate template concerns from domain logic (§3).

# Interface

- `WheelViewData{Wheel, Arcs, TextPositions, ReadOnly}` — template-shaped wheel
- `WheelOptionView{Index, Text, Weight, DisplayWeight}` — option row
- `BracketViewData{QuarterFinals, SemiFinals, Final, Winner}` — full bracket view
- `wheelViewFromWheel(Wheel, readOnly bool) WheelViewData` — domain→view; computes arcs + text positions; single-option special case
- `bracketViewFromBracket(*Bracket) BracketViewData` — marks propagated SF/Final wheels `ReadOnly=true`

# Dependencies

- Consumes [wheel domain types](/data-models/wheel.md) and [bracket-state-machine](/modules/bracket-state-machine.md)
- Uses [svg-arc-geometry](/functions/svg-arc-geometry.md) for arc computation
- Consumed by [template-system](/services/template-system.md)

# Invariants

- View-models are read-only snapshots — never mutate domain state
- `ReadOnly=true` for propagated SF/Final wheels (hides add-option form)
- Single-option wheel → special-case arc rendering

# Pure/Effectful

Pure. No I/O, no mutation. Pure transformation functions.

# Citations

- [handlers_wheel.go](handlers_wheel.go)
- [handlers.go](handlers.go)
