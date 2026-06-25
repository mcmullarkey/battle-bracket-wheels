---
type: Service
title: Embedded Static Assets
description: CSS, JS, HTMX served via embed.FS — no filesystem dependency at runtime
resource: main.go
tags: [go, embed, static, §2-effectful-shell]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Serve static assets (CSS, JS, HTMX library) from embedded filesystem. No external file dependencies at runtime.

# Interface

- `http.FileServer(embed.FS)` at `/static/*`
- Assets: `static/css/space.css`, `static/js/wheel.js`, `static/js/htmx.min.js`
- Embedded via `//go:embed static/*`

# Dependencies

- Serves [client-animation](/presentation/client-animation.md) JS
- Serves [space-theme](/presentation/space-theme.md) CSS
- Serves HTMX 2.x library

# Invariants

- All assets embedded at compile time
- No filesystem reads at runtime

# Pure/Effectful

Effectful. HTTP file serving from embed.FS.

# Citations

- [main.go](main.go)
- [static/](static/)
