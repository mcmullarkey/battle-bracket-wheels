---
type: Module
title: Test Architecture
description: BDD/TDD style — 114 tests across 10 files, race testing, deterministic CSS assertions
resource: main_test.go
tags: [go, testing, bdd, race, §5-function-discipline]
timestamp: 2026-06-24
pure: true
---

# Responsibility

Test coverage for all layers. BDD/TDD style (red-phase first per AC plan notes). Race testing for concurrency. Deterministic CSS/layout assertions in code (not LLM vision).

# Interface

- 114 test functions across 10 `_test.go` files
- Package `main` tests: `httptest.NewServer` + real `setupRouter`
- Internal package tests: unit tests for pure logic
- Race tests: `TestConcurrentWheelMutation`, `TestConcurrentReadWriteRace`, `TestConcurrentSessionCreation`, `TestBattleHandler_ConcurrentResolve`
- CSS asserts: `TestSpaceCSS_*`, `TestRenderYAML_*`, `TestLayoutRenders`
- Canonical command: `go test -race ./...`

# Dependencies

- Tests all [modules](/modules/app-architecture.md), [interfaces](/interfaces/http-router.md), [services](/services/template-system.md)
- Evidence screenshots at `docs/evidence/<issue>/` per OKF screenshot convention

# Invariants

- Race testing canonical (`-race` flag)
- Visual/layout assertions are deterministic code checks, not LLM vision
- BDD style: tests written red-phase first

# Pure/Effectful

Pure. Test functions, no production side effects.

# Citations

- [main_test.go](main_test.go)
- [internal/wheel/wheel_test.go](internal/wheel/wheel_test.go)
- [internal/battle/battle_test.go](internal/battle/battle_test.go)
- [internal/bracket/bracket_test.go](internal/bracket/bracket_test.go)
