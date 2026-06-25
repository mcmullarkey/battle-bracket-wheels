---
type: Interface
title: Session Middleware & Cookie Lifecycle
description: Ensure session exists per request, refresh stale cookies, rewrite Cookie header so fresh ID wins
resource: handlers.go
tags: [go, http, session, middleware, §2-effectful-shell]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Wrap the home handler: ensure a session exists for every request, set/refresh the `bbw_session` cookie. Handle stale cookie IDs by creating new sessions and rewriting the Cookie header so the fresh ID wins.

# Interface

- `sessionMiddleware(store *Store, next http.Handler) http.Handler` — middleware wrapper
- Cookie: `bbw_session`, `SameSite=Lax` (NOT Secure — allows HTTP on Render free tier)

# Dependencies

- Uses [session-store](/modules/session-store.md) for session creation/lookup
- Wraps `homeHandler` in [http-router](/interfaces/http-router.md)

# Invariants

- Session always present after middleware runs
- Stale cookie ID → new session created, Cookie header rewritten
- `SameSite=Lax` chosen for Render free-tier HTTP compatibility

# Pure/Effectful

Effectful. Cookie I/O, session creation, header manipulation.

# Citations

- [handlers.go:126 sessionMiddleware](handlers.go)
