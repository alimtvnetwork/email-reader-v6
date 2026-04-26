# Workflow status

Last updated: 2026-04-26 (UTC) — Settings view scaffold shipped

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
| 11 | **Settings view scaffold (this slice)** — NavSettings + theme/poll/Chrome/density form + pure helpers | `internal/ui/views/settings*.go`, `internal/ui/nav.go` |

Verification: 16 packages green under `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`; fn-length linter still **0/0** across 78 files.

## Remaining tracked work

See `spec/21-app/99-consistency-report.md` §6 for the canonical delta list. Open items:

1. **Wire real `watcher.Run` behind `core.LoopFactory`** — thread `config.Account` / `rules.Engine` / `browser.Launcher` / `store.Store` from CLI + UI; bridge `watcher.Bus` events into the `core.WatchEvent` stream (currently the factory uses a placeholder loop).
2. **Activate Delta #1 `OpenedUrls` filters in callers** — schema + columns + filter validation done; no caller passes `Alias` / `Origin` yet through `Tools.RecentOpenedUrls`.
3. **App boot smoke test** (user-side) — once the desktop binary can be launched, validate Settings page renders, theme live-switches, and density toggle visibly tightens paddings.
4. **Persist Density preference** (deferred per design-system §8) — when persistence lands, swap `Settings` view's local-only density handler for a `SettingsInput` field write.

## Next logical step for the next AI session
Pick item **1** — the watcher wiring is the riskiest remaining slice and unblocks real end-to-end verification of the Watch view + dashboard counters.
