# App Error Log View

**Version:** 1.0.0
**Status:** Active
**Updated:** 2026-04-27
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Keywords

`error-log`, `errtrace`, `desktop-ui`, `diagnostics`, `ring-buffer`,
`persistence`, `jsonl`, `sidebar-badge`, `toast`, `cli-tail`

---

## Scoring

| Criterion | Status |
|-----------|--------|
| `00-overview.md` present | ✅ |
| AI Confidence assigned | ✅ |
| Ambiguity assigned | ✅ |
| Keywords present | ✅ |
| Scoring table present | ✅ |
| Acceptance criteria | ✅ (see §AC below) |

---

## Purpose

Specifies the **desktop-app Error Log surface** — the in-process
diagnostics ring buffer rendered under **Diagnostics → Error Log**,
its sidebar unread badge, the first-error toast, the persisted
`data/error-log.jsonl` file, and the matching `email-read errors
tail` CLI subcommand.

This is distinct from the React-based **Error Modal** described in
[`02-error-architecture/04-error-modal/`](../02-error-architecture/04-error-modal/00-overview.md),
which targets a different (web) surface. Both surfaces share the
underlying `internal/errtrace` wrap convention but have independent
storage, UX, and acceptance criteria.

---

## System Boundary

```
                  ┌────────────────────────────────────────────┐
                  │  Any errtrace.Wrap / errtrace.New call     │
                  │  in internal/{ui,store,watcher,core,…}     │
                  └─────────────────┬──────────────────────────┘
                                    │ errlog.ReportError(component, err)
                                    ▼
                  ┌────────────────────────────────────────────┐
                  │  internal/ui/errlog.Store (singleton)      │
                  │   • ring buffer, cap = 500                 │
                  │   • monotonic Seq                          │
                  │   • atomic unread counter                  │
                  └─┬────────────┬──────────────────┬──────────┘
                    │            │                  │
        Append ◀────┘            ▼                  ▼
       (sync)             Subscribe()        persister(Entry)
                          (live fan-out)     (best-effort, async)
                                │                  │
                                ▼                  ▼
                       ┌──────────────────┐   ┌──────────────────────┐
                       │ Sidebar badge    │   │ data/error-log.jsonl │
                       │ + first-error    │   │ (rotated at 5 MiB    │
                       │   toast          │   │  → error-log.jsonl.1)│
                       └──────────────────┘   └──────────┬───────────┘
                                                         │
                                                         ▼
                                            email-read errors tail [-f]
```

---

## Components & Source Map

| Layer | Source | Role |
|---|---|---|
| Wrap convention | `internal/errtrace/` | `Wrap` / `Wrapf` / `New` / `Errorf` capture `file:line`. `Format` renders the chain. |
| Reporter API | `internal/ui/errlog/errlog.go` | `ReportError(component, err)` — single entry point used by every UI handler. |
| Ring buffer | `internal/ui/errlog/errlog.go::Store` | Bounded; default cap = `DefaultCap = 500`. Monotonic `Seq`. |
| Persistence | `internal/ui/errlog/persist.go::Persistence` | JSONL append, 5 MiB rotation (`DefaultSizeCap`), one rotated file kept. |
| Boot wiring | `internal/ui/app.go::enableErrorLogPersistence` | Resolves `<dataDir>/error-log.jsonl`, restores prior history into the ring, stores path in `errLogPath`. |
| List + detail UI | `internal/ui/views/error_log.go` | Split list (left, newest-first) ↔ monospace trace pane (right) + Clear / Copy / Open log file footer. |
| Sidebar badge | `internal/ui/sidebar_badge.go::formatNavRowLabel` | `"Title  (N)"` with **99+** collapse. Pure helper, fyne-free. |
| First-error toast | `internal/ui/errlog_notifier.go::ErrLogNotifier` | Fires once on the 0→1 unread transition, then stays badge-only until view opens. |
| CLI tail | `internal/cli/errors_tail.go` | `email-read errors tail [-f] [-n N]`. Shares `errlog.LoadFromFile` parser. |

---

## Data Model

