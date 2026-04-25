# 04 — Watcher log output is too noisy to read

**Status:** solved in v0.14.0
**Severity:** Medium
**Area:** 05-watch, logging
**Opened:** 2026-04-22
**Resolved:** 2026-04-22
**Spec links:** [../../02-features/05-watch/](../../02-features/05-watch/), [../../05-logging-strategy.md](../../05-logging-strategy.md)
**Source:** `.lovable/solved-issues/04-noisy-watcher-log-output.md`

---

## Symptom

User pasted ~3 minutes of `email-read watch admin` output. Every poll printed 7 lines (poll start, watch state, dialing, connected, mailbox stats, fetching, no new messages). With a 3-second poll interval, that is ~140 lines/min of pure noise. The one actual event — a new message arriving at 18:58:27 — was buried in the wall of text.

> **User quote:** "This log is so confusing. Can you make it readable? It looks mass."

## Root cause

The watcher had no log-level concept. Every step of every poll was unconditionally logged at the same level. Per-frame error stack traces (from the v0.12 errtrace work) and per-poll diagnostic lines (from issue 02 / v0.11) were both designed for *debugging* sessions, but they ran in the *default* mode too. The defaults were optimised for a developer hunting a bug, not for an admin who just wants to know "is it working and did anything happen?".

This was a direct over-correction of issue 02 (silent watcher).

## Fix (v0.14.0)

Two-tier logging: **quiet by default**, opt-in `--verbose` (or `-v`).

### Quiet mode (default) prints only

- One-line startup banner: `[alias] watching email@host:port (poll=3s) press Ctrl+C to stop`
- Baseline-set event (one-time, when `LastUid=0` → `UIDNEXT-1`)
- New-mail arrival: `[alias] ✉ N new message(s)` followed by indented per-message lines `uid=N from=addr subj="..."`
- URL opens (`→ opened https://...`)
- Errors with full errtrace (de-duplicated: identical consecutive errors print once, not on every retry)
- `UIDVALIDITY` changes (mailbox reset on server)
- Heartbeat every ~3 minutes (60 polls): `♥ alive — 3m, 60 polls, mailbox messages=N uidNext=N (no new mail since last heartbeat)`

### Verbose mode (`--verbose` / `-v`) adds back

- Per-poll start, watch-state load, dial, connect-time, select-stats lines
- "no new messages" idle confirmation
- "duplicate uid=N" and "skip url (already opened)" lines
- "server state: messages X→Y" diagnostic on every change

### Code changes

- `internal/watcher/watcher.go` — added `Options.Verbose bool`; gated all per-step logs behind `if v { ... }`; new-mail and error paths are unconditional. Heartbeat interval raised from 20 → 60 polls. Added `lastError` string to suppress identical consecutive errors in quiet mode (verbose still prints every occurrence).
- `internal/cli/cli.go` — added persistent `--verbose` / `-v` flag on root command.

## Spec encoding

- `spec/21-app/05-logging-strategy.md` §Quiet vs Verbose — codifies the two-tier rule. Default is quiet; `--verbose` is opt-in.
- `spec/21-app/02-features/05-watch/01-backend.md` §Logging modes — explicit list of quiet-only / verbose-only / always-emitted events.
- `spec/21-app/02-features/05-watch/02-frontend.md` §Live log toolbar — Fyne UI exposes a "Verbose" toggle and a "De-dup repeats" toggle that mirror the CLI flag.

## What NOT to repeat

- **Do not** unconditionally log every step in a polling loop. Distinguish *event* logs (new mail, errors, URL opens) from *trace* logs (poll boundaries, dial/connect timings) and gate the latter behind a verbosity flag.
- **Do not** remove heartbeats when reducing noise — they were added for issue 02 and must remain in quiet mode (just at a lower frequency).
- **Do** de-duplicate identical consecutive errors in quiet mode; print every occurrence in verbose.
