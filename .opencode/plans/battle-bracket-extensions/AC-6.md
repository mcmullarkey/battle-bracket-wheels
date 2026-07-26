---
ac: 6
depends_on: AC-5
risk: medium
status: complete
---

## AC-6: Auto-populate UI — textarea+submit form, HTMX re-render 8 QF slots, visible error surface

### Executable Spec
- **predicate:**
  (1) FORM STRUCTURE (GET /): response body contains .populate-form with textarea[name="items"], button[type="submit"], hx-post="/wheels/populate", hx-target="#populate-status".
  (2) SUCCESS (POST 8 valid items): handler returns 200, body contains 8 hx-swap-oob fragments targeting #slot-1..#slot-8 (exact spelling — not slot-01), fragment option text, AND 1 non-OOB swap (HTMX 2.x requirement).
  (3) ERROR (POST < 8 items, empty, whitespace-only): handler returns 400, error text rendered inside #populate-status, AND element user-visible: getComputedStyle(#populate-status).display !== 'none' AND getComputedStyle(#populate-status).visibility !== 'hidden' AND getComputedStyle(#populate-status).opacity !== '0' AND #populate-status.getBoundingClientRect().height > 0. Form remains interactive after error.
- **probe:** go test -race -run TestPopulateUI (httptest server contract) + rodney (client-side computed visibility + slot re-render)
- **negative:** handler returns error inside <div id="populate-status" style="display:none"> — element exists in DOM, passes querySelector, but user never sees it. Caught by getComputedStyle.display !== 'none' AND getBoundingClientRect.height > 0. Also: slot ID misspelled as slot-01.
- **verification:** code + rodney (httptest: status codes, OOB fragment count, error text, no inline display:none + rodney: computed visibility + slot re-render)
- **fixture status:** NEW testPopulateServer helper in handlers_populate_test.go; rodney fixture script at docs/evidence/<issue>/populate-rodney.sh
- **rubric anchor:** §1 (Renderer interface reuse), §2 (handler = thin shell), §3 (populate form as separate {{define}} template — cuts layout.html joint), §5 (handler one responsibility: wire form to endpoint)

### Design Intent
- **Types / interfaces (§1):** Reuse existing Renderer/template-parse pipeline. OOB slot fragments reuse wheel template. Form data: r.ParseForm() → r.Form["items"] → AC-5 populate logic.
- **Pure / effectful (§2):** handlers_populate.go handler thin shell: parse form → delegate to AC-5 populate → render OOB fragments or error. No business logic in AC-6.
- **Boundary cuts (§3):** Populate form lives in templates/populate.html {{define "populateForm"}}, included in layout.html via single {{template "populateForm"}} line. Follows existing repo convention (wheel.html/bracket.html/match.html all use {{define}} + {{template}}). Minimizes hot-conflict with AC-2 on layout.html to one line.
- **Module responsibility (§4):** populate.html header: "Auto-populate form textarea + submit, renders in layout, swaps #populate-status on response. NOT: populate logic (AC-5), wheel rendering (wheel.html), bracket layout (layout.html)."
- **Function discipline (§5):** Handler one thing: translate HTTP POST → AC-5 populate call + HTMX response assembly. No direct session.Wheels mutation in AC-6 (delegated to AC-5).

