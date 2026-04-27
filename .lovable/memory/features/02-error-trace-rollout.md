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

## Phase 2.4 — views/{rules,emails}.go migrated (2026-04-27)

3 sites converted, all string-only constructors:
- `rules.go` L259 (rename validator): `fmt.Errorf("name required")`
  → `errtrace.New(...)`.
- `emails.go` L50 (degraded service banner): `fmt.Errorf(...)`
  → `errtrace.New(...)`.
- `emails.go` L162 (`openURLUnwired`): `fmt.Errorf(...)`
  → `errtrace.New(...)`.

`fmt` import retained in both files (still used for other
formatting). Error strings unchanged → status banners and
validator messages render identically.

Linter delta: `check-no-fmt-errorf` 3 → **0** ✅ (target cleared).
`check-no-bare-return-err` 23 (unchanged).
`check-no-errors-new` 4 (unchanged). Total **30 → 27**.

`go vet -tags nofyne ./internal/ui/views/...` clean; views tests green.

## Phase 2.5 — bare `return err` wrap pass complete (2026-04-27)

23 sites wrapped across 8 files, all with `errtrace.Wrap(err, "<func>: <step>")`.
All wraps are nil-safe (errtrace.Wrap returns nil on nil err) so success
paths remain unchanged.

| File | Sites | Wrap context labels |
|---|---|---|
| `internal/core/settings_validate.go` | 8 | `validateInput.{PollSeconds,Theme,Density,Schemes,ChromePath,IncognitoArg,RetentionDays,MaintenanceKnobs}` |
| `internal/config/seed.go` | 4 | `MarkSeedDeleted.{loadTombstones,marshal,tombstonePath,writeTmp}` (+ rename now wrapped via expression form) |
| `internal/cli/rules_export.go` | 3 | `rules.add: rulesService`, `rules.list: rulesService`, `rules.toggle: rulesService` |
| `internal/cli/cli.go` | 3 | `runWatch: resolveWatchAccount`, `runAdd: promptAddIdentity`, `runAdd: promptAddServer` |
| `internal/ui/watch_runtime.go` | 2 | `watch_runtime: buildLoopFactory`, `watch_runtime: attachWatchAndBridge` |
| `internal/browser/browser.go` | 1 | `browser.Open: resolve path` |
| `internal/core/settings.go` | 1 | `saveRaw: readConfigAsMap` |
| `internal/ui/services.go` | 1 | `openURLAdapter: browser factory` (+ launcher.Open also wrapped) |

Imports: added `errtrace` to `seed.go` and `rules_export.go`; the other
six files already imported it.

Linter delta: `check-no-bare-return-err` 23 → **0** ✅ (target cleared).
`check-no-fmt-errorf` 0. `check-no-errors-new` 4 (unchanged).
**Total backlog 27 → 4** — only the 4 `errors.New` sites remain
before flipping `LINT_MODE=fail`.

`go vet -tags nofyne ./...` clean. Tests green for
`internal/{config,cli,browser,ui,ui/views,ui/accessibility,ui/errlog,ui/theme}`.
`internal/core` shows the same two pre-existing seed-pollution
failures (`TestSettings_Save_UnknownKeys_Untouched`,
`TestSettings_SavePreservesAccountsAndRules`,
`TestDiagnose_NoAccounts`) documented in the Phase 1.5 memory entry —
not introduced by this slice (errtrace.Wrap is nil-safe and the
saveRaw success path is unchanged). Those failures will be cleaned up
in a future slice tracking `withIsolatedConfig`/seed bleed-through.

## Phase 2.6 — `errors.New` migration complete (2026-04-27)

4 sites converted to `errtrace.New` for frame capture:

| File | Site | Notes |
|---|---|---|
| `internal/ui/views/tools_openurl.go:105` | `errToolsFactoryUnavailable` sentinel | added `errtrace` import; dropped `errors`. |
| `internal/ui/services.go:240` | `errBrowserUnavailable` sentinel | dropped `errors` import (only one use). |
| `internal/store/shims.go:372` | `parseSqliteRFC3339` cause inside `errtrace.Wrap` | dropped `errors` import. |
| `internal/store/migrate/migrate.go:163` | `migrate.Apply` nil-db guard | dropped `errors` import. |

Linter delta: `check-no-errors-new` 4 → **0** ✅. All three errtrace
lints (`fmt.Errorf`, bare `return err`, `errors.New`) now report **0
violations**. **Backlog 4 → 0.**

`go vet -tags nofyne ./...` clean. Tests green for
`internal/store/{,migrate,queries}` and `internal/ui/{,accessibility,errlog,theme,views}`.

## Phase 2.7 — lints flipped to `LINT_MODE=fail` (2026-04-27)

All three errtrace guardrail scripts now default to `LINT_MODE=fail`:
- `linter-scripts/check-no-fmt-errorf.sh`
- `linter-scripts/check-no-bare-return-err.sh`
- `linter-scripts/check-no-errors-new.sh`

