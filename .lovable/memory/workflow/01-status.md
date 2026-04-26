# Workflow status

Last updated: 2026-04-26 (UTC) — Three slices landed in one session: #27 NotFound.tsx fn-length cleanup; #28 Watch backoff jitter (CF-W-BACKOFF) — `NextPollDelay(base, streak, jitter)` + `nextDelay` wired into `runLoop`, with `pollState.consecutiveErrors` reset on every `EventPollOK`; #29 ANALYZE-after-N-deletes — `store.Analyze` + `ShouldAnalyze`, plus a cumulative tally + `Analyzer` seam in `core.Maintenance` with new `OnAnalyze` observer surfaced via `FormatAnalyzeRun`. All 16 packages still green under `-race -count=2`.

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
| 29 | **ANALYZE-after-N-deletes (this slice, completes spec/23-app-database/04 §2 row 3)** — `internal/store/vacuum.go` gained `Analyze(ctx)` + the pure `ShouldAnalyze(cum int64) bool` plus `AnalyzeThreshold = 1000`. `core.MaintenanceOptions` gained an optional `Analyzer` seam + `OnAnalyze` observer + `AnalyzeThresholdRows` mirror const. The per-tick `maybeSweep` now returns `(lastRun, cum)`; a new `maybeAnalyze` fires Analyzer when cum ≥ threshold and resets to 0 (or keeps the tally on Analyzer error so the next sweep retries). Production path in `startMaintenance` wires `rt.Store.Analyze` + `logAnalyzeRun`; `internal/ui/maintenance_log.go` gained `FormatAnalyzeRun(triggeredAt, err)` (`"ui: maintenance: analyze: triggered_at=N {ok|error=…}"`) with format pinned by 2 new tests. New `internal/core/maintenance_analyze_test.go` adds 3 cases (threshold-fire+reset, error-keeps-tally, no-Analyzer-no-fire). | `internal/store/{vacuum,vacuum_test}.go`, `internal/core/{maintenance,maintenance_analyze_test}.go`, `internal/ui/{maintenance_log,maintenance_log_test,watch_runtime}.go` |

Verification: 17 packages green under `nix run nixpkgs#go -- {vet,test} -tags nofyne -race -count=2 ./...`; `go build -tags nofyne ./...` clean; fn-length linter **0/0** across edited Go files.

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch, density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB, retention-days field round-trips a Save, **at least one `ui: maintenance: retention sweep` log line within ~1min of boot**, AND on a populated DB after enough deletes accumulate, an `ui: maintenance: analyze: triggered_at=…` line. Also: simulate a flaky network and confirm the new `⏳ backing off after N consecutive error(s)` line appears. (Requires manual user run.)
2. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
3. **Static "Accounts" / "Rules enabled" tile auto-refresh** — hook the dashboard refresh into `core.AccountEvent` and a future `core.RuleEvent`.
4. **CI integration of `-race`** — gate PR merges on race-detector once external CI runner lands.
5. **Weekly `VACUUM` + `wal_checkpoint(TRUNCATE)`** — spec/23-app-database/04 §2 rows 4-5. ANALYZE landed in #29; the remaining maintenance jobs (weekly Sunday 03:00-local VACUUM gated by free-list ≥5%, plus 6-hourly WAL checkpoint) still need a clock-aware scheduler.
6. **Per-decision prune queries** — `Q-OPEN-PRUNE-LAUNCHED` (365d) vs `Q-OPEN-PRUNE-BLOCKED` (90d) split, blocked on the OpenedUrls `Decision` column landing.

## Next logical step for the next AI session
Top candidates (sandbox-runnable): (a) **#5 weekly VACUUM + WAL checkpoint** — adds a clock-aware scheduler to `core.Maintenance` (Sunday 03:00 local + 6h cadence) plus `store.Vacuum`/`WalCheckpoint`; closes spec/23-app-database/04 §2 in full. (b) **structured-logging migration** — replace stdlib `log.Print` in `internal/ui/maintenance_log.go` with slog now that there are 2 log callsites with shared structure. Items #1/#2/#3/#4/#6 still need user-side, infra, or upstream-schema work.
