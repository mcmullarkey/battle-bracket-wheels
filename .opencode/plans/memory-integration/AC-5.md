---
ac: 5
depends_on: none
risk: low
status: complete
---

## AC spec: Project-level AGENTS.md grounding + OKF→mem-store mirror verification

### Executable Spec
- **predicate:** 3-phase AND — (1) AGENTS.md exists at repo root AND contains repo-specific anchors: `docs/okf`, shard path `project_f45adc73d392ef18_shard_0.db`, `docs/agent-notes`, `bbw_session`, `SameSite=Lax`, `go test -race`, `HTMX.*OOB`, `Store\.Update`, `Exercising the App` section; (2) `okf-mem-sync.ts` runs with exit 0 AND log contains no `Failed: [1-9]` AND no `pin failed`; (3) pinned okf-tagged memories in mem-store `>= expected` where `expected` = count of `docs/okf/**/*.md` files with `type:` frontmatter (excl. nav files), AND `expected >= 21`
- **probe:**
  ```bash
  set -eo pipefail
  # Phase 1: AGENTS.md structural conformance (catches boilerplate sneaky-pass)
  test -f AGENTS.md
  grep -q 'docs/okf' AGENTS.md
  grep -q 'project_f45adc73d392ef18_shard_0.db' AGENTS.md
  grep -q 'docs/agent-notes' AGENTS.md
  grep -q 'bbw_session' AGENTS.md
  grep -q 'SameSite=Lax' AGENTS.md
  grep -q 'go test -race' AGENTS.md
  grep -qE 'HTMX.*OOB' AGENTS.md
  grep -q 'Store\.Update' AGENTS.md
  grep -q 'Exercising the App' AGENTS.md
  # Phase 2: sync runs clean — no Failed counter, no pin-failed soft-failure
  bun run ~/.config/opencode/scripts/okf-mem-sync.ts 2>&1 | tee /tmp/okf-sync.log
  ! grep -qE 'Failed: [1-9]' /tmp/okf-sync.log
  ! grep -q 'pin failed' /tmp/okf-sync.log
  # Phase 3: mirror verification — pinned okf memories >= expected concept-doc count
  expected=$(grep -rl "^type:" docs/okf --include="*.md" | grep -vE "(^|/)(index|log)\.md$" | wc -l | tr -d ' ')
  pinned=$(curl -s "http://127.0.0.1:4747/api/memories?tag=opencode_project_f45adc73d392ef18&pageSize=10000&includePrompts=false" | \
    python3 -c "import sys,json; d=json.load(sys.stdin); items=d.get('data',{}).get('items',[]); print(sum(1 for m in items if m.get('isPinned') and 'okf' in (m.get('tags') or [])))")
  [ "$pinned" -ge "$expected" ] && [ "$expected" -ge 21 ]
  echo "expected=$expected pinned=$pinned"
  ```
  Run from repo root (sync script uses `process.cwd()` for container-tag hash + defaults to `./docs/okf`).
