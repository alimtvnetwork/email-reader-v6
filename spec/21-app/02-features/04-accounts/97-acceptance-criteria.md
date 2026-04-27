# 97 — Accounts — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **definitive pass/fail gate** for shipping the Accounts feature. Every checkbox below must be `[x]` before merge. CI runs every automated check; manual checks are signed off in the PR description.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes `21000–21099` (config), `21200–21299` (mailclient), `21700–21799` (core)


<!-- sandbox-feasibility-legend v1 -->

## Sandbox feasibility legend

Each criterion below is tagged for the implementing AI so it can pick sandbox-implementable rows first:

| Tag | Meaning | Where it runs |
|---|---|---|
| 🟢 | **headless** — pure Go logic, AST scanner, SQL, registry, lint rule, errtrace, code-quality check | Sandbox: `nix run nixpkgs#go -- test -tags nofyne ./...` |
| 🟡 | **cgo-required** — Fyne canvas / widget render / focus ring / hover / pulse / pixel contrast / screen-reader runtime | Workstation only (CGO + display server) |
| 🔴 | **bench / E2E** — perf gate (`P-*`), benchmark, race detector under UI, multi-process integration | CI infra only |

See also: [`mem://design/schema-naming-convention.md`](mem://design/schema-naming-convention.md), `.lovable/cicd-issues/03-fyne-canvas-needs-cgo.md`, `.lovable/cicd-issues/05-no-bench-infra.md`.

---
---

## 1. Functional (must-pass)

- [ ] **F-01** 🟡 Mounting the Accounts tab calls `core.Accounts.List` exactly once and renders accounts sorted by `Order ASC, Alias ASC`.
- [ ] **F-02** 🟡 With 0 accounts → Empty state with "+ Add account" CTA renders.
- [ ] **F-03** 🟡 "+ Add account" opens the form pane with defaults (`UseTls=true`, `Order=max+10`); focus on `Alias`.
- [ ] **F-04** 🟡 Clicking a row populates the right pane with read-only `DetailPane`; form does NOT auto-open.
- [ ] **F-05** 🟢 Editing any field flips `isDirty` to true; Cancel button enables.
- [ ] **F-06** 🟡 Save (Add) inserts in `config.json` AND `WatchState` atomically; toast `"Account {Alias} added"`.
- [ ] **F-07** 🟡 Save (Update) updates `config.json` only; toast `"Account {Alias} updated"`.
- [ ] **F-08** 🟡 Update with `spec.Alias != currentAlias` returns 21715 (rename via separate op).
- [ ] **F-09** 🟡 Rename dialog target-taken returns 21716; field error highlighted; dialog stays open.
- [ ] **F-10** 🟡 Rename atomically updates `WatchState.Alias` and `config.json` in one logical operation; **`LastSeenUid` preserved**.
- [ ] **F-11** 🟢 Remove confirmation shows live `EmailCount` (via `SELECT COUNT(*) FROM Email WHERE Alias=?`); on confirm removes account + `WatchState` row; **preserves `Email` rows** with `Alias` set to NULL.
- [ ] **F-12** 🟡 Auto-suggest fills `Host`/`Port`/`UseTls` for the top 10 providers (gmail, outlook, yahoo, icloud, fastmail, gmx, protonmail-bridge, zoho, aol, hostinger).
- [ ] **F-13** 🟡 Auto-suggest does NOT overwrite a user-entered `Host` (guarded by `_hostUserEdited` flag).
- [ ] **F-14** 🟡 Unknown provider (`Source == Unknown`) auto-expands manual `Host`/`Port` fields and shows yellow badge.
- [ ] **F-15** 🟡 Test connection runs server-side via `core.Accounts.TestConnection`; updates `TestResultBadge` with latency on success or error-code-specific message on failure.
- [ ] **F-16** 🟡 Test connection writes nothing — verified by zero `cfg.Save` and zero SQL `EXEC` invocations.
- [ ] **F-17** 🟡 Save with wrong password returns 21720; field error inline; nothing written to `config.json` or `WatchState` (verified by file mtime + DB row count).
- [ ] **F-18** 🟡 Edit with empty `Password` preserves the existing `PasswordB64` (does not blank it).
- [ ] **F-19** 🟡 Edit with only `Order` changed does NOT re-run `TestConnection` (zero `Login` calls in fake dialer).
- [ ] **F-20** 🟡 Drag-reorder is debounced 300 ms; on drop `Order` is reassigned `(i+1)*10` for every account.
- [ ] **F-21** 🟡 Reorder with set-mismatch returns 21717 and is logged at WARN.
- [ ] **F-22** 🟡 Sidebar account-picker reflects add/remove/rename within **1 s** via the live event channel (no polling).
- [ ] **F-23** 🟡 OnHide with dirty form shows `DiscardChangesDialog` (Save / Discard / Cancel); Cancel vetoes nav.
- [ ] **F-24** 🟡 Right-click row → context menu: Edit, Rename, Remove, Copy email address.
- [ ] **F-25** 🟡 Eye-toggle on `PasswordEntry` reveals plaintext for exactly **3 s** then auto-re-masks.

## 2. Live-Update

- [ ] **L-01** 🟡 `AccountEvent{Added}` appends a row and shows toast within 1 s of `bus.Publish`.
- [ ] **L-02** 🟡 `AccountEvent{Removed}` removes the row; if it was selected, `selected = nil` and `DetailPane → EmptyState`.
- [ ] **L-03** 🟡 `AccountEvent{Renamed}` performs in-place rename; **scroll position preserved**.
- [ ] **L-04** 🟡 `AccountEvent{Updated}` while in Edit mode on the same row prompts `DiscardChangesDialog` ("This account was edited elsewhere").
- [ ] **L-05** 🟡 `WatchEvent{AccountConnected}` flips dot to green and clears `LastConnectError` within 40 ms.
- [ ] **L-06** 🟡 `WatchEvent{AccountConnectError}` flips dot to red and populates `LastConnectError` within 40 ms.
- [ ] **L-07** 🟡 Channel overflow (`WARN AccountEventOverflow`) triggers a full `Refresh()` to recover canonical state.
- [ ] **L-08** 🟡 `AttachLive` survives view hide/show; `DetachLive` is called only on app shutdown (sidebar depends on it).
- [ ] **L-09** 🔴 App close emits no subscriber-leak WARN.

## 3. Error Handling

- [ ] **E-01** 🟡 `List` returning 21701/21702 shows `ErrorPanel` with Retry; previous rows preserved.
- [ ] **E-02** 🟡 Field-level errors (21710/21711/21712/21713/21714/21716/21718/21719) set `fieldErrs` and focus the offending field.
- [ ] **E-03** 🟡 `TestConnection` failure populates `TestResultBadge` with the error-code-specific message; never throws.
- [ ] **E-04** 🟡 `Reorder` failure rolls back optimistic UI and shows error toast with code 21717/21743.
- [ ] **E-05** 🟡 `Add` atomicity failure (21731) shows toast `"Add failed; state restored"`; both stores revert.
- [ ] **E-06** 🟡 `Remove` atomicity failure (21730) shows toast `"Remove failed; state restored"`; `WatchState` row reinserted.
- [ ] **E-07** 🟡 `Rename` atomicity failure (21732) shows toast `"Rename failed; state restored"`; both stores revert.
- [ ] **E-08** 🟡 Wrapped underlying mailclient errors (21200/21201/21207/21208) appear inside the envelope but never escape as the top-level code.
- [ ] **E-09** 🟢 All errors wrapped with `errtrace.Wrap(err, "Accounts.<Method>")` (verified via `errtrace.Frames`).
- [ ] **E-10** 🔴 No `panic()` reachable from Accounts view — fuzzed for 60 s in CI.

## 4. Performance (CI-gated benchmarks)

- [ ] **P-01** 🔴 `List` p95 ≤ 15 ms with 50 accounts.
- [ ] **P-02** 🔴 Cold mount → first paint ≤ 100 ms.
- [ ] **P-03** 🔴 Initial render of 50 accounts ≤ 40 ms.
- [ ] **P-04** 🔴 `TestConnection` runs off the UI goroutine — UI never blocks (asserted via `runtime.NumGoroutine` delta).
- [ ] **P-05** 🔴 `TestConnection` respects 5 s deadline ± 100 ms.
- [ ] **P-06** 🔴 Drag-reorder visual feedback ≤ 16 ms (60 FPS).
- [ ] **P-07** 🔴 Live event → row dot color change ≤ 40 ms.
- [ ] **P-08** 🔴 Sidebar `PickerSnapshot()` read ≤ 1 ms (RWMutex slice copy).
- [ ] **P-09** 🔴 `SuggestImap` debounce window = 300 ms after last keystroke.
- [ ] **P-10** 🔴 Memory ≤ 2 MB for 50 accounts + form open.
- [ ] **P-11** 🔴 Slow-call WARN (`AccountsListSlow`) emitted when `List` exceeds 15 ms.

## 5. Code Quality

- [ ] **Q-01** 🟢 No method body in `internal/core/accounts.go` exceeds 15 lines.
- [ ] **Q-02** 🟢 No `interface{}` / `any` in `internal/core/accounts.go` or `internal/ui/views/accounts.go`.
- [ ] **Q-03** 🟢 No hex color literals in `internal/ui/views/accounts.go` (lint rule `no-hex-in-views`).
- [ ] **Q-04** 🟡 No `os.Exit`, `fmt.Print*`, `log.Fatal*` in core or view files.
- [ ] **Q-05** 🟢 All exported identifiers PascalCase; all SQL columns PascalCase; all JSON tags PascalCase.
- [ ] **Q-06** 🟢 `internal/ui/views/accounts.go` is the **only** Fyne-importing file for this feature.
- [ ] **Q-07** 🟡 `internal/core/accounts.go` does not import `fyne.io/*`, `internal/ui/*`, or `internal/cli/*`.
- [ ] **Q-08** 🟢 UI does not import `internal/imapdef`, `internal/mailclient`, `internal/config`, or `internal/store`.
- [ ] **Q-09** 🟢 No `SELECT *` in any Accounts SQL.
- [ ] **Q-10** 🟡 Sidebar reads accounts via `vm.PickerSnapshot()` only; never imports `core.Accounts` directly.

## 6. Security & PII

- [ ] **S-01** 🟡 Plaintext password is never bound to a `binding.String` (verified by code search for `binding.NewString` in proximity to `Password`).
- [ ] **S-02** 🟡 `PasswordEntry.Text` is `SetText("")`-zeroed after every Save attempt, success or failure.
- [ ] **S-03** 🟡 Password byte-slice is zeroed on `OnAppQuit` (verified via `unsafe.Slice` memory inspection in test).
- [ ] **S-04** 🟡 `PasswordRedaction_NeverAppearsInLogs` test passes: constructs Account with `Password="HuntER2"`, replays every method, asserts substring count == 0 in log buffer.
- [ ] **S-05** 🟢 Hidden Unicode / C0 control char in password rejected with `ER-CFG-21003`.
- [ ] **S-06** 🟡 `ServerGreeting` truncated to 256 bytes in logs (chatty server protection).
- [ ] **S-07** 🟡 `EmailAddr` IS logged (operationally necessary; not PII per app threat model — documented).

## 7. Testing

- [ ] **T-01** 🔴 `internal/core/accounts_test.go` coverage ≥ 90 %.
- [ ] **T-02** 🟢 All 31 required core test cases (per `01-backend.md` §9) present and passing.
- [ ] **T-03** 🟡 All 25 required smoke tests (per `02-frontend.md` §9) present and passing.
- [ ] **T-04** 🔴 Race detector clean: `go test -race ./internal/core/... ./internal/mailclient/... ./internal/imapdef/... ./internal/ui/views/...`.
- [ ] **T-05** 🔴 Benchmark `BenchmarkAccountsList_50` exists and meets P-01.
- [ ] **T-06** 🟡 Atomicity tests `Add_AtomicAcrossConfigAndSqlite`, `Remove_AtomicAcrossConfigAndSqlite`, `Rename_AtomicAcrossConfigAndSqlite` all fault-inject `cfg.Save` failure and assert both stores revert.
- [ ] **T-07** 🟡 Integration test `Rename_PreservesLastSeenUid` asserts `WatchState.LastSeenUid` of renamed alias equals pre-rename value.
- [ ] **T-08** 🟡 Integration test `Remove_DoesNotCascadeEmailRows` asserts `Email.Alias` set to NULL per FK rule.
- [ ] **T-09** 🟡 Stress test `Sidebar_PickerSnapshot_LockFreeRead_NoBlockingDuringLiveUpdate` runs concurrent reader + writer for 5 s with no deadlock.

## 8. Logging

- [ ] **G-01** 🟢 `DEBUG AccountsList` emitted on every `List` with documented fields.
- [ ] **G-02** 🟢 `INFO AccountAdded/Updated/Removed/Renamed/Reordered` emitted on the corresponding mutation.
- [ ] **G-03** 🟢 `DEBUG AccountSuggestImap` emitted on every `SuggestImap` call.
- [ ] **G-04** 🟢 `DEBUG AccountTestConnection` emitted on every `TestConnection` call with `Ok` and `LatencyMs`.
- [ ] **G-05** 🟢 `WARN AccountsListSlow` emitted when `DurationMs > 15`.
- [ ] **G-06** 🟢 `WARN AccountEventOverflow` emitted on subscriber channel overflow.
- [ ] **G-07** 🟢 `ERROR AccountsFailed` emitted on any wrapped error with `TraceId`, `Method`, `ErrorCode`.
- [ ] **G-08** 🟢 No PII (`Password`, `PasswordB64`) appears in any log line — enforced by S-04.

## 9. Database

- [ ] **D-01** 🟢 Accounts feature adds NO new tables (`WatchState` is owned by Watch feature; this is documented in §3 of `01-backend.md`).
- [ ] **D-02** 🟢 All Accounts SQL uses singular PascalCase table names (`WatchState`, `Email`).
- [ ] **D-03** 🟢 `Email.Alias` FK uses `ON DELETE SET NULL` (not CASCADE) — historical archive preserved.
- [ ] **D-04** 🟢 `Add` / `Remove` / `Rename` use `BEGIN IMMEDIATE` SQLite tx; failure paths documented in §6 of `01-backend.md`.
- [ ] **D-05** 🟢 `cfg.Save` uses atomic temp-file write + `os.Rename` (single fsync).
- [ ] **D-06** 🟢 No `SELECT *`.

## 10. Atomicity & Safety

- [ ] **X-01** 🟡 `Add` is atomic across `config.json` + SQLite (revert verified by T-06).
- [ ] **X-02** 🟡 `Remove` is atomic across `config.json` + SQLite; deleted `WatchState` row reinserted on rollback (T-06).
- [ ] **X-03** 🟡 `Rename` is atomic across `config.json` + SQLite; both stores revert on either failure (T-06).
- [ ] **X-04** 🟡 `Update` is config-only and idempotent for unchanged fields.
- [ ] **X-05** 🟡 `SetOrder` uses atomic temp-file write + `os.Rename` for `config.json`.
- [ ] **X-06** 🟡 `TestConnection` is read-only; zero `cfg.Save` and zero SQL `EXEC` (F-16).
- [ ] **X-07** 🟡 Concurrent `Add` + `SetOrder` cannot interleave to break `Order` uniqueness invariant.

## 11. Accessibility

- [ ] **A-01** 🟡 Every interactive widget has a Fyne `Hint` tooltip (Alias, EmailAddr, Password, eye-toggle, Host, Port, Use TLS, Test, Save, Cancel, drag handle, dots).
- [ ] **A-02** 🟡 Tab order: Alias → EmailAddr → Password → eye-toggle → ImapDefaults change-link → Host → Port → UseTls → Test → Cancel → Save.
- [ ] **A-03** 🟡 Connected dot has off-screen `widget.Label` for screen-reader semantics ("Connected" / "Disconnected — {Error}" / "Not yet polled").
- [ ] **A-04** 🟡 High-contrast theme distinguishes connected/disconnected by color AND icon (`✓`/`✗`/`—`) — golden-image test.
- [ ] **A-05** 🟡 Keyboard shortcuts: `Cmd/Ctrl+N` opens Add form; `Cmd/Ctrl+S` saves; `Esc` discards (with dirty-guard).
- [ ] **A-06** 🟡 Form fields with `fieldErrs` entry expose `aria-invalid="true"` and `aria-describedby` pointing to `FieldErrLabel`.

## 12. Sign-off

| Reviewer        | Date       | Signature |
|-----------------|------------|-----------|
| Backend lead    |            |           |
| UI lead         |            |           |
| QA              |            |           |
| Security        |            |           |
| Architecture    |            |           |

A merge is permitted only when **all** boxes above are `[x]` and all five signatures are present. (Security is required for the Accounts feature specifically due to credential handling.)

---

**End of `04-accounts/97-acceptance-criteria.md`**
