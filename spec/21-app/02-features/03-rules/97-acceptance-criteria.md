# 97 — Rules — Acceptance Criteria

**Version:** 1.0.0
**Updated:** 2026-04-25
**Status:** Approved
**AI Confidence:** Production-Ready
**Ambiguity:** None

---

## Purpose

The **definitive pass/fail gate** for shipping the Rules feature. Every checkbox below must be `[x]` before merge. CI runs every automated check; manual checks are signed off in the PR description.

Cross-references:
- Overview: [`./00-overview.md`](./00-overview.md)
- Backend: [`./01-backend.md`](./01-backend.md)
- Frontend: [`./02-frontend.md`](./02-frontend.md)
- Errors: [`../../06-error-registry.md`](../../06-error-registry.md) — codes 21300–21399


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

## 1. Functional (must-pass)

- [ ] **F-01** 🟡 Mounting the Rules tab calls `core.Rules.List` exactly once and renders rules sorted by `Order ASC, Name ASC`.
- [ ] **F-02** 🟡 With 0 rules → Empty state with "New rule" CTA renders.
- [ ] **F-03** 🟡 "New rule" populates the form pane with defaults (`Action=OpenUrl`, `Enabled=true`, `Order=max+10`).
- [ ] **F-04** 🟡 Clicking a row populates the form pane with that rule's data; `isDirty` becomes false.
- [ ] **F-05** 🟢 Editing any field flips `isDirty` to true; Cancel button enables.
- [ ] **F-06** 🟡 Save (Create) inserts and re-lists; toast "Rule {Name} created".
- [ ] **F-07** 🟡 Save (Update) updates and re-lists; toast "Rule {Name} updated".
- [ ] **F-08** 🟡 Update with `spec.Name != currentName` returns 21313 (rename via separate op).
- [ ] **F-09** 🟡 Rename dialog target-taken returns 21314; field error highlighted; dialog stays open.
- [ ] **F-10** 🟡 Rename atomically updates `RuleStat.RuleName` and `OpenedUrl.RuleName` in one transaction.
- [ ] **F-11** 🟡 Delete confirmation warns hit-history loss; on confirm removes rule + `RuleStat` row; preserves `OpenedUrl` history.
- [ ] **F-12** 🟡 Toggle Enabled is idempotent (re-issue is a no-op, not a 2× write).
- [ ] **F-13** 🟡 Drag-reorder is debounced 300 ms; on drop, `Order` is reassigned as `(i+1)*10` for every rule.
- [ ] **F-14** 🟡 Reorder with set-mismatch returns 21320 and is logged at WARN.
- [ ] **F-15** 🟡 Dry-run runs against user-provided `EmailSample` and shows per-field match flags + extracted URLs.
- [ ] **F-16** 🟡 Dry-run writes nothing — verified by zero SQL `EXEC` and zero `config.Save` invocations.
- [ ] **F-17** 🟡 OpenUrl rule without `UrlRegex` returns 21316 at Create/Update.
- [ ] **F-18** 🟢 Invalid regex on any field returns 21312 with `Field` set; UI highlights that field only.
- [ ] **F-19** 🟡 OnHide with dirty form shows Save/Discard toast (5 s); silent dismiss = discard.
- [ ] **F-20** 🟡 Right-click row → Duplicate pre-fills form with copy named `"{Name} copy"`.
- [ ] **F-21** 🟡 First-match-wins by `Order` for `MarkRead` and `Tag`; verified by an integration test that runs two competing rules.
- [ ] **F-22** 🟡 `OpenUrl` matches across rules union the URL set; deduped via `IxOpenedUrlsUnique`.

## 2. Live-Update

- [ ] **L-01** 🟡 A `WatchEvent.Kind == RuleMatched` for a visible rule increments that row's `MatchCount` and bumps `LastMatchedAt` within 16 ms.
- [ ] **L-02** 🟡 No full re-list is triggered by a single `RuleMatched` event.
- [ ] **L-03** 🟡 Switching tabs calls `vm.DetachLive()` and the channel closes within 50 ms.
- [ ] **L-04** 🟡 Returning to tab re-subscribes and re-fetches the list.
- [ ] **L-05** 🔴 App close emits no `WatchSubscriberLeak` WARN.

