# Feature: Space-Themed Battle Pointer

## Goal
Add a space-themed pointer that visually points at the wheel that won a battle after the result is visible. Currently the winner is indicated only by 0-indexing — users have to understand 0-indexing to know which wheel won. The pointer eliminates this ambiguity.

## AC Dependency Order
- AC-1 (no deps) → AC-2 (depends on AC-1)

## ACs

### AC-1: Add space-themed battle pointer to match template + CSS (visual only, no logic)

---
ac: 1
depends_on: none
risk: low
status: spec
---

## AC spec: Add space-themed battle pointer to match template + CSS (visual only, no logic)

### Executable Spec
- **predicate:** given a session with options in wheels `0` and `1`, when `POST /battle/qf1`, then (1) the response body contains a `<div class="battle-pointer">` element inside the `matchResult` OOB fragment (swapped into `#match-qf1`), rendered as an arrow/indicator via inline `<svg>` OR CSS-only shape (`clip-path`/border-triangle) with **no** `url()` image; AND (2) `static/css/space.css` contains a `.battle-pointer` rule block (or its `::before`/`::after`) that: (a) references ≥2 space-theme custom properties from `{--neon-*, --cosmic-*, --star-*, --nebula-*}`, (b) does NOT set `display:none`, `visibility:hidden`, or `opacity:0`, (c) has visual substance — at least one of `border.*solid`, `clip-path`, non-empty `content` with non-`transparent` color, `filter:drop-shadow` with a space-theme color, or `box-shadow` with a neon color, (d) is NOT hidden by `@media (max-width: 640px)`; AND (3) the element has non-zero browser-computed dimensions and is within viewport bounds (rodney).
- **probe:**
  ```bash
  go test -race -run TestBattlePointer_SpaceTheme ./...
  ```
- **negative:** (a) response lacks `.battle-pointer` element; (b) CSS lacks `.battle-pointer` rule; (c) pointer uses `<img>` or `url()` image; (d) pointer includes JS positioning logic in this AC; (e) empty `<div class="battle-pointer"></div>` with `.battle-pointer { }` — no visual properties, zero dimensions (selector-presence probe passes, user sees nothing); (f) `.battle-pointer { display: none; }` — element in DOM but hidden; (g) `.battle-pointer { color: var(--neon-cyan); font-size: 0; }` — styled but invisible; (h) only 1 theme token used (weak theming sneaky-pass); (i) `@media (max-width: 640px) { .battle-pointer { display: none; } }` — hidden on mobile.
- **verification:** `code` (Go test reads embedded CSS + battle response HTML) + `rodney` (browser computed-style assertion for zero-size/off-screen/display:none guard)
- **fixture status:** existing `handlers_battle_test.go:30` `battleTestServer` + `addOptionToWheel` helper (line 68) + `extractCSSRule` helper (`main_test.go:335`); NEW test function `TestBattlePointer_SpaceTheme`
- **rubric anchor:** §1.5 (HTML class name contract), §3.1 (boundary cuts — template/CSS/JS seams), §5.1 (function discipline — single visual element, no branching)

### UI Block
- **selectors:** `.battle-pointer` (specific enough to not match `.match-winner`, `.match-result`, or any existing element)
- **layout_assertions:**
  ```js
  document.querySelector('.battle-pointer') !== null
  getComputedStyle(el).display !== 'none'
  getComputedStyle(el).visibility !== 'hidden'
  parseFloat(getComputedStyle(el).opacity) > 0
  const rect = el.getBoundingClientRect();
  rect.width > 0 && rect.height > 0
  rect.bottom >= 0 && rect.top <= window.innerHeight
  rect.right >= 0 && rect.left <= window.innerWidth
  ```
