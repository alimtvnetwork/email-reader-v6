# Workflow status

Last updated: 2026-04-26 (UTC) — Settings form now exposes the **Opened-URLs retention (days)** input (0–3650, hint "0 = never prune"); pure helpers `ParseRetentionDays` + extended `ProjectSettingsInput` cover bounds + zero-disables semantics. Bonus fix: dropped a dead `core` import in `tools_recent_opens.go` that was breaking the Fyne (desktop) build.

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
| 25 | **Settings UI: retention days field (this slice)** — added `ParseRetentionDays` (bounds 0..3650, friendly error) to `settings_logic.go`; extended `ProjectSettingsInput` to take a `retentionDays uint16` so the user-edited value flows into `core.SettingsInput.OpenUrlsRetentionDays` (previously the form silently dropped it on every Save, fighting slice #22's persistence path). The Fyne form gains a fifth row "Opened-URLs retention (days)" with hint "0–3650. 0 = never prune.", populated from `snap.OpenUrlsRetentionDays`. `repopulateWidgets` resets it on Reset, `readSettingsInput` parses + projects it, and the widget bundle gains a `retentionEntry *widget.Entry`. Tests added under headless `nofyne`: `Test_ParseRetentionDays_Bounds` (5 valid + 6 invalid inputs), `Test_ProjectSettingsInput_RetentionZeroDisablesPruning`, plus extended invariants test asserting retention round-trips. Bonus fix: removed a dead `core` import in `tools_recent_opens.go` that was breaking the `!nofyne` (desktop) build with `"core" imported and not used`. Verification: `go vet ./...` clean (both tag matrices); `go build -tags nofyne ./...` clean; `-race -count=3 ./internal/{ui/views,core,config}` 3/3 green; fn-length linter clean across edited files. | `internal/ui/views/settings.go`, `internal/ui/views/settings_logic.go`, `internal/ui/views/settings_logic_test.go`, `internal/ui/views/tools_recent_opens.go` |

Verification: 17 packages green under `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`; `go build -tags nofyne ./...` clean; race sweep `-race -count=3 ./...` 16/16 green (no flakes); fn-length linter **0/0** across edited Go files. Fyne (cgo) link unavailable in this sandbox (X11/GL libs absent), so the desktop binary itself is verified by build-flag matrix only — the dead-import fix unblocks the next user-side smoke run.

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch, density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB, **the new retention-days field round-trips a Save**. (Requires manual user run.)
2. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
3. **Watch backoff jitter** — the existing watcher loop has fixed cadence (`PollSeconds`); spec mentions exponential backoff with jitter on consecutive `EventPollError`. No code seam yet; implementation + CF-W-BACKOFF test would land together.
4. **Static "Accounts" / "Rules enabled" tile auto-refresh** — hook the dashboard refresh into `core.AccountEvent` and a future `core.RuleEvent`.
5. **CI integration of `-race`** — gate PR merges on race-detector once external CI runner lands.
6. **Retention sweep observability** — wire a structured-log line on every `OnSweep` (deleted count, error). Currently silent. Becomes critical once anyone leaves the app running for >24h in production.
7. **VACUUM / ANALYZE / WAL checkpoint** — spec/23-app-database/04 §2 documents the full maintenance schedule; we only ship the prune part. ANALYZE-after-1k-deletes and weekly-VACUUM are still TODO.
8. **Frontend `CODE-RED-004` cleanup** — `src/pages/NotFound.tsx` (16 lines) needs extraction into a smaller helper. Trivial.

## Next logical step for the next AI session
Top candidates (sandbox-runnable): (a) **#6 retention sweep logging** — single log line + 1 test, makes slices #22/#23 actually observable; (b) **#3 Watch backoff jitter** — the next greenfield feature slice; (c) **#7 ANALYZE-after-N-deletes** in `internal/store/vacuum.go`; (d) **#8 NotFound.tsx fn-length cleanup** — 5-minute lint hygiene win. Items #1/#2/#4/#5 still need user-side or CI infra.