### Technical Context
- **Files:** templates/layout.html (single {{template "populateForm"}} line near bracket region), templates/populate.html NEW ({{define "populateForm"}} with .populate-form, textarea[name="items"], button[type="submit"], #populate-status div), static/css/space.css (.populate-form scoped styles), main.go (//go:embed templates/populate.html + route if not AC-5)
- **Architecture notes:** HTMX OOB: 8 OOB (#slot-1..#slot-8) + 1 non-OOB (#populate-status). Non-OOB REQUIRED for HTMX 2.x. 400 error renders into #populate-status (non-OOB, visible). hx-target="#populate-status" on form. Error text rendered into already-visible div — NOT inline CSS. Form CSS scoped .populate-form in space.css.
- **Optional enhancements (recommended if low-cost):** echo-back user input on error (preserve textarea value), hx-disabled-elt on submit button (double-submit prevention), <label for="populate-items"> + aria-live="polite" on #populate-status (accessibility).

### UI Block
- **selectors:** .populate-form, textarea[name="items"], button[type="submit"], #populate-status, #slot-1..#slot-8
- **layout_assertions:** .populate-form display !== none, textarea exists with rows >= 3, submit button visible, #populate-status exists, form hx-post="/wheels/populate", form hx-target="#populate-status"
- **deterministic_check:** rodney: form exists + textarea + submit + hx-post + hx-target. Error case: submit 3 items → 400 + #populate-status visible (getComputedStyle display/visibility/opacity + getBoundingClientRect height > 0). Success case: submit 8 items → 8 slots with option text.
- **subjective_residual:** form styling matches space theme (glassmorphism, neon glow on focus), error text visual urgency, layout spacing, textarea comfortable row count

### Dependencies
- **Depends on:** AC-5 (POST /wheels/populate endpoint contract: 200→8 OOB+1 non-OOB, 400→error, replace mode). AC-6 wires UI to AC-5's endpoint; cannot succeed until AC-5 ships.
- **Blocks:** none (terminal AC in auto-populate chain: AC-4→AC-5→AC-6)
- **Conflict set:** templates/layout.html (AC-2/AC-6 — mitigated by single {{template}} line), main.go (AC-2/AC-5/AC-6 — embed directive + route), static/css/space.css (AC-1/AC-2/AC-6 — .populate-form scoped block)
- **Risk level:** medium (depends on AC-5 contract; hot-conflict on 3 files with parallel AC-2 work)

### Progress
- [x] Red: write integration test, confirm fails — 2026-07-26T13:40
- [x] Inner loop: unit red → code → unit green → refactor — 2026-07-26T13:44
- [x] Green: integration passes → commit — 2026-07-26T13:44
- [x] E2E self-validation: produce evidence at docs/evidence/42/ — 2026-07-26T13:46

### Decision Log
- 2026-07-26 — Separate template file (templates/populate.html): follows repo convention (wheel.html/bracket.html/match.html all use {{define}} + {{template}}). Isolates layout.html conflict with AC-2 to single line.
- 2026-07-26 — Error visibility enforced: getComputedStyle display/visibility/opacity + getBoundingClientRect height > 0. Catches display:none/visibility:hidden/opacity:0/zero-height sneaky-passes.
- 2026-07-26 — Form field "items": matches AC-5 endpoint contract.
- 2026-07-26 — Changed error response from JSON to HTML: AC-5 used writeJSONError (JSON), but AC-3 requires error text rendered inside #populate-status as HTML for HTMX swap. Created writePopulateError helper that renders populateStatus template with IsError=true. Updated TestPopulateHandler_TooFewEntries to expect text/html instead of application/json.
- 2026-07-26 — Added IsError field to populateStatusData: enables conditional populate-error CSS class for visual distinction between success and error states. Minimal change to bracket.html template (one {{if}} addition).
- 2026-07-26 — Added hx-disabled-elt on submit button (double-submit prevention), aria-live="polite" on #populate-status (accessibility), label for textarea (accessibility). Low-cost optional enhancements from spec.

### Surprises & Discoveries
- AC-5's TestPopulateHandler_TooFewEntries expected JSON error response (application/json + json.Decode). AC-6 changes error rendering to HTML fragments for HTMX swap compatibility. Updated the test to expect text/html + string matching instead of JSON decoding. This is a shared-behavior change — the test was updated in the same commit as the handler change.
- The populateStatus template in bracket.html renders a full <div id="populate-status"> element. When HTMX swaps this into the target #populate-status (innerHTML), it creates nested divs with the same ID. This is the existing pattern from AC-5's success case — not ideal but consistent. The outer div has aria-live="polite" for screen reader announcements.

### Idempotence & Recovery
- Safe retry: re-run go test -race -run TestPopulateUI
- Rollback: delete templates/populate.html + revert layout.html {{template}} line + revert space.css .populate-form + revert main.go embed
