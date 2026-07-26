---
ac: 2
depends_on: none
risk: medium
status: spec
---

## AC-2: Theme infrastructure — registry, POST /theme cookie, server-rendered stylesheet link, dropdown selector

### Executable Spec
- **predicate:**
  (1) REGISTRY (pure, internal/theme): Registry type with Register(key,name,cssPath)/Resolve(key)→(Theme,bool)/Default()→Theme/Names()→[]string. After Register("space","Space","/static/css/space.css"): Resolve("space").CSSPath == "/static/css/space.css"; Resolve("unregistered") == (Theme{}, false); Resolve("../etc") == (Theme{}, false); Default().CSSPath == "/static/css/space.css"; Names() contains "space".
  (2) ENDPOINT: POST /theme form body theme=space → 303 (Location "/") + Set-Cookie bbw_theme=space (HttpOnly, Path=/, SameSite=Lax, Secure=false). POST /theme empty/missing/unknown → 400, no Set-Cookie. GET /theme → 405.
  (3) RENDERING (cookie→href, NOT hardcoded): GET / no cookie → link href="/static/css/space.css". GET / bbw_theme=space → same. GET / bbw_theme=unregistered → href == Default().CSSPath. href ALWAYS registered CSSPath (closed set).
  (4) DROPDOWN: GET / body contains form.theme-selector[action="/theme"][method="post"] with select[name="theme"], one option per registered theme, current theme option carries selected attribute (server-rendered).
  (5) SESSION-INDEPENDENCE: /theme route registered WITHOUT sessionMiddleware. Rotating/deleting bbw_session does not alter bbw_theme.
- **probe:** go test -run TestTheme -race ./...
- **negative:** POST /theme without theme → 400 + no cookie; GET /theme → 405; unknown cookie → fallback to Default; missing HttpOnly/SameSite=Lax/Path=/ or Secure=true → fail. Sneaky-pass: POST always 303 with hardcoded space.css AND layout.html unchanged. Caught by registry unit test (register 2nd test theme, verify Resolve) + href == resolved Theme.CSSPath.
- **verification:** code (httptest body+cookie assertions + go test -race)
- **fixture status:** main_test.go:18 (testTemplate) reused; NEW internal/theme/theme_test.go; NEW handlers_theme_test.go
- **rubric anchor:** §1.2 (closed registry), §1.3 (unknown rejected at boundary), §2.1/§2.4 (effectful themeHandler + pure Registry.Resolve), §3.1/§3.4 (internal/theme decoupled), §4.1/§4.2 (new handlers_theme.go + internal/theme), §5.1 (single-responsibility helpers)

### Design Intent
- **Types / interfaces (§1):** Theme{Key, Name, CSSPath string}. Registry holds closed set. Unregistered key → (Theme{}, false). Illegal states unrepresentable.
- **Pure / effectful (§2):** internal/theme.Registry pure (no I/O, mirrors internal/wheel). themeHandler + setThemeCookie effectful. homeHandler reads cookie → pure Resolve → passes ThemeCSS to template.
- **Boundary cuts (§3):** internal/theme — pure domain, no net/http import. handlers_theme.go — HTTP adapter. layout.html — presentation. bbw_theme cookie mirrors bbw_session attrs.
- **Module responsibility (§4):** internal/theme — what themes exist + resolve key→CSSPath; NOT HTTP, NOT templates. handlers_theme.go — POST /theme adapter: validate→resolve→set cookie→303. homeHandler — read theme cookie, resolve, inject ThemeCSS+Themes into template data.
- **Function discipline (§5):** themeHandler (validate→resolve→setCookie→303); setThemeCookie (mirror SetCookie attrs); resolveTheme (pure wrapper). One responsibility each.

### Technical Context
- **Files:** internal/theme/theme.go NEW, internal/theme/theme_test.go NEW, handlers_theme.go NEW, handlers_theme_test.go NEW, main.go (setupRouter add POST /theme NOT wrapped in sessionMiddleware + main() construct registry), handlers.go (homeHandler read theme cookie + inject ThemeCSS/Themes), templates/layout.html:8 (dynamic link + dropdown form), static/css/space.css (.theme-selector scoped), main_test.go (testTemplate reuse)
- **Architecture notes:** /theme unwrapped — precedent /health, /static/ both unwrapped. bbw_theme independent of bbw_session. Registry constructed once in main(), injected. Dropdown = native form POST → 303 → GET / (full reload, no JS dependency). AC-3 calls registry.Register("y2k","/static/css/y2k.css").

### Dependencies
- **Depends on:** none (foundational infrastructure)
- **Blocks:** AC-3 (Y2K CSS — needs registry.Register); AC-6 (layout.html conflict)
- **Conflict set:** main.go (AC-2/AC-5), templates/layout.html (AC-2/AC-6), static/css/space.css (AC-1/AC-2/AC-6)

### UI Block
- **selectors:** link[rel="stylesheet"], form.theme-selector, select[name="theme"], option[value="space"], option[selected]
- **deterministic_check:** httptest body — strings.Contains(body, link href) + strings.Contains(body, option selected). Rodney supplementary: assert stylesheet href.
- **subjective_residual:** none for AC-2 (dropdown appearance/styling is AC-3 territory)

### Progress
- [ ] Red: write integration test, confirm fails
- [ ] ADR if new interface/boundary
- [ ] Inner loop: unit red → code → unit green → refactor
- [ ] Green: integration passes → commit
- [ ] E2E self-validation: produce evidence at docs/evidence/<issue-number>/

### Decision Log
- 2026-07-26 — internal/theme package (not inline in handlers): follows internal/wheel, internal/battle, internal/bracket convention
- 2026-07-26 — /theme NOT wrapped in sessionMiddleware: theme is per-browser, session-independent. Precedent: /health, /static/ both unwrapped.
- 2026-07-26 — closed-set registry makes path traversal impossible: Resolve(unregistered) → default, never interpolates cookie value

### Surprises & Discoveries
- (none yet)

### Idempotence & Recovery
- Safe retry: re-run go test -run TestTheme -race ./...
- Rollback: revert internal/theme package + handlers_theme.go + layout.html changes + main.go route
