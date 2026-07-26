---
ac: 3
depends_on: AC-2
risk: medium
status: spec
---

## AC-3: Y2K theme stylesheet — complete self-contained theme, registered in theme registry

### Executable Spec
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

### Design Intent
- **Types / interfaces (§1):** Theme{Key, Name, CSSPath} from AC-2. 20 token names are implicit interface contract between space.css and y2k.css.
- **Pure / effectful (§2):** y2k.css = pure static asset. registry.Register = thin one-liner. No business logic.
- **Boundary cuts (§3):** One self-contained stylesheet per theme + registry indirection. y2k.css mirrors space.css selector vocabulary but reinterprets token values + animations + backgrounds.
- **Module responsibility (§4):** static/css/y2k.css = Y2K tokens + component overrides + animations + breakpoints. NOT: theme switching mechanism (AC-2), HTML layout (shared templates), bracket state, wheel SVG geometry.
- **Function discipline (§5):** registry.Register one call, one job, one line. TestY2K* family groups structural assertions.

### Technical Context
- **Files:** static/css/y2k.css NEW (~400-600 lines mirroring space.css structure), main.go (one-line Register alongside space), main_test.go (TestY2K_TokenParity, TestY2K_SelectorSurface, TestY2K_Keyframes, TestY2K_Breakpoints, TestY2K_NoURL, TestY2K_Registry, TestY2K_HTTPServesCSS, TestY2K_ThemeLinkRendered + TestLayoutRenders update)
- **Architecture notes:** y2k.css must be COMPLETE and SELF-CONTAINED — when y2k is active, space.css is NOT loaded. Cannot rely on space.css for base rules. Token values: Y2K aesthetic (hot pink #ff69b4, electric blue #00bfff, lime green #ccff00, chrome silver #c0c0c0). body::before/::after: grid pattern (repeating-linear-gradient) instead of starfield. Keyframe names match but bodies differ.

### Y2K Aesthetic Direction
- Color palette: hot pink, electric blue, lime green, chrome silver, metallic gradients
- Background: CSS grid pattern instead of starfield
- Panels: bubble/rounded (border-radius >= 16px), glossy gradient, chrome borders
- Buttons: chrome gradient pill, bright solid borders, hover scale
- Text: monospace, chrome-era text-shadow (hard shadow, not neon glow)
- Animations: same keyframe names, Y2K bodies (pulse → scale bounce, border-glow → color-cycle)

### Dependencies
- **Depends on:** AC-2 (theme registry infrastructure — MUST be merged first)
- **Blocks:** none (leaf AC)
- **Conflict set:** main.go (AC-2/AC-3/AC-5), main_test.go (AC-2/AC-3), static/css/space.css (AC-1/AC-2/AC-6 — y2k.css must mirror .battle-pointer.pointer-left if AC-1 adds it)

### UI Block
- **selectors:** link[href="/static/css/y2k.css"], h1, .cosmic-panel, .battle-pointer, .neon-button, body
- **deterministic_check:** rodney assert getComputedStyle (h1 color !== space gold, .cosmic-panel backgroundColor !== space, .cosmic-panel borderRadius >= 16, body backgroundImage includes gradient, .battle-pointer display !== none)
- **subjective_residual:** Y2K aesthetic feel — palette dominance, glossy 3D buttons, pixel-art headings, bubble/rounded impression. Evaluated by builder-vision-eval.

### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

### Decision Log
- 2026-07-26 — Complete self-contained stylesheet (not layered override): AC-2 design = one stylesheet per theme via server-rendered link. y2k.css cannot rely on space.css.
- 2026-07-26 — .battle-pointer.pointer-left deferred to AC-3 builder: AC-1 (Batch 1) merges first, adds .pointer-left to space.css. AC-3 (Batch 2) builder mirrors it in y2k.css. AC-3 predicate excludes .pointer-left (doesn't exist on main at spec time).
- 2026-07-26 — TestLayoutRenders update owned by AC-3: AC-3 is first AC that makes the test break (second theme registered). AC-3 parameterizes the test over registered themes.

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run go test -race ./... -run TestY2K
- Rollback: delete static/css/y2k.css + revert main.go Register call + revert main_test.go TestY2K* additions
