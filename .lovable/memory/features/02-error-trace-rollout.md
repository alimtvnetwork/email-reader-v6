---
name: Error-trace migration rollout baseline
description: Phase 1 baseline counts for the errtrace migration. Update after each Phase 2 slice so we know how far we've come.
type: feature
---
# Error-trace migration rollout

## Baseline (2026-04-27, Phase 1 lint scripts wired into run.sh / run.ps1)

Production-code sites that still bypass `internal/errtrace` and therefore
do **not** contribute a `file:line` frame to the chain rendered by
`errtrace.Format` in `cmd/email-read/main.go`.

| Lint script | Sites | Notes |
|---|---|---|
| `linter-scripts/check-no-fmt-errorf.sh`     | **18** | Mostly `internal/store/migrate/m000*` + `internal/ui/views/{settings_logic,rules,emails}.go` + 2 in `internal/watcher/pollonce.go`. |
| `linter-scripts/check-no-bare-return-err.sh`| **24** | Heaviest in `internal/core/settings_validate.go` (8) and `internal/cli/{cli,rules_export}.go` (6). |
| `linter-scripts/check-no-errors-new.sh`     | **4**  | `internal/ui/services.go`, `internal/store/{shims,migrate/migrate}.go`, `internal/ui/views/tools_openurl.go`. |
| **Total**                                   | **46** | |

Already-traced sites: ~912 (`rg -g '*.go' 'errtrace\.' | wc -l`).

## Rules

1. Lint scripts run in `run.sh -d` / `run.ps1 -d` via Step C.1, **warn-only**.
2. CI may flip them by exporting `LINT_MODE=fail` (or `$env:LINT_MODE='fail'`).
3. After every Phase-2 slice, re-run the three scripts and update the table
   above. When all three reach 0, flip the run-script invocation to
   `LINT_MODE=fail` (Phase 2 slice 2.7 in `.lovable/plan.md`).

## Why this matters
Without a frame at every wrap site, `errtrace.Format` shortens to one
line and the user can't paste a useful trace. The 46 production sites
above are exactly the spots where today's logs collapse to a single
message.

## Phase 3.3 — Error Log view shipped (2026-04-27)

- `internal/ui/views/error_log.go` renders the Diagnostics → Error Log
  detail pane: split list (left, newest-first) ↔ monospace trace
  detail (right) + Clear / Copy footer. Defaults to the
  `internal/ui/errlog` singleton; `ErrorLogOptions` provides seams
  for tests.
- `internal/ui/nav.go` gains `NavErrorLog` + a `Group: "Diagnostics"`
  field on `NavItem`. `internal/ui/sidebar_rows.go` (new, fyne-free)
  expands NavItems into a flat row list with one italic group header
  before the first item of each group.
- `internal/ui/app.go::viewFor` dispatches `NavErrorLog` to
  `views.BuildErrorLog` and threads the active window's clipboard.
- Tests added: `views/error_log_test.go` (sort + truncate),
  `ui/sidebar_rows_test.go` (group header inserted exactly once
  before NavErrorLog). `ui/sidebar_test.go` updated to expect 8 nav
  rows. `accessibility/a11y_render_harness_test.go` allowlists
  `views/error_log.go` (passive log surface, default focus order).
- `go vet -tags nofyne ./...` clean. `go test -tags nofyne
  ./internal/ui/...` all green.

## Phase 3.4 — Sidebar unread badge shipped (2026-04-27)

- New fyne-free `internal/ui/sidebar_badge.go::formatNavRowLabel`
  renders "Title  (N)" with a 99+ collapse rule. Pure function;
  unit-tested in `sidebar_badge_test.go` (no build tag, headless).
- `SidebarOptions` gains `BadgeFor(NavKind) int64` and
  `BadgeSubscribe() <-chan errlog.Entry`. Defaults wire NavErrorLog
  to `errlog.Unread()` and the singleton Subscribe channel.
- `NewSidebar` uses the badge in the list binder and starts a
  goroutine that refreshes the list on every Subscribe tick. The
  OnSelected handler also calls `list.Refresh()` after opening
  NavErrorLog because `errlog.MarkRead` (called synchronously
  inside `BuildErrorLog`) does not fan out on Subscribe.
