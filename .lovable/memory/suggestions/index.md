# Suggestions index

| ID | Status | Priority | Title |
|---|---|---|---|
| 20260427-141500-grow-error-registry | open (steady-state, blocked on STO/DB human verdict — see `mem://workflow/er-code-gap-audit.md`) | high | Grow `06-error-registry.md` (11 remaining Cat-A gaps) |
| 20260427-141501-fix-broken-spec-links | ✅ closed (Slice #164/#172 — `Test_NoBrokenSpecLinks_GreenInCi` already passing) | medium | Fix 33 broken cross-tree spec links |
| 20260427-141502-flip-oi-rows-closed | ✅ closed (Slice #182 — audit-only; OI rows already ✅ Closed in #133-#139, AC-PROJ-35 already off allowlist via #144) | medium | Flip OI-1..6 in tools/99-consistency-report to ✅ Closed |
| 20260427-141503-update-feature-overview-status | ✅ closed (Slice #181) | low | Update feature index UI status `Planned` → `✅ Done` |
| 20260427-141504-restate-watchevents-name-lock | ✅ closed (Slice #183 — added 🔒 callouts to §4 OpenedUrls + §5 WatchEvents in `spec/23-app-database/01-schema.md`, bumped to v1.0.1) | medium | Restate `WatchEvents` table-name lock in `23-app-database/01-schema.md` |
| 20260427-141505-add-sandbox-feasibility-tag | ✅ closed (Slice #184 — legend + per-section tag in 9 AC files via deterministic script; risk-report Tier table cross-referenced) | medium | Add 🟢/🟡/🔴 sandbox-feasibility tag to each per-feature 97 file |
| 20260427-155000-rotate-seeded-credentials-broader-scope | open (blocked on user policy verdict a/b/c — see Slice #185 finding) | **HIGH (security)** | Rotate seeded IMAP credentials — **plaintext password in `internal/config/seed.go` line 33** ships in binary |
| 20260427-160000-errlog-persist-nil-writer-race | open (sandbox-doable, discovered by Slice #189 race-stress sweep) | medium | `errlog.Persistence.Write` panics with nil `*bufio.Writer` under `-race -count=2` — package-level state survives test iterations |

Archive: `.lovable/memory/suggestions/archive/` (empty).

---

## Slice #186 — Closed-via-test (no formal suggestion file)

| Origin | Closed by |
|---|---|
| `.lovable/suggestions.md` "Add unit tests for rules edge cases" | ✅ Slice #186 — `internal/rules/edge_cases_test.go` (4 new tests covering invalid-regex no-panic, ampersand-URL not split, multi-URL open-all, case-sensitivity via inline `(?i)`). Inline `FIXME(rules.go)` comment flags latent `firstErr == nil` short-circuit bug for future fix. |
