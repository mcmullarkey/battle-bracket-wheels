---
title: "AC-6: Space theme polish + responsive layout + Render deploy config"
status: "complete"
feature: "battle-bracket-wheels"
created: "2026-06-24"
---

## Acceptance Criteria

- [x] P1: GET / returns 200 + HTML + link to /static/css/space.css + >=2 space-theme markers
- [x] P2: space.css served via embed.FS (proves static serving in prod)
- [x] P3: space.css contains @media (max-width: 1024px) (tablet responsive breakpoint)
- [x] P4: space.css contains .movie-hero rule with font-size >=1.5rem AND (color non-transparent OR text-shadow)
- [x] P5: space.css <=150KB + contains @keyframes + NO url() referencing image files (CSS-only starfield)
- [x] P6: POST /battle/qf1 returns 200 (app wired)
- [x] P7: render.yaml contains startCommand + PORT envVar + healthCheckPath /health
- [x] R1-R4: Rodney screenshots at 1920px and 768px, movie hero, starfield twinkle

## Progress

- [x] 2026-06-24: Created plan file, branch 6-theme-polish-deploy
- [x] 2026-06-24: Wrote enhanced space.css with full space theme (starfield, cosmic panels, neon accents, themed buttons/wheels, bracket connectors, hero result, responsive breakpoints)
- [x] 2026-06-24: Updated templates (layout.html: removed inline styles, added neon-button/bracket-connector/movie-hero classes; bracket.html: movie-hero on OOB fragment)
- [x] 2026-06-24: Finalized render.yaml (buildCommand, startCommand, healthCheckPath, PORT)
- [x] 2026-06-24: Wrote CSS verification tests in main_test.go (size, @keyframes, no images, breakpoint, movie-hero, render.yaml checks)
- [x] 2026-06-24: Created rodney deploy checklist at .opencode/rodney/AC-6-deploy-checklist.md
- [x] 2026-06-24: Taken rodney screenshots (theme-desktop.png, theme-tablet.png, hero-result.png)
- [x] 2026-06-24: All tests pass (go test -count=1 -race ./...), build clean
- [x] 2026-06-24: Space.css verification: 17KB (<150KB), 6 @keyframes, no image url(), @media breakpoint, .movie-hero with font-size 2rem + color + text-shadow

## Decision Log

- **Movie hero class**: Use `class="movie-hero"` on the `<h2>` in movie-result, matching the OOB template
- **CSS Grid layout**: Use implicit grid via grid-template-columns, responsive by changing column count at breakpoints (4 cols → 2 cols → 1 col)
- **No image files**: All decorative elements use CSS-only techniques (radial-gradient, box-shadow)
- **Deploy verification deferred**: Per user decision, no live Render URL needed
- **Starfield layers**: body::before for radial-gradient stars, body::after for dense box-shadow star points, both with twinkle animation
- **Connectors**: `<hr class="bracket-connector">` elements between bracket sections with neon gradient

## Surprises & Discoveries

- The existing TestLayoutRenders checked for inline @keyframes in the HTML body; after moving to external space.css, had to update the check to look for the CSS link and theme markers instead
- box-shadow stars need a non-transparent element (1px × 1px with transparent background) to render correctly
