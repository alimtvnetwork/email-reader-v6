# Memory: index.md
Updated: 2026-04-27

# Project Memory

## Core
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
IMAP `AUTHENTICATIONFAILED`: always run `email-read doctor <alias>` first to rule out hidden Unicode (U+2060 word-joiner from chat copy-paste) before blaming code.
All errors must use `internal/errtrace` so failures show file:line stack traces.
App spec lives at `spec/21-app/` (App Project Template). Old paths `spec/21-golang-email-reader/` and `spec/22-fyne-ui/` were merged here on 2026-04-25 — never recreate them.
Sandbox has no preinstalled Go; verify with `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`. Files outside `.lovable/` may revert between sessions — verify on disk before assuming.
AC coverage rollout (Phase 3) is the active workstream: shrink `coverageGapAllowlist` monotonically. Current: 51.6% (96/186), allowlist 90. See `mem://workflow/progress-tracker`.
Honest-scope principle: skipped tests with `t.Logf` + `t.Skip` are tripwires, not coverage; never cite an AC ID in a deferred-skip test (audit treats it as covered).

## Memories
- [Workflow status](mem://workflow/01-status) — Current milestone, completed slices through #131, remaining work, verification commands.
- [AC coverage rollout pattern](mem://decisions/06-ac-coverage-rollout-pattern) — Slice template, AST/linter/log-scan patterns, honest-scope principle, anti-patterns.
- [Desktop run procedure](mem://decisions/05-desktop-run-procedure) — How to run `email-read` CLI + UI on Windows/macOS/Linux + smoke-test checklist.
- [Architecture decisions](mem://decisions/01-architecture) — Core architectural choices.
- [Build and deploy](mem://decisions/02-build-and-deploy) — Build pipeline + run.ps1/run.sh.
- [IMAP auth failed root causes](mem://decisions/03-imap-auth-debugging) — Wrong password OR hidden Unicode; check doctor first.
- [spec/21-app folder rename](mem://decisions/04-spec-21-app-rename) — 2026-04-25 merge of 21-golang-email-reader + 22-fyne-ui.
- [Error stack traces preference](mem://preferences/01-error-stack-traces) — Always use `internal/errtrace`.
- [Testing guide](mem://testing-guide) — Project-wide test conventions.
- [Session 2026-04-21](mem://sessions/01-2026-04-21) — IMAP auth debugging session.
- [Session 2026-04-21 debugging](mem://sessions/02-2026-04-21-debugging) — Verbose poll logging round.
- [Archived: spec-21-app tasklist](mem://archive/02-spec-21-app-tasklist) — Closed 35-task authoring tasklist.
