# Workflow status

Last updated: 2026-04-26 (UTC) — Daily retention tick wired: `core.Maintenance` service runs `PruneOpenedUrlsBefore` on a goroutine, started by the UI runtime; pre-existing CF_A2 flake noted (passes 5/5 in isolation).

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
| 22 | **OpenedUrls retention sweeper (this slice)** — Settings now exposes `OpenUrlsRetentionDays` (uint16; default 90, 0 = disabled, validated 0..3650 with new code `ER-SET-21782`). The new field round-trips through the camelCase extension JSON block (`openUrlsRetentionDays`) and is included in `SettingsSnapshot`. The store gained `PruneOpenedUrlsBefore(ctx, cutoff time.Time) (int64, error)` performing a single bounded DELETE on `OpenedAt < ?` (zero `cutoff` → no-op). Two pure scheduling helpers `RetentionCutoff` and `ShouldRunRetentionTick` in `internal/core/retention.go` decide *what cutoff* and *whether the daily tick is due* (interval defaults to 24h; gated to false when retention is disabled). Tests: `internal/store/vacuum_test.go` (2 cases: deletes only old rows / zero cutoff is no-op), `internal/core/retention_test.go` (4 + 8 table-driven cases for the helpers), `internal/core/cf_acceptance_retention_test.go` (CF-S-RET — full chain: Settings default cutoff prunes only the back-dated row; days=0 is a no-op; round-trip through Save/Get + 9999-day rejection). The actual daily tick wiring into the watcher loop is *not* in this slice — the helpers are deliberately consumer-agnostic so the tick can be added behind the existing `core.LoopFactory` seam later without re-touching store/Settings. `snapshotFromRaw` was refactored into 3 functions (`resolveSnapshotPaths`, `projectExtension`) to stay under the 15-statement linter cap. | `internal/core/settings_types.go`, `internal/core/settings.go`, `internal/core/settings_validate.go`, `internal/core/retention.go`, `internal/store/vacuum.go`, `internal/errtrace/codes.go`, plus the 3 test files |

Verification: all 16 packages green under `nix run nixpkgs#go -- {vet,test,test -race} -tags nofyne ./...`; fn-length linter still **0/0** across 84 files (post-refactor).

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch, density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB. (Requires manual user run.)
2. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
3. **Daily retention tick wiring** — `RetentionCutoff` + `ShouldRunRetentionTick` + `store.PruneOpenedUrlsBefore` exist; need a goroutine inside (or alongside) the `core.Watch` runner that consults them once per minute and calls Prune when the helper returns true. Could also live in a tiny `core.Maintenance` service to keep Watch focused.
4. **Watch backoff jitter** — the existing watcher loop has fixed cadence (`PollSeconds`); spec mentions exponential backoff with jitter on consecutive `EventPollError`. No code seam yet; implementation + CF-W-BACKOFF test would land together.
5. **Static "Accounts" / "Rules enabled" tile auto-refresh** — would require hooking the dashboard refresh into `core.AccountEvent` and a future `core.RuleEvent`.
6. **CI integration of `-race`** — the local sandbox runs `-race` cleanly; gate PR merges on it once a CI runner with the race-detector lands.
7. **Settings UI surface for retention** — the Settings view does not yet expose the new `OpenUrlsRetentionDays` field; once Density persistence lands the same form should grow a numeric input (range 0..3650, hint "0 = never prune").

## Next logical step for the next AI session
The retention *mechanism* is in place. The two highest-value next slices are: (a) **#3 — wire the daily tick** so retention actually fires in production (small: ~30 lines + a fakeable clock/ticker test), or (b) **#4 — Watch backoff jitter** (greenfield, larger). After those, **#7 — Settings UI surface for retention** is a tiny UI follow-up. Items #1/#2/#5/#6 still need user-side or CI infra.