- **viewports:** desktop 1025px+, tablet 641–1024px, mobile ≤640px (MUST remain visible — existing `@media (max-width: 640px)` at `space.css:796` must not hide it)
- **deterministic_check:**
  ```bash
  uvx rodney assert --url http://localhost:8080 \
    --script "const el = document.querySelector('.battle-pointer');
              if (!el) throw new Error('pointer element not found');
              const cs = getComputedStyle(el);
              if (cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) === 0)
                throw new Error('pointer is invisible');
              const rect = el.getBoundingClientRect();
              if (rect.width === 0 || rect.height === 0)
                throw new Error('pointer has zero dimensions');
              if (rect.bottom < 0 || rect.top > window.innerHeight || rect.right < 0 || rect.left > window.innerWidth)
                throw new Error('pointer is off-screen');
              const color = cs.color || cs.borderBottomColor || cs.backgroundColor;
              if (!color) throw new Error('pointer has no color')"
  ```
- **subjective_residual:** arrow shape feel is space-themed; glow intensity matches existing neon palette; arrow color complements the neon/cosmic theme

### Design Intent
- **Types / interfaces (§1):** surface contract is HTML class name `.battle-pointer`; no new Go types needed for a decorative element.
- **Pure / effectful (§2):** CSS is pure declarations; template rendering stays the only effect, handler logic untouched. No JS, no HTMX attributes on the pointer.
- **Boundary cuts (§3):** template owns markup, CSS owns theme, future JS owns positioning — three seams, no mixing. Representation change in one file doesn't force the other to change.
- **Module responsibility (§4):** `templates/match.html` grows the pointer inside `matchResult` (within `.pending-reveal` wrapper so it inherits reveal timing — appears after spin animation at 3700ms); `static/css/space.css` grows its space-themed styling. CSS naming `.battle-pointer` aligns with existing `.match-result`, `.match-winner`, `.cosmic-panel`, `.neon-button`.
- **Function discipline (§5):** single visual element, no branching or state mutation.

### Technical Context
- **Files likely touched:**
  - `templates/match.html:1` — `matchResult` fragment (add `.battle-pointer` element inside `.pending-reveal` wrapper at line 2)
  - `static/css/space.css:1` — add `.battle-pointer` rule with ≥2 theme tokens; ensure not hidden by `@media (max-width: 640px)` at line 796
  - `handlers_battle_test.go:30` — extend fixture usage with new test `TestBattlePointer_SpaceTheme`
  - `handlers_battle.go:294` — `matchResult` template execution (unchanged, confirms where pointer renders)
  - `main.go:24-28` — embeds templates/static; no edits needed
- **Architecture notes:** `matchResult` is an OOB fragment swapped into `#match-qf1`; it carries `pending-reveal` and is revealed after spin animation (JS swaps `pending-reveal` → `revealed` at 3700ms). Pointer belongs inside this fragment so it appears only after a battle result exists. Use existing `:root` neon/cosmic/star/nebula tokens and CSS-only shapes (SVG polygon/path, `clip-path`, or border-triangle), never image URLs, per space-theme invariant. Follow existing `extractCSSRule` helper (`main_test.go:335`) for CSS rule verification in the Go test, mirroring `TestSpaceThemeIntegration` pattern.

### Dependencies
- **Depends on:** none (first slice, no prerequisites)
- **Blocks:** AC that positions pointer at winning wheel via JS (rotate/translate toward winner slot)
- **Conflict set:** `templates/match.html`, `static/css/space.css` — overlap with later battle-visual ACs
- **Risk level:** low

### Progress
- [ ] Red: write TestBattlePointer_SpaceTheme, confirm fails — <timestamp>
- [ ] Add .battle-pointer element to templates/match.html inside .pending-reveal wrapper
- [ ] Add .battle-pointer CSS rule to static/css/space.css with ≥2 theme tokens
- [ ] Green: test passes
- [ ] Rodney assert: computed-style + bounding-rect guard
- [ ] Commit

### Decision Log
- <date> — resolver merged A (clean probe) + B (adversarial visibility guards): B's rodney script relocated to ui: deterministic_check

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run `go test -race -run TestBattlePointer_SpaceTheme ./...`
- Rollback: revert templates/match.html + static/css/space.css changes

---

### AC-2: Position pointer at winning wheel after battle reveal, hide between rounds

---
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

## Open Questions
- None — both resolvers reported minor disagreements, all resolved.

## Implementation Schedule
- Batch 1: AC-1 (no dependencies)
- Batch 2: AC-2 (depends on AC-1)
