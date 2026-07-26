---
<<<<<<< HEAD
feature: space-pointer
ac: 2
status: complete
---

# AC-2: Position battle pointer at winning wheel after reveal

## Acceptance Criteria
- [x] Server: HX-Trigger spin-wheel array has exactly one entry with "winner":true, matching battleResult.WinnerID
- [x] Client: pointer hidden during spin (t < 3700ms), visible after reveal (t > 3800ms)
- [x] Client: pointer center within ±10px of winner slot center (slot ID from trigger data, NOT scraped from HTML)
- [x] Solo-spin guard: pointer never visible on solo spin
- [x] Round-reset guard: pointer hides at start of new battle before new reveal timeout

## Progress
- [2026-06-25 12:33] Read spec, codebase conventions, and existing tests
- [2026-06-25 12:33] Verified test `TestBattleHandler_HXTrigger_WinnerInTrigger` exists and passes
- [2026-06-25 12:33] Verified server code has `"winner"` field in trigger entries
- [2026-06-25 12:33] Verified client JS has `hidePointer()`, `positionPointerAtSlot()`, `revealPointerWithResults()`
- [2026-06-25 12:33] Verified `window.__lastSpinItems` is set for rodney probes
- [2026-06-25 12:33] Verified `handlers_spin.go` NOT modified (solo-spin guard)
- [2026-06-25 12:33] `go test -race ./...` — all tests pass
- [2026-06-25 12:33] E2E evidence — curl probe confirms HX-Trigger has winner:true
- [2026-06-25 12:33] Evidence committed to `docs/evidence/34/`
- [2026-06-25 12:33] All ACs complete — committing

## Decision Log
- **`winner` field added to trigger entries**: Both entries get `"winner": true/false` based on `battleResult.WinnerID == whA.ID` / `whB.ID`. This is explicit per entry (not implicit absent=false) for clarity.
- **`hidePointer()` uses `setTimeout(0)`**: HX-Trigger fires BEFORE OOB fragment swap completes, so `#battle-pointer` doesn't exist yet in DOM. `setTimeout(0)` defers execution until after the swap.
- **Positioning uses `position: fixed`**: Fixed positioning breaks out of `.match-result` flow, placing pointer relative to viewport (where slots live).
- **No `SpinTrigger` struct introduced**: The trigger remains `map[string]interface{}` to avoid changing the marshal boundary. `winner:bool` is added as a key-value pair.

## Surprises & Discoveries
- HX-Trigger fires before OOB swap completes — without `setTimeout(0)`, `document.getElementById("battle-pointer")` returns null on first battle. The `setTimeout(0)` approach was validated by the AC-1 PR review cycle.
- `handlers_battle_test.go` already had the test written and server code had the `winner` field — the worktree was partially prepped before being dispatched to builder. All changes were uncommitted.

