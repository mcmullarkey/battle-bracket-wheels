---
type: Module
title: Application Architecture & Layering
description: Package main as effectful shell over pure internal packages — HTTP wiring, session lifecycle, view-model construction
resource: main.go
tags: [go, architecture, §2-pure-effectful, §3-cut-at-joints]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Entry point and effectful shell. Wires HTTP routes, manages session lifecycle, constructs view-models, executes templates. Does NOT contain domain logic — delegates to pure `internal/*` packages.

# Interface

- `main()` — parse embedded templates, build `*Store`, call `setupRouter`, `http.ListenAndServe`
- `setupRouter(*Store, *template.Template) http.Handler` — route table (canonical route list)
- `newSpinSource() rand.Source` — crypto/rand→math/rand seed; pid xor fallback

# Dependencies

- Imports [wheel domain types](/data-models/wheel.md), [battle-resolution](/modules/battle-resolution.md), [bracket-state-machine](/modules/bracket-state-machine.md)
- Uses [session-store](/modules/session-store.md) for state
- Renders via [template-system](/services/template-system.md)
- Serves [static-assets](/services/static-assets.md)

# Invariants

- All `internal/*` packages are pure; `package main` is the only effectful layer (§2)
- Module boundaries reflect domain boundaries (§3): wheel, battle, bracket are separate packages
- `Renderer` interface decouples handlers from `html/template` (§3)

# Pure/Effectful

Effectful shell. HTTP I/O, cookies, in-memory store, crypto/rand seeding, template execution, embed.FS.

# Citations

- [main.go](main.go)
