---
type: Interface
title: Wheel CRUD Handlers
description: Add/remove options on wheels — return HTMX wheel fragments
resource: handlers_wheel.go
tags: [go, http, htmx, §5-function-discipline]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Handle option add/remove on wheels. Return rendered wheel HTML fragments for HTMX swap. Does NOT handle spin or battle — those have dedicated handlers.

# Interface

- `addOptionHandler` — POST `/wheel/{id}/option`; text + optional weight; returns wheel fragment
- `deleteOptionHandler` — DELETE `/wheel/{id}/option/{idx}`; returns wheel fragment

# Dependencies

- Uses [session-store](/modules/session-store.md) for state access
- Uses [view-models](/modules/view-models.md) for rendering
- Uses [template-system](/services/template-system.md) for fragment execution
- Validates wheel ID via `parseWheelID`

# Invariants

- Always return a complete wheel fragment (HTMX swap target)
- Weight parsing: empty string → nil (equal share), numeric → float64

# Pure/Effectful

Effectful. HTTP I/O, session mutation, template execution.

# Citations

- [handlers_wheel.go](handlers_wheel.go)
