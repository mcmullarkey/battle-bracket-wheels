---
type: Interface
title: Spin Handler & HX-Trigger
description: Weighted-random spin — returns wheel fragment + HX-Trigger header for client animation
resource: handlers_spin.go
tags: [go, http, htmx, spin, §2-effectful-shell]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Handle single-wheel spin. Call pure spin algorithm, update session with result, return wheel fragment + `HX-Trigger: spin-wheel` header to trigger client-side animation.

# Interface

- `spinHandler` — POST `/wheel/{id}/spin`; returns wheel fragment + `HX-Trigger: {"spin-wheel": spinResult}` header

# Dependencies

- Uses [wheel-spin-algorithm](/functions/wheel-spin-algorithm.md) for `Spin`
- Uses [session-store](/modules/session-store.md) for state
- Uses [view-models](/modules/view-models.md) + [template-system](/services/template-system.md) for rendering
- Triggers [client-animation](/presentation/client-animation.md) via HX-Trigger

# Invariants

- `HX-Trigger` header carries spin result (index, target angle) for client JS
- Empty/all-zero wheel → error response

# Pure/Effectful

Effectful. HTTP I/O, session mutation, template execution. Delegates pure spin to [wheel-spin-algorithm](/functions/wheel-spin-algorithm.md).

# Citations

- [handlers_spin.go](handlers_spin.go)