Header comments updated from "Phase 1 (warn-only)" to
"Phase 2 (enforcing) error-trace guardrail. Default LINT_MODE=fail."
Any future regression — a lone `fmt.Errorf`, bare `return err`, or
`errors.New` in non-test, non-errtrace code — now exits non-zero
and breaks any caller (CI, pre-commit, manual run). Local override
still available via `LINT_MODE=warn ./linter-scripts/check-…sh` for
in-flight refactors.

End of Phase 2 — error trace migration is complete and locked.

## Phase 4.1 — disk persistence for the error-log ring (2026-04-27)

Added `internal/ui/errlog/persist.go` (+ `persist_test.go`, 6 tests).
The in-memory ring now optionally writes through to a JSONL file at
`<dataDir>/error-log.jsonl` and restores prior history on boot.

### Design

- **Format**: newline-delimited JSON. One `Entry` per line (stdlib
  `encoding/json`). Append is O(1); a half-written tail can't corrupt
  the rest; `tail -f`/`jq -c`/`grep` work unchanged.
- **Rotation**: when active file ≥ `sizeCap` (default 5 MiB) after a
  write, rename to `<path>.1` (overwriting any prior rotation), open
  fresh active file. Exactly one rotation kept — in-memory ring (500)
  + one rotation is enough context without growing data/ unbounded.
- **Concurrency**: `Persistence` owns its own mutex; the persister
  callback runs after `Store.mu.Unlock()` so a slow disk never blocks
  in-process subscribers.
