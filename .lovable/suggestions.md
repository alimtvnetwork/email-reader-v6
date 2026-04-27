# Suggestions

## Active Suggestions

### Grow `06-error-registry.md` to unblock AC-PROJ-31
- **Status:** Pending
- **Priority:** High
- **Description:** Slice #131's `Test_AllErrorRefsResolveInRegistry` scanner is wired and ratchet-ready but currently `t.Skip`s because ~39 ER codes are referenced in specs without being defined in the registry. Concentrated in Settings 217xx (ER-SET-21770..21782), Migrations 218xx (ER-MIG-21800..21806), UI 219xx (ER-UI-21900..21913), plus stragglers (ER-ACC-21430/21431, ER-COR-21710..21712/21770, ER-MAIL-2120, ER-RUL-21260, ER-TOOL-2176/21761/21762, ER-WATCH-21503, ER-WCH-21412). Closing this row is registry growth = behaviour/spec work, not test work.
- **Added:** Slice #131 (2026-04-27)

### Fix ~33 broken cross-tree spec links to unblock AC-PROJ-33
- **Status:** Pending
- **Priority:** Medium
- **Description:** Slice #131's `Test_NoBrokenSpecLinks_GreenInCi` scanner found ~33 broken local links, almost all in adjacent doc trees imported from other repos: `02-coding-guidelines/`, `08-generic-update/`, `13-cicd-pipeline-workflows/`. Closing is documentation cleanup. Scanner output in test logs lists every (file, target) pair.
- **Added:** Slice #131 (2026-04-27)

### Flip OI-1..OI-6 in `06-tools/99-consistency-report.md` to ✅ Closed
- **Status:** Pending
- **Priority:** Medium
- **Description:** `spec/21-app/02-features/06-tools/99-consistency-report.md` lists six open issues marked "scheduled" but never flipped to ✅ Closed. Either resolve and mark closed, or remove if abandoned. Unblocks AC-PROJ-35.
- **Added:** Slice #131 (2026-04-27)

### Implement Fyne UI per spec/21-app/02-features/
- **Status:** ✅ Implemented (Phases 1+2 — Slices #1-#41)
- **Priority:** High
- **Description:** All 7 features shipped. Now in Phase 3 (AC coverage rollout).
- **Added:** 2026-04-25 spec session

### Rotate seeded credentials in spec
- **Status:** Pending
- **Priority:** High
- **Description:** Legacy spec at `spec/21-app/legacy/spec.md` may contain seeded `atto` account credentials. Rotate IMAP app password and scrub/redact after end-to-end verification.
- **Added:** 2026-04-21 session

### Add unit tests for rules edge cases
- **Status:** Pending
- **Priority:** Medium
- **Description:** Cover invalid regex (no-crash), URLs with `&` query params, multi-URL emails (open all vs first), case-sensitivity flag.
- **Added:** 2026-04-21 session

### Cross-platform build target
- **Status:** Pending
- **Priority:** Low
- **Description:** Today's bootstrap is Windows-only via `run.ps1`. Add `Makefile` or `run.sh` for macOS/Linux (`run.sh` already exists — formalise it).
- **Added:** 2026-04-21 session

### Quiet-mode flag for the watcher
- **Status:** Pending
- **Priority:** Low
- **Description:** Add `--quiet` (errors + new mail only) and `--verbose` (current behaviour, default), or use `log/slog` levels.
- **Added:** 2026-04-21 debugging session

## Implemented Suggestions

### Author 23-app-database and 24-app-design-system-and-ui
- **Implemented:** 2026-04-25 → present
- **Notes:** Both spec folders are now populated. `23-app-database` has the schema, queries, and 97-acceptance-criteria.md driving Slices #34-#41 + #129. `24-app-design-system-and-ui` has tokens, theme implementation, and 97-acceptance-criteria.md driving the AC-DS family.

### Add `email-read doctor` diagnostic subcommand
- **Implemented:** 2026-04-21 (later)
- **Notes:** Shipped — `email-read doctor <alias>` runs TCP/TLS/IMAP login probe + mailbox stats + MX lookup. Used as the canonical first step for all auth issues (see `mem://decisions/03-imap-auth-debugging`).

### Structured logging in watcher
- **Status:** ✅ Implemented (full slog migration in Slice #33)
- **Implemented:** 2026-04-21 (initial) → Slice #33 (slog migration)
- **Notes:** Initial round added per-step structured-ish log lines. Slice #33 completed the `log/slog` migration — `internal/ui/maintenance_log.go` adopts package-private `maintenanceSlog` with `component=maintenance`, INFO on success / WARN on error, `event=…` tail format pinned by emit-test.

### AC coverage audit infrastructure
- **Implemented:** Slices #117-#127
- **Notes:** `internal/specaudit/coverage_audit_test.go` enforces monotonic allowlist shrink + zero stale code refs. Drives every Slice #128+ rollout.

### AC-SB family 100% coverage
- **Implemented:** Slices #126-#128
- **Notes:** All 24 settings-backend rows have citing tests. `internal/core/settings_long_tail_test.go` closed the final 7 (12/13/14/18/20/21/23).

### AC-SX family 100% headless coverage
- **Implemented:** Slice #130 (2026-04-27)
- **Notes:** 5 AST/log scanners in `internal/specaudit/ast_settings_security_test.go`. Discovery: 2 real production `IncognitoArg` log leaks fixed via `redactIncognito`/`redactIncog` helpers in `internal/cli/read.go` and `internal/watcher/watcher.go`.

### AC-PROJ spec linters (3 closed + 2 wired-deferred)
- **Implemented:** Slice #131 (2026-04-27)
- **Notes:** `internal/specaudit/ast_project_linters_test.go`. Closed AC-PROJ-18/32/34 cleanly; wired AC-PROJ-31/33 in `t.Skip` mode so they auto-ratchet when underlying spec/registry debt is cleaned (see Active Suggestions above).

### Desktop-run procedure documentation
- **Implemented:** 2026-04-27 (this session)
- **Notes:** `.lovable/memory/decisions/05-desktop-run-procedure.md` documents Windows + macOS/Linux build/run + smoke-test checklist.
