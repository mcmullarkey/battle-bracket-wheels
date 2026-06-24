---
title: "AC-1: Go scaffold + Render Web Service + space-themed shell + session store"
status: "in-progress"
feature: "battle-bracket-wheels"
created: "2026-06-23"
---

## Acceptance Criteria

- [ ] `go build ./...` succeeds; `go.mod` module path == `battle-bracket-wheels`
- [ ] Server binds `0.0.0.0:$PORT` (PORT=9123 → /health on :9123 not :8080; PORT unset → defaults :8080, no panic)
- [ ] GET /health → 200 + JSON `{"status":"ok"}`
- [ ] GET / → 200 text/html with HTMX script include, starfield CSS (@keyframes or canvas), non-empty title + h1, bracket skeleton (.bracket div with 8 .slot divs id=slot-1..8)
- [ ] GET / sets Set-Cookie: bbw_session=value — HttpOnly, Path=/, SameSite=Lax, NOT Secure, >=32 hex chars (crypto/rand)
- [ ] Session round-trip: same cookie → same session; no cookie → new session + cookie set
- [ ] Concurrent: 100 goroutines sharing ONE Store → 100 unique IDs, zero races (go test -race)
- [ ] render.yaml: Go Web Service, startCommand: ./battle-bracket-wheels (NOT go run), PORT in envVars
- [ ] Static assets served via embed.FS (works in prod binary)

## Progress

- [ ] 2026-06-23: Initialized Go module, created branch
- [ ] 2026-06-23: Wrote test files (red phase)
- [ ] 2026-06-23: Implemented session.go
- [ ] 2026-06-23: Implemented handlers.go
- [ ] 2026-06-23: Created templates/layout.html
- [ ] 2026-06-23: Created static/css/space.css
- [ ] 2026-06-23: Implemented main.go
- [ ] 2026-06-23: Created render.yaml and .gitignore
- [ ] 2026-06-23: All tests pass with -race
- [ ] 2026-06-23: Committed, pushed, created PR

## Decision Log

- **Go 1.22+** for enhanced routing patterns (net/http.ServeMux patterns)
- **No template.Must** — use log.Fatalf on parse error as spec requires
- **bbw_session** cookie name, SameSite=Lax, NOT Secure (Render edge terminates TLS)
- **bind 0.0.0.0** not 127.0.0.1 for Render compatibility
- **embed.FS** for static assets, compiled into binary

## Surprises & Discoveries

- TBD
