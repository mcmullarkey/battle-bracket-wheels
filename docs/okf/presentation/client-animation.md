---
type: Module
title: Client-Side Spin Animation
description: SVG rotation animation via requestAnimationFrame — spin-wheel event listener, 5 full spins + target angle
resource: static/js/wheel.js
tags: [javascript, animation, svg, §2-effectful-shell]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Listen for `spin-wheel` HTMX custom event, animate SVG `<g>` rotation to target angle. Handle single-object (solo spin) vs array (battle) trigger data. Use `requestAnimationFrame` for SVG reflow workaround.

# Interface

- IIFE — no exports
- Listens: `spin-wheel` custom event on `body`
- Constants: `FULL_SPINS=5`, `SPIN_DURATION_MS=3500`, `REVEAL_DELAY_MS=3700`
- Normalizes: single object vs array (battle sends 2 results)
- `requestAnimationFrame` workaround — SVG lacks `offsetHeight` reflow
- `.pending-reveal` → `.revealed` class transition after animation

# Dependencies

- Triggered by [spin-handler](/interfaces/spin-handler.md) and [battle-handler](/interfaces/battle-handler.md) via HX-Trigger
- Animates SVG from [view-models](/modules/view-models.md) / [template-system](/services/template-system.md)
- Coordinates with [htmx-oob-protocol](/interfaces/htmx-oob-protocol.md) for reveal timing

# Invariants

- 5 full rotations + target angle for visual effect
- `requestAnimationFrame` used because SVG elements don't trigger reflow like HTML
- Reveal delayed until after spin animation completes (3700ms > 3500ms duration)

# Pure/Effectful

Effectful. DOM manipulation, event handling, animation.

# Citations

- [static/js/wheel.js](static/js/wheel.js)
