# Memory: index.md
Updated: 2026-04-28

# Project Memory

## Core
mail.attobondcleaning.store `nc` timeout on 993/143 means TCP never reaches IMAP login; fix hosting/Dovecot/firewall/DNS-only, not app password.
Account Test `ER-ACC-22201` is only a wrapper; inspect wrapped `ER-MAIL-*`. Timeout/dial = unreachable endpoint, not rejected password.
IMAP `poll ok` then `i/o timeout` means valid config plus intermittent reachability; keep 5s fixed-rate, preserve ER-MAIL-21208, prefer session reuse/IDLE.
Watch Raw log `start → stop` only means first poll was likely cancelled during long IMAP dial; add/inspect progress telemetry before network tweaks.
Email watcher RCA: stable IMAP `messages`/`uidNext` means no new server-visible mail; diagnose delivery/folder before watcher logic.
IMAP `AUTHENTICATIONFAILED`: always run `email-read doctor <alias>` first to rule out hidden Unicode (U+2060 word-joiner from chat copy-paste) before blaming code.
Test Connection auth failures mean IMAP LOGIN was reached; sanitize password bytes, then verify/reset the mailbox password at the mail host.
All errors must use `internal/errtrace` (no `fmt.Errorf` / `errors.New` / bare `return err` — 3 lints in fail mode); UI handlers additionally call `errlog.ReportError(component, err)` to populate the ring buffer + `data/error-log.jsonl` + `email-read errors tail`.
App spec lives at `spec/21-app/` (App Project Template). Old paths `spec/21-golang-email-reader/` and `spec/22-fyne-ui/` were merged here on 2026-04-25 — never recreate them.
Sandbox has no preinstalled Go; verify with `nix run nixpkgs#go -- {vet,test} -tags nofyne ./...`. Files outside `.lovable/` may revert between sessions — verify on disk before assuming.
AC coverage rollout (Phase 3) is the active workstream: shrink `coverageGapAllowlist` monotonically. Current: 51.6% (96/186), allowlist 90. See `mem://workflow/progress-tracker`.
Honest-scope principle: skipped tests with `t.Logf` + `t.Skip` are tripwires, not coverage; never cite an AC ID in a deferred-skip test (audit treats it as covered).

## Memories
- [Workflow status](mem://workflow/01-status) — Current milestone, completed slices through #131, remaining work, verification commands.
- [Spec hand-off risk report](.lovable/reports/01-spec-handoff-risk.md) — Tiered success-probability map, failure modes, corrective actions. Read before handing slices to a fresh AI.
- [Suggestions folder convention](mem://suggestions/00-readme) — Structured per-suggestion files; index in `mem://suggestions/index`.
- [Root plan.md](plan.md) — AI-handoff backlog with Next task selection menu.
- [AC coverage rollout pattern](mem://decisions/06-ac-coverage-rollout-pattern) — Slice template, AST/linter/log-scan patterns, honest-scope principle, anti-patterns.
- [Desktop run procedure](mem://decisions/05-desktop-run-procedure) — How to run `email-read` CLI + UI on Windows/macOS/Linux + smoke-test checklist.
- [Architecture decisions](mem://decisions/01-architecture) — Core architectural choices.
- [Build and deploy](mem://decisions/02-build-and-deploy) — Build pipeline + run.ps1/run.sh.
- [IMAP auth failed root causes](mem://decisions/03-imap-auth-debugging) — Wrong password OR hidden Unicode; check doctor first.
- [Watch Raw log empty RCA](mem://decisions/07-watch-raw-log-empty-rca) — "● Watching" but Raw log empty: alias-filter mismatch, watcher exited fast (auth fail), seed-password env unset, or stale singleton after config edit. Diagnose via Error Log first.
- [spec/21-app folder rename](mem://decisions/04-spec-21-app-rename) — 2026-04-25 merge of 21-golang-email-reader + 22-fyne-ui.
- [Error stack traces preference](mem://preferences/01-error-stack-traces) — Always use `internal/errtrace`.
- [Error-trace migration rollout](mem://features/02-error-trace-rollout) — Phase 1 baseline (46 production sites still bypass errtrace); update after each Phase-2 slice. Lint scripts wired into run.sh / run.ps1 (warn-only).
- [Testing guide](mem://testing-guide) — Project-wide test conventions.
- [Session 2026-04-21](mem://sessions/01-2026-04-21) — IMAP auth debugging session.
- [Session 2026-04-21 debugging](mem://sessions/02-2026-04-21-debugging) — Verbose poll logging round.
- [Archived: spec-21-app tasklist](mem://archive/02-spec-21-app-tasklist) — Closed 35-task authoring tasklist.
- [Watch Raw log lifecycle mirror](mem://workflow/watch-raw-log-lifecycle-slice199) — Start/Stop/Error are mirrored from core.Watch to watcher.Bus so Raw log gets an immediate line even on fast-fail.
- [Watch start/stop only RCA (Slice #205)](mem://workflow/watch-start-stop-only-rca-slice205) — Raw log with only lifecycle lines means first poll was cancelled during long dial; solution is visible progress telemetry before more network changes.
- [IMAP intermittent timeout after successful poll-ok (Slice #206/#208)](mem://workflow/imap-intermittent-timeout-after-pollok-slice206) — Poll-ok then timeout proves valid config but intermittent reachability; keep 5s fixed-rate, preserve timeout code, prefer session reuse/IDLE.
- [Account Test Connection auth failure](mem://workflow/account-test-auth-failed-slice200) — Sanitized Test Connection password path and wrapped IMAP LOGIN rejects as ER-ACC-22201 with actionable RCA.
- [Account Test Connection timeout misclassified (Slice #209)](mem://workflow/account-test-timeout-misclassified-slice209) — ER-ACC-22201 wraps all test failures; wrapped ER-MAIL-21208/21201 means endpoint unreachable, not password rejected.
- [mail.attobondcleaning.store IMAP timeout RCA (Slice #210)](mem://workflow/mail-attobondcleaning-imap-timeout-rca-slice210) — External `nc` timeout to 993/143 means TCP never reaches IMAP login; fix hosting/Dovecot/firewall/DNS-only or hostname.
- [Blockers 1/2/3/5 shipped (Slice #201)](mem://workflow/blockers-1-2-3-5-shipped-slice201) — `doctor browser` subcommand, GitHub Actions race-gate workflow, `internal/mockimap` (E.1 scope), `internal/benchgate` advisory harness; #4 canvas + #6 human-review remain deferred.
- [ER-WCH-21412 stale rt.cfg after AddAccount (Slice #211)](mem://workflow/wch-21412-stale-rt-cfg-slice211.md) — Watch failed with "account not found" right after Add Account; WatchRuntime.cfg was a one-shot snapshot. Fix: AccountAdded/Updated reloads config + re-registers Refresher.
