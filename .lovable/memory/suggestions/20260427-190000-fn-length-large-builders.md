---
id: 20260427-190000-fn-length-large-builders
status: open
priority: medium
title: 4 UI builder functions still exceed 15-statement linter budget (AC-PROJ-20)
opened-by: Slice #193
---

# 4 UI builders > 15 statements — blocking AC-PROJ-20 fully green

`linter-scripts/check-fn-length.sh` reports 4 remaining violations after Slice #193 closed the four small overshoots (16-19 stmts). User scoped Slice #193 to small overshoots only — these large UI builders are explicitly deferred:

| File | Function | Statements |
|---|---|---|
| `internal/ui/views/dashboard.go:53` | `BuildDashboard` | **34** |
| `internal/ui/views/error_log.go:67` | `BuildErrorLog` | **27** |
| `internal/ui/views/rules.go:160` | `ruleRow` | 17 |
| `internal/ui/views/accounts.go:136` | `accountRow` | 16 |

## Spec link
- `spec/21-app/97-acceptance-criteria.md` AC-PROJ-20: "every function ≤ 15 statements; verified by `linter-scripts/check-fn-length.sh` in CI."
- Test gate: `Project_FnLengthLinter_GreenInCi` (currently red against these 4).

## Suggested approach (1-2 follow-up slices)
- **`BuildDashboard` (34)** — extract section builders: `buildDashboardHeader`, `buildDashboardCounters`, `buildDashboardWatchPanel`, `buildDashboardActions`. Each section already has a logical grouping in the source; the wrapper just composes them.
- **`BuildErrorLog` (27)** — extract `newErrorLogTable` and `wireErrorLogControls` (the toolbar + filter callbacks).
- **`ruleRow` (17)** — extract `buildRuleActionButtons(r, opts, status, reload, hasReorder)` returning the `actionWidgets` slice; collapses the 4 conditional appends into one helper call.
- **`accountRow` (16)** — extract `buildAccountActionButtons(a, opts, status, reload)` returning the HBox; or extract `formatAccountLastSeen(ws)` to consolidate the 2 mailbox/lastUid prep statements.

## Risk
Touches user-visible UI build code. Visual diff should be zero (pure refactor — extracted helpers receive identical args and return identical widgets). No behaviour change. Recommend running the full test suite + a manual smoke of the Dashboard / Error Log views before declaring closed.

## Sandbox feasibility
🟢 **Sandbox-doable.** Pure Go refactor under `-tags nofyne`; no cgo/GL needed. Tests in `internal/ui/views/` are plain Go and already green.
