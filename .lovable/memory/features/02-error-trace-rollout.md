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
