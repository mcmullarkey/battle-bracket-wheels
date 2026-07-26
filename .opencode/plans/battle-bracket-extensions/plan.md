# Master Plan: battle-bracket-extensions

## Feature Goal
Three user requests for the battle-bracket-wheels app:
1. BUG FIX: Right wheel always wins (static pointer never moves)
2. THEME TOGGLE: Dropdown selector for space/Y2K themes, extensible for monthly themes
3. AUTO-POPULATE: Paste flexible text list, auto-distribute evenly across 8 wheels

## User Clarifications
- All three go through full spec pipeline (no fast-track)
- Auto-populate: replace mode (clears wheels + resets bracket), round-robin distribution, reject < 8 entries
- Theme toggle: dropdown selector, cookie-based persistence (bbw_theme, SameSite=Lax)

## AC Table
| AC | Description | Deps | Risk | Status |
|----|-------------|------|------|--------|
| 1 | Fix right-wheel-always-wins bug | none | high | spec |
| 2 | Theme infrastructure (registry, POST /theme, cookie, dropdown) | none | medium | spec |
| 3 | Y2K theme stylesheet | AC-2 | medium | spec |
| 4 | Pure list-parsing + round-robin distribution | none | low | spec |
| 5 | POST /wheels/populate endpoint | AC-4 | medium | spec |
| 6 | Auto-populate UI (textarea + HTMX) | AC-5 | medium | spec |

## Dependency DAG
```
AC-1 (bug fix — isolated chain)
AC-2 → AC-3 (theme chain)
AC-4 → AC-5 → AC-6 (populate chain)
```

## Batch Schedule
- **Batch 1 (parallel):** AC-1, AC-2, AC-4 — independent, zero cross-chain conflicts
- **Batch 2 (parallel):** AC-3 (needs AC-2 merged), AC-5 (needs AC-4 merged, rebases main.go onto post-AC-2)
- **Batch 3 (sequential):** AC-6 (needs AC-5 merged, rebases layout.html onto post-AC-2)

## Hot Conflict Files
- main.go (setupRouter): AC-2, AC-3, AC-5, AC-6 — serialize across batches
- templates/layout.html: AC-2, AC-6 — serialize; AC-6 uses {{template "populateForm"}} to isolate to single line
- static/css/space.css: AC-1, AC-2, AC-3, AC-6 — serialize; prefer scoped selectors
- main_test.go: AC-2, AC-3 — AC-3 owns TestLayoutRenders parameterization

## Open Questions (Resolved)
- Auto-populate replace vs append: REPLACE + reset bracket ✓
- Distribution algorithm: round-robin ✓
- Minimum list size: reject < 8 ✓
- Theme persistence: cookie-based (bbw_theme, SameSite=Lax) ✓
- .battle-pointer.pointer-left AC-1/AC-3 conflict: AC-3 builder mirrors in y2k.css ✓
- TestLayoutRenders ownership: AC-3 owns (first AC that breaks it) ✓

## AC Summaries

### AC-1: Fix right-wheel-always-wins bug
Winner flag per-array-item in spin-wheel HX-Trigger (not top-level — HTMX event detail = array only). CSS scaleX(-1) flip on .battle-pointer directional triangle (not geometry positioning). Flip deferred to reveal-time (3700ms, after OOB swap completes) inside revealResults. querySelectorAll for all instances — no first-match-only bug. Solo-spin sets no winner flag.

### AC-2: Theme infrastructure — registry, POST /theme, cookie, dropdown
internal/theme package with closed-set Registry{Key, Name, CSSPath}. POST /theme NOT wrapped in sessionMiddleware (theme is per-browser, independent of session). Cookie bbw_theme (SameSite=Lax, no Secure, matching bbw_session attrs). Native form POST + 303 redirect — no JS dependency. Registry constructed once in main(), injected into handler closures.

### AC-3: Y2K theme stylesheet
Complete self-contained stylesheet (not layered over space.css — when y2k active, space.css NOT loaded). Mirrors all ~20 custom properties, 6 @keyframes names, 3 @media breakpoints, 12 selector surface from space.css. Y2K aesthetic: hot pink #ff69b4, electric blue #00bfff, lime green #ccff00, chrome silver #c0c0c0, grid pattern background, bubble/rounded panels, chrome gradient buttons. .battle-pointer.pointer-left mirrored after AC-1 merges.

