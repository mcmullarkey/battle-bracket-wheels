---
type: Module
title: Session Store & Concurrency Model
description: In-memory session store with RWMutex — closure-based atomic View/Update access
resource: session.go
tags: [go, concurrency, session, §2-effectful-shell]
timestamp: 2026-06-24
pure: false
---

# Responsibility

In-memory session storage with thread-safe access. Each session holds 8 wheels + bracket state + resolved-match set. Does NOT persist to disk — lost on restart.

# Interface

- `Session{ID, CreatedAt, Wheels[8], Bracket, ResolvedMatches}` — per-user game state
- `Store` — `sync.RWMutex` + `map[string]*Session`
- `NewStore() *Store`
- `(*Store) View(id string, fn func(*Session))` — read under RLock; pointer must not escape
- `(*Store) Update(id string, fn func(*Session))` — write under Lock; pointer must not escape

# Dependencies

- Holds [wheel domain types](/data-models/wheel.md) and [bracket-state-machine](/modules/bracket-state-machine.md) state
- Created/managed by [session-middleware](/interfaces/session-middleware.md)
- Used by all [handlers](/interfaces/wheel-handlers.md)

# Invariants

- `View` uses RLock (concurrent reads); `Update` uses Lock (exclusive writes)
- Closure-based access prevents pointer escape — caller cannot retain `*Session` outside closure
- `ResolvedMatches` gates idempotency — prevents double-resolve of same battle
- Wheel index range 0–7 enforced by `parseWheelID`

# Pure/Effectful

Effectful. Mutex synchronization, map mutation. No disk persistence.

# Examples

```go
store.Update(sessionID, func(s *Session) {
    s.Wheels[0] = wheel.AddOption(s.Wheels[0], "option", nil)
})
```

# Citations

- [session.go](session.go)
