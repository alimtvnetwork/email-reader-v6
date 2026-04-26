# Workflow status

Last updated: 2026-04-26 (UTC) — CF acceptance tests batch #1 (6/11)

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
| 17 | **CF acceptance tests batch #1 (this slice)** — 6/11 spec-mandated CFs (T2/T3×2/R1/W1/A1/A2) locked as executable Go tests; 21 PASS sub-tests | `internal/core/cf_acceptance_*.go` |

Verification: 16 packages green under `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`; fn-length linter still **0/0** across 82 files.

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **CF acceptance tests batch #2 — UI-side (5 outstanding)**:
   - CF-T1 — `Test_Tools_OpenUrl_RespectsNewChromePath` (browser launcher live-reload from Settings).
   - CF-T4 — `Test_Browser_IncognitoArg_OverrideVsAuto` (per-browser auto-pick vs. user override).
   - CF-W3 — `Test_Watch_LiveCadenceUpdate` (UI cadence label updates ≤1s of SettingsSaved).
   - CF-D1 — `Test_Dashboard_AutoStartIndicator_Live` (Dashboard indicator reflects AutoStartWatch event).
   - CF-R2 — `Test_AST_RulesUI_NoSchemeBypass` (AST guard: no widget exposes a scheme-allowlist bypass toggle).
2. **App boot smoke test** (user-side) — launch desktop binary; validate Settings render/live-switch, density toggle, Watch Start/Stop, Dashboard tiles incrementing, Recent opens against a populated DB.
3. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.
4. **Dashboard auto-refresh of static tiles on Bus events** — trigger `refresh()` on `EventNewMail` so Emails-stored count auto-bumps without manual refresh.

## Next logical step for the next AI session
Pick item **1** — CF batch #2 (UI-side acceptance tests). Start with CF-T1 + CF-T4 (backend-adjacent: browser launcher contract); CF-W3 + CF-D1 require build-tag `!nofyne` and the WatchRuntime singleton; CF-R2 is a small AST guard scanning `internal/ui/views/rules*.go` for forbidden widget kinds.
