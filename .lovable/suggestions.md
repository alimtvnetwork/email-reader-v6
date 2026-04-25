# Suggestions

## Active Suggestions

### Rotate seeded credentials in spec
- **Status:** Pending
- **Priority:** High
- **Description:** The legacy spec at `spec/21-app/legacy/spec.md` (formerly `spec/21-golang-email-reader/spec.md`) may contain seeded `atto` account credentials used during development. After end-to-end verification, rotate the IMAP app password and scrub or redact the legacy spec.
- **Added:** 2026-04-21 session

### Add unit tests for rules edge cases
- **Status:** Pending
- **Priority:** Medium
- **Description:** Existing `internal/rules/rules_test.go` covers the happy path. Add cases for: invalid regex (should not crash watcher), URLs with query strings containing `&`, multi-URL emails (open all vs first only), case-sensitivity flag.
- **Added:** 2026-04-21 session

### Cross-platform build target
- **Status:** Pending
- **Priority:** Low
- **Description:** Today the bootstrap is Windows-only via `run.ps1`. User is actually running on macOS and using PowerShell Core for `run.ps1`. Consider a `Makefile` or `run.sh` for macOS/Linux users — Go code is already portable; only `internal/browser` Chrome launcher and PATH handling need OS branching.
- **Added:** 2026-04-21 session

### Add `email-read doctor` diagnostic subcommand
- **Status:** Pending
- **Priority:** Medium
- **Description:** When auth or delivery issues happen, the user has to manually run `openssl s_client`, decode Base64, etc. A `doctor <alias>` subcommand could automate: TCP connect test, TLS handshake, IMAP LOGIN, INBOX select, `messages` count, MX lookup of the recipient domain. Would have shortcut the entire 2026-04-21 debugging session.
- **Added:** 2026-04-21 debugging session

### Quiet-mode flag for the watcher
- **Status:** Pending
- **Priority:** Low
- **Description:** New verbose per-poll logging is great for debugging but noisy in steady-state. Add `--quiet` (errors + new mail only) and `--verbose` (current behaviour, default), or use `log/slog` levels.
- **Added:** 2026-04-21 debugging session

## Implemented Suggestions

### Structured logging in watcher
- **Status:** ✅ Implemented (partial — string-based, not slog yet)
- **Original Priority:** Low
- **Implemented:** 2026-04-21 debugging session
- **Notes:** Did not migrate to `log/slog`, but rewrote `pollOnce` to emit per-step structured-ish log lines (poll start, dial, login timing, mailbox stats, fetch range, results, per-message, per-rule, poll complete). See `.lovable/memory/sessions/02-2026-04-21-debugging.md`. A future iteration can promote these to `log/slog` with proper levels (see "Quiet-mode flag" above).