### AC-4: Pure list-parsing + round-robin distribution
ParseAndDistribute(input string) (Result, error) in new internal/populate package. Pure — no I/O, no HTTP, no session. Round-robin: entry[i] → wheel[i%8]. Tolerant parser: newlines/commas/tabs, strip bullets (•/-/*) and numbering (N.), skip blanks, no dedup. Fixed [8]wheel.Wheel array enforces exactly 8 wheels. ErrTooFewEntries sentinel.

### AC-5: POST /wheels/populate endpoint
Thin effectful shell wrapping pure parser. Form field "items". 200 response: 8 OOB nextRoundSlot fragments (#slot-1..#slot-8) + 1 non-OOB #populate-status. 400 error: JSON via writeJSONError matching addOptionHandler convention. Entire mutation under single store.Update: s.Wheels, s.Bracket (fresh), s.ResolvedMatches (fresh map). 6 refusal arms: GetCookie, ParseForm, empty items, ErrTooFewEntries, ErrSessionNotFound, else → 500.

### AC-6: Auto-populate UI — textarea + HTMX
Separate templates/populate.html with {{define "populateForm"}}. Layout includes via single {{template}} line (minimizes hot-conflict with AC-2). Form: textarea[name="items"], button[type="submit"], hx-post="/wheels/populate", hx-target="#populate-status". Error visibility enforced: getComputedStyle display/visibility/opacity + getBoundingClientRect height > 0. 8 OOB slot fragments + 1 non-OOB. Optional: echo-back input on error, hx-disabled-elt, aria-live="polite".

## Full AC Specifications

### AC-1: Fix right-wheel-always-wins bug

#### Executable Spec
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

#### Design Intent
- **Types / interfaces (§1):** `winner: true` bool per spin-wheel array item. Exactly-one invariant enforced by handler + Go test.
- **Pure / effectful (§2):** ResolveBattle stays pure (unchanged). Handler computes winner flag. JS applies CSS class at reveal. Template renders neutral SVG.
- **Boundary cuts (§3):** Winner data rides existing spin-wheel event (array value). Winner = server concern; pointer direction = client concern. HTMX event-flow constraint: spin-wheel handler detail.value = array only; winner MUST live inside array items.
- **Module responsibility (§4):** handlers_battle.go emits winner data; match.html renders neutral .battle-pointer + class hook; wheel.js flips direction at reveal via querySelectorAll; space.css defines .battle-pointer.pointer-left { transform: scaleX(-1) }.
- **Function discipline (§5):** Pointer flip = one focused fn called from revealResults; uses querySelectorAll (all instances); defers to existing reveal timer; exposes window.__lastSpinItems as test oracle.

#### Technical Context
- **Files:** handlers_battle.go:260-275 (add winner to trigger items), match.html:20-24 (class hook on .battle-pointer), wheel.js:69-86 (flip in revealResults + __lastSpinItems oracle), space.css:693 (.pointer-left transform rule), handlers_battle_test.go:230+1374 (extend) + NEW test
- **Architecture notes:** HTMX event-flow constraint (load-bearing): spin-wheel event detail = array only; top-level winnerID invisible to handler. Timing: spin-wheel fires before OOB swap; pointer flip MUST defer to revealResults (3700ms). Directional not positional: .battle-pointer is a directional triangle; fix = flip via scaleX(-1), not move-to-slot geometry. Random winner: __lastSpinItems serves as oracle; getComputedStyle.transform verifies render.

#### Dependencies
- **Depends on:** none
- **Blocks:** none directly
- **Conflict set:** handlers_battle.go, templates/match.html, static/js/wheel.js, static/css/space.css, handlers_battle_test.go

#### UI Block
- **selectors:** .battle-pointer, .winner-text, .pending-reveal/.revealed, window.__lastSpinItems
- **deterministic_check:** rodney assert using getComputedStyle().transform (not just classList) + window.__lastSpinItems oracle for random winner. querySelectorAll for multi-element.
- **subjective_residual:** pointer visual polish (color, glow, arrow shape)

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] ADR if new interface/boundary
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — winner flag per-array-item (not top-level winnerID): HTMX event detail = array only; top-level field invisible to spin-wheel handler
- 2026-07-26 — CSS scaleX(-1) flip (not geometry positioning): .battle-pointer is directional triangle, not positional element
- 2026-07-26 — reveal-time timing (not setTimeout(0)): OOB swap guaranteed complete at 3700ms

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -race ./... and rodney probes
- Rollback: revert handler trigger payload change + wheel.js pointer flip + CSS rule

---

### AC-2: Theme infrastructure — registry, POST /theme cookie, server-rendered stylesheet link, dropdown selector

#### Executable Spec
- **predicate:**
  (1) REGISTRY (pure, internal/theme): Registry type with Register(key,name,cssPath)/Resolve(key)→(Theme,bool)/Default()→Theme/Names()→[]string. After Register("space","Space","/static/css/space.css"): Resolve("space").CSSPath == "/static/css/space.css"; Resolve("unregistered") == (Theme{}, false); Resolve("../etc") == (Theme{}, false); Default().CSSPath == "/static/css/space.css"; Names() contains "space".
  (2) ENDPOINT: POST /theme form body theme=space → 303 (Location "/") + Set-Cookie bbw_theme=space (HttpOnly, Path=/, SameSite=Lax, Secure=false). POST /theme empty/missing/unknown → 400, no Set-Cookie. GET /theme → 405.
  (3) RENDERING (cookie→href, NOT hardcoded): GET / no cookie → link href="/static/css/space.css". GET / bbw_theme=space → same. GET / bbw_theme=unregistered → href == Default().CSSPath. href ALWAYS registered CSSPath (closed set).
  (4) DROPDOWN: GET / body contains form.theme-selector[action="/theme"][method="post"] with select[name="theme"], one option per registered theme, current theme option carries selected attribute (server-rendered).
  (5) SESSION-INDEPENDENCE: /theme route registered WITHOUT sessionMiddleware. Rotating/deleting bbw_session does not alter bbw_theme.
- **probe:** go test -run TestTheme -race ./...
- **negative:** POST /theme without theme → 400 + no cookie; GET /theme → 405; unknown cookie → fallback to Default; missing HttpOnly/SameSite=Lax/Path=/ or Secure=true → fail. Sneaky-pass: POST always 303 with hardcoded space.css AND layout.html unchanged. Caught by registry unit test (register 2nd test theme, verify Resolve) + href == resolved Theme.CSSPath.
- **verification:** code (httptest body+cookie assertions + go test -race)
- **fixture status:** main_test.go:18 (testTemplate) reused; NEW internal/theme/theme_test.go; NEW handlers_theme_test.go
- **rubric anchor:** §1.2 (closed registry), §1.3 (unknown rejected at boundary), §2.1/§2.4 (effectful themeHandler + pure Registry.Resolve), §3.1/§3.4 (internal/theme decoupled), §4.1/§4.2 (new handlers_theme.go + internal/theme), §5.1 (single-responsibility helpers)

#### Design Intent
- **Types / interfaces (§1):** Theme{Key, Name, CSSPath string}. Registry holds closed set. Unregistered key → (Theme{}, false). Illegal states unrepresentable.
- **Pure / effectful (§2):** internal/theme.Registry pure (no I/O, mirrors internal/wheel). themeHandler + setThemeCookie effectful. homeHandler reads cookie → pure Resolve → passes ThemeCSS to template.
- **Boundary cuts (§3):** internal/theme — pure domain, no net/http import. handlers_theme.go — HTTP adapter. layout.html — presentation. bbw_theme cookie mirrors bbw_session attrs.
- **Module responsibility (§4):** internal/theme — what themes exist + resolve key→CSSPath; NOT HTTP, NOT templates. handlers_theme.go — POST /theme adapter: validate→resolve→set cookie→303. homeHandler — read theme cookie, resolve, inject ThemeCSS+Themes into template data.
- **Function discipline (§5):** themeHandler (validate→resolve→setCookie→303); setThemeCookie (mirror SetCookie attrs); resolveTheme (pure wrapper). One responsibility each.

#### Technical Context
- **Files:** internal/theme/theme.go NEW, internal/theme/theme_test.go NEW, handlers_theme.go NEW, handlers_theme_test.go NEW, main.go (setupRouter add POST /theme NOT wrapped in sessionMiddleware + main() construct registry), handlers.go (homeHandler read theme cookie + inject ThemeCSS/Themes), templates/layout.html:8 (dynamic link + dropdown form), static/css/space.css (.theme-selector scoped), main_test.go (testTemplate reuse)
- **Architecture notes:** /theme unwrapped — precedent /health, /static/ both unwrapped. bbw_theme independent of bbw_session. Registry constructed once in main(), injected. Dropdown = native form POST → 303 → GET / (full reload, no JS dependency). AC-3 calls registry.Register("y2k","/static/css/y2k.css").

#### Dependencies
- **Depends on:** none (foundational infrastructure)
- **Blocks:** AC-3 (Y2K CSS — needs registry.Register); AC-6 (layout.html conflict)
- **Conflict set:** main.go (AC-2/AC-5), templates/layout.html (AC-2/AC-6), static/css/space.css (AC-1/AC-2/AC-6)

#### UI Block
- **selectors:** link[rel="stylesheet"], form.theme-selector, select[name="theme"], option[value="space"], option[selected]
- **deterministic_check:** httptest body — strings.Contains(body, link href) + strings.Contains(body, option selected). Rodney supplementary: assert stylesheet href.
- **subjective_residual:** none for AC-2 (dropdown appearance/styling is AC-3 territory)

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] ADR if new interface/boundary
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — internal/theme package (not inline in handlers): follows internal/wheel, internal/battle, internal/bracket convention
- 2026-07-26 — /theme NOT wrapped in sessionMiddleware: theme is per-browser, session-independent. Precedent: /health, /static/ both unwrapped.
- 2026-07-26 — closed-set registry makes path traversal impossible: Resolve(unregistered) → default, never interpolates cookie value

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -run TestTheme -race ./...
- Rollback: revert internal/theme package + handlers_theme.go + layout.html changes + main.go route

---

### AC-3: Y2K theme stylesheet — complete self-contained theme, registered in theme registry

#### Executable Spec
- **predicate:** ALL of the following hold:
  1. static/css/y2k.css exists in embedded FS and is >0 bytes
  2. :root in y2k.css redeclares every custom property from space.css:41-62 — token-name parity (floor guard: >= 20 custom properties declared)
  3. At least 8 tokens have Y2K-identifiable values NOT equal to corresponding space.css values (anti-copy sneaky-pass)
  4. y2k.css defines all 6 @keyframes names from space.css (twinkle, twinkle-slow, meteor, cosmic-glow, pulse, border-glow) — bodies may differ, names must match
  5. y2k.css mirrors all @media breakpoints from space.css (3: min-width:1025px, max-width:1024px, max-width:640px)
  6. y2k.css contains rules for minimum selector surface (all 12): .cosmic-panel, .movie-hero, .neon-button, .bracket, .bracket-connector, .slot, .wheel-slice, .pending-reveal, .revealed, .center-display, .match-result, .battle-pointer
  7. y2k.css contains NO url() references (CSS-only, no external image assets)
  8. registry.Register("y2k", "Y2K", "/static/css/y2k.css") succeeds; Resolve("y2k") returns Theme{Key:"y2k", Name:"Y2K", CSSPath:"/static/css/y2k.css"} with ok==true
  9. GET /static/css/y2k.css returns 200 with Content-Type: text/css
  10. GET / with bbw_theme=y2k renders <link rel="stylesheet" href="/static/css/y2k.css">
- **probe:** go test -race ./... -run TestY2K + go build. Rodney: assert link href, h1 color !== space gold, .cosmic-panel backgroundColor !== space cosmic-bg, .battle-pointer display !== none, .cosmic-panel borderRadius >= 16, body backgroundImage includes 'gradient'
- **negative:** Registration omission → Resolve returns false. Incomplete tokens → token-name parity fails. Incomplete selector surface (body+h1 only) → 12-selector check fails (primary sneaky-pass). url() violation → fails no-external-assets invariant. Copied space.css values → fails >=8 Y2K-identifiable guard.
- **verification:** code (Go test: structural predicates 1-9) + rodney (getComputedStyle runtime cascade: predicate 10 + ui) + visual (subjective Y2K aesthetic — builder-vision-eval)
- **fixture status:** static/css/y2k.css NEW, main.go (one-line Register), main_test.go (new TestY2K* family + TestLayoutRenders update to be theme-aware/parameterized)
- **rubric anchor:** §3 (boundary cut — one stylesheet per theme), §4 (module responsibility — y2k.css owns Y2K tokens + overrides, NOT theme switching)

#### Design Intent
- **Types / interfaces (§1):** Theme{Key, Name, CSSPath} from AC-2. 20 token names are implicit interface contract between space.css and y2k.css.
- **Pure / effectful (§2):** y2k.css = pure static asset. registry.Register = thin one-liner. No business logic.
- **Boundary cuts (§3):** One self-contained stylesheet per theme + registry indirection. y2k.css mirrors space.css selector vocabulary but reinterprets token values + animations + backgrounds.
- **Module responsibility (§4):** static/css/y2k.css = Y2K tokens + component overrides + animations + breakpoints. NOT: theme switching mechanism (AC-2), HTML layout (shared templates), bracket state, wheel SVG geometry.
- **Function discipline (§5):** registry.Register one call, one job, one line. TestY2K* family groups structural assertions.

#### Technical Context
- **Files:** static/css/y2k.css NEW (~400-600 lines mirroring space.css structure), main.go (one-line Register alongside space), main_test.go (TestY2K_TokenParity, TestY2K_SelectorSurface, TestY2K_Keyframes, TestY2K_Breakpoints, TestY2K_NoURL, TestY2K_Registry, TestY2K_HTTPServesCSS, TestY2K_ThemeLinkRendered + TestLayoutRenders update)
- **Architecture notes:** y2k.css must be COMPLETE and SELF-CONTAINED — when y2k is active, space.css is NOT loaded. Cannot rely on space.css for base rules. Token values: Y2K aesthetic (hot pink #ff69b4, electric blue #00bfff, lime green #ccff00, chrome silver #c0c0c0). body::before/::after: grid pattern (repeating-linear-gradient) instead of starfield. Keyframe names match but bodies differ.

#### Y2K Aesthetic Direction
- Color palette: hot pink, electric blue, lime green, chrome silver, metallic gradients
- Background: CSS grid pattern instead of starfield
- Panels: bubble/rounded (border-radius >= 16px), glossy gradient, chrome borders
- Buttons: chrome gradient pill, bright solid borders, hover scale
- Text: monospace, chrome-era text-shadow (hard shadow, not neon glow)
- Animations: same keyframe names, Y2K bodies (pulse → scale bounce, border-glow → color-cycle)

#### Dependencies
- **Depends on:** AC-2 (theme registry infrastructure — MUST be merged first)
- **Blocks:** none (leaf AC)
- **Conflict set:** main.go (AC-2/AC-3/AC-5), main_test.go (AC-2/AC-3), static/css/space.css (AC-1/AC-2/AC-6 — y2k.css must mirror .battle-pointer.pointer-left if AC-1 adds it)

#### UI Block
- **selectors:** link[href="/static/css/y2k.css"], h1, .cosmic-panel, .battle-pointer, .neon-button, body
- **deterministic_check:** rodney assert getComputedStyle (h1 color !== space gold, .cosmic-panel backgroundColor !== space, .cosmic-panel borderRadius >= 16, body backgroundImage includes gradient, .battle-pointer display !== none)
- **subjective_residual:** Y2K aesthetic feel — palette dominance, glossy 3D buttons, pixel-art headings, bubble/rounded impression. Evaluated by builder-vision-eval.

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — Complete self-contained stylesheet (not layered override): AC-2 design = one stylesheet per theme via server-rendered link. y2k.css cannot rely on space.css.
- 2026-07-26 — .battle-pointer.pointer-left deferred to AC-3 builder: AC-1 (Batch 1) merges first, adds .pointer-left to space.css. AC-3 (Batch 2) builder mirrors it in y2k.css. AC-3 predicate excludes .pointer-left (doesn't exist on main at spec time).
- 2026-07-26 — TestLayoutRenders update owned by AC-3: AC-3 is first AC that makes the test break (second theme registered). AC-3 parameterizes the test over registered themes.

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -race ./... -run TestY2K
- Rollback: delete static/css/y2k.css + revert main.go Register call + revert main_test.go TestY2K* additions

---

### AC-4: Pure list-parsing + round-robin distribution across 8 wheels

#### Executable Spec
- **predicate:** Given raw input with 9 items "Alpha"–"Iota" using mixed delimiters (newlines, commas, tabs, bullets •/-/*, numbering 1./2., blank lines, leading/trailing whitespace), when calling populate.ParseAndDistribute(raw), then:
  1. result.Wheels[i].ID == fmt.Sprint(i) for all i ∈ [0,7] — IDs match newWheels() convention
  2. result.Wheels[0].Options[0].Text == "Alpha" — round-robin start
  3. result.Wheels[0].Options[1].Text == "Iota" — 9th item overflows to wheel[0]
  4. max(len(result.Wheels[i].Options)) - min(len(result.Wheels[i].Options)) <= 1 — even distribution
  5. result.Wheels[i].Options[j].Weight == nil for all i,j — equal-share default
  6. len(result.Entries) == 9 — blanks skipped, no dedup (duplicates preserved)
  7. result.Entries[0] == "Alpha" && result.Entries[8] == "Iota" — parse order preserved
- **probe:** go test -race ./internal/populate/...
- **negative:** (a) < 8 entries (empty string, whitespace-only, 7 items) → returns ErrTooFewEntries containing "at least 8 entries". (b) Sneaky-pass: input "Alpha\n\nBeta\n" (1 blank line between 2 real entries) → len(result.Entries) == 2, NOT 3. If parser counts blanks as entries, predicate catches.
- **verification:** code (go test -race)
- **fixture status:** NEW — internal/populate/populate.go, internal/populate/populate_test.go
- **rubric anchor:** §1.3 (type [8]wheel.Wheel enforces 8; ErrTooFewEntries sentinel; nil Weight invariant), §2.1 (pure core — no I/O), §2.2 (this package IS the pure core), §4.1 (package header documents what/where/what-NOT), §5.1 (one coupled operation, table-driven tests, no patches needed)

#### Design Intent
- **Types / interfaces (§1):** Result{Wheels [8]wheel.Wheel, Entries []string} — fixed-size array enforces exactly 8 wheels. ErrTooFewEntries sentinel (distinguishes "input entries" from "wheel options"). Option.Weight == nil invariant. IDs assigned as fmt.Sprint(i) matching newWheels() in session.go.
- **Pure / effectful (§2):** Pure package — no I/O, no mutation, no HTTP, no session access. Deterministic. Returns new values. Imports only internal/wheel.
- **Boundary cuts (§3):** internal/populate owns parsing + distribution only. Does NOT import session, bracket, or net/http. AC-5 calls ParseAndDistribute, writes wheels to session, resets bracket. AC-6 consumes AC-5's HTML response.
- **Module responsibility (§4):** Package populate parses Google-Docs-style list input (tolerant: newlines, commas, tabs, bullets, numbering, blanks, whitespace) into entries, distributes evenly across 8 wheels via round-robin. NOT: HTTP handling (AC-5), session mutation (AC-5), bracket reset (AC-5), UI rendering (AC-6).
- **Function discipline (§5):** ParseAndDistribute(input string) (Result, error) — one coupled operation. Name avoids populate.Populate stutter. Concise signature (1 param). Testable without patches. Table-driven parser tolerance tests + property-based evenness assertion.

#### Technical Context
- **Files:** internal/populate/populate.go (NEW), internal/populate/populate_test.go (NEW)
- **Architecture notes:** New pure package, no existing files modified. Follows internal/wheel pattern. Result.Entries provides stable interface for AC-5/AC-6. Round-robin: entry[i] → wheel[i%8]. Parser: split on newlines/commas/tabs, strip bullets (•/-/*) and numbering (N.), trim whitespace, skip empty lines, NO deduplication. Test suite: table-driven parser tolerance covering edge cases (tabs, trailing/leading delimiters, emoji, long text, >16 cyclic, exactly-8 boundary, duplicates preserved).

#### Dependencies
- **Depends on:** internal/wheel (Wheel, Option types)
- **Blocks:** AC-5 (endpoint calls ParseAndDistribute, uses Result.Wheels + Result.Entries), AC-6 (UI consumes AC-5 response)
- **Conflict set:** none (new package, new files)

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — ParseAndDistribute name (not Populate): avoids populate.Populate stutter (Go Effective Go convention)
- 2026-07-26 — No IDs parameter: wheel IDs always "0".."7" via fmt.Sprint(i) per newWheels() convention in session.go
- 2026-07-26 — Result struct with Entries: provides stable interface for AC-5/AC-6 downstream consumption
- 2026-07-26 — ErrTooFewEntries (not ErrTooFewItems): "Entries" = input; "Items" ambiguous with wheel Options
- 2026-07-26 — No dedup: pure function shouldn't silently alter input

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -race ./internal/populate/...
- Rollback: delete internal/populate/ package (no existing files modified)

---

### AC-5: POST /wheels/populate endpoint — validate, mutate session under single lock, render 8 OOB wheel fragments + status

#### Executable Spec
- **predicate:** POST /wheels/populate with form field "items" containing 16 newline-separated entries → HTTP 200, Content-Type: text/html, response body contains exactly 8 hx-swap-oob="true" elements with IDs slot-1..slot-8 (rendered via existing nextRoundSlot template, each containing round-robin-distributed wheel SVG) AND 1 non-OOB element (populate-status). MUTATION GATE (read back via store.View): s.Wheels[i].Options matches populate.Result.Wheels[i].Options for all 8 wheels (round-robin + replace verified — pre-existing "OLD" option absent), s.Bracket is fresh *bracket.Bracket from bracket.New(result.Wheels) (pointer != pre-populate, s.Bracket.Slots == result.Wheels), s.ResolvedMatches is fresh make(map[string]bool) (len==0, != nil, != pre-populate reference). All three mutations within single store.Update closure. Idempotency NOT enforced (second populate replaces again).
- **probe:** go test -race ./... -run TestPopulate. Test mirrors TestBattleHandler_PostBattleStoreState (handlers_battle_test.go:416-491): httptest.NewServer + http.DefaultClient.Do(POST) + store.View read-back.
- **negative:** (1) POST with 7 entries → 400, JSON {"error":"...at least 8 entries..."}, session unchanged (store.View read-back confirms). (2) Sneaky-pass: handler renders OOB from result.Wheels but store.Update closure has dead code/early return that never assigns s.Wheels → response check passes but mutation-gate fails.
- **verification:** code (Go test: HTTP request via httptest + store.View read-back)
- **fixture status:** NEW handlers_populate.go + handlers_populate_test.go; MODIFY main.go (register POST /wheels/populate under sessionMiddleware), templates/bracket.html (add populateStatus fragment — nextRoundSlot reused as-is)
- **rubric anchor:** §2 (pure/effectful — pure parser AC-4 → effectful handler AC-5, mutation under single lock), §3 (module cut — new handlers_populate.go isolates endpoint)

#### Design Intent
- **Types / interfaces (§1):** Handler consumes typed populate.Result{Wheels [8]wheel.Wheel, Entries []string} from AC-4. No untyped data crosses boundaries.
- **Pure / effectful (§2):** Pure parser (AC-4) → effectful handler (AC-5). Handler is thin shell: parse form → delegate to populate.ParseAndDistribute → mutate under single store.Update lock → render. Render AFTER Update (read mutated session, not Result directly).
- **Boundary cuts (§3):** New file handlers_populate.go (isolated from handlers_battle.go/handlers_wheel.go). Route in main.go. populateStatus template fragment co-located in bracket.html.
- **Module responsibility (§4):** handlers_populate.go = POST /wheels/populate endpoint only. Reuses existing helpers: writeJSONError, wheelViewFromWheel, slotIDFromWheelIdx, nextRoundSlot template.
- **Function discipline (§5):** Handler does one thing (populate). Testable without patches — httptest.Server + injected *Store. Refusal arms mirror addOptionHandler: GetCookie→401, ParseForm→400, empty items→400, ErrTooFewEntries→400, store.Update ErrSessionNotFound→401, else→500.

#### Technical Context
- **Files:** main.go (route: mux.Handle("POST /wheels/populate", sessionMiddleware(store, populateHandler(store, tmpl)))), handlers_populate.go NEW, handlers_populate_test.go NEW, templates/bracket.html (add {{define "populateStatus"}}<div id="populate-status">...</div>{{end}})
- **Architecture notes:** Handler flow: r.ParseForm() → items := r.FormValue("items") → result, err := populate.ParseAndDistribute(items) → on ErrTooFewEntries: writeJSONError(w, 400, "at least 8 entries required"); on success: store.Update(sessionID, func(s *Session) error { s.Wheels = result.Wheels; s.Bracket = bracket.New(result.Wheels); s.ResolvedMatches = make(map[string]bool); return nil }) → render 8 nextRoundSlot OOB fragments (slotIDFromWheelIdx(i) for i=0..7) + 1 populateStatus non-OOB. Error format: JSON via writeJSONError. HTMX 2.x requires ≥1 non-OOB swap target — populate-status satisfies this.

#### Stable Interface for AC-6
- Endpoint: POST /wheels/populate
- Form field: "items" (newline-separated text)
- Success (200): text/html — 8 OOB slot-N fragments + 1 non-OOB populate-status
- Error (400): application/json — {"error": "at least 8 entries required"}
- Error (401): application/json — {"error": "session required"}
- HTMX swap: form hx-target="#populate-status"; 8 OOB slot-N fragments auto-swap

#### Dependencies
- **Depends on:** AC-4 (populate.ParseAndDistribute, Result, ErrTooFewEntries — RESOLVED)
- **Blocks:** AC-6 (UI form submission — consumes stable endpoint contract above)
- **Conflict set:** main.go (AC-2/AC-5 route registration), templates/bracket.html (AC-5/AC-6)
- **Risk level:** medium (mutates session state under write lock, new endpoint, race detector mandatory)

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — Mutation gate (store.View read-back): follows TestBattleHandler_PostBattleStoreState convention. Verifies actual session state, not just response HTML.
- 2026-07-26 — JSON error format via writeJSONError: matches existing addOptionHandler convention. Not HTML error pages.
- 2026-07-26 — Reuse nextRoundSlot template: no new OOB template needed. Only populateStatus (non-OOB) is new, co-located in bracket.html.
- 2026-07-26 — 6 refusal arms matching addOptionHandler: GetCookie→401, ParseForm→400, empty→400, ErrTooFewEntries→400, ErrSessionNotFound→401, else→500.

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -race ./... -run TestPopulate
- Rollback: delete handlers_populate.go + handlers_populate_test.go + revert main.go route + revert bracket.html populateStatus fragment

---

### AC-6: Auto-populate UI — textarea+submit form, HTMX re-render 8 QF slots, visible error surface

#### Executable Spec
- **predicate:**
  (1) FORM STRUCTURE (GET /): response body contains .populate-form with textarea[name="items"], button[type="submit"], hx-post="/wheels/populate", hx-target="#populate-status".
  (2) SUCCESS (POST 8 valid items): handler returns 200, body contains 8 hx-swap-oob fragments targeting #slot-1..#slot-8 (exact spelling — not slot-01), fragment option text, AND 1 non-OOB swap (HTMX 2.x requirement).
  (3) ERROR (POST < 8 items, empty, whitespace-only): handler returns 400, error text rendered inside #populate-status, AND element user-visible: getComputedStyle(#populate-status).display !== 'none' AND getComputedStyle(#populate-status).visibility !== 'hidden' AND getComputedStyle(#populate-status).opacity !== '0' AND #populate-status.getBoundingClientRect().height > 0. Form remains interactive after error.
- **probe:** go test -race -run TestPopulateUI (httptest server contract) + rodney (client-side computed visibility + slot re-render)
- **negative:** handler returns error inside <div id="populate-status" style="display:none"> — element exists in DOM, passes querySelector, but user never sees it. Caught by getComputedStyle.display !== 'none' AND getBoundingClientRect.height > 0. Also: slot ID misspelled as slot-01.
- **verification:** code + rodney (httptest: status codes, OOB fragment count, error text, no inline display:none + rodney: computed visibility + slot re-render)
- **fixture status:** NEW testPopulateServer helper in handlers_populate_test.go; rodney fixture script at docs/evidence/<issue>/populate-rodney.sh
- **rubric anchor:** §1 (Renderer interface reuse), §2 (handler = thin shell), §3 (populate form as separate {{define}} template — cuts layout.html joint), §5 (handler one responsibility: wire form to endpoint)

#### Design Intent
- **Types / interfaces (§1):** Reuse existing Renderer/template-parse pipeline. OOB slot fragments reuse wheel template. Form data: r.ParseForm() → r.Form["items"] → AC-5 populate logic.
- **Pure / effectful (§2):** handlers_populate.go handler thin shell: parse form → delegate to AC-5 populate → render OOB fragments or error. No business logic in AC-6.
- **Boundary cuts (§3):** Populate form lives in templates/populate.html {{define "populateForm"}}, included in layout.html via single {{template "populateForm"}} line. Follows existing repo convention (wheel.html/bracket.html/match.html all use {{define}} + {{template}}). Minimizes hot-conflict with AC-2 on layout.html to one line.
- **Module responsibility (§4):** populate.html header: "Auto-populate form textarea + submit, renders in layout, swaps #populate-status on response. NOT: populate logic (AC-5), wheel rendering (wheel.html), bracket layout (layout.html)."
- **Function discipline (§5):** Handler one thing: translate HTTP POST → AC-5 populate call + HTMX response assembly. No direct session.Wheels mutation in AC-6 (delegated to AC-5).

#### Technical Context
- **Files:** templates/layout.html (single {{template "populateForm"}} line near bracket region), templates/populate.html NEW ({{define "populateForm"}} with .populate-form, textarea[name="items"], button[type="submit"], #populate-status div), static/css/space.css (.populate-form scoped styles), main.go (//go:embed templates/populate.html + route if not AC-5)
- **Architecture notes:** HTMX OOB: 8 OOB (#slot-1..#slot-8) + 1 non-OOB (#populate-status). Non-OOB REQUIRED for HTMX 2.x. 400 error renders into #populate-status (non-OOB, visible). hx-target="#populate-status" on form. Error text rendered into already-visible div — NOT inline CSS. Form CSS scoped .populate-form in space.css.
- **Optional enhancements (recommended if low-cost):** echo-back user input on error (preserve textarea value), hx-disabled-elt on submit button (double-submit prevention), <label for="populate-items"> + aria-live="polite" on #populate-status (accessibility).

#### UI Block
- **selectors:** .populate-form, textarea[name="items"], button[type="submit"], #populate-status, #slot-1..#slot-8
- **layout_assertions:** .populate-form display !== none, textarea exists with rows >= 3, submit button visible, #populate-status exists, form hx-post="/wheels/populate", form hx-target="#populate-status"
- **deterministic_check:** rodney: form exists + textarea + submit + hx-post + hx-target. Error case: submit 3 items → 400 + #populate-status visible (getComputedStyle display/visibility/opacity + getBoundingClientRect height > 0). Success case: submit 8 items → 8 slots with option text.
- **subjective_residual:** form styling matches space theme (glassmorphism, neon glow on focus), error text visual urgency, layout spacing, textarea comfortable row count

#### Dependencies
- **Depends on:** AC-5 (POST /wheels/populate endpoint contract: 200→8 OOB+1 non-OOB, 400→error, replace mode). AC-6 wires UI to AC-5's endpoint; cannot succeed until AC-5 ships.
- **Blocks:** none (terminal AC in auto-populate chain: AC-4→AC-5→AC-6)
- **Conflict set:** templates/layout.html (AC-2/AC-6 — mitigated by single {{template}} line), main.go (AC-2/AC-5/AC-6 — embed directive + route), static/css/space.css (AC-1/AC-2/AC-6 — .populate-form scoped block)
- **Risk level:** medium (depends on AC-5 contract; hot-conflict on 3 files with parallel AC-2 work)

#### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

#### Decision Log
- 2026-07-26 — Separate template file (templates/populate.html): follows repo convention (wheel.html/bracket.html/match.html all use {{define}} + {{template}}). Isolates layout.html conflict with AC-2 to single line.
- 2026-07-26 — Error visibility enforced: getComputedStyle display/visibility/opacity + getBoundingClientRect height > 0. Catches display:none/visibility:hidden/opacity:0/zero-height sneaky-passes.
- 2026-07-26 — Form field "items": matches AC-5 endpoint contract.

#### Surprises & Discoveries
- (none yet)

#### Idempotence & Recovery
- Safe retry: re-run go test -race -run TestPopulateUI
- Rollback: delete templates/populate.html + revert layout.html {{template}} line + revert space.css .populate-form + revert main.go embed
