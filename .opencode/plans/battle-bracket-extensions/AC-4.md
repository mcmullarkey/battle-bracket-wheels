---
ac: 4
depends_on: none
risk: low
status: spec
---

## AC-4: Pure list-parsing + round-robin distribution across 8 wheels

### Executable Spec
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

### Design Intent
- **Types / interfaces (§1):** Result{Wheels [8]wheel.Wheel, Entries []string} — fixed-size array enforces exactly 8 wheels. ErrTooFewEntries sentinel (distinguishes "input entries" from "wheel options"). Option.Weight == nil invariant. IDs assigned as fmt.Sprint(i) matching newWheels() in session.go.
- **Pure / effectful (§2):** Pure package — no I/O, no mutation, no HTTP, no session access. Deterministic. Returns new values. Imports only internal/wheel.
- **Boundary cuts (§3):** internal/populate owns parsing + distribution only. Does NOT import session, bracket, or net/http. AC-5 calls ParseAndDistribute, writes wheels to session, resets bracket. AC-6 consumes AC-5's HTML response.
- **Module responsibility (§4):** Package populate parses Google-Docs-style list input (tolerant: newlines, commas, tabs, bullets, numbering, blanks, whitespace) into entries, distributes evenly across 8 wheels via round-robin. NOT: HTTP handling (AC-5), session mutation (AC-5), bracket reset (AC-5), UI rendering (AC-6).
- **Function discipline (§5):** ParseAndDistribute(input string) (Result, error) — one coupled operation. Name avoids populate.Populate stutter. Concise signature (1 param). Testable without patches. Table-driven parser tolerance tests + property-based evenness assertion.

### Technical Context
- **Files:** internal/populate/populate.go (NEW), internal/populate/populate_test.go (NEW)
- **Architecture notes:** New pure package, no existing files modified. Follows internal/wheel pattern. Result.Entries provides stable interface for AC-5/AC-6. Round-robin: entry[i] → wheel[i%8]. Parser: split on newlines/commas/tabs, strip bullets (•/-/*) and numbering (N.), trim whitespace, skip empty lines, NO deduplication. Test suite: table-driven parser tolerance covering edge cases (tabs, trailing/leading delimiters, emoji, long text, >16 cyclic, exactly-8 boundary, duplicates preserved).

### Dependencies
- **Depends on:** internal/wheel (Wheel, Option types)
- **Blocks:** AC-5 (endpoint calls ParseAndDistribute, uses Result.Wheels + Result.Entries), AC-6 (UI consumes AC-5 response)
- **Conflict set:** none (new package, new files)

### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

### Decision Log
- 2026-07-26 — ParseAndDistribute name (not Populate): avoids populate.Populate stutter (Go Effective Go convention)
- 2026-07-26 — No IDs parameter: wheel IDs always "0".."7" via fmt.Sprint(i) per newWheels() convention in session.go
- 2026-07-26 — Result struct with Entries: provides stable interface for AC-5/AC-6 downstream consumption
- 2026-07-26 — ErrTooFewEntries (not ErrTooFewItems): "Entries" = input; "Items" ambiguous with wheel Options
- 2026-07-26 — No dedup: pure function shouldn't silently alter input

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run go test -race ./internal/populate/...
- Rollback: delete internal/populate/ package (no existing files modified)