- **Restore semantics**: `EnablePersistence(p, prior)` seeds the ring
  with `prior` (trimming to cap if needed), preserves Seq numbers,
  and bumps `nextSeq` to `max(prior.Seq) + 1` so the monotonic
  invariant survives restart. Live appends after restore start at the
  next Seq and write through to disk; the seeded prior entries are
  NOT re-written (they're already on disk).
- **Loader tolerance**: `LoadFromFile` skips unparseable lines
  silently — a half-flushed final line on hard kill doesn't prevent
  restoring everything before it. Missing file → `(nil, nil)` (fresh
  install is not an error).

### Wiring

- `Store` gained a `persister func(Entry)` field, called from
  `Append` after `Unlock`. Default nil → existing tests + headless
  callers see zero behavior change.
- `internal/ui/app.go::Run()` calls `enableErrorLogPersistence()`
  before `errLogNotifier` setup. Resolves `config.DataDir()`, opens
  `error-log.jsonl`, restores prior entries, defers `Close()` on
  shutdown. Failure is logged but non-fatal (degraded mode = in-memory
  only — the user only loses cross-restart history).

### Tests (all green under `-tags nofyne`)

- `TestPersistence_WriteRoundTrip`: marshal → file → load returns
  identical entries.
- `TestLoadFromFile_MissingIsEmpty`: missing path → (nil, nil).
- `TestLoadFromFile_SkipsCorruptLines`: junk + valid mix → only valid
  entries restored.
- `TestPersistence_RotatesAtSizeCap`: tiny cap forces rotation;
  `.1` exists, active file < cap.
- `TestStore_EnablePersistence_SeedsAndAppends`: ring seeded with
  prior; new Append uses `max(prior.Seq)+1`; only the live append
  hits disk (seed isn't re-written).
- `TestStore_EnablePersistence_RingCapTrimsPrior`: cap=3 + 10 prior
  → only last 3 (seqs 8/9/10) survive in the ring.
- `TestEnableDefaultPersistence_RestoresAcrossInstances`: end-to-end
  restart simulation via singleton reset — entries from process #1
  reappear in process #2.

`go vet -tags nofyne ./...` clean. All three errtrace lints still **0
violations** (LINT_MODE=fail). All `internal/ui/...` packages green.

### Files changed
- created `internal/ui/errlog/persist.go`
- created `internal/ui/errlog/persist_test.go`
- edited `internal/ui/errlog/errlog.go` (persister field + Append hook)
- edited `internal/ui/app.go` (enableErrorLogPersistence + Run wiring)

## Phase 4.2 — "Open log file" button wired (2026-04-27)

Added an "Open log file" affordance to the Error Log view's footer.
Hands the persisted `error-log.jsonl` path off to the OS default
handler via `fyne.CurrentApp().OpenURL(file://…)` so the user can
forward the raw log to a bug report without copy-pasting trace by
trace.

### Wiring
- `views.ErrorLogOptions` gained `LogPath string` and
  `OpenPath func(path string) error`. Both optional; empty `LogPath`
  disables the button (boot-time persistence failure → in-memory
  only). nil `OpenPath` shows "Open handler not wired." instead of
  panicking.
- `internal/ui/app.go::Run()` records the resolved path in a new
  package-level `errLogPath` after `enableErrorLogPersistence`
  succeeds (empty when degraded). The `viewFor(NavErrorLog, …)` arm
  threads `errLogPath` + `openLogFileWithFyne` into the view.
- `openLogFileWithFyne` resolves to absolute path, builds a
  `&url.URL{Scheme:"file", Path: abs}`, and calls
  `fyne.CurrentApp().OpenURL`. nil `CurrentApp()` returns a typed
  errtrace error so the headless test path stays sane.

### View status feedback
Rather than a popup, a low-importance `widget.Label` next to the
buttons flips to one of:
- `"Disk log unavailable."` — `LogPath == ""` (button also disabled)
- `"Open handler not wired."` — `OpenPath == nil`
- `"Open failed: <err>"` — opener returned an error
- `"Opened <path>"` — success

Pulled the routing logic into a pure helper `openLogFile(path,
opener)` so headless tests assert behavior without a Fyne app.

### Tests (4 new in `error_log_test.go`)
- `TestOpenLogFile_EmptyPath`: empty `LogPath` → "Disk log unavailable.".
- `TestOpenLogFile_NilOpener`: nil opener → "Open handler not wired.".
- `TestOpenLogFile_Success`: opener receives configured path; status
  is "Opened <path>".
- `TestOpenLogFile_Failure`: opener error → status carries
  "Open failed:" + the error message.

`go vet -tags nofyne ./...` clean. All three errtrace lints still
**0 violations** (LINT_MODE=fail). All `internal/ui/...` packages
green.

### Files changed
- edited `internal/ui/views/error_log.go` (added LogPath/OpenPath +
  button + status label + openLogFile helper)
- edited `internal/ui/views/error_log_test.go` (4 new tests +
  sentinelErr helper)
- edited `internal/ui/app.go` (errLogPath var + openLogFileWithFyne
  + NavErrorLog wiring + net/url + errtrace imports)

## Phase 5.1 — Spec page shipped (2026-04-27)

Created `spec/03-error-manage/04-app-error-log-view/00-overview.md`
formalizing the desktop Error Log surface so future regressions have
a documented baseline to lint against.

### Contents
- **System Boundary** ASCII diagram: errtrace.Wrap call sites →
  errlog.Store → {Subscribe → badge/toast, persister → JSONL → CLI tail}.
- **Components & Source Map** table: 8 layers, each pinned to its
  source file (errtrace, errlog Store, persistence, boot wiring,
  view, sidebar badge, toast notifier, CLI tail).
- **Data Model**: Entry struct + JSONL-on-disk format + rotation policy.
- **12 Acceptance Criteria** (AC-ELV-01 … AC-ELV-12) covering:
  - capture with stack trace + nil-error / empty-component edges,
  - bounded ring + monotonic Seq,
  - non-blocking subscriber fan-out,
  - badge label format with 99+ collapse,
  - first-error toast with quiet-period re-arm + 140-char body cap,
  - persistence round-trip across restart with Seq monotonicity,
  - rotation at 5 MiB with at-most-2-files invariant,
  - graceful degradation when boot persistence fails,
  - "Open log file" routing (4 status branches),
  - Copy/Clear footer (Clear is view-only, JSONL preserved),
  - `errors tail` CLI output contract incl. -n, -f, missing-file,
  - errtrace lints stay at 0 violations under LINT_MODE=fail.
- **Cross-references** to peer Error Modal spec (web surface) +
  README §9 + memory files + runtime sources.

### Registry update
- Appended row 04 to the parent `spec/03-error-manage/00-overview.md`
  Categories table so the spec is discoverable from the index.

### Files changed
- created `spec/03-error-manage/04-app-error-log-view/00-overview.md`
- edited `spec/03-error-manage/00-overview.md` (added category row 04)

## Phase 5.2 — Preference memory refreshed (2026-04-27)

Rewrote `mem://preferences/01-error-stack-traces.md` to reflect the
post-rollout reality. The previous version captured Phase 1 baseline
intent only; this version is the canonical convention reference for
all future work.

### Key changes vs. prior version
- **Status header**: notes `LINT_MODE=fail` is live in `run.sh` /
  `run.ps1` / CI; cross-references the new spec page and rollout log.
- **Rule 1/2/3 each cite their enforcing linter** by filename so a
  contributor who hits a violation can find the script that flagged
  it without grepping.
- **New Rule 5** lists the three legitimate `errtrace.Format` call
  sites (main, watcher loggers, `errlog.ReportError`) — clarifies
  that `Format` is an edge-only operation, not a per-wrap-site one.
- **New "UI surface rule" section**: codifies
  `errlog.ReportError(component, err)` as the single entry point for
  every UI handler that observes an error. Notes nil-safety and the
  `component` tagging convention.
- **Three canonical examples** instead of one: classic wrap, sentinel
  via `errtrace.New`, and the UI handler "wrap + report" pattern.
- **Related** footer cross-links: Core memory line (Phase 5.3),
  rollout log, AC spec page (AC-ELV-01…12), README §9.

### Files changed
- edited `.lovable/memory/preferences/01-error-stack-traces.md`
