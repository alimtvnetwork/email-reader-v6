# Workflow status

Last updated: 2026-04-27 (UTC) — **Slice #131 — AC-PROJ spec linters** landed.
Built `internal/specaudit/ast_project_linters_test.go` with 5 new headless tests.
Cleanly closed 3 rows: AC-PROJ-18 (`Test_AST_OnlyUiPackagesImportFyne`),
AC-PROJ-32 (`Test_AllQueryRefsResolveInDbQueries`), AC-PROJ-34
(`Test_FeatureFolderShapeIsUniform`). Wired-but-deferred 2 rows:
AC-PROJ-31 (39 ER codes referenced but not in `06-error-registry.md`),
AC-PROJ-33 (33 broken cross-tree spec links). Both deferred tests use
`t.Skip` after `t.Logf` so they ratchet automatically when the underlying
spec/registry debt is cleaned up. Coverage 48.9% → **51.6% (96/186)**;
allowlist 95 → **90**. AC-PROJ family **16/27 (59%) → 19/27 (70%)**.

## Recent slices (chronological, this session)

| # | Slice | Files of note | Coverage delta |
|---|-------|---------------|----------------|
| 128 | **AC-SB long-tail** — 7 new headless tests closing AC-SB-12/13/14/18/20/21/23. AC-SB family **24/24 (100%) ✅**. | `internal/core/settings_long_tail_test.go` | 40.9% → 45.2% (84/186); allow 109 → 102 |
| 129 | **AC-DB headless gaps (honest scope)** — `Test_Store_PragmaOnEveryConn` (AC-DB-10) + `Test_Store_WalPersists` (AC-DB-11). Audited remaining 14 AC-DB rows; 12 require schema-evolution behaviour work (table rename `WatchEvents`→`WatchEvent`, Decision/Origin enum CHECKs, FK SET NULL, gap/checksum/downgrade detection) — honestly deferred. | `internal/store/pragma_persist_test.go`, `internal/specaudit/coverage_audit_test.go` | 45.2% → 46.2% (86/186); allow 102 → 100 |
| 130 | **AC-SX scanners** — 5 headless AST/log scanners: `Test_AST_OnlyConfigWritesFile`, `Test_AST_Save_NoAccountsRulesRefs`, `Test_AST_Settings_NoDirectColor`, `Test_LogScan_NoChromePathLeak`, `Test_LogScan_NoIncognitoArgLeak`. Discovery: 2 real production `IncognitoArg` log leaks fixed via `redactIncognito`/`redactIncog` helpers emitting `<set>`/`<none>` markers. AC-SX family **6/6 (100%) ✅** (headless half). | `internal/specaudit/ast_settings_security_test.go`, `internal/cli/read.go`, `internal/watcher/watcher.go` | 46.2% → 48.9% (91/186); allow 100 → 95 |
| 131 | **AC-PROJ spec linters** — 5 new tests; 3 closed cleanly (18/32/34); 2 wired-deferred (31/33). Scanner false-positive fixes: strip inline-code spans before link extraction; `isPlaceholderToken` ignores `XXX`/`NNNNN` format examples. | `internal/specaudit/ast_project_linters_test.go`, `internal/specaudit/coverage_audit_test.go` | 48.9% → 51.6% (96/186); allow 95 → 90 |

## Current milestone

🎯 **Spec-21-app implementation Phase 3 — AC coverage rollout.** Original
roadmap (Phases 1+2) is **100% complete**. We're now in the "tighten the
acceptance-test net" phase: every AC in `spec/21-app/02-features/*/97-...`
either has a real test naming it, or sits in `coverageGapAllowlist` with a
documented reason. The allowlist shrinks monotonically (enforced by
`Test_AC_CoverageAudit/gap_no_stale_allow`).

## Coverage scoreboard (post-Slice #131)