## Self-Review
- [x] All ACs exercised by tests
- [x] Predicates match spec
- [x] Negative case tested (solo-spin guard, round-reset guard, no winner on solo)
- [x] Verification medium satisfied (code done, rodney is builder-vision's job)
- [x] Design intent respected (§1-5)
- [x] No scope creep
- [x] `handlers_spin.go` NOT modified
- [x] `window.__lastSpinItems` set for rodney probes
=======
ac: 2
depends_on: AC-1
risk: medium
status: spec
---

## AC spec: Position pointer at winning wheel after battle reveal, hide between rounds

### Executable Spec
- **predicate:**
  1. **Server contract:** Battle `POST /battle/{matchID}` response `HX-Trigger` JSON `spin-wheel` is an array of exactly 2 objects; exactly one entry has `"winner": true`; that entry's `wheelID` equals `battleResult.WinnerID`.
  2. **Client timing (hidden during spin):** For any `t < REVEAL_DELAY_MS` (3700ms) after `spin-wheel` event dispatch, `#battle-pointer` has `getComputedStyle().display === 'none'` OR `getComputedStyle().opacity === '0'`.
  3. **Client visibility (shown after reveal):** For `t > 3800ms` after dispatch, `#battle-pointer` has `getComputedStyle().display !== 'none'` AND `getComputedStyle().opacity > 0.1` AND `getBoundingClientRect().width > 0`.
  4. **Client positioning:** `#battle-pointer` center (x,y) is within ±10px of the winner `.slot` container's center — winner slot ID read from the trigger-data array entry where `winner === true` (NOT scraped from `matchResult` HTML).
  5. **Solo-spin guard:** For solo spin (`items.length === 1`), `#battle-pointer` never becomes visible within 10s.
  6. **Round-reset guard:** When a second battle `spin-wheel` event fires after a prior battle completed, `#battle-pointer` returns to hidden state BEFORE the new reveal timeout fires (hide at START of event handler).
- **probe:**
  ```
  # Server contract — append to handlers_battle_test.go (after line 1367):
  go test -race -run 'TestBattleHandler_HXTrigger_WinnerInTrigger' ./...

  # Client — rodney assert, two timed runs + solo guard (→ docs/evidence/<issue>/rodney.log):
  # Run A (hidden during spin, probe at t=2000ms):
  rodney assert --url http://localhost:8080 --step battle-qf1 --at 2000ms \
    --check 'const p=document.querySelector("#battle-pointer");const cs=getComputedStyle(p);cs.display==="none"||cs.opacity==="0"'

  # Run B (visible + positioned at winner, probe at t=4000ms):
  rodney assert --url http://localhost:8080 --step battle-qf1 --at 4000ms \
    --check 'const p=document.querySelector("#battle-pointer");const cs=getComputedStyle(p);const r=p.getBoundingClientRect();const items=window.__lastSpinItems;const w=items.find(i=>i.winner===true);const slot=document.getElementById(w.slotID);const sr=slot.getBoundingClientRect();cs.display!=="none"&&parseFloat(cs.opacity)>0.1&&r.width>0&&Math.abs((r.left+r.width/2)-(sr.left+sr.width/2))<=10&&Math.abs((r.top+r.height/2)-(sr.top+sr.height/2))<=10'

  # Solo-spin guard (probe over 10s window, solo wheel click):
  rodney assert --url http://localhost:8080 --step solo-spin --watch 10000ms \
    --check 'getComputedStyle(document.querySelector("#battle-pointer")).display==="none"||getComputedStyle(document.querySelector("#battle-pointer")).opacity==="0"'
  ```
- **negative:**
  - No `"winner"` field on either `spin-wheel` trigger object (or `winner:true` on both) → must fail server contract.
  - Pointer visible before 3700ms (flash-early sneaky-pass: show on event, hide before reveal, passes t=2000 and t=4000 snapshots but flashed during spin) → must fail timing + round-reset guard.
  - Pointer center >10px from winner slot center (e.g. points at loser, or sits between two wheels) → must fail positioning.
  - Pointer stays visible when a second battle begins → must fail round-reset guard.
  - Solo spin activates pointer logic → must fail solo-spin guard.
  - Client re-derives winner by scraping `matchResult` HTML instead of reading `winner` flag from trigger → design-intent violation (§3.2).
- **verification:** visual · rodney JS probes (client timing/positioning/visibility) + Go test (server trigger payload); subjective residual → builder-vision
- **fixture status:** `handlers_battle_test.go:1367` (add `TestBattleHandler_HXTrigger_WinnerInTrigger`) | `handlers_battle.go:260-275` (modify trigger — add `"winner": true/false` per entry) | `static/js/wheel.js:69` (modify `revealResults`) + `:88-112` (modify event handler) | NEW `docs/evidence/<issue>/rodney.log`
- **rubric anchor:** §1.5 (predicate tolerance — ±10px not ±100px, rejects too-loose impl that points "near" winner), §2.1 (winner decision pure in Go / DOM positioning effectful in client), §3.2 (trigger JSON is the seam; server single source of truth, client must not scrape matchResult)

#### ui:
- **selectors:** `#battle-pointer`, `[id^="slot-"]`, `.slot`, `.winner-text`, `.pending-reveal`, `.revealed`
- **layout_assertions:**
  - `display !== 'none'` AND `opacity > 0.1` AND `getBoundingClientRect().width > 0` at t>3800ms
  - `display === 'none'` OR `opacity === '0'` at t<3700ms
  - pointer center within ±10px of winner slot center (x and y)
  - pointer within viewport bounds
  - NOT visible during solo spin (10s window)
  - hidden before second battle's reveal timeout fires
- **viewports:** mobile (≤640px), tablet (≤1024px), desktop (>1024px)
- **deterministic_check:** rodney assert with computed-style + bounding-rect + winner-slot proximity checks (NOT selector existence alone)
- **subjective_residual:** pointer shape/glow, arrow orientation, space-theme harmony, animation feel — builder-vision (owned by AC-1 for element/CSS; this AC owns the positioning logic)

### Design Intent
- **Types / interfaces (§1):** Trigger seam currently `[]map[string]interface{}` (untyped — §3.4.1 signal). Add `"winner": bool` field per entry. Recommended: introduce `SpinTrigger` struct (`WheelID`, `SlotID`, `TargetIndex`, `TargetAngle`, `Winner bool`) to pin the seam contract; if deferred, grep-anchored TODO naming this slice as cutover.
- **Pure / effectful (§2):** Winner determination pure in Go (`battleResult.WinnerID` already computed). Pointer positioning effectful in client. JS separates pure "which slot is winner?" (read `winner` flag from items) from DOM-mutating `positionPointerAtSlot()` / `revealPointerWithResults()`.
- **Boundary cuts (§3):** Server/client boundary = `spin-wheel` trigger JSON. Server emits winner metadata (single source of truth); client consumes — never re-derives by scraping `matchResult` HTML.
- **Module responsibility (§4):** `handlers_battle.go` owns trigger payload (adds `winner`); `static/js/wheel.js` owns pointer positioning/timing; templates/CSS own element + space theme (AC-1).
- **Function discipline (§5):** Split into `hidePointer()`, `positionPointerAtSlot(slotID)`, `revealPointerWithResults()`. Hide call at START of `spin-wheel` handler (round-reset guard). Position+reveal inside `revealResults()` after `.revealed` toggle.

### Technical Context
- **Server change — `handlers_battle.go:258–275`:** Add `"winner": true` to entry whose `whA.ID == battleResult.WinnerID` (index 0) or `whB.ID == battleResult.WinnerID` (index 1); other entry gets `"winner": false`. Solo spin handler (`handlers_spin.go:68`) must NOT add `winner` (sends single object, unchanged).
- **Client change — `static/js/wheel.js:88–112`:** At top of `spin-wheel` handler, call `hidePointer()`. After `scheduleReveal()` (battles only, `items.length > 1`), store winner's `slotID` from `items[i].slotID` where `items[i].winner === true`. In `revealResults()` (line 69), after toggling `.revealed`, compute winner slot's `getBoundingClientRect()`, position `#battle-pointer` center at slot center, make visible.
- **Template/CSS:** from AC-1 (`#battle-pointer` element + space.css).
- **Slot-ID mapping:** trigger already carries `slotID` per entry (handlers_battle.go:264,270) — reuse it; no new mapping needed.
- **Architecture notes:** Battle trigger already normalized to array when `items.length > 1` (wheel.js:100). Hook pointer reveal into existing `revealResults()`; hide at start of battle spin. Solo path (`items.length === 1`) skips `scheduleReveal()` and must skip pointer logic entirely.

### Dependencies
- **Depends on:** AC-1 (pointer element + CSS)
- **Blocks:** none (terminal UI AC for pointer)
- **Conflict set:** `handlers_battle.go` (trigger payload), `static/js/wheel.js` (positioning logic), `handlers_battle_test.go` (trigger test); `handlers_spin.go` must NOT change (solo guard)
- **Risk level:** medium (timing-dependent rodney probes + producer-shape change on `HX-Trigger`)

### Pattern Detectors
- **Rodney-feasibility:** ⚠ Timing-dependent predicates. Use generous tolerance — probe at t=4000ms (past 3800ms deadline) and t=2000ms (before 3700ms). Two separate rodney runs + solo-spin watch window.
- **Bidirectional-contract:** ⚠ Server trigger shape (`winner` field) + client JS behavior must both be verified independently — Go test for payload, rodney for client.
- **Route-existence:** ✅ `/battle/{matchID}` exists (main.go:125); `qf1` valid (bracket.go:31).
- **Refusal-arm enumeration:** 4xx/5xx return before trigger construction (no pointer). Solo spin sends single-object with no `winner` field. Already-resolved (409) returns before trigger. Concurrency: pointer hide must happen at START of new event handler.
- **Producer-shape change:** ⚠ `HX-Trigger` payload shape changes (adds `winner`). Propagation: producer `handlers_battle.go`, consumer `wheel.js`; solo spin (`handlers_spin.go`) must NOT change; existing trigger tests must update to expect `winner` field.
- **UI sneaky-pass:** ⚠ Selector-existence insufficient. Must check `display`, `opacity`, `boundingRect.width`, viewport bounds, AND winner-slot proximity (±10px, not just "a slot exists"). Flash-early-then-hide caught only by round-reset guard (hide at handler start), not by post-reveal snapshot alone.

### Progress
- [ ] Red: write TestBattleHandler_HXTrigger_WinnerInTrigger, confirm fails — <timestamp>
- [ ] Add "winner": true/false to spin-wheel trigger array in handlers_battle.go
- [ ] Add hidePointer() / positionPointerAtSlot() / revealPointerWithResults() to wheel.js
- [ ] Hook into spin-wheel event handler + revealResults()
- [ ] Green: Go test passes
- [ ] Rodney assert: timing + positioning + solo guard
- [ ] Commit

### Decision Log
- <date> — resolver merged A (minimal predicate) + B (adversarial timing/sneaky-pass guards): ±10px tolerance adopted (tighter than B's ±100px); B's concrete probe/fixtures adopted over A's prose

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run `go test -race -run 'TestBattleHandler_HXTrigger_WinnerInTrigger' ./...`
- Rollback: revert handlers_battle.go trigger change + wheel.js pointer logic
>>>>>>> 3c58487 (style: gofmt internal/battle/battle.go internal/bracket/bracket_test.go)
