---
type: Module
title: Space Theme CSS System
description: CSS-only space theme — custom properties, keyframe animations, glassmorphism, neon text, responsive breakpoint
resource: static/css/space.css
tags: [css, theme, design, §5-function-discipline]
timestamp: 2026-06-24
pure: true
---

# Responsibility

CSS-only space aesthetic. Custom property theme tokens, 6 keyframe animations, CSS-only starfield (no `url()` images), glassmorphism panels, neon text, responsive tablet breakpoint.

# Interface

- `:root` custom properties — theme tokens
- 6 `@keyframes`: twinkle, twinkle-slow, meteor, cosmic-glow, pulse, border-glow
- CSS-only starfield: radial-gradient + box-shadow multi-star
- `.cosmic-panel` — glassmorphism
- `.neon-button` — battle buttons
- `.movie-hero` — 2rem + text-shadow for final winner
- `@media (max-width: 1024px)` — tablet breakpoint

# Dependencies

- Served by [static-assets](/services/static-assets.md)
- Styles templates from [template-system](/services/template-system.md)
- `.pending-reveal` / `.revealed` classes coordinate with [client-animation](/presentation/client-animation.md)

# Invariants

- No `url()` images — pure CSS starfield
- Responsive at 1024px breakpoint
- Theme tokens centralized in `:root`

# Pure/Effectful

Pure. CSS declarations only. No JavaScript.

# Citations

- [static/css/space.css](static/css/space.css)
