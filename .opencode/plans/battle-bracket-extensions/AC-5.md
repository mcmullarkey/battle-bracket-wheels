---
ac: 5
depends_on: AC-4
risk: medium
status: spec
---

## AC-5: POST /wheels/populate endpoint — validate, mutate session under single lock, render 8 OOB wheel fragments + status

### Executable Spec
- **predicate:** POST /wheels/populate with form field "items" containing 16 newline-separated entries → HTTP 200, Content-Type: text/html, response body contains exactly 8 hx-swap-oob="true" elements with IDs slot-1..slot-8 (rendered via existing nextRoundSlot template, each containing round-robin-distributed wheel SVG) AND 1 non-OOB element (populate-status). MUTATION GATE (read back via store.View): s.Wheels[i].Options matches populate.Result.Wheels[i].Options for all 8 wheels (round-robin + replace verified — pre-existing "OLD" option absent), s.Bracket is fresh *bracket.Bracket from bracket.New(result.Wheels) (pointer != pre-populate, s.Bracket.Slots == result.Wheels), s.ResolvedMatches is fresh make(map[string]bool) (len==0, != nil, != pre-populate reference). All three mutations within single store.Update closure. Idempotency NOT enforced (second populate replaces again).
- **probe:** go test -race ./... -run TestPopulate. Test mirrors TestBattleHandler_PostBattleStoreState (handlers_battle_test.go:416-491): httptest.NewServer + http.DefaultClient.Do(POST) + store.View read-back.
- **negative:** (1) POST with 7 entries → 400, JSON {"error":"...at least 8 entries..."}, session unchanged (store.View read-back confirms). (2) Sneaky-pass: handler renders OOB from result.Wheels but store.Update closure has dead code/early return that never assigns s.Wheels → response check passes but mutation-gate fails.
- **verification:** code (Go test: HTTP request via httptest + store.View read-back)
- **fixture status:** NEW handlers_populate.go + handlers_populate_test.go; MODIFY main.go (register POST /wheels/populate under sessionMiddleware), templates/bracket.html (add populateStatus fragment — nextRoundSlot reused as-is)
- **rubric anchor:** §2 (pure/effectful — pure parser AC-4 → effectful handler AC-5, mutation under single lock), §3 (module cut — new handlers_populate.go isolates endpoint)

### Design Intent
- **Types / interfaces (§1):** Handler consumes typed populate.Result{Wheels [8]wheel.Wheel, Entries []string} from AC-4. No untyped data crosses boundaries.
- **Pure / effectful (§2):** Pure parser (AC-4) → effectful handler (AC-5). Handler is thin shell: parse form → delegate to populate.ParseAndDistribute → mutate under single store.Update lock → render. Render AFTER Update (read mutated session, not Result directly).
- **Boundary cuts (§3):** New file handlers_populate.go (isolated from handlers_battle.go/handlers_wheel.go). Route in main.go. populateStatus template fragment co-located in bracket.html.
- **Module responsibility (§4):** handlers_populate.go = POST /wheels/populate endpoint only. Reuses existing helpers: writeJSONError, wheelViewFromWheel, slotIDFromWheelIdx, nextRoundSlot template.
- **Function discipline (§5):** Handler does one thing (populate). Testable without patches — httptest.Server + injected *Store. Refusal arms mirror addOptionHandler: GetCookie→401, ParseForm→400, empty items→400, ErrTooFewEntries→400, store.Update ErrSessionNotFound→401, else→500.

### Technical Context
- **Files:** main.go (route: mux.Handle("POST /wheels/populate", sessionMiddleware(store, populateHandler(store, tmpl)))), handlers_populate.go NEW, handlers_populate_test.go NEW, templates/bracket.html (add {{define "populateStatus"}}<div id="populate-status">...</div>{{end}})
- **Architecture notes:** Handler flow: r.ParseForm() → items := r.FormValue("items") → result, err := populate.ParseAndDistribute(items) → on ErrTooFewEntries: writeJSONError(w, 400, "at least 8 entries required"); on success: store.Update(sessionID, func(s *Session) error { s.Wheels = result.Wheels; s.Bracket = bracket.New(result.Wheels); s.ResolvedMatches = make(map[string]bool); return nil }) → render 8 nextRoundSlot OOB fragments (slotIDFromWheelIdx(i) for i=0..7) + 1 populateStatus non-OOB. Error format: JSON via writeJSONError. HTMX 2.x requires ≥1 non-OOB swap target — populate-status satisfies this.

### Stable Interface for AC-6
- Endpoint: POST /wheels/populate
- Form field: "items" (newline-separated text)
- Success (200): text/html — 8 OOB slot-N fragments + 1 non-OOB populate-status
- Error (400): application/json — {"error": "at least 8 entries required"}
- Error (401): application/json — {"error": "session required"}
- HTMX swap: form hx-target="#populate-status"; 8 OOB slot-N fragments auto-swap

### Dependencies
- **Depends on:** AC-4 (populate.ParseAndDistribute, Result, ErrTooFewEntries — RESOLVED)
- **Blocks:** AC-6 (UI form submission — consumes stable endpoint contract above)
- **Conflict set:** main.go (AC-2/AC-5 route registration), templates/bracket.html (AC-5/AC-6)
- **Risk level:** medium (mutates session state under write lock, new endpoint, race detector mandatory)

### Progress
- [ ] Red: write integration test, confirm fails
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

### Decision Log
- 2026-07-26 — Mutation gate (store.View read-back): follows TestBattleHandler_PostBattleStoreState convention. Verifies actual session state, not just response HTML.
- 2026-07-26 — JSON error format via writeJSONError: matches existing addOptionHandler convention. Not HTML error pages.
- 2026-07-26 — Reuse nextRoundSlot template: no new OOB template needed. Only populateStatus (non-OOB) is new, co-located in bracket.html.
- 2026-07-26 — 6 refusal arms matching addOptionHandler: GetCookie→401, ParseForm→400, empty→400, ErrTooFewEntries→400, ErrSessionNotFound→401, else→500.

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run go test -race ./... -run TestPopulate
- Rollback: delete handlers_populate.go + handlers_populate_test.go + revert main.go route + revert bracket.html populateStatus fragment
