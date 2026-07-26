# Decomposition: battle-bracket-extensions

## Feature Goal

Three user requests: (1) **BUG** — the wheel on the right side of the screen appears to always win, killing the fun; (2) **THEME TOGGLE** — a dropdown UI to switch between the existing space theme and a new Y2K theme, architected so new themes can be added monthly; (3) **AUTO-POPULATE** — paste a flexible flat list (from Google Docs etc.) and have the app distribute entries evenly across all 8 wheels, replacing manual per-option entry.

### Bug investigation findings (verified by direct source read, not just explore-agent report)

- `internal/battle/battle.go` `ResolveBattle` is **symmetric and fair**: `rollA > rollB` → A wins, `rollB > rollA` → B wins, tie re-rolls to `maxTies`. No positional bias. Existing test `TestResolveBattle_TiebreakerNoBias` passes.
- `handlers_battle.go` maps `indices[0]`=left, `indices[1]`=right correctly for QF; SF/Final load from bracket pointers symmetrically. Bracket progression (`ApplyBattleResult`, `SlotMapping`) verified symmetric.
- **Director context correction:** the battle `HX-Trigger` payload on main contains **no `winner` flag** — only `{wheelID, slotID, targetIndex, targetAngle}` per item. The `winner`-flag JS described in the context lives on the **unmerged branch** `34-position-battle-pointer-at-winner` (issue #34 is OPEN).
- **Prime suspect:** `templates/match.html` renders `.battle-pointer` as a **static inline SVG polygon `points="2,2 22,12 2,22"` — a right-pointing triangle that never moves**. No JS on main positions it (main's `wheel.js` has zero pointer code). After every battle, a static right-facing arrow displays beside the result → user perceives "right wheel always wins."
- Speculator for AC-1 MUST confirm root cause by reproduction (rodney) before speccing the fix. If the pointer is the cause, the fix overlaps with open issue #34 — builder may salvage branch work (note: memory flags the branch's `hidePointer` used `getElementById` first-match-only while `positionPointerAtSlot` pruned duplicates to `document.body`; hide must use `querySelectorAll` over all instances).
- Working tree is **dirty on main** (uncommitted gofmt-only changes to `internal/battle/battle.go`, `internal/bracket/bracket_test.go`, `AGENTS.md`). Director should commit or stash before batch build.

## AC Table

| AC | Description | Dependencies | Conflict Set | Risk |
|----|-------------|--------------|--------------|------|
| 1 | Fix right-wheel-always-wins bug: root-cause via reproduction, then make the battle result display (pointer + any winner indicator) reflect the actual `ResolveBattle` winner, adding winner data to the trigger payload and positioning JS as needed | none | `handlers_battle.go`, `templates/match.html`, `static/js/wheel.js`, `handlers_battle_test.go`, possibly `static/css/space.css` | high |
| 2 | Implement theme infrastructure: theme registry, `POST /theme` endpoint setting a theme cookie, server-rendered stylesheet link selection in layout, and a dropdown theme selector UI | none | `main.go` (setupRouter), `templates/layout.html`, new `handlers_theme.go`, new `static/js/theme.js` (if client enhancement), `static/css/space.css` (dropdown styles) | medium |
| 3 | Add Y2K theme stylesheet (`static/css/y2k.css`) covering the full `:root` token surface + component overrides, registered in the theme registry | AC-2 | new `static/css/y2k.css`, theme registry location from AC-2 (`handlers_theme.go` or `internal/theme`), `docs/evidence/` screenshots | medium |
| 4 | Implement pure list-parsing + even-distribution logic (`internal/populate`): tolerant parsing of Google-Docs-style input (newlines, commas, bullets, numbering, blank lines, whitespace) and even split across 8 wheels | none | new `internal/populate/populate.go`, new `internal/populate/populate_test.go` | low |
| 5 | Implement `POST /wheels/populate` endpoint: validate parsed list, apply distribution to `Session.Wheels` under a single `Store.Update` write lock, return re-rendered wheel fragments | AC-4 | `main.go` (setupRouter), new `handlers_populate.go`, new `handlers_populate_test.go`, `session.go` (read-only usage expected) | medium |
| 6 | Add auto-populate UI: paste textarea + submit button in layout, HTMX-driven re-render of all 8 QF wheel slots on success, user-facing error surface for invalid input | AC-5 | `templates/layout.html`, `templates/bracket.html` or OOB fragment strategy, `static/css/space.css` (form styles), possibly `static/js/wheel.js` | medium |

## Dependency DAG

```
AC-1 (bug fix — isolated chain)
AC-2 → AC-3 (theme chain)
AC-4 → AC-5 → AC-6 (populate chain)
```

Cross-chain file conflicts (not dependencies): AC-2 and AC-5 both touch `main.go`; AC-2 and AC-6 both touch `templates/layout.html`. These chains must not land simultaneously — see batch schedule.

## Hot Conflict Files

- `main.go` (setupRouter): touched by AC-2 (`/theme` route), AC-5 (`/wheels/populate` route) — serialize; second builder rebases on merged first
- `templates/layout.html`: touched by AC-2 (theme dropdown + stylesheet link), AC-6 (populate UI) — serialize across batches; AC-6 lands last
- `static/css/space.css`: potentially touched by AC-1 (pointer styles — already present from PR #35, may need adjustment), AC-2 (dropdown styles), AC-6 (form styles) — serialize; prefer new scoped selectors over edits to shared rules
- `handlers_battle_test.go`: AC-1 only (no conflict, listed for awareness)

Mitigation convention: new handlers go in NEW files (`handlers_theme.go`, `handlers_populate.go`) — never append to existing `handlers.go`/`handlers_wheel.go`. New CSS selectors scoped (`.theme-selector`, `.populate-form`) — no edits to existing rules except where AC-1 requires.

## Suggested Batch Schedule

- **Batch 1 (parallel):** AC-1, AC-2, AC-4
  - AC-1: bug chain, isolated files (battle handler/template/JS/tests)
  - AC-2: theme infra (`main.go`, `layout.html` — first claim on both hot files)
  - AC-4: pure logic, new package, zero conflicts
- **Batch 2 (parallel):** AC-3 (needs AC-2 merged), AC-5 (needs AC-4 merged; rebases `main.go` route registration onto post-AC-2 main)
- **Batch 3 (sequential):** AC-6 (needs AC-5 merged; rebases `layout.html` onto post-AC-2 main)

## Open Questions

- [needs-clarification] **Auto-populate replace vs append:** should pasting a list REPLACE all existing options on all 8 wheels, or append to them? Decomposer assumption: **replace** ("create the wheels"), and bracket state must be reset/guarded — populating after battles have resolved should either be rejected or reset the bracket (resolved matches reference absorbed options that would vanish). Recommend: populate clears all wheel options AND resets bracket/ResolvedMatches. Confirm.
- [needs-clarification] **Distribution algorithm:** "evenly" across 8 wheels — round-robin (entry i → wheel i mod 8, maximizes spread) or contiguous chunks (first N/8 → wheel 0, etc., preserves source grouping)? Decomposer assumption: **round-robin**. Confirm.
- [needs-clarification] **Minimum list size:** reject lists with fewer than 8 entries (one per wheel), or allow smaller lists (some wheels empty)? Recommend: reject < 8 with a clear error message. Confirm.
- [needs-clarification] **Bug scope confirmation:** speculator must verify via rodney reproduction that the perceived "right always wins" is the static unpositioned `.battle-pointer` (right-pointing triangle in `match.html`, never moved by JS) and not an outcome-distribution bias. Server-side resolution verified fair by direct source read + existing `TestResolveBattle_TiebreakerNoBias`. If reproduction shows actual outcome bias, re-scope AC-1 before spec. Note overlap with open issue #34 — fixing AC-1 likely closes #34.
- [needs-clarification] **Theme persistence:** cookie-based persistence (new cookie, e.g. `bbw_theme`, `SameSite=Lax`, no `Secure` — matching `bbw_session` conventions) assumed. Theme choice is per-browser, not tied to session ID. Confirm acceptable.
- **Non-question (decided by convention):** theme implementation = separate stylesheet per theme + server-rendered `<link>` selection (scales for monthly additions, no CSS variable runtime switching complexity). Speculator may propose `data-theme` token-override alternative if simpler; resolver picks.

## Notes for Speculators

- **AC-1:** verification medium = `rodney` (client-side pointer position) + `code` (trigger payload winner flag, template winner classes). If pointer fix included: hide/show symmetry rule from memory — any code moving DOM elements out-of-flow and pruning duplicates at REVEAL must use `querySelectorAll` and iterate ALL instances at HIDE. Never invent rodney CLI options (`--step`, `--at`, `--watch` do not exist); use sequential commands + `sleep` for timed probes.
- **AC-2:** follow existing conventions — `SameSite=Lax`, no `Secure` cookie; session middleware pattern; HTMX 2.x requires a non-OOB swap target if any HX-Trigger is used (prefer plain full-form POST + redirect or hx-boost for theme switch to avoid OOB complexity).
- **AC-4:** pure package, no I/O — matches `internal/battle`/`internal/bracket` pattern. Property tests for distribution evenness (|count diff| ≤ 1 across wheels) and parser tolerance table tests.
- **AC-5:** entire mutation under single `Store.Update` write lock (atomicity convention). Idempotency not required (re-population is legal if it resets state — see open question 1).
- **AC-6:** HTMX OOB protocol — re-rendering 8 wheel slots needs 8 OOB fragments + 1 non-OOB main swap (see AGENTS.md HTMX 2.x protocol note; battle handler is the reference implementation).