- **negative:**
  - AGENTS.md missing/empty → Phase 1 `test -f` fails
  - Boilerplate AGENTS.md with zero repo-specific content → Phase 1 grep for `bbw_session`/`Store\.Update`/`go test -race` fails
  - Sync exits non-zero (dir-not-found, API-down) → Phase 2 `bun run` exits 1, `pipefail` catches
  - Sync `Failed: N>0` (individual POST failures masked by overall exit 0) → Phase 2 `! grep -qE 'Failed: [1-9]'` catches
  - Pin failures (soft-fail: memories import but auto-cleanup in 30d) → Phase 2 `! grep -q 'pin failed'` catches
  - Count-masking (prior sync's stale entries hiding fresh failure on new docs) → Phase 3 filters by container tag + `okf` tag + `isPinned`, not raw count; fresh failures show as `pinned < expected`
  - Nav files (index.md/log.md) imported as memories → sync script skips files without `type:` frontmatter; if nav files gained `type:`, `expected` rises but sync's `isNavigationFile` skip causes `pinned < expected` → fails
- **verification:** code · shell predicate (grep + sync exit + curl/JSON count). Subjective residual: AGENTS.md prose quality/tone — manual review optional, not load-bearing.
- **fixture status:** NEW (AGENTS.md at repo root; `docs/agent-notes/` dir convention documented in AGENTS.md but dir itself created on-demand by future builder learnings)
- **rubric anchor:** §4.1 (AGENTS.md header documents what/where/what-NOT — agent entry point, NOT a concept doc), §4.2 (`AGENTS.md` is a responsibility-describing convention name), §1.5 (predicate pins invariant tightly — catches boilerplate/pin-fail/count-masking sneaky-passes)

### Design Intent
- **Types / interfaces (§1):** N/A — documentation artifact. §1.5 applies to the verification predicate.
- **Pure / effectful (§2):** Effectful. Sync script = HTTP I/O. AGENTS.md creation = file write. Verification predicate = pure.
- **Boundary cuts (§3):** Three knowledge surfaces — AGENTS.md (repo root, agent entry point), docs/okf/ (curated concept docs), mem-store shard (vector mirror). AGENTS.md bridges agents to both.
- **Module responsibility (§4):** AGENTS.md header names WHAT (repo grounding), WHERE (repo root — first file agents read), WHAT NOT (not a concept doc, not a feedback log, not user-level config).
- **Function discipline (§5):** Sync script = one job (mirror OKF → mem-store). Idempotent. Verification predicate = one job (prove mirror succeeded).

### Technical Context
- **Files likely touched:** `AGENTS.md` (NEW, repo root), `docs/agent-notes/` (NEW dir convention, created on-demand)
- **Architecture notes:** OKF bundle = 29 files (21 concept docs with `type:` frontmatter + 8 nav files skipped). Sync script walks docs/okf/, POSTs each concept doc to `:4747/api/memories` with `type:"okf"` + container tag, pins each. Container-tag hash = sha256(git-common-dir) first 16 hex = `f45adc73d392ef18`. Shard: `~/.opencode-mem/data/projects/project_f45adc73d392ef18_shard_0.db`.
- **Bidirectional contract resolution:** AGENTS.md preamble states "read me before docs/okf/". docs/okf/index.md NOT edited (lower blast-radius).

### Dependencies
- **Depends on:** OKF bundle exists (29 files), sync script exists, opencode-mem plugin running on :4747, `bun` runtime, `python3` + `curl` + `grep` in probe environment
- **Blocks:** all future ACs that rely on agents having repo grounding or mem-store OKF memories
- **Conflict set:** `AGENTS.md` (root — new file, no conflicts)
- **Risk level:** low

### Progress
- [x] spec complete — 2026-06-25
- [x] AGENTS.md written — 2026-06-25 (all 10 grep anchors pass)
- [x] okf-mem-sync.ts run — 2026-06-25 (21 imported, 0 failed, all pinned)
- [x] Full probe passed — 2026-06-25 (expected=21, pinned=63)
- [x] Commit pushed, PR created — 2026-06-25

### Decision Log
- 2026-06-25 — bidirectional contract: AGENTS.md preamble path chosen (lower blast-radius, no edit to curated bundle)
- 2026-06-25 — API shape corrected by resolver: both speculators got mem API wrong (`data.items` not array, `isPinned` not `pinned`, `?tag=` is container tag)

### Surprises & Discoveries
- OKF bundle already existed at docs/okf/ (commit #28) — initial explore missed it
- Both speculators got mem-store API shape wrong — resolver caught via direct API probe

### Idempotence & Recovery
- Safe retry: re-run `bun ~/.config/opencode/scripts/okf-mem-sync.ts` (dedup 0.90 collapses re-runs)
- Rollback: `git rm AGENTS.md` (single new file, no other changes)