```go
// internal/ui/errlog/errlog.go
type Entry struct {
    Seq       uint64    // monotonic, per-Store
    Timestamp time.Time // UTC, set at ReportError call
    Component string    // short tag: "emails", "watcher", …
    Summary   string    // err.Error() — one-line user-facing message
    Trace     string    // errtrace.Format output (multi-line). Falls back to Summary if unwrapped.
}
```

**JSONL on disk:** one `Entry` per line via `encoding/json`. Corrupt
or truncated lines are skipped on load — a half-written tail must
never prevent restoring the rest of the log.

**Rotation policy:** when active file size ≥ `DefaultSizeCap`
(5 MiB), rename to `<path>.1` (overwriting any prior `.1`) and open
a fresh active file. Exactly one rotated file is kept by design —
the in-memory ring + one rotation cover "what happened in the last
few minutes" without unbounded disk growth.

---

## Acceptance Criteria

### AC-ELV-01: Every UI error is captured with a stack trace

**GIVEN** any code path under `internal/ui/...`, `internal/store/...`,
`internal/watcher/...`, or `internal/core/...` returns a non-nil error
**WHEN** the surface handler calls `errlog.ReportError(component, err)`
**THEN** an `Entry` is appended whose `Trace` contains the full
`errtrace.Format` chain (summary + one `at file:line (func)` line per
wrap site)
**AND** `Trace` falls back to `Summary` when `err` was not wrapped
through errtrace, so the view never displays empty trace.

**Edge cases:**
- nil error → `ReportError` is a no-op; no entry, no badge bump.
- Component string empty → entry is still appended; CLI `tail`
  renders `-` in the component column (`nonEmpty` fallback).

### AC-ELV-02: Ring buffer is bounded and monotonic

**GIVEN** more than `DefaultCap` (500) errors are reported in a
session
**WHEN** the 501st error arrives
**THEN** the oldest entry (index 0) is dropped
**AND** every surviving entry's `Seq` strictly increases with insertion
order (no gaps within the ring; gaps relative to the dropped prefix
are expected).

### AC-ELV-03: Live subscribers receive every new entry, slow ones are dropped

**GIVEN** one or more `Subscribe()` channels are active
**WHEN** `ReportError` appends an entry
**THEN** every subscriber whose channel buffer has room receives the
`Entry`
**AND** subscribers whose buffer is full are skipped (non-blocking
send) — a stuck UI must never wedge the producer.

### AC-ELV-04: Sidebar badge reflects unread count with 99+ collapse

**GIVEN** `N` unread entries exist
**WHEN** the sidebar renders the **Diagnostics → Error Log** row
**THEN** the label is `"Error Log  (N)"` for `1 ≤ N ≤ 99`
**AND** the label is `"Error Log  (99+)"` for `N ≥ 100`
**AND** the label is `"Error Log"` (no parenthetical) for `N == 0`.

### AC-ELV-05: First-error toast fires once per quiet period

**GIVEN** the unread counter is 0
**WHEN** the next `ReportError` raises it to 1
**THEN** exactly one desktop notification is dispatched via
`fyne.App.SendNotification`
**AND** subsequent appends within the same quiet period stay
badge-only (no toast storm)
**AND** opening the Error Log view (which calls `errlog.MarkRead`)
re-arms the notifier so the next 0→1 transition fires again.

**Edge cases:**
- Toast body > 140 chars → truncated to 140 with `…` suffix.
- Empty `Component` → toast title falls back to `"App error"`.

### AC-ELV-06: Persistence survives restart

**GIVEN** `enableErrorLogPersistence` succeeded at boot
**WHEN** entries are appended during the session
**THEN** each is written to `<dataDir>/error-log.jsonl` as one JSON
line, fsync'd via `bufio.Writer.Flush`
**AND** on next boot the in-memory ring is re-seeded with up to
`DefaultCap` most-recent entries from disk
**AND** the next live `Append` produces `Seq = max(restored.Seq) + 1`
(monotonic across restarts).

### AC-ELV-07: Persistence rotation is bounded

**GIVEN** the active log file size meets or exceeds `DefaultSizeCap`
(5 MiB) after a write
**WHEN** the rotate-on-write check runs
**THEN** the active file is renamed to `<path>.1` (any prior `.1` is
removed first)
**AND** a fresh empty active file is reopened
**AND** at most two log files exist on disk
(`error-log.jsonl` + `error-log.jsonl.1`).

