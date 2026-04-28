# Suggestions index

| ID | Status | Priority | Title |
|---|---|---|---|
| 20260427-141500-grow-error-registry | ✅ closed (Slice #196 — chose `A1-grow`. Renamed spec ER-STO-* → ER-DB-* across 10 files; renumbered 21100→21101 to match registry; added 5 SQLite-class consts ErrDb{Unique,Fk}Violation/Busy/ReadOnly/Corrupt at 21108-21112; codegen 109→114. Cat A closed entirely. **NEW** 12-code spec-doc gap surfaced — see `mem://workflow/a1-grow-slice196.md`.) | high | Grow `06-error-registry.md` (11 remaining Cat-A gaps) |
| 20260427-141501-fix-broken-spec-links | ✅ closed (Slice #164/#172 — `Test_NoBrokenSpecLinks_GreenInCi` already passing) | medium | Fix 33 broken cross-tree spec links |
| 20260427-141502-flip-oi-rows-closed | ✅ closed (Slice #182 — audit-only; OI rows already ✅ Closed in #133-#139, AC-PROJ-35 already off allowlist via #144) | medium | Flip OI-1..6 in tools/99-consistency-report to ✅ Closed |
| 20260427-141503-update-feature-overview-status | ✅ closed (Slice #181) | low | Update feature index UI status `Planned` → `✅ Done` |
| 20260427-141504-restate-watchevents-name-lock | ✅ closed (Slice #183 — added 🔒 callouts to §4 OpenedUrls + §5 WatchEvents in `spec/23-app-database/01-schema.md`, bumped to v1.0.1) | medium | Restate `WatchEvents` table-name lock in `23-app-database/01-schema.md` |
| 20260427-141505-add-sandbox-feasibility-tag | ✅ closed (Slice #184 — legend + per-section tag in 9 AC files via deterministic script; risk-report Tier table cross-referenced) | medium | Add 🟢/🟡/🔴 sandbox-feasibility tag to each per-feature 97 file |
| 20260427-155000-rotate-seeded-credentials-broader-scope | ✅ closed (Slice #195 — chose `seed-policy: b`. Removed plaintext literal from `seed.go`; `PasswordB64` now resolved from `EMAIL_READ_SEED_PASSWORD` env var at seed time via `resolveSeedPassword`; entries with no env-supplied password are skipped rather than persisted unusable. `internal/config` + `internal/core` green under nofyne.) | **HIGH (security)** | Rotate seeded IMAP credentials — **plaintext password in `internal/config/seed.go` line 33** ships in binary |
| 20260427-160000-errlog-persist-nil-writer-race | ✅ closed (Slice #190 — `Write` now returns `ErrPersistenceClosed` instead of nil-deref; `TestEnableDefaultPersistence_RestoresAcrossInstances` adds `t.Cleanup(resetSingletonForTest)`; new `TestPersistence_WriteAfterCloseIsSafe` pins regression; `errlog` green under `-race -count=2`) | medium | `errlog.Persistence.Write` panics with nil `*bufio.Writer` under `-race -count=2` |
| 20260427-170000-migrate-race-count2-not-idempotent | ✅ closed (Slice #191 — added `SwapRegistryForTest`/`RestoreBaselineRegistryForTest` with once-captured baseline; black-box `migrate_test` now snapshot/restore instead of wipe) | medium | `internal/store/migrate` m0011..m0014 fail under `-race -count=2` |
| 20260427-180000-perfgates-flap-under-race | ✅ closed (Slice #192 — `perfgate_{race,norace}_test.go` build-tag pair + `perfGateSkipRace(t)` helper wired into all 6 PerfGate tests; SKIP under `-race`, RUN otherwise; entire repo green under `-race -count=2`) | low | Wall-clock perf gates flap under `-race` due to detector overhead |
| 20260427-190000-fn-length-large-builders | ✅ closed (Slice #194 — `BuildDashboard` 34→≤15 via 7 helpers + `dashboardActivitySection` + `dashboardContentParts` bag; `BuildErrorLog` 27→≤15 via 6 helpers + `errorLogSelection` holder; `ruleRow` 17→≤15 via `buildRuleEnabledCheck` + `buildRuleActions`; `accountRow` 16→≤15 via `accountRowMeta` + `accountRowDataGrid` + `accountRowActions`. Linter now PASS: 0/0 violations across 146 files. AC-PROJ-20 fully green.) | medium | 4 UI builder functions still exceed 15-statement linter budget (AC-PROJ-20) — `BuildDashboard` 34, `BuildErrorLog` 27, `ruleRow` 17, `accountRow` 16 |

Archive: `.lovable/memory/suggestions/archive/` (empty).

---

## Slice #186 — Closed-via-test (no formal suggestion file)

| Origin | Closed by |
|---|---|
| `.lovable/suggestions.md` "Add unit tests for rules edge cases" | ✅ Slice #186 — `internal/rules/edge_cases_test.go` (4 new tests covering invalid-regex no-panic, ampersand-URL not split, multi-URL open-all, case-sensitivity via inline `(?i)`). Inline `FIXME(rules.go)` comment flags latent `firstErr == nil` short-circuit bug for future fix. |
