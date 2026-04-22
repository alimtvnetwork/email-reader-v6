# Memory: index.md
Updated: just now

# Project Memory

## Core
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
All Go errors MUST use internal/errtrace (Wrap/Wrapf/New). Never bare `fmt.Errorf("...: %w", err)` or bare `return err` at package boundaries.

## Memories
- [Email delivery RCA for IMAP watcher](mem://email-delivery-rca) — Stable IMAP stats despite Gmail test means upstream delivery/folder issue; use diagnose command.
- [Error stack traces convention](mem://preferences/01-error-stack-traces) — Use errtrace.Wrap everywhere; main + watcher print errtrace.Format with file:line frames.
