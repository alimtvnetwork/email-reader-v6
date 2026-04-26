# Workflow status

Last updated: 2026-04-26 (UTC) — CF coverage extension: 6 informational CF tests added (UpdatedAt monotonicity + Tools cache E2E)

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
| 20 | **Race-build sanity sweep (this slice)** — `go test -race -tags nofyne -count=1 ./...` PASSED across all 16 packages first try; then `-count=10` over bridge-heavy tests (Bus / Bridge / PollChans / Subscribe / Forward / Dashboard) also clean. No fixes required — every concurrency primitive landed since slice #5 is race-correct. Codified `-race` as the recommended verification step in `mem://go-verification-path` for any future goroutine / channel / shared-state change. | `mem://go-verification-path` |

Verification: 16 packages green under `nix run nixpkgs#go -- {vet,test,test -race} -tags nofyne ./...`; fn-length linter still **0/0** across 82 files.

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch, density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB. (Requires manual user run.)
2. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
3. **CF coverage extension** — regression tests for the four "informational" CFs documented in 99-consistency-report.md but not in the original 11-row matrix: Settings UpdatedAt monotonicity, OpenedUrls retention vacuum, Tools cache invalidation post-AccountEvent, Watch backoff jitter.
4. **Static "Accounts" / "Rules enabled" tile auto-refresh** — currently only "Emails stored" benefits from the auto-refresh hook. Account-add and rule-toggle don't publish on the watcher Bus; if desired, hook into `core.AccountEvent` (already exists) and a future `core.RuleEvent`.
5. **CI integration of `-race`** — the local sandbox now runs `-race` cleanly; consider gating PR merges on it once a CI runner with the race-detector lands.

## Next logical step for the next AI session
**#3 — CF coverage extension** is the next gradable spec contract to lock down. Start with **Settings UpdatedAt monotonicity** (smallest scope: single property, no UI dependency) and **Tools cache invalidation post-AccountEvent** (highest catch potential: re-uses the existing `internal/core/account_events.go` seam). Defer #4 until users ask for it; defer #1/#2/#5 (require user / out-of-sandbox infra).
