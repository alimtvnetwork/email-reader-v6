# Workflow status

Last updated: 2026-04-26 (UTC) — **Slice #46 landed: AC-DB-55 `Test_LogScan_NoOriginalUrlLeak`.** New `internal/core/tools_log_scan_test.go` installs a buffering `slog.Handler` as the process default + redirects the legacy `log` package writer, then drives `Tools.OpenUrl` with `https://user:pw@example.com/login?otp=123456&q=ok` across both the launch-success and dedup-hit branches (and a persistent-write branch with `EmailId=99`). After each pass, scans every captured INFO+ slog record + every `log.Printf` line for `user:pw` / `123456` substrings — fails on any leak. Includes setup-bug guards (asserts the redactor really does scrub canonical, and that the `OriginalUrl` API contract still carries the unredacted form to in-process callers — that's allowed; it's only logs that must stay clean). Custom `captureHandler` implements all four `slog.Handler` methods (Enabled/Handle/WithAttrs/WithGroup) so structured groups + WithAttrs chains are also captured. **Verified:** `go vet -tags nofyne ./...` clean; `internal/core/` + `internal/store/` packages green.

## Previous slice (kept for context)
Slice #45: AC-DB-54 `Test_BooleanPositive` — schema-driven PRAGMA walk rejecting `Is*`/`Has*` columns defaulting to `1`.

## Current milestone
🎯 **Spec-21-app implementation Phase 2** — turning the spec/21-app deltas into shipped code. Spec authoring round (35 tasks) **closed**; tasklist archived to `mem://archive/02-spec-21-app-tasklist`.

## Completed in this implementation round (chronological)