## 3. Error Handling

- [ ] **E-01** 🟡 `List` returning 21301/21302 shows `ErrorPanel` with Retry; previous rows preserved.
- [ ] **E-02** 🟡 Field-level errors (21310/21311/21312/21315/21316/21317/21318) set `fieldErrs` and focus the offending field.
- [ ] **E-03** 🟡 `SetEnabled` failure rolls back optimistic flip and shows error toast with code.
- [ ] **E-04** 🟡 `Reorder` failure rolls back optimistic UI and shows error toast with code.
- [ ] **E-05** 🟡 `Rename` atomicity failure (21319) shows toast "Rename failed; state restored"; both stores in original state.
- [ ] **E-06** 🟡 `BumpStat` failure (21350) is logged at WARN only; never blocks the watcher.
- [ ] **E-07** 🟢 All errors wrapped with `errtrace.Wrap(err, "Rules.<Method>")` (verified via `errtrace.Frames`).
- [ ] **E-08** 🔴 No `panic()` reachable from Rules view — fuzzed for 60 s in CI.

## 4. Performance (CI-gated benchmarks)

- [ ] **P-01** 🔴 `List` p95 ≤ 20 ms with 200 rules.
- [ ] **P-02** 🔴 Cold mount → first paint ≤ 100 ms.
- [ ] **P-03** 🔴 `Refresh` round-trip (200 rules) ≤ 40 ms.
- [ ] **P-04** 🔴 `DryRun` ≤ 15 ms server-side on a 100 KB body; ≤ 30 ms end-to-end with render.
- [ ] **P-05** 🔴 Drag-reorder visual feedback ≤ 16 ms (60 FPS).
- [ ] **P-06** 🔴 Live `RuleMatched` row update ≤ 16 ms.
- [ ] **P-07** 🔴 Field-error highlight after server reply ≤ 16 ms.
- [ ] **P-08** 🔴 Memory ≤ 3 MB for 200 rules + dialog open.
- [ ] **P-09** 🔴 Slow-call WARN (`RulesListSlow`) emitted when `List` exceeds 20 ms.

## 5. Code Quality

- [ ] **Q-01** 🟢 No method body in `internal/core/rules.go` exceeds 15 lines.
- [ ] **Q-02** 🟢 No `interface{}` / `any` in `internal/core/rules.go` or `internal/ui/views/rules.go`.
- [ ] **Q-03** 🟢 No hex color literals in `internal/ui/views/rules.go` (lint rule `no-hex-in-views`).
- [ ] **Q-04** 🟡 No `os.Exit`, `fmt.Print*`, `log.Fatal*` in core or view files.
- [ ] **Q-05** 🟢 All exported identifiers PascalCase; all SQL columns PascalCase; all JSON tags PascalCase.
- [ ] **Q-06** 🟢 `internal/ui/views/rules.go` is the **only** Fyne-importing file for this feature.
- [ ] **Q-07** 🟡 `internal/core/rules.go` does not import `fyne.io/*`, `internal/ui/*`, or `internal/cli/*`.
- [ ] **Q-08** 🟢 No `SELECT *` in any Rules SQL.

## 6. Testing

- [ ] **T-01** 🔴 `internal/core/rules_test.go` coverage ≥ 90 %.
- [ ] **T-02** 🟢 All 17 required core test cases (per `01-backend.md` §9) present and passing.
- [ ] **T-03** 🟡 All 14 required smoke tests (per `02-frontend.md` §10) present and passing.
- [ ] **T-04** 🔴 Race detector clean: `go test -race ./internal/core/... ./internal/rules/... ./internal/ui/views/...`.
- [ ] **T-05** 🟡 Benchmarks `BenchmarkRulesList_200` and `BenchmarkDryRun_100KB` exist and meet P-01 / P-04.
- [ ] **T-06** 🟡 Atomicity test `Rename_AtomicAcrossConfigAndSqlite` fault-injects SQLite failure and asserts `config.json` reverted.
- [ ] **T-07** 🟡 Integration test `MarkRead_FirstMatchWinsByOrder` runs two competing rules and asserts only the lowest-`Order` rule fires.

