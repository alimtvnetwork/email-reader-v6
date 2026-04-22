# 04 — Watcher log output is too noisy to read

**Status:** solved in v0.14.0
**Date:** 2026-04-22 (Asia/Kuala_Lumpur)

## Symptom
User pasted ~3 minutes of `email-read watch admin` output. Every poll printed
7 lines (poll start, watch state, dialing, connected, mailbox stats,
fetching, no new messages). With a 3s poll interval, that's ~140 lines/min
of pure noise. The one actual event — a new message arriving at 18:58:27 —
was buried in the wall of text. Quote: "This log is so confusing. Can you
make it readable? It looks mass."

## Root cause
The watcher had no log-level concept. Every step of every poll was
unconditionally logged at the same level. Per-frame error stack traces (from
the v0.12 errtrace work) and per-poll diagnostic lines (from the v0.11 RCA
work) were both designed for *debugging* sessions, but they ran in the
*default* mode too. The defaults were optimised for a developer hunting a
bug, not for an admin who just wants to know "is it working and did anything
happen?".

## Fix (v0.14.0)
Two-tier logging: **quiet by default**, opt-in `--verbose` (or `-v`).

### Quiet mode (default) prints only:
- One-line startup banner: `[alias] watching email@host:port (poll=3s) press Ctrl+C to stop`
- Baseline-set event (one-time, when `LastUid=0` → `UIDNEXT-1`)
- New-mail arrival: `[alias] ✉ N new message(s)` followed by indented
  per-message lines `uid=N from=addr subj="..."`
- URL opens (`→ opened https://...`)
- Errors with full errtrace (de-duplicated: identical consecutive errors
  print once, not on every retry)
- UIDVALIDITY changes (mailbox reset on server)
- Heartbeat every ~3 minutes (60 polls): `♥ alive — 3m, 60 polls,
  mailbox messages=N uidNext=N (no new mail since last heartbeat)`

### Verbose mode (`--verbose` / `-v`) adds back:
- Per-poll start, watch-state load, dial, connect-time, select-stats lines
- "no new messages" idle confirmation
- "duplicate uid=N" and "skip url (already opened)" lines
- "server state: messages X→Y" diagnostic on every change

### Code changes
- `internal/watcher/watcher.go`: added `Options.Verbose bool`; gated all
  per-step logs behind `if v { ... }`; new-mail and error paths are
  unconditional. Heartbeat interval raised from 20 → 60 polls. Added
  `lastError` string to suppress identical consecutive errors in quiet mode
  (verbose still prints every occurrence).
- `internal/cli/cli.go`: added persistent `--verbose`/`-v` flag on root
  command (so `email-read -v admin` and `email-read watch -v admin` both
  work); threaded through `runWatch(ctx, alias, verbose)` to
  `watcher.Options.Verbose`.
- `cmd/email-read/main.go`: bumped to `0.14.0`.

## How to verify
```powershell
.\run.ps1
email-read watch admin              # quiet — silent until mail/error
email-read watch admin --verbose    # full per-poll trace (old behaviour)
```

In quiet mode, an idle 3-minute window now prints ~3 lines (startup banner +
1 heartbeat) instead of ~140.

## Why this still satisfies "use error stack trace"
The errtrace requirement applies to **error paths** — and those still print
the full `errtrace.Format(err)` chain in both modes. We only suppressed
*non-error* per-step success logs in quiet mode. When something actually
fails, the user still sees:
```
[admin] ✗ poll error:
error: dial imap: imap login admin@host: EOF
  at internal/watcher/watcher.go:NN (watcher.pollOnce)
  at internal/mailclient/mailclient.go:NN (mailclient.Dial)
```
