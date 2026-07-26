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
