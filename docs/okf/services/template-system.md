---
type: Service
title: Embedded Template System
description: Go html/template with embed.FS — Renderer interface decouples handlers from template execution
resource: main.go
tags: [go, template, embed, §3-cut-at-joints]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Parse and execute HTML templates embedded via `//go:embed`. Provide `Renderer` interface so handlers depend on abstraction, not `*template.Template` directly (§3).

# Interface

- `Renderer` interface — `RenderTemplate(w io.Writer, name string, data interface{}) error`
- Templates: `layout.html`, `wheel.html`, `match.html`, `bracket.html`
- Embedded via `//go:embed templates/*`
- No `template.Must` — errors handled gracefully

# Dependencies

- Consumes [view-models](/modules/view-models.md) for template data
- Used by all [handlers](/interfaces/wheel-handlers.md)
- Templates define [htmx-oob-protocol](/interfaces/htmx-oob-protocol.md) fragments

# Invariants

- `Renderer` interface decouples handlers from `html/template` (§3 — cut at joints)
- No `template.Must` — parse errors returned, not panicked
- Templates embedded at compile time (no filesystem dependency at runtime)

# Pure/Effectful

Effectful. Template parsing, I/O execution. But `Renderer` interface enables testability (mock renderer).

# Citations

- [main.go](main.go)
- [templates/](templates/)
