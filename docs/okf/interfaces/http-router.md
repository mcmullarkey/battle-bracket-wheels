---
type: Interface
title: HTTP Router & Route Table
description: Go 1.22+ pattern-based routing — canonical route list for the application
resource: main.go
tags: [go, http, routing, §5-function-discipline]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Define all HTTP routes using Go 1.22+ `http.ServeMux` pattern routing. Single source of truth for the API surface.

# Interface

| Method | Path | Handler |
|--------|------|---------|
| ANY | `/health` | `healthHandler` |
| GET | `/` | `homeHandler` (via sessionMiddleware) |
| POST | `/wheel/{id}/option` | `addOptionHandler` |
| DELETE | `/wheel/{id}/option/{idx}` | `deleteOptionHandler` |
| POST | `/wheel/{id}/spin` | `spinHandler` |
| POST | `/battle/{matchID}` | `battleHandler` |
| GET | `/static/*` | `http.FileServer(embed.FS)` |

`{id}` validated 0–7 by `parseWheelID`. `{matchID}` ∈ [match-id](/data-models/match-id.md) constants.

# Dependencies

- Wires [session-middleware](/interfaces/session-middleware.md) for `/`
- Dispatches to [wheel-handlers](/interfaces/wheel-handlers.md), [spin-handler](/interfaces/spin-handler.md), [battle-handler](/interfaces/battle-handler.md)
- Serves [static-assets](/services/static-assets.md)

# Invariants

- All routes except `/health` and `/static/*` go through session middleware
- `parseWheelID` rejects out-of-range wheel IDs

# Pure/Effectful

Effectful. HTTP handler construction.

# Citations

- [main.go:101 setupRouter](main.go)
