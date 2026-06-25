---
type: Module
title: Battle Resolution & Tiebreaker
description: Pure match resolution — roll comparison, tiebreaker re-roll, loser-option absorption with text dedup
resource: internal/battle/battle.go
tags: [go, domain, battle, pure, §1-types, §2-pure-core]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Resolve a battle between two landed options: compare rolls, re-roll ties up to a cap, absorb loser's option into winner's wheel with text dedup. Does NOT perform HTTP, I/O, or randomness directly — randomness injected via `RollFunc`.

# Interface

- `RollFunc func() int` — injected randomness contract
- `BattleResult{WinnerID, LoserID, WinnerRoll, LoserRoll, WinnerLanded, LoserLanded, Ties}` — resolution outcome
- `ResolveBattle(landedA, landedB Option, idA, idB string, roll RollFunc, maxTies int) (BattleResult, error)`
- `AbsorbOption(winner Wheel, loserLanded Option) Wheel` — dedup by exact text, appended with nil weight
- `ErrTiebreakerExhausted`

# Dependencies

- Imports [wheel domain types](/data-models/wheel.md) for `Wheel`, `Option`
- Consumed by [bracket-state-machine](/modules/bracket-state-machine.md) via `ApplyBattleResult`
- Called by [battle-handler](/interfaces/battle-handler.md) with injected `RollFunc`

# Invariants

- Rolls outside [1,100] → error (§1 — illegal state prevented)
- Higher roll wins; ties re-roll up to `maxTies`; exhaustion → `ErrTiebreakerExhausted`
- Absorption dedups by exact text match — duplicate option not added twice
- Absorbed option gets nil weight (equal share in future spins)

# Pure/Effectful

Pure. No `math/rand` import — randomness injected via `RollFunc` (§2). All functions return new values.

# Citations

- [internal/battle/battle.go](internal/battle/battle.go)
