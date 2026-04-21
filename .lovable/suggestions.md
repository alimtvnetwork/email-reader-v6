# Suggestions

## Active Suggestions

### Rotate seeded credentials in spec
- **Status:** Pending
- **Priority:** High
- **Description:** The spec under `spec/21-golang-email-reader/spec.md` may contain seeded `atto` account credentials used during development. After end-to-end verification, rotate the IMAP app password and scrub or redact the spec.
- **Added:** 2026-04-21 session

### Add unit tests for rules edge cases
- **Status:** Pending
- **Priority:** Medium
- **Description:** Existing `internal/rules/rules_test.go` covers the happy path. Add cases for: invalid regex (should not crash watcher), URLs with query strings containing `&`, multi-URL emails (open all vs first only), case-sensitivity flag.
- **Added:** 2026-04-21 session

### Cross-platform build target
- **Status:** Pending
- **Priority:** Low
- **Description:** Today the bootstrap is Windows-only via `run.ps1`. Consider a `Makefile` or `run.sh` for macOS/Linux users — Go code is already portable; only `internal/browser` Chrome launcher and PATH handling need OS branching.
- **Added:** 2026-04-21 session

### Structured logging
- **Status:** Pending
- **Priority:** Low
- **Description:** Replace `fmt.Println` calls in the watcher with a leveled logger (`log/slog`) so users can filter watch noise vs rule matches vs errors.
- **Added:** 2026-04-21 session

## Implemented Suggestions

_None yet._