### AC-ELV-08: Persistence failure degrades gracefully

**GIVEN** boot-time persistence cannot be enabled (no data dir,
permission error, …)
**WHEN** the app continues to start
**THEN** `errLogPath` stays empty
**AND** the in-memory ring still works (live UI unaffected)
**AND** the Error Log view's **Open log file** button is disabled
with status label `"Disk log unavailable."`.

### AC-ELV-09: "Open log file" routing

**GIVEN** the Error Log view is rendered
**WHEN** the user clicks **Open log file**
**THEN** if `LogPath == ""` → status is `"Disk log unavailable."`
**AND** if `OpenPath == nil` → status is `"Open handler not wired."`
**AND** if `OpenPath` returns an error → status is
`"Open failed: <err>"`
**AND** on success → status is `"Opened <path>"` and the OS default
handler launches with `file://<absolute-path>`.

### AC-ELV-10: Copy / Clear footer

**GIVEN** an entry is selected in the list
**WHEN** the user clicks **Copy**
**THEN** the clipboard receives the full `Entry.Trace` (not Summary).
**WHEN** the user clicks **Clear**
**THEN** the in-memory ring is emptied, unread counter resets to 0,
sidebar badge re-renders without parenthetical, and the detail pane
clears. (The on-disk JSONL is **not** truncated — Clear is a
view-state operation; the persistent log remains for forensic
purposes.)

### AC-ELV-11: `errors tail` CLI contract

**GIVEN** `data/error-log.jsonl` exists with N entries
**WHEN** the user runs `email-read errors tail`
**THEN** every entry prints in oldest→newest file order as:
```
[<seq>] <RFC3339-UTC>  <component-or-dash>  <summary>
    <trace-line-1>
    <trace-line-2>
    …
<blank>
```
**AND** the trace block is omitted when `Trace == Summary` (one-line
errors).

**Flags:**
- `-n N` / `--lines N` → keep only the most-recent N entries.
- `-f` / `--follow` → after the initial dump, poll the file every 1 s
  and emit any entry whose `Seq > lastSeq`. Cancels cleanly on
  Ctrl-C / SIGTERM via `signal.NotifyContext`.
- Missing file (no errors yet) → `(no entries in <path>)` and exit 0.

### AC-ELV-12: errtrace lint guardrails stay at zero

**GIVEN** the three guardrail scripts in `linter-scripts/`
(`check-no-fmt-errorf.sh`, `check-no-bare-return-err.sh`,
`check-no-errors-new.sh`)
**WHEN** CI runs them with `LINT_MODE=fail`
**THEN** each reports **0 violations**
**AND** any new `fmt.Errorf`, bare `return err`, or `errors.New` in
production code (`internal/...`, excluding `_test.go`) breaks the
build, forcing the use of `errtrace.{Wrap,Wrapf,New,Errorf}`.

---

## Cross-References

- Parent: [Error Management Overview](../00-overview.md)
- Sibling (web surface): [Error Modal](../02-error-architecture/04-error-modal/00-overview.md)
- Sibling (cross-stack): [APPError Package](../02-error-architecture/06-apperror-package/00-overview.md)
- Memory:
  - `mem://features/02-error-trace-rollout` — full slice-by-slice rollout log
  - `mem://preferences/01-error-stack-traces` — wrap-style preference
- Runtime sources:
  - `internal/errtrace/` — wrap primitives + `Format`
  - `internal/ui/errlog/` — ring buffer + persistence
  - `internal/ui/views/error_log.go` — view
  - `internal/ui/sidebar_badge.go` — badge label helper
  - `internal/ui/errlog_notifier.go` — first-error toast
  - `internal/cli/errors_tail.go` — CLI subcommand
- User docs: [README §9 "Reporting a bug"](../../../README.md#9-reporting-a-bug)

---

## Versioning

| Version | Date | Change |
|---|---|---|
| 1.0.0 | 2026-04-27 | Initial spec — formalizes Phase 3 + Phase 4 of the error-trace logging upgrade (`mem://features/02-error-trace-rollout`). |
