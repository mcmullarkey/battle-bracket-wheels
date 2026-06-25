---
type: Interface
title: Battle Handler Orchestration
description: Full battle orchestration under single write lock — spin both wheels, resolve, absorb, propagate bracket, return 5 OOB fragments
resource: handlers_battle.go
tags: [go, http, htmx, battle, §2-effectful-shell, §5-function-discipline]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Orchestrate a full battle: check dependencies → load both wheels → spin both → resolve battle → absorb loser's option → propagate bracket → mark resolved. Return 5 HTMX fragments (4 OOB + 1 main swap) + `HX-Trigger`.

# Interface

- `battleHandler` — POST `/battle/{matchID}`
- Response: 5 fragments — `matchResult` (OOB), `nextRoundSlot` (OOB), `movieResult` (OOB, Final only), `centerDisplay` (OOB), `disabledButton` (non-OOB, main swap)
- `HX-Trigger: {"spin-wheel": [resultA, resultB]}` — array of 2 spin results

# Dependencies

- Uses [wheel-spin-algorithm](/functions/wheel-spin-algorithm.md) for both spins
- Uses [battle-resolution](/modules/battle-resolution.md) for `ResolveBattle` + `AbsorbOption`
- Uses [bracket-state-machine](/modules/bracket-state-machine.md) for `ValidateDependencies` + `ApplyBattleResult`
- Uses [session-store](/modules/session-store.md) `Update` for atomic write
- Uses [htmx-oob-protocol](/interfaces/htmx-oob-protocol.md) for fragment structure

# Invariants

- Entire battle under single `Store.Update` write lock (atomicity)
- `ResolvedMatches` checked before resolve (idempotency)
- Non-OOB `disabledButton` fragment required — HTMX 2.x skips HX-Trigger without it
- `spin-wheel` trigger sends array of 2 results (battle) vs single object (solo spin)

# Pure/Effectful

Effectful. HTTP I/O, session mutation, template execution. All pure logic delegated to internal packages.

# Citations

- [handlers_battle.go:99 battleHandler](handlers_battle.go)
