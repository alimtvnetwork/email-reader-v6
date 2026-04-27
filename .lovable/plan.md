# Plan — Full-Stack Error Trace Logging Upgrade

**Goal:** Whenever something fails in the desktop app or CLI, the user (and we)
must see *what* failed, *why* it failed, and the *exact file:line chain* that
led to the failure — both in the log file and in the UI itself.

**Status today:**
- `internal/errtrace` package already exists (Wrap / Wrapf / New / Errorf /
  Format / Frames). 912 sites use it.
- 38 stragglers still use raw `fmt.Errorf` (production code, not just tests/codegen).
- 36 bare `return err` sites at package boundaries (no frame captured).
- 54 `errors.New(…)` sites (no frame captured).
- UI surfaces only `err.Error()` (one short line) — the multi-line trace is
  thrown away before the user ever sees it.
- No persistent in-app **Error Log** view. Only the maintenance slog stream.
- CLI `main` does call `errtrace.Format` → stderr ✅ (one good entry point).

---

## Phase 1 — Inventory & Guardrails (no behavior change)

| # | Slice | File(s) | AC |
|---|-------|---------|----|
| 1.1 | Add `linter-scripts/check-no-fmt-errorf.sh` that fails on any `fmt.Errorf(` outside `internal/errtrace/`, `linter-scripts/`, and `*_test.go`. Wire into `run.sh -d` pre-build step (warn-only first). | new file + run.sh/run.ps1 | Script lists today's 38 hits; build still succeeds. |
| 1.2 | Add `linter-scripts/check-no-bare-return-err.sh` that flags `^\s+return err$` outside tests. Warn-only. | new file | Script lists 36 hits. |
| 1.3 | Add `linter-scripts/check-no-errors-new.sh` flagging `errors.New(` outside tests + errtrace. Warn-only. | new file | Script lists 54 hits. |
| 1.4 | Add a memory note `mem://error-trace-rollout` recording the 3 counters as the baseline, so future sessions know how far migration has gone. | mem file | Memory persisted. |

Exit criteria: three lint scripts give us a precise, machine-checkable
backlog. Nothing breaks.

---

## Phase 2 — Migrate the production stragglers (38 + 36 + 54 sites)

Rip-and-replace, package by package, smallest blast radius first.

| # | Package | Sites | Change |
|---|---------|-------|--------|
| 2.1 | `internal/watcher/pollonce.go` | 3 | `fmt.Errorf` → `errtrace.New` / `errtrace.Wrapf`; bare `return err` → `errtrace.Wrap(err, "pollOnce")`. |
| 2.2 | `internal/store/migrate/m0005,m0010,m0012,m0014` | 8 | Each `fmt.Errorf("…: %w", err)` → `errtrace.Wrapf(err, "…")`. Migrations are the #1 spot users see schema failures. |
| 2.3 | `internal/ui/views/settings_logic.go` | 5 | Validation `fmt.Errorf` → `errtrace.New` (no cause but still want the frame for "field X invalid" traces). |
| 2.4 | `internal/ui/views/rules.go` + `emails.go` | 3 | Same treatment. |
| 2.5 | `internal/cli/cli.go`, `rules_export.go`, `internal/browser/browser.go`, `internal/ui/services.go`, `internal/ui/watch_runtime.go`, `internal/config/seed.go`, `internal/core/settings*.go` | 36 bare returns | Wrap each with one short context message. |
| 2.6 | `errors.New(…)` audit pass | 54 | Replace with `errtrace.New(…)` everywhere outside `errtrace_test.go` and codegen. |
| 2.7 | Flip Phase-1 lint scripts from **warn** to **fail**. | run.sh / run.ps1 | Build now refuses to compile if a regression sneaks back in. |

Exit criteria: every error returned from production code carries a frame.
`go vet ./...` clean. All existing tests green.

---

## Phase 3 — Surface the trace in the UI (the bit the user actually asked for)

