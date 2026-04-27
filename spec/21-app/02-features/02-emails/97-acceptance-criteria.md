# 97 — Emails — Acceptance Criteria

**Version:** 1.0.1
**Updated:** 2026-04-27
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **definitive pass/fail gate** for shipping the Emails feature. Every checkbox below must be `[x]` before merge. CI runs every automated check; manual checks are signed off in the PR description.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21200–21299

---

---

## Sandbox feasibility legend (added Slice #184 — see `mem://workflow/progress-tracker.md`)

A fresh AI picking up an unchecked row should consult the `**Sandbox:**` tag immediately under
each section header to decide whether the row is implementable in the Lovable sandbox or must
be deferred to a workstation/CI runner.

| Tag | Meaning | Implementable in sandbox? |
|---|---|---|
| 🟢 **headless** | Go unit/integration tests, AST scanners, log greps, spec-doc edits. Verified via `nix run nixpkgs#go -- test -tags nofyne ./...`. | **Yes** — preferred sandbox work. |
| 🟡 **cgo-required** | Fyne canvas widget tests, real driver behaviour. Needs cgo + GL/X11. See `mem://workflow/canvas-harness-starter.md` (Slice #180). | **No** — defer to workstation; planned. |
| 🔴 **needs bench / E2E infra** | p95 perf gates (bench infra) or multi-process IMAP+browser E2E. See `mem://workflow/{bench,e2e}-harness-starter.md` (Slices #178/#179). | **No** — defer to CI runner; planned. |
| ⚪ **N/A** | Manual sign-off checklist; no automated test possible. | **No** — human reviewer. |

A section may carry **two** tags when its rows split (e.g. `🟢 + 🔴`); pick the right tag per row by reading the row itself.

## 1. Functional (must-pass)

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **F-01** Mounting the Emails tab restores the persisted `LastEmailQuery` (or defaults: `Limit=50`, `SortBy=ReceivedAtDesc`).
- [ ] **F-02** With 0 emails for the active alias → Empty state with "Refresh" CTA renders.
- [ ] **F-03** Alias select lists every alias in `config.Accounts[]`, plus an "All aliases" option (empty `Alias`).
- [ ] **F-04** Search input matches across `Subject`, `FromAddr`, `BodyText` (case-insensitive `LIKE`).
- [ ] **F-05** "Unread only" filter hides rows where `IsRead = 1`.
- [ ] **F-06** "Include deleted" reveals soft-deleted rows at 60 % opacity with strike-through subject.
- [ ] **F-07** Click on a list row populates the detail pane with full headers + body (text + HTML tabs).
- [ ] **F-08** Click on .eml link calls `core.Tools.OpenUrl("file://" + FilePath)`.
- [ ] **F-09** Click on a URL in body opens via `core.Tools.OpenUrl` and writes to `OpenedUrl` table.
- [ ] **F-10** SelectAllCheck selects only currently visible UIDs (not the whole result set).
- [ ] **F-11** Mark-read button updates only checked UIDs; unchecked rows untouched.
- [ ] **F-12** Delete shows confirmation modal before mutating.
- [ ] **F-13** Undo button restores the most recent delete batch; disabled when undoStack empty.
- [ ] **F-14** Refresh button calls `core.Watch.PollOnce(alias)` and on success re-runs `List`.
- [ ] **F-15** `WatchEvent.Kind == EmailStored` for active alias increments NewBadge but does **not** auto-refetch.
- [ ] **F-16** Pagination Prev/Next correctly adjusts `Offset` by `Limit`; disabled at boundaries.
- [ ] **F-17** Counts label reflects `Total / Unread / Deleted` after every mutation.
- [ ] **F-18** `Get` does **not** mark the opened email as read (separate explicit action).
- [ ] **F-19** Detail pane "R" hotkey toggles read state of the open email.
- [ ] **F-20** Navigation from Dashboard with `NavParams{Alias}` pre-applies the alias filter on first mount.

## 2. Live-Update

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **L-01** A `WatchEvent` for the active alias increments `NewBadge` within 16 ms.
- [ ] **L-02** A `WatchEvent` for an inactive alias is ignored (no badge change).
- [ ] **L-03** Switching tabs calls `vm.DetachLive()`; channel closes within 50 ms.
- [ ] **L-04** Returning to tab re-subscribes and re-fetches list.
- [ ] **L-05** App close emits no `WatchSubscriberLeak` WARN.

## 3. Error Handling

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **E-01** `List` returning 21202 shows `ErrorPanel{ErrorCode=21202, RetryButton}`; previous rows preserved.
- [ ] **E-02** `Get` returning 21210 shows "not found" empty state in detail pane.
- [ ] **E-03** `MarkRead` failure rolls back the optimistic flip and shows error toast with code.
- [ ] **E-04** `Delete` failure rolls back the optimistic remove and shows error toast.
- [ ] **E-05** `Refresh` failure shows error envelope; list state preserved.
- [ ] **E-06** Caller bug `Limit > 200` returns 21201 and is logged at WARN.
- [ ] **E-07** All errors wrapped with `errtrace.Wrap(err, "Emails.<Method>")` (verified via `errtrace.Frames`).
- [ ] **E-08** No `panic()` reachable from Emails view — fuzzed for 60 s in CI.

## 4. Performance (CI-gated benchmarks)

**Sandbox:** 🔴 **needs bench infra** — see `mem://workflow/bench-infra-starter.md` (Slice #178).

- [ ] **P-01** `List` p95 ≤ 60 ms with 100 000 emails + 3-char `Search`.
- [ ] **P-02** Cold mount → first paint ≤ 120 ms.
- [ ] **P-03** `ApplyFilter` round-trip (warm DB, 100k rows) ≤ 80 ms.
- [ ] **P-04** `Get` (detail open) ≤ 30 ms.
- [ ] **P-05** `MarkRead` 500 UIDs ≤ 150 ms (SQL); UI optimistic flip ≤ 16 ms.
- [ ] **P-06** Live `WatchEvent` badge increment ≤ 16 ms.
- [ ] **P-07** Memory ≤ 4 MB for one page of 50 rows.
- [ ] **P-08** Slow-call WARN emitted when `List` exceeds 60 ms.

## 5. Code Quality

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **Q-01** No method body in `internal/core/emails.go` exceeds 15 lines.
- [ ] **Q-02** No `interface{}` / `any` in `internal/core/emails.go` or `internal/ui/views/emails.go`.
- [ ] **Q-03** No hex color literals in `internal/ui/views/emails.go` (lint rule `no-hex-in-views`).
- [ ] **Q-04** No `os.Exit`, `fmt.Print*`, `log.Fatal*` in core or view files.
- [ ] **Q-05** All exported identifiers PascalCase; all SQL columns PascalCase; all JSON tags PascalCase.
- [ ] **Q-06** `internal/ui/views/emails.go` is the **only** Fyne-importing file for this feature.
- [ ] **Q-07** `internal/core/emails.go` does not import `fyne.io/*`, `internal/ui/*`, or `internal/cli/*`.
- [ ] **Q-08** No `SELECT *` in any Emails SQL.

## 6. Testing

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **T-01** `internal/core/emails_test.go` coverage ≥ 90 %.
- [ ] **T-02** All 16 required core test cases (per `01-backend.md` §7) present and passing.
- [ ] **T-03** All 10 required smoke tests (per `02-frontend.md` §10) present and passing.
- [ ] **T-04** Race detector clean: `go test -race ./internal/core/... ./internal/ui/views/...`.
- [ ] **T-05** Benchmarks `BenchmarkEmailsList_100k_3CharSearch` and `BenchmarkEmailsMarkRead_500` exist and meet P-01 / P-05.
- [ ] **T-06** Idempotency test: `MarkRead` re-issued affects 0 rows the second time.

## 7. Logging

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **G-01** `DEBUG EmailsList` emitted on every `List` call with the documented fields.
- [ ] **G-02** `WARN EmailsListSlow` emitted when `DurationMs > 60`.
- [ ] **G-03** `INFO EmailsRefreshStarted` and `EmailsRefreshFinished` bracket every Refresh.
- [ ] **G-04** `ERROR EmailsFailed` emitted on any wrapped error with `TraceId`, `Method`, `ErrorCode`.
- [ ] **G-05** No PII (BodyText, BodyHtml, full From/To/Cc) appears in any log line.
- [ ] **G-06** `Subject` is truncated to ≤ 80 chars in any log emission and only at DEBUG level.

## 8. Database

**Sandbox:** 🟢 **headless** — Go unit/integration tests verifiable via `nix run nixpkgs#go -- test -tags nofyne ./...`.

- [ ] **D-01** Migration `M0010_AddEmailFlags` applied idempotently on app start.
- [ ] **D-02** Indexes `IxEmailAliasReceived`, `IxEmailAliasIsRead`, `IxEmailDeletedAt` exist after migration.
- [ ] **D-03** All Emails SQL uses singular PascalCase table names (`Email`, `OpenedUrl`).
- [ ] **D-04** Positive booleans only (`IsRead`, never `IsUnread`).
- [ ] **D-05** Bulk `MarkRead` / `Delete` batch UIDs at ≤ 999 per batch in a single transaction.
- [ ] **D-06** Bound parameters used for all user-supplied `Search` input (no string concat).

## 9. Accessibility

**Sandbox:** 🟡 **cgo-required** — needs Fyne canvas harness; see `mem://workflow/canvas-harness-starter.md` (Slice #180).

- [ ] **A-01** `EmailRow` exposes role `"button"` with the documented `aria-label` template.
- [ ] **A-02** Selection checkboxes expose role `"checkbox"` with correct `aria-checked`.
- [ ] **A-03** Toolbar buttons expose `aria-label` matching their text.
- [ ] **A-04** Focus order matches §9 of `02-frontend.md`.
- [ ] **A-05** Screen-reader announcement on `Loaded` matches the documented template.
- [ ] **A-06** Bulk action announcement fires after success.

## 10. Sign-off

**Sandbox:** ⚪ **N/A** — manual sign-off checklist; no automated gate.

| Reviewer        | Date       | Signature |
|-----------------|------------|-----------|
| Backend lead    |            |           |
| UI lead         |            |           |
| QA              |            |           |
| Architecture    |            |           |

A merge is permitted only when **all** boxes above are `[x]` and all four signatures are present.

---

**End of `02-emails/97-acceptance-criteria.md`**
