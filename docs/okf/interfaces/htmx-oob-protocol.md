---
type: Interface
title: HTMX OOB Fragment Protocol
description: Out-of-band swap protocol for battle results — 4 OOB + 1 main swap, pending-reveal class for animation sync
resource: templates/bracket.html
tags: [htmx, oob, protocol, §3-cut-at-joints]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Define the HTMX 2.x out-of-band swap protocol for battle responses. Coordinate server fragments with client animation timing via `pending-reveal` class.

# Interface

- OOB fragments: `matchResult`, `nextRoundSlot`, `movieResult` (Final only), `centerDisplay`
- Main swap: `disabledButton` (non-OOB — required for HX-Trigger processing)
- `HX-Trigger: {"spin-wheel": ...}` — single object (solo spin) or array (battle)
- `pending-reveal` class hides results during 3.5s spin animation
- JS reveals after `REVEAL_DELAY_MS=3700` → `.revealed` class

# Dependencies

- Produced by [battle-handler](/interfaces/battle-handler.md)
- Consumed by [client-animation](/presentation/client-animation.md) via `spin-wheel` event
- Templates: [template-system](/services/template-system.md) `match.html`, `bracket.html`

# Invariants

- HTMX 2.x requires at least one non-OOB swap target to process HX-Trigger
- `pending-reveal` → `revealed` transition timed to match spin animation duration
- Battle sends array of 2 spin results; solo spin sends single object

# Pure/Effectful

Effectful (protocol definition over HTTP responses). Template rendering.

# Citations

- [templates/bracket.html](templates/bracket.html)
- [templates/match.html](templates/match.html)
- [handlers_battle.go](handlers_battle.go)
