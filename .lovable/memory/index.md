# Memory: index.md
Updated: 2026-04-26

# Project Memory

## Core
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
IMAP `AUTHENTICATIONFAILED`: always run `email-read doctor <alias>` first to rule out hidden Unicode (U+2060 word-joiner from chat copy-paste) before blaming code.
All errors must use `internal/errtrace` so failures show file:line stack traces.
App spec lives at `spec/21-app/` (App Project Template). Old paths `spec/21-golang-email-reader/` and `spec/22-fyne-ui/` were merged here on 2026-04-25 — never recreate them.
Sandbox has no preinstalled Go; verify with `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`. Files outside `.lovable/` may revert between sessions — verify on disk before assuming.
Spec-21-app authoring round (35 tasks) is COMPLETE; live work tracked in `mem://workflow/01-status`. Implementation deltas catalogued in `spec/21-app/99-consistency-report.md` §6.

## Memories
- [Workflow status](mem://workflow/01-status) — Current milestone + completed slices + remaining tracked work for spec-21-app implementation.
- [Email delivery RCA for IMAP watcher](mem://email-delivery-rca) — Stable IMAP stats despite Gmail test means upstream delivery/folder issue; use diagnose command.
- [IMAP auth failed root causes](mem://decisions/03-imap-auth-debugging) — Two known causes: wrong password OR hidden Unicode in stored password; check doctor command output first.
- [spec/21-app folder rename](mem://decisions/04-spec-21-app-rename) — 2026-04-25 merge of 21-golang-email-reader + 22-fyne-ui into 21-app; legacy under spec/21-app/legacy/.
- [Go verification path](mem://go-verification-path) — Sandbox lacks Go toolchain; use `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`.
- [Workspace revert on resume](mem://workspace-revert-on-resume) — Files outside `.lovable/` may revert between sessions; verify on disk.
- [Archived: spec-21-app tasklist](mem://archive/02-spec-21-app-tasklist) — Closed 35-task authoring tasklist kept for audit.
