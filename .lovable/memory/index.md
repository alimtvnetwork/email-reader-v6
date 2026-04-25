# Memory: index.md
Updated: just now

# Project Memory

## Core
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
IMAP `AUTHENTICATIONFAILED`: always run `email-read doctor <alias>` first to rule out hidden Unicode (U+2060 word-joiner from chat copy-paste) before blaming code.
All errors must use `internal/errtrace` so failures show file:line stack traces.
App spec lives at `spec/21-app/` (App Project Template). Old paths `spec/21-golang-email-reader/` and `spec/22-fyne-ui/` were merged here on 2026-04-25 — never recreate them.

## Memories
- [Email delivery RCA for IMAP watcher](mem://email-delivery-rca) — Stable IMAP stats despite Gmail test means upstream delivery/folder issue; use diagnose command.
- [IMAP auth failed root causes](mem://decisions/03-imap-auth-debugging) — Two known causes: wrong password OR hidden Unicode in stored password; check doctor command output first.
- [spec/21-app folder rename](mem://decisions/04-spec-21-app-rename) — 2026-04-25 merge of 21-golang-email-reader + 22-fyne-ui into 21-app; legacy under spec/21-app/legacy/.
