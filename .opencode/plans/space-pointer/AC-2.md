---
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
