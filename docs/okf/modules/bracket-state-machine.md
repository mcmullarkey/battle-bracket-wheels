---
type: Module
title: Bracket State Machine
description: Pure bracket progression — positional slots, dependency gating, idempotency, winner propagation
resource: internal/bracket/bracket.go
tags: [go, domain, bracket, pure, §1-types, §2-pure-core, §3-cut-at-joints]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Manage bracket progression: 8 QF slots → 2 SF → 1 Final. Enforce dependency ordering (can't play SF before QFs resolve), idempotency (can't re-resolve a match), and winner propagation (loser's option absorbed into winner's wheel, winner advances).

# Interface

- `Bracket{Slots[8], SFLeft[2], SFRight[2], FinalLeft, FinalRight, Winner *FinalResult}` — pointer fields nil = unfilled
- `New(wheels [8]Wheel) *Bracket`
- `(*Bracket) ApplyBattleResult(mid MatchID, result BattleResult, whA, whB Wheel) (Wheel, error)` — mutates in place, returns absorbed winner
- `(*Bracket) ValidateDependencies(mid MatchID) error`
- `SlotMapping(mid MatchID) string` — HTML slot ID for next round
- `FinalResult{Wheel, LandedOption}`
- Errors: `ErrDependencyNotMet`, `ErrAlreadyResolved`, `ErrUnknownMatch`

# Dependencies

- Imports [battle-resolution](/modules/battle-resolution.md) for `BattleResult`
- Imports [wheel domain types](/data-models/wheel.md) for `Wheel`
- Uses [match-id](/data-models/match-id.md) typed constants
- Consumed by [battle-handler](/interfaces/battle-handler.md) for progression

# Invariants

- Each pointer field maps to exactly one match position (§1 — illegal states unrepresentable)
- Filled slot cannot be re-filled → `ErrAlreadyResolved` (idempotency)
- QF dependency: slot has ≥1 option
- SF/Final dependency: both input pointers non-nil
- Positional mapping: QF1→SFLeft[0], QF2→SFRight[0], QF3→SFLeft[1], QF4→SFRight[1], SF1→FinalLeft, SF2→FinalRight, Final→Winner

# Pure/Effectful

Pure. No I/O, no randomness. Mutates receiver but deterministic given inputs (§2 — pure core, mutation is in-place state transition).

# Citations

- [internal/bracket/bracket.go](internal/bracket/bracket.go)