## 7. Logging

- [ ] **G-01** 🟢 `DEBUG RulesList` emitted on every `List` with documented fields.
- [ ] **G-02** 🟢 `INFO RuleCreated/Updated/Deleted/EnabledToggled/Reordered/Renamed` emitted on the corresponding mutation.
- [ ] **G-03** 🟢 `WARN RulesListSlow` emitted when `DurationMs > 20`.
- [ ] **G-04** 🟢 `WARN RuleStatBumpFailed` emitted (never ERROR — never blocks watcher).
- [ ] **G-05** 🟢 `ERROR RulesFailed` emitted on any wrapped error with `TraceId`, `Method`, `ErrorCode`.
- [ ] **G-06** 🟢 No PII (sample `BodyText`, `FromAddr`) appears in any log line. `RuleDryRun` logs structural counters only.

## 8. Database

- [ ] **D-01** 🟢 Migrations `M0011_CreateRuleStat` and `M0012_CreateEmailTag` applied idempotently on app start.
- [ ] **D-02** 🟢 Index `IxRuleStatLastMatchedAt` and unique index `IxEmailTagUnique` exist after migration.
- [ ] **D-03** 🟢 All Rules SQL uses singular PascalCase table names (`RuleStat`, `EmailTag`, `OpenedUrl`).
- [ ] **D-04** 🟢 `EmailTag` has `FOREIGN KEY(EmailId) REFERENCES Email(Id) ON DELETE CASCADE`.
- [ ] **D-05** 🟢 Rename uses `BEGIN IMMEDIATE` SQLite tx; failure path documented in §6 of `01-backend.md`.
- [ ] **D-06** 🟢 `BumpStat` uses single `UPSERT` (no read-modify-write race).
- [ ] **D-07** 🟢 No `SELECT *`.

## 9. Atomicity & Safety

- [ ] **X-01** 🟡 `Rename` is atomic across `config.json` + SQLite (revert verified by T-06).
- [ ] **X-02** 🟡 `Delete` removes `Rule` and `RuleStat` in one SQLite tx; `OpenedUrl.RuleName` preserved.
- [ ] **X-03** 🟡 `Reorder` uses atomic temp-file write + `os.Rename` for `config.json`.
- [ ] **X-04** 🟡 `SetEnabled` is idempotent; same-state call performs zero writes (asserted in T-12 of backend tests via fake).
- [ ] **X-05** 🟡 `DryRun` is read-only; zero SQL `EXEC` and zero `config.Save` invocations (F-16).

## 10. Accessibility

- [ ] **A-01** 🟡 `RuleRow` exposes role `"button"` with the documented `aria-label` template.
- [ ] **A-02** 🟡 Drag handle exposes role `"button"` with reorder `aria-label`; `Ctrl+ArrowUp/Down` reorders when focused.
- [ ] **A-03** 🟡 Enabled dot exposes role `"switch"` with `aria-checked`.
- [ ] **A-04** 🟡 Form fields with `fieldErrs` entry expose `aria-invalid="true"` and `aria-describedby` pointing to `FieldErrLabel`.
- [ ] **A-05** 🟡 Focus order matches §9 of `02-frontend.md`.
- [ ] **A-06** 🟡 Screen-reader announcements fire on Loaded, Save, and field error.

## 11. Sign-off

| Reviewer        | Date       | Signature |
|-----------------|------------|-----------|
| Backend lead    |            |           |
| UI lead         |            |           |
| QA              |            |           |
| Architecture    |            |           |

A merge is permitted only when **all** boxes above are `[x]` and all four signatures are present.

---

**End of `03-rules/97-acceptance-criteria.md`**