| # | Slice | Files of note |
|---|-------|---------------|
| 1 | `errtrace.Result[T]` foundations + 7 `core.*` migrations | `internal/errtrace/result.go`, all `core/*.go` |
| 2 | Settings backend (`Get`/`Save`/`ResetToDefaults`/`Subscribe`/`DetectChrome`) | `internal/core/settings*.go` |
| 3 | Theme tokens, palettes, sizes, density, fonts, alpha-blend, tnum | `internal/ui/theme/*` |
| 4 | Fyne adapter + bootstrap apply | `internal/ui/theme/fyne_*.go`, `internal/ui/app.go` |
| 5 | Live consumers — CF-W1 poll cadence, CF-T1 browser path, CF-D1 dashboard auto-start, theme live-switch | `internal/ui/watch_runtime.go`, `internal/core/poll_chans.go` |
| 6 | Watch service shell (`core.Watch`, `eventbus.Bus[T]`, factory seam) + Watch view event consumers | `internal/core/watch*.go`, `internal/eventbus/*`, `internal/ui/views/watch*.go` |
| 7 | Doctor / Diagnose / OpenUrl / ExportCsv / RecentOpenedUrls + AccountEvent invalidation hook | `internal/core/tools*.go`, `internal/core/account_events.go` |
| 8 | OpenedUrls Delta #1 PascalCase migration + 6 new columns + TraceId | `internal/store/store.go`, `internal/core/tools_invalidate.go` |
| 9 | OFL font assets dropped (`Inter-Variable.ttf` + `JetBrainsMono-Variable.ttf`) | `internal/ui/theme/fonts/` |
| 10 | `go vet` cleanup + `mem://go-verification-path` codified | `linter-scripts/validate-guidelines.go` |
| 11 | Settings view scaffold — NavSettings + theme/poll/Chrome/density form + pure helpers | `internal/ui/views/settings*.go`, `internal/ui/nav.go` |
| 12 | Real `watcher.Run` behind `core.LoopFactory` (factory + UI runtime singleton) | `internal/core/watch_factory.go`, `internal/ui/watch_runtime.go` |
| 13 | `BridgeWatcherBus` + `TranslateWatcherEvent` (10 EventKind → core.WatchEvent) | `internal/ui/watch_runtime.go`, race tests |
| 14 | Delta #1 activated end-to-end — Recent opens tab, first prod caller of `Tools.RecentOpenedUrls` | `internal/ui/views/tools_recent_opens*.go`, `internal/ui/views/tools.go` |
| 15 | Dashboard live wiring — 5-tile counter row subscribing to `watcher.Bus` | `internal/ui/views/dashboard*.go`, `internal/ui/app.go` |
| 16 | **Audit: Emails / Rules / Accounts views already shipped** (no work needed; previous status was stale) | `internal/ui/views/{emails,rules,accounts}.go` |
| 17 | CF acceptance tests batch #1 — 6/11 (T2/T3×2/R1/W1/A1/A2) | `internal/core/cf_acceptance_*.go` |
| 18 | CF acceptance tests batch #2 — final 5/11 (T1/T4/W3/D1/R2). All 11 spec-mandated CFs now locked. | `internal/browser/cf_t4_test.go`, `internal/ui/cf_runtime_test.go`, `internal/core/cf_d1_dashboard_test.go`, `internal/ui/views/cf_r2_ast_guard_test.go` |
| 19 | Dashboard auto-refresh on EventNewMail — debounced (750 ms) | `internal/ui/views/dashboard_counters.go`, `internal/ui/views/dashboard.go` |
| 20 | Race-build sanity sweep — 16 packages clean under `-race` (single + 10× stress). | `mem://go-verification-path` |
| 21 | CF coverage extension — 6 new informational-CF tests (CF-S-MONO×2, CF-S-EVENT-MONO, CF-AT1/AT2/AT3). | `internal/core/cf_acceptance_settings_extra_test.go`, `internal/core/cf_acceptance_tools_invalidate_test.go` |
| 22 | OpenedUrls retention sweeper — Settings knob `OpenUrlsRetentionDays` + `store.PruneOpenedUrlsBefore` + pure helpers + CF-S-RET test. | `internal/core/settings*.go`, `internal/core/retention.go`, `internal/store/vacuum.go` |
| 23 | Daily retention tick wired — `core.Maintenance` + UI runtime spawn + 6 tests; CF_A2 noted as pre-existing flake (now fixed in #24). | `internal/core/maintenance.go`, `internal/core/maintenance_test.go`, `internal/ui/watch_runtime.go` |
| 24 | CF_A2 race eliminated — `config.WithWriteLock` makes Load+mutate+Save atomic across `AddAccount`/`RemoveAccount`/`Settings.persist`. Bonus: `withTempConfig` test isolation fix. | `internal/config/config.go`, `internal/core/{accounts,settings,doctor_test,diagnose_test}.go` |
| 25 | Settings UI: retention days field — `ParseRetentionDays` + extended `ProjectSettingsInput` + form row + 3 tests. Bonus: dead-import fix in `tools_recent_opens.go`. | `internal/ui/views/settings*.go`, `internal/ui/views/tools_recent_opens.go` |
| 26 | Retention sweep observability — `MaintenanceOptions.OnSweep` wired to `log.Print` via `FormatRetentionSweep`. Format pinned by 4-case test. | `internal/ui/maintenance_log.go`, `internal/ui/watch_runtime.go` |
| 27 | Frontend `CODE-RED-004` cleanup — extracted `useLog404` hook + `NotFoundContent` subcomponent in `src/pages/NotFound.tsx`. | `src/pages/NotFound.tsx` |
| 28 | **Watch backoff jitter (CF-W-BACKOFF)** — added `internal/watcher/backoff.go` with the pure `NextPollDelay(base, streak, jitter)` (doubling pattern, capped at 5min, full additive jitter). `pollState` gained `consecutiveErrors`; `logPollError` increments, `handlePollOK` resets. `runLoop`'s `tick.Reset` now goes through `nextDelay`, which logs `"⏳ [alias] backing off after N consecutive error(s): next poll in …"` when the streak is >0. 5 unit tests + 2 CF-W-BACKOFF integration tests; all 16 packages green under `-race -count=2`. | `internal/watcher/{backoff,backoff_test,cf_w_backoff_test,watcher}.go` |
| 29 | ANALYZE-after-N-deletes — `store.Analyze` + `ShouldAnalyze`; cumulative tally + `Analyzer` seam in `core.Maintenance`; `FormatAnalyzeRun` log line. | `internal/store/{vacuum,vacuum_test}.go`, `internal/core/{maintenance,maintenance_analyze_test}.go`, `internal/ui/{maintenance_log,maintenance_log_test,watch_runtime}.go` |
| 30 | **Weekly VACUUM + 6-hourly `wal_checkpoint(TRUNCATE)` (completes spec/23-app-database/04 §2 rows 4-5)** — `internal/store/vacuum.go` gained `Vacuum`/`FreelistRatio`/`ShouldVacuum`/`WalCheckpointTruncate`. New `internal/core/schedule.go` introduces `ShouldRunWalCheckpoint` and `ShouldRunWeeklyVacuum`. `core.MaintenanceOptions` grew `Vacuumer`/`VacuumGate`/`WalCheckpointer` seams + cadence knobs + observers. New tests across store/core/ui packages. | `internal/store/{vacuum,vacuum_jobs_test}.go`, `internal/core/{schedule,schedule_test,maintenance,maintenance_jobs_test}.go`, `internal/ui/{maintenance_log,maintenance_log_jobs_test,watch_runtime}.go` |
| 31 | **Settings UI surface for maintenance knobs** — `SettingsInput`/`SettingsSnapshot`/`DefaultSettingsInput` extended with `WeeklyVacuumOn`/`WeeklyVacuumHourLocal`/`WalCheckpointHours`/`PruneBatchSize`; `validateMaintenanceKnobs` (`ER-SET-21778`); 4 new form rows + parsing helpers; `maintenanceOptionsFor` reads live snapshot via `snapshotForMaintenance`. | `internal/core/{settings,settings_types,settings_validate,settings_maintenance_test}.go`, `internal/ui/views/{settings,settings_logic,settings_logic_test}.go`, `internal/ui/watch_runtime.go` |
| 32 | **Chunked `PruneBatchSize`** — `store.PruneOpenedUrlsBeforeBatched(ctx, cutoff, batchSize)` loops `DELETE … WHERE rowid IN (SELECT rowid … LIMIT ?)`; `DefaultPruneBatchSize = 5000`. Runtime closure reads live `PruneBatchSize` from settings. Closes AC-DB-43. | `internal/store/{vacuum,vacuum_batched_test}.go`, `internal/ui/watch_runtime.go` |
| 33 | **Structured-logging migration** — `internal/ui/maintenance_log.go` adopts `log/slog` with package-private `maintenanceSlog` carrying `component=maintenance`; INFO on success / WARN on error; `Format*` helpers re-canonicalised to `event=…` tail; emit-test verifies all 4 callbacks. | `internal/ui/{maintenance_log,maintenance_log_test,maintenance_log_jobs_test,maintenance_log_emit_test}.go` |
| 34 | **AC-DB-47 AST guard** — `Test_AST_MaintenanceOnly` walks repo, parses every production `.go`, rejects any `*ast.BasicLit` whose trimmed body equals `VACUUM`/`ANALYZE` or starts with `pragma wal_checkpoint`. Allowlist: `internal/store/vacuum.go`. Statement-shaped matcher avoids false positives on UI labels. | `internal/store/ast_maintenance_only_test.go` |
| 35 | **AC-DB-50 AST guard** — `Test_AST_DriverImportLimit` walks repo with `parser.ImportsOnly`, rejects any production `.go` outside `internal/store/` that imports a known SQL driver. `driverImportPaths` covers modernc/sqlite, mattn/go-sqlite3, lib/pq, jackc/pgx/v5(+stdlib), go-sql-driver/mysql, microsoft/go-mssqldb, and bare `database/sql/driver`. Reuses `repoRootForMaintenanceGuard` + `skipUninterestingDir` from #34. | `internal/store/ast_driver_import_limit_test.go` |
| 36 | **AC-DB-51 AST guard** — `Test_AST_NoSqlTypeLeak` walks `internal/store/`, parses every production `.go`, and for each top-level `*ast.FuncDecl` whose name + receiver are both exported, walks the return list with `ast.Inspect` to find any `*ast.SelectorExpr` `sql.{DB,Tx,Rows}` at any depth. **Architectural gap noted out of scope:** `Store.DB *sql.DB` is exported and read by `internal/exporter` + 2 `internal/core/tools_*.go` files; closing that leak is AC-DB-52's territory. | `internal/store/ast_no_sql_type_leak_test.go` |
| 37 | **AC-DB-53 RFC 3339 UTC datetime storage + test** — new `internal/store/datetime.go` exports `formatRFC3339UTC(t)` (returns `2006-01-02T15:04:05.000Z` or `""` for zero) and `sqliteRFC3339NowExpr` (`(strftime('%Y-%m-%dT%H:%M:%fZ','now'))`). `migrate()` swaps three `DEFAULT CURRENT_TIMESTAMP` (Emails.CreatedAt / WatchState.UpdatedAt / OpenedUrls.OpenedAt) for the strftime expr. `UpsertEmail` binds `formatRFC3339UTC(e.ReceivedAt)`; `UpsertWatchState` extracted to a named const + swaps inline `CURRENT_TIMESTAMP` (×2). Reads unchanged — `sql.NullTime` parses both formats (probe-verified). New `datetime_test.go` regex-matches every datetime column via `CAST(... AS TEXT)`. | `internal/store/{datetime,datetime_test,store}.go` |
| 38 | **AC-DB-54 `Test_BooleanPositive`** — schema-driven test: walks `sqlite_master` → `PRAGMA table_info(<t>)` for every user table; rejects any `Is*`/`Has*` column whose default is `1`. Helpers: `userTables`, `tableInfo`, `quoteIdent`, `isBooleanPrefixed` (PascalCase-aware: `IsDeduped` ✓, `Issue` ✗). `IsDeduped`/`IsIncognito` both default to `0` → green. | `internal/store/boolean_positive_test.go` |
| 39 | **AC-DB-55 `Test_LogScan_NoOriginalUrlLeak`** — installs buffering slog handler + log writer; drives `Tools.OpenUrl` with userinfo+otp URL across launch / dedup / persistent-write branches; asserts no INFO+ record contains the pre-redaction sensitive substrings. Custom `captureHandler` covers WithAttrs/WithGroup. | `internal/core/tools_log_scan_test.go` |

Verification: 16 packages green under `nix run nixpkgs#go -- {vet,test} -tags nofyne -race -count=2 ./...`; `go build -tags nofyne ./...` clean; new file `datetime.go` 0 fn-length violations (96 in `store.go` are pre-existing, unchanged by this slice).

## Remaining tracked work

### A. spec/23-app-database — sandbox-runnable AC tests still missing (named in spec/23-app-database/97-acceptance-criteria.md)

Behaviour-equivalents may already exist in `internal/store/store_test.go` / `internal/core/cf_acceptance_*_test.go` under different names, but the spec-named tests below have not been added yet:

- AC-DB-01 `Test_Open_FreshSchema`
- AC-DB-02 `Test_Schema_ColumnsMatchSpec`
- AC-DB-03 `Test_Schema_IndexesMatchSpec`
- AC-DB-04 `Test_Email_UniqueAliasMessageId`
- AC-DB-05 `Test_OpenedUrl_Dedup_PartialIndex`
- AC-DB-06 `Test_OpenedUrl_FkSetNull`
- AC-DB-07 `Test_OpenedUrl_OriginCheck`
- AC-DB-08 `Test_OpenedUrl_DecisionCheck` *(blocked on `Decision` column landing)*
- AC-DB-09 `Test_Email_HasAttachmentCheck`
- AC-DB-10 `Test_Store_PragmaOnEveryConn`
- AC-DB-11 `Test_Store_WalPersists`
- AC-DB-20 `Test_Queries_AllImplemented` *(blocked on `internal/store/queries/` landing)*
- AC-DB-21 `Test_AST_NoStraySql` *(needs allowlist for store/queries + store/migrate)*
- AC-DB-22…26 query-behaviour tests
- AC-DB-27 `Test_Queries_PlanGolden`
- AC-DB-28 `Test_Queries_Perf`
- AC-DB-30…36 migrate suite *(blocked on `internal/store/migrate/` landing)*
- AC-DB-37 `Test_AST_DdlOnlyInMigrate` *(blocked on migrate package; meanwhile no DDL exists outside the bootstrap path)*
- AC-DB-40…46 maintenance-behaviour tests *(spec-named — present behaviour is locked under different names today)*
- AC-DB-52 `Test_AST_CoreUsesStoreOnly` *(would surface today's `Store.DB` exported-field leak; pair with a typed-method shim slice first)*
- ~~AC-DB-54 `Test_BooleanPositive`~~ ✅ landed in Slice #45
- ~~AC-DB-55 `Test_LogScan_NoOriginalUrlLeak`~~ ✅ landed in Slice #46

### B. Long-running cross-cutting items
1. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch (incl. the 4 maintenance knob rows), density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB, retention-days field round-trips a Save, all four canonical maintenance log lines appear (`event=prune`, `event=analyze`, `event=wal_checkpoint`, `event=vacuum`) at the configured cadences, and a flaky-network simulation produces the `⏳ backing off after N consecutive error(s)` line. (Requires manual user run.)
2. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
3. **Static "Accounts" / "Rules enabled" tile auto-refresh** — hook the dashboard refresh into `core.AccountEvent` and a future `core.RuleEvent`.
4. **CI integration of `-race`** — gate PR merges on race-detector once external CI runner lands.
5. **Per-decision prune queries** — `Q-OPEN-PRUNE-LAUNCHED` (365d) vs `Q-OPEN-PRUNE-BLOCKED` (90d) split, blocked on the OpenedUrls `Decision` column landing.

## Next logical step for the next AI session
With the two trivial behavioural slices closed (AC-DB-54, AC-DB-55), the next sandbox-runnable item is the **`Store.DB *sql.DB` typed-shim refactor** — it's the prerequisite for AC-DB-52 and closes the architectural leak surfaced in Slice #36's note. Then AC-DB-52 itself (one-file AST guard) ships green.

Recommended order:

(a) **`Store.DB` typed-shim refactor** — survey `internal/exporter` + `internal/core/tools_*.go` callers; add typed `Store.*` methods (e.g., `Store.QueryEmailExportRows(ctx, …)`, `Store.RecentOpenedUrlsRich(ctx, …)`); demote `Store.DB` to lowercase `db`. Net-zero behaviour change but unlocks AC-DB-52 + AC-DB-51's stronger form.

(b) **AC-DB-52 `Test_AST_CoreUsesStoreOnly`** — `parser.ImportsOnly` walk of `internal/core/*` rejecting any direct `database/sql` or driver imports. One file, identical idiom to Slice #35.

Audit-roadmap items (higher-impact, each ≥ multi-slice) for later cycles: Symbol-Map sweep across `01-backend.md` files, schema reconciliation (singular vs plural / missing `WatchEvent`), commit `linters/*.sh` scripts, A11y rewrite (ARIA → Fyne primitives), `errtrace/codes_gen.go` codegen, `goleak` + `*_bench_test.go` per feature, `EventHeartbeat` → `EventPollHeartbeat` rename + export `BackoffLadder`.

Recommended for the next `next` slice: **(a) `Store.DB` typed-shim refactor**.
