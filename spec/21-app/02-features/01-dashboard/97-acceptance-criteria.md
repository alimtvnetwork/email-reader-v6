# 97 — Dashboard — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **definitive pass/fail gate** for shipping the Dashboard feature. Every checkbox below must be `[x]` before merge. CI runs every automated check; manual checks are signed off in the PR description.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md)

---

## 1. Functional (must-pass)

- [ ] **F-01** Mounting the Dashboard tab calls `core.Dashboard.Summary` exactly once.
- [ ] **F-02** All four `StatCard`s render with non-empty `Primary` text.
- [ ] **F-03** With 0 accounts in config → Empty state with "Add account" CTA renders.
- [ ] **F-04** With ≥ 1 account but no `WatchState` row → that row shows `Health="Warning"` and `LastPollAt="Never"`.
- [ ] **F-05** `ConsecutiveFailures ≥ 3` → row shows `Health="Error"` with red dot.
- [ ] **F-06** Click on `AccountHealthList` row navigates to Emails tab with `Alias` filter pre-applied.
- [ ] **F-07** Click on `RecentActivityList` row of kind `PollFailed` opens `ErrorDetailDialog` with `ErrorCode` and trace.
- [ ] **F-08** Pressing **F5** triggers `vm.Refresh` (verified by debounce timestamp).
- [ ] **F-09** Refresh button is disabled while a refresh is in flight (no double-fire).
- [ ] **F-10** Activity list never exceeds 20 visible rows; oldest evicted when 21st arrives.

## 2. Live-Update

- [ ] **L-01** A `WatchEvent` arriving while the tab is visible prepends to `RecentActivityList` within 16 ms.
- [ ] **L-02** A matching `WatchEvent` patches the corresponding `AccountHealthRow` (e.g., `LastPollAt` updates) without a full refetch.
- [ ] **L-03** Switching away from the tab calls `vm.DetachLive()` and the channel is closed within 50 ms.
- [ ] **L-04** Returning to the tab re-subscribes and re-fetches the summary.
- [ ] **L-05** Closing the app cleanly closes all event subscriptions (no `WatchSubscriberLeak` WARN log).

## 3. Error Handling

- [ ] **E-01** `core.Dashboard.Summary` returning code `21101` shows `ErrorPanel{ErrorCode=21101, RetryButton}` — never a blank panel.
- [ ] **E-02** Retry button re-invokes `Summary` and clears the `ErrorPanel` on success.
- [ ] **E-03** `RecentActivity` with `limit < 1` returns code `21102` and is logged at WARN (caller bug).
- [ ] **E-04** All errors are wrapped with `errtrace.Wrap(err, "Dashboard.<Method>")` (verified by `errtrace.Frames` containing the method name).
- [ ] **E-05** No `panic()` reachable from the Dashboard view — fuzzed for 60 s in CI.

## 4. Performance (CI-gated benchmarks)

- [ ] **P-01** `Summary` p95 ≤ 40 ms with 100 000 emails / 10 accounts.
- [ ] **P-02** Cold mount → first paint ≤ 100 ms.
- [ ] **P-03** Warm `Refresh` round-trip ≤ 50 ms.
- [ ] **P-04** Live event insertion ≤ 16 ms.
- [ ] **P-05** Memory footprint ≤ 2 MB with 20 activity rows + 100 health rows.
- [ ] **P-06** Slow-call WARN log emitted when any method exceeds 100 ms.

## 5. Code Quality

- [ ] **Q-01** No method body in `internal/core/dashboard.go` exceeds 15 lines.
- [ ] **Q-02** No `interface{}` / `any` in `internal/core/dashboard.go` or `internal/ui/views/dashboard.go`.
- [ ] **Q-03** No hex color literals in `internal/ui/views/dashboard.go` (lint rule `no-hex-in-views`).
- [ ] **Q-04** No `os.Exit`, `fmt.Print*`, `log.Fatal*` in core or view files.
- [ ] **Q-05** All exported identifiers PascalCase; all SQL columns PascalCase; all JSON tags PascalCase.
- [ ] **Q-06** `internal/ui/views/dashboard.go` is the **only** Fyne-importing file for this feature.
- [ ] **Q-07** `internal/core/dashboard.go` does not import `fyne.io/*`, `internal/ui/*`, or `internal/cli/*`.

## 6. Testing

- [ ] **T-01** `internal/core/dashboard_test.go` coverage ≥ 90 %.
- [ ] **T-02** All 8 required core test cases (per `01-backend.md` §6) present and passing.
- [ ] **T-03** All 6 required smoke tests (per `02-frontend.md` §9) present and passing.
- [ ] **T-04** Race detector clean: `go test -race ./internal/core/... ./internal/ui/views/...`.
- [ ] **T-05** Benchmarks `BenchmarkDashboardSummary_100k` and `BenchmarkDashboardLiveInsert` exist and meet budgets P-01 and P-04.

## 7. Logging

- [ ] **G-01** `DEBUG DashboardSummary` emitted on every `Summary` call with `TraceId`, `DurationMs`, `EmailsTotal`, `Accounts`.
- [ ] **G-02** `WARN DashboardSlow` emitted when `DurationMs > 100`.
- [ ] **G-03** `ERROR DashboardFailed` emitted on any wrapped error with `TraceId`, `ErrorCode`, `Method`.
- [ ] **G-04** No PII (email body, password, token) appears in any Dashboard log line.

## 8. Database

- [ ] **D-01** Migrations `M0007_AddEmailIsRead`, `M0008_CreateWatchEvent`, `M0009_AddWatchStateHealth` applied idempotently on app start.
- [ ] **D-02** `WatchEvent` table trimmed to ≤ 10 000 rows after every successful poll.
- [ ] **D-03** All Dashboard SQL uses singular PascalCase table names (`Email`, `WatchState`, `WatchEvent`).
- [ ] **D-04** No `SELECT *` in Dashboard queries — explicit column list only.

## 9. Accessibility

- [ ] **A-01** All `StatCard` instances expose role `"region"` with `aria-label`.
- [ ] **A-02** All clickable rows expose role `"button"`.
- [ ] **A-03** Focus order matches §8 of `02-frontend.md`.
- [ ] **A-04** Screen-reader announcement on `Loaded` matches the template in §8.

## 10. Sign-off

| Reviewer        | Date       | Signature |
|-----------------|------------|-----------|
| Backend lead    |            |           |
| UI lead         |            |           |
| QA              |            |           |
| Architecture    |            |           |

A merge is permitted only when **all** boxes above are `[x]` and all four signatures are present.

---

**End of `01-dashboard/97-acceptance-criteria.md`**