- All `internal/ui/...` tests green under `-tags nofyne`.

## Phase 3.5 — First-error toast shipped (2026-04-27)

**Phase 3 complete.** Every UI error path now: (1) records a full
file:line trace into the in-memory ring, (2) bumps a sidebar badge
with a 99+ collapse rule, (3) raises one desktop toast on the 0→1
unread transition, then stays badge-only until the user opens the
Error Log view (which calls `errlog.MarkRead` + resets the
notifier's quiet-period flag).

- `internal/ui/errlog_notifier.go` (new, fyne-free): `ErrLogNotifier`
  holds an `inStorm` bool guarded by a mutex; `handle` flips it and
  toasts on the first event. `ResetQuietPeriod` re-arms it.
  `NewErrLogNotifier(toast ToastFn)` starts a goroutine on
  `errlog.Subscribe()`. `toastTitle`/`toastBody` are pure helpers
  with a 140-char body cap.
- `internal/ui/errlog_notifier_test.go`: 6 tests covering first-fire,
  storm collapse, reset re-arms, nil-toast no-op, title fallback,
  body truncation. Pure handle-channel bypass = deterministic.
- `internal/ui/sidebar.go`: `SidebarOptions` gains
  `OnErrorLogOpened func()`. The OnSelected handler now calls it
  alongside `list.Refresh()` when NavErrorLog opens.
- `internal/ui/app.go`: `Run()` constructs the notifier with
  `app.SendNotification` as the toast adapter, stores it in the
  package-level `errLogNotifier` var. Both `NewSidebar` calls in
  `BuildShell` thread `OnErrorLogOpened: sidebarErrorLogReset` —
  nil-safe so headless tests (where `Run` is never called) keep
  passing.
- `go vet -tags nofyne ./...` clean. All `internal/ui/...` tests
  green. (Pre-existing failures in `internal/core` from the seeded
  Attobond account polluting fresh-dir tests are orthogonal — not
  introduced by this slice.)

## Phase 2.1 — watcher/pollonce.go migrated (2026-04-27)

3 sites converted to errtrace:
- L66 `Register`: `fmt.Errorf` → `errtrace.New` (no cause).
- L89 `PollOnce` no-account: `fmt.Errorf("…%q", alias)` → `errtrace.Errorf`.
- L96 bare `return err` → `errtrace.Wrap(err, "watcher.PollOnce")`
  (nil-safe, so the success path stays nil).

Linter delta: `check-no-fmt-errorf` 18 → **16**;
`check-no-bare-return-err` 24 → **23**; `check-no-errors-new` 4
(unchanged). Total 46 → **43**.

Watcher tests + UI tests still green under `-tags nofyne`.

## Phase 2.2 — store/migrate m000{5,10,12,14} migrated (2026-04-27)

8 sites across 4 migration files: every `fmt.Errorf("…: %w", err)` →
`errtrace.Wrapf(err, "…")`. Identical mechanical pattern; `fmt`
import removed from each.

- m0005 (opened_urls_audit_columns): 2 sites (introspect + per-column ADD).
- m0010 (add_email_flags): 2 sites.
- m0012 (add_email_deletedat): 2 sites.
- m0014 (watchstate_consecutive_failures): 2 sites.

Linter delta: `check-no-fmt-errorf` 16 → **8** (eight migrations
sites cleared). `check-no-bare-return-err` 23 (unchanged).
`check-no-errors-new` 4 (unchanged). Total 43 → **35**.

`go vet -tags nofyne ./...` clean; `internal/store/...` tests green.

## Phase 2.3 — settings_logic.go migrated (2026-04-27)

5 validation `fmt.Errorf("…")` → `errtrace.New("…")` in
`ParsePollSeconds`, `ParseRetentionDays`, `ParseVacuumHourLocal`,
`ParseWalCheckpointHours`, `ParsePruneBatchSize`. `fmt` import
removed. Error message text preserved exactly — `errtrace.New(s)
.Error() == s` so existing tests / status-bar UX are unchanged.

Linter delta: `check-no-fmt-errorf` 8 → **3**. Total **35 → 30**.

`go vet -tags nofyne ./...` clean; `internal/ui/views/...` tests green.
