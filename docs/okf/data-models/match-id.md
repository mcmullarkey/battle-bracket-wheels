---
type: Data Model
title: MatchID Typed Constants
description: Typed string for bracket match positions — illegal match IDs unrepresentable
resource: internal/bracket/bracket.go
tags: [go, domain, bracket, §1-types]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Encode the 7 valid match positions as typed constants. Make invalid match IDs unrepresentable at the type level (§1).

# Interface

- `MatchID` — string type
- Constants: `MatchQF1`, `MatchQF2`, `MatchQF3`, `MatchQF4`, `MatchSF1`, `MatchSF2`, `MatchFinal`

# Dependencies

- Used by [bracket-state-machine](/modules/bracket-state-machine.md) for `ApplyBattleResult`, `ValidateDependencies`, `SlotMapping`
- Used by [http-router](/interfaces/http-router.md) for `/battle/{matchID}` route
- Used by [battle-handler](/interfaces/battle-handler.md) for orchestration

# Invariants

- Only 7 valid values; any other string → `ErrUnknownMatch`
- Positional mapping fixed: QF1→SFLeft[0], QF2→SFRight[0], QF3→SFLeft[1], QF4→SFRight[1], SF1→FinalLeft, SF2→FinalRight, Final→Winner

# Pure/Effectful

Pure. Type definition + constants only.

# Citations

- [internal/bracket/bracket.go](internal/bracket/bracket.go)