Today the UI only writes `err.Error()` — that throws away the chain. Fix:

| # | Slice | File(s) |
|---|-------|---------|
| 3.1 | Add `internal/ui/error_log.go` — an in-memory ring buffer (cap 500) of `{Timestamp, Component, Summary, Trace}` entries plus `Subscribe(chan)` for live UI updates. | new |
| 3.2 | Add helper `ui.ReportError(component string, err error)` that formats with `errtrace.Format(err)`, appends to the ring buffer, and also writes to the existing slog stream. Every existing `status.SetText("⚠ … " + err.Error())` site grows a sibling `ReportError("emails", err)` call. | edit ~10 view files |
| 3.3 | Build a new view `internal/ui/views/error_log.go` — a Fyne list bound to the ring buffer. Each row shows summary; click expands to full trace + Copy button. | new |
| 3.4 | Add **"Error Log"** entry to the sidebar nav (`internal/ui/nav.go` + `sidebar.go`). Badge shows count of unread errors since last open. | edit |
| 3.5 | When a status label currently shows `"⚠ Open failed: <short>"`, also append `" — see Error Log"` so users discover the new view. | edit views |

Exit criteria: every UI error path is one click away from a full
file:line trace. Demo: trigger a bad password on the Attobond account →
status bar shows short message → Error Log view shows the full
`Wrap → Wrap → mailclient.Dial` chain.

---

## Phase 4 — Persist + export (so users can paste traces to us)

| # | Slice |
|---|-------|
| 4.1 | Persist the ring buffer to `data/error-log.jsonl` (append-only, capped at 5 MB with rotation to `.1`). |
| 4.2 | Add **"Copy all as text"** + **"Open log file"** buttons in the Error Log view. |
| 4.3 | Add `email-read errors tail` CLI subcommand that prints the last N entries from `data/error-log.jsonl` so non-UI users can grab them. |
| 4.4 | Update `README.md` "Reporting a bug" section to: *"Open Error Log → Copy all → paste here"*. |

Exit criteria: a user can hand us a single paste that contains every
recent failure with full traces.

---

## Phase 5 — Spec + memory housekeeping

| # | Slice |
|---|-------|
| 5.1 | Add `spec/03-error-manage/04-app-error-log-view/00-overview.md` documenting the new UI surface, ring buffer cap, persistence rules. |
| 5.2 | Update `mem://preferences/01-error-stack-traces.md` to add: "Every UI error path must call `ui.ReportError(component, err)` in addition to setting the status label." |
| 5.3 | Update `.lovable/memory/index.md` Core section with one-liner: "Errors: errtrace everywhere; UI must call ui.ReportError; Error Log view is canonical surface." |

Exit criteria: a fresh session would do this the right way without being
told.

---

## Sequencing & estimated effort

- **Phase 1** — 1 round (small).
- **Phase 2** — 3–4 rounds (mechanical, parallelisable per-package).
- **Phase 3** — 2 rounds (the user-visible win — biggest UX impact).
- **Phase 4** — 1–2 rounds.
- **Phase 5** — 1 round.

Phases are independent in the sense that we can ship Phase 3 *immediately
after* Phase 1 if you'd rather see the UI win before we finish the
mechanical migration in Phase 2.

---

## Open questions for you before we start coding

1. **Where should the Error Log live in the sidebar?** Top (above
   Dashboard), bottom (below Settings), or as a sub-item under a new
   "Diagnostics" group?
2. **How loud should new errors be?** Silent badge only, toast popup, or
   both? (My default: badge + toast for the first error, badge-only after.)
3. **Persist traces to disk?** (Phase 4.) Yes is more useful for bug
   reports but writes a few MB to `data/`. Default: yes, capped at 5 MB.
4. **Order of phases** — start with Phase 3 (UI win, immediate visible
   value) or strictly 1→2→3→4→5? Default: 1 → 3 → 2 → 4 → 5 so you see
   the trace in the UI sooner.
