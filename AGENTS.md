# AGENTS.md — battle-bracket-wheels

Read this file BEFORE `docs/okf/index.md`. This grounds you in repo conventions; `docs/okf/` is the concept-doc bundle.

## Repo

- **Language/version:** Go 1.25+ (`go 1.25.3` in `go.mod`)
- **Module:** `battle-bracket-wheels`
- **Build:** `go build -o battle-bracket-wheels .`
- **Run:** `./battle-bracket-wheels` (defaults to `:8080`, overridable via `PORT` env)
- **Project memories:** `~/.opencode-mem/data/projects/project_f45adc73d392ef18_shard_0.db`

## Test Command

Run all tests with the race detector:

```bash
go test -race ./...
```

The race detector is **REQUIRED** — not optional. The codebase uses `sync.RWMutex` for concurrent session access, and the race detector catches data races in test.

## OKF Bundle

Read `docs/okf/index.md` for concept documentation. The bundle covers:

- session-store, session-middleware, battle-handler, wheel-handlers
- spin-handler, http-router, htmx-oob-protocol, static-assets
- render-deploy, template-system, client-animation, test-architecture
- space-theme, app-architecture, bracket-state-machine, view-models
- battle-resolution, svg-arc-geometry, wheel-spin-algorithm, match-id, wheel

## Agent Notes

Code-scoped learnings and invariants discovered during implementation go in `docs/agent-notes/<topic>.md`. This directory is created on-demand when a builder or reviewer records a lesson. Notes written here are for agent consumption — terse, technical, actionable. They supplement (not replace) the OKF bundle.

## Conventions

### Session Cookie

- Cookie name: `bbw_session`
- `SameSite=Lax` — NOT `SameSite=Strict` or `SameSite=None`
- NOT `Secure` (allows HTTP on Render free tier)
- Created by `sessionMiddleware` for every request
- Stale cookie IDs → new session created, `Cookie` header rewritten

### HTMX 2.x OOB Protocol

- Battle response includes 4 OOB fragments + 1 non-OOB main swap (`disabledButton`)
- HTMX 2.x requires at least one non-OOB swap target to process `HX-Trigger`
- `spin-wheel` trigger sends an **array of 2 results** for battle, a **single object** for solo spin
- OOB fragments: `matchResult`, `nextRoundSlot`, `movieResult` (Final only), `centerDisplay`

### Concurrency

- Entire battle executes under a single `Store.Update` write lock (atomicity)
- `ResolvedMatches` checked before resolve (idempotency — prevents double-resolve of same battle)
- `View` uses `RLock` (concurrent reads), `Update` uses `Lock` (exclusive writes)
- Closure-based access prevents pointer escape

## Exercising the App

- **How to run the app locally:**
  ```bash
  go build -o battle-bracket-wheels .
  ./battle-bracket-wheels
  ```
  App serves on `http://localhost:8080`.

- **How to reach the affected feature:**
  Open `http://localhost:8080` in a browser. The app presents a bracket UI with 8 wheels. Add options to wheels (text input per slot), spin wheels (click to spin), and advance through bracket rounds (winners advance automatically). The `bbw_session` cookie is set on first request.

- **How to verify the change behaves correctly:**
  - **Wheel CRUD:** Add an option to a wheel — the wheel fragment re-renders via HTMX swap. Delete an option — the wheel updates.
  - **Spin:** Click a wheel to spin. The wheel animates and lands on a random option. The result fragment appears after the animation delay.
  - **Battle:** When two wheels meet in a bracket match, click the battle button. Both wheels spin, the match resolves (higher roll wins), the loser's option is absorbed into the winner's wheel, and the bracket advances.
  - **Session:** The first request sets `bbw_session` cookie. Stale/missing cookies create a new session transparently.

- **Evidence to produce:**
  - API/vet evidence: `go test -race ./...` output → `docs/evidence/<issue-number>/test-suite.log`
  - Run logs at `docs/evidence/<issue-number>/run.log` capturing app startup and curl probes
  - For session middleware: `curl -v http://localhost:8080/ 2>&1 | grep -i 'set-cookie'` to confirm `bbw_session` + `SameSite=Lax`
  - For endpoint contracts: `curl` responses at `docs/evidence/<issue-number>/response.json`
  - Browser probe results (rodney assert) for client-side JS state at `docs/evidence/<issue-number>/rodney.log`

## What NOT

- **Not a concept doc.** Concept documentation lives in `docs/okf/`. This file is the entry point, not the bundle.
- **Not a feedback log.** Code-scoped learnings go in `docs/agent-notes/<topic>.md`. Broader pipeline lessons go to the postmortem queue (`~/.claude/postmortem-queue/`).
- **Not user-level config.** User-level agent configuration is at `~/.config/opencode/AGENTS.md`.

## Reference

See [Agent E2E Self-Validation Convention][e2e-ref] for detailed rationale on E2E evidence requirements.

[e2e-ref]: ~/.config/opencode/references/agent-self-validation-convention.md
