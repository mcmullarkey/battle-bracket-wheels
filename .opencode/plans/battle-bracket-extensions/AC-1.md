---
ac: 1
depends_on: none
risk: high
status: spec
---

## AC-1: Fix right-wheel-always-wins bug

### Executable Spec
- **predicate:**
  (1) HX-Trigger `spin-wheel` array contains exactly one item with `"winner": true`, and that item's `wheelID` equals the `matchResult` OOB fragment's `WinnerID`;
  (2) `templates/match.html` `.battle-pointer` is NOT a hardcoded static right-pointing-only element — its rendered direction is driven by the winner (left-winner → points left, right-winner → points right) via a class hook + CSS transform;
  (3) after reveal (≥ REVEAL_DELAY_MS=3700ms, post-OOB-swap), every `.battle-pointer` instance (querySelectorAll) has a computed `transform` correlating with the winner: left-winner → `matrix(-1, 0, 0, 1, 0, 0)` (i.e. `scaleX(-1)`), right-winner → `none`/identity;
  (4) pointer flip is applied at/after reveal time (inside `revealResults` or a fn it calls), NOT synchronously in the `spin-wheel` event handler (which fires before the `matchResult` OOB swap completes);
  (5) solo-spin (single-item `spin-wheel` trigger) sets NO `winner:true` and triggers NO pointer flip.
- **probe:** Go test (TestBattleHandler_HXTriggerBothWheels + TestBattlePointer_SpaceTheme + NEW TestBattlePointer_WinnerDirectionMatchesTrigger) + rodney assert (getComputedStyle transform + window.__lastSpinItems oracle)
- **negative:** Static right-pointing residual (template keeps hardcoded polygon); timing race (JS flips synchronously before OOB swap); zombie pointer (stale .battle-pointer, querySelector first-match); CSS-only fake (class added but no CSS rule); solo-spin erroneously sets winner
- **verification:** code + visual (rodney computed-style transform)
- **fixture status:** handlers_battle_test.go:230 (extend TestBattleHandler_HXTriggerBothWheels) + handlers_battle_test.go:1374 (extend TestBattlePointer_SpaceTheme) + NEW TestBattlePointer_WinnerDirectionMatchesTrigger
- **rubric anchor:** §3.2 (server emits winner through trigger payload) + §2.4 (handler emits winner data, not render positioning) + §5.1 (JS pointer-flip is one focused testable behavior) + §3.3 (cross-module: server→trigger→JS→DOM)

### Design Intent
- **Types / interfaces (§1):** `winner: true` bool per spin-wheel array item. Exactly-one invariant enforced by handler + Go test.
- **Pure / effectful (§2):** ResolveBattle stays pure (unchanged). Handler computes winner flag. JS applies CSS class at reveal. Template renders neutral SVG.
- **Boundary cuts (§3):** Winner data rides existing spin-wheel event (array value). Winner = server concern; pointer direction = client concern. HTMX event-flow constraint: spin-wheel handler detail.value = array only; winner MUST live inside array items.
- **Module responsibility (§4):** handlers_battle.go emits winner data; match.html renders neutral .battle-pointer + class hook; wheel.js flips direction at reveal via querySelectorAll; space.css defines .battle-pointer.pointer-left { transform: scaleX(-1) }.
- **Function discipline (§5):** Pointer flip = one focused fn called from revealResults; uses querySelectorAll (all instances); defers to existing reveal timer; exposes window.__lastSpinItems as test oracle.

### Technical Context
- **Files:** handlers_battle.go:260-275 (add winner to trigger items), match.html:20-24 (class hook on .battle-pointer), wheel.js:69-86 (flip in revealResults + __lastSpinItems oracle), space.css:693 (.pointer-left transform rule), handlers_battle_test.go:230+1374 (extend) + NEW test
- **Architecture notes:** HTMX event-flow constraint (load-bearing): spin-wheel event detail = array only; top-level winnerID invisible to handler. Timing: spin-wheel fires before OOB swap; pointer flip MUST defer to revealResults (3700ms). Directional not positional: .battle-pointer is a directional triangle; fix = flip via scaleX(-1), not move-to-slot geometry. Random winner: __lastSpinItems serves as oracle; getComputedStyle.transform verifies render.

### Dependencies
- **Depends on:** none
- **Blocks:** none directly
- **Conflict set:** handlers_battle.go, templates/match.html, static/js/wheel.js, static/css/space.css, handlers_battle_test.go

### UI Block
- **selectors:** .battle-pointer, .winner-text, .pending-reveal/.revealed, window.__lastSpinItems
- **deterministic_check:** rodney assert using getComputedStyle().transform (not just classList) + window.__lastSpinItems oracle for random winner. querySelectorAll for multi-element.
- **subjective_residual:** pointer visual polish (color, glow, arrow shape)

### Progress
- [ ] Red: write integration test, confirm fails
- [ ] ADR if new interface/boundary
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

### Decision Log
- 2026-07-26 — winner flag per-array-item (not top-level winnerID): HTMX event detail = array only; top-level field invisible to spin-wheel handler
- 2026-07-26 — CSS scaleX(-1) flip (not geometry positioning): .battle-pointer is directional triangle, not positional element
- 2026-07-26 — reveal-time timing (not setTimeout(0)): OOB swap guaranteed complete at 3700ms

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run go test -race ./... and rodney probes
- Rollback: revert handler trigger payload change + wheel.js pointer flip + CSS rule