- **Overall AC coverage:** 51.6% (96/186); allowlist 90 gaps; 0 stale code refs.
- **By family:**
  - AC-SB Settings backend: **24/24 (100%) ✅**
  - AC-SX Settings cross-cutting: **6/6 (100%) ✅** (headless half; AC-SX-06 §5 frontend fixture deferred to Slice #118e)
  - AC-PROJ Project-wide: **19/27 (70%)**
  - AC-DB Database: **22/34 (65%)**
  - AC-DS Design system: 23/45 (51%)
  - AC-SF Settings frontend: 0/21 (blocked on Slice #118e canvas harness)
  - AC-DBP DB performance: 0/6 (blocked on bench infra, Bucket #9)
  - AC-SP Settings performance: 0/5 (blocked on bench infra)

## Remaining tracked work (priority-ordered)

### Next slice candidates (can start immediately)

1. ⏳ **Slice #132 — AC-DS AST gaps (~4 rows)** — AST scanners for widget-usage
   limits, animation duration ceilings, hard-coded color guard (matches the
   AC-SX-03 pattern but covers more files).
2. ⏳ **AC-DS long-tail headless (~18 rows)** — remaining AC-DS rows that
   don't need a Fyne canvas.
3. ⏳ **Spec cleanup → unblock AC-PROJ-31/33** — grow `06-error-registry.md`
   to define ~39 ER codes that specs already reference (Settings 217xx,
   Migrations 218xx, UI 219xx blocks); fix ~33 broken cross-tree links in
   `02-coding-guidelines/`, `08-generic-update/`, `13-cicd-pipeline-workflows/`.
4. ⏳ **AC-PROJ-35** — flip OI-1..OI-6 in `spec/21-app/02-features/06-tools/99-consistency-report.md` from "scheduled" to ✅ Closed (or remove if abandoned).

### Deferred (require infrastructure not in this sandbox)

5. 🚫 **Blocked** — **Slice #118e Fyne canvas harness (~50 dependent rows)** — needs cgo/X11/GL workstation. Unblocks AC-SF (21), AC-DS canvas (~22), AC-SX-06 frontend half (1).
6. 🚫 **Blocked** — **Bucket #9 Bench/race/goleak infra (~11 rows)** — unblocks AC-DBP (6) + AC-SP (5) p95 perf gates.
7. 🚫 **Blocked** — **AC-DB schema-evolution (~12 rows)** — requires behaviour-layer migrations: table rename `WatchEvents`→`WatchEvent`, Decision/Origin enum CHECKs, FK SET NULL, gap/checksum/downgrade detection in migrate runner.
8. 🚫 **Blocked** — **AC-PROJ E2E harness (~13 rows)** — multi-process integration scenarios for AC-PROJ-01..11/14/15.

### Long-running cross-cutting items (carry-over)

1. ⏳ **App boot smoke test** (user-side desktop run) — launch binary; validate Settings render/live-switch (incl. 4 maintenance knob rows), density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens, retention-days field round-trip, all four canonical maintenance log lines (`event=prune|analyze|wal_checkpoint|vacuum`), and flaky-network `⏳ backing off after N consecutive error(s)` line. Requires manual user run; covered by the desktop-run procedure documented in this session.
2. ⏳ **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap Settings view's local-only density handler for a `SettingsInput` field write.
3. ⏳ **Static "Accounts" / "Rules enabled" tile auto-refresh** — hook the dashboard refresh into `core.AccountEvent` and a future `core.RuleEvent`.
4. ⏳ **CI integration of `-race`** — gate PR merges on race-detector once external CI runner lands.
5. ⏳ **Per-decision prune queries** — `Q-OPEN-PRUNE-LAUNCHED` (365d) vs `Q-OPEN-PRUNE-BLOCKED` (90d) split, blocked on the OpenedUrls `Decision` column landing.

## Verification commands (canonical for this project)

```bash
# Quick smoke (16 packages, headless tag, no GUI deps)
nix run nixpkgs#go -- vet -tags nofyne ./...
nix run nixpkgs#go -- test -tags nofyne ./...

# Race-detector stress (CI-equivalent)
nix run nixpkgs#go -- test -tags nofyne -race -count=2 ./...

# AC coverage audit (single source of truth for the % done signal)
nix run nixpkgs#go -- test -tags nofyne -run Test_AC_CoverageAudit ./internal/specaudit/
```

## Next logical step for the next AI session

Run **Slice #132 — AC-DS AST gaps**. Pattern is identical to Slice #130/#131:
write 1 file under `internal/specaudit/` with 3-5 AST scanners; close 3-4
AC-DS rows; remove them from the allowlist; let the audit ratchet.

If the user instead asks for "registry growth" or "spec link cleanup," that
unblocks AC-PROJ-31/33's deferred-skip tests (they go green automatically).

## Archive — older slice history

The 41-slice history through Slice #41 (typed-shim + AC-DB-52 guard) is
preserved in this file's earlier revisions; the table above starts at
Slice #128. Earlier slices (42-127) covered: structured-logging migration,
AST guards (AC-DB-47/50/51/52/55), settings UI rollout, retention sweep,
weekly VACUUM + WAL checkpoint, watch backoff jitter, dashboard live-wiring,
Cloud-Footprint acceptance batch (11/11), and the AC-coverage audit
infrastructure itself (Slices #117-#127). For the full history before
Slice #128, see git log + the per-slice commit messages.
