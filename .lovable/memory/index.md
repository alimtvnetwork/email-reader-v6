# Memory: index.md
Updated: just now

# Project Memory

## Core
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
IMAP `AUTHENTICATIONFAILED`: always run `email-read doctor <alias>` first to rule out hidden Unicode (U+2060 word-joiner from chat copy-paste) before blaming code.
All errors must use `internal/errtrace` so failures show file:line stack traces.

## Memories
- [Email delivery RCA for IMAP watcher](mem://email-delivery-rca) — Stable IMAP stats despite Gmail test means upstream delivery/folder issue; use diagnose command.
- [IMAP auth failed root causes](mem://decisions/03-imap-auth-debugging) — Two known causes: wrong password OR hidden Unicode in stored password; check doctor command output first.
