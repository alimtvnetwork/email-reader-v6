# 02 — Watcher appears frozen when idle (no log output)

**Status:** solved in v0.11.0
**Severity:** High
**Area:** 05-watch, logging
**Opened:** 2026-04-21
**Resolved:** 2026-04-21
**Spec links:** [../../02-features/05-watch/](../../02-features/05-watch/), [../../05-logging-strategy.md](../../05-logging-strategy.md), [../../02-features/05-watch/01-backend.md](../../02-features/05-watch/01-backend.md)
**Source:** `.lovable/solved-issues/02-watcher-silent-on-healthy-idle.md`

---

## Symptom

After `email-read watch <alias>` printed:

```
starting watcher (poll=3s, host=...)
```

…the terminal stayed silent for minutes. The user assumed the process was hung or broken, even though `WatchState` in SQLite proved the watcher had successfully completed at least one poll (`ab|1|`).

## Root cause

`internal/watcher/watcher.go` `pollOnce` only logged on:

- error
- first-run baseline
- new messages found
- per-rule URL open

A healthy poll with **zero new messages emitted no log lines**, making a healthy watcher visually identical to a frozen one. There was also no way for the user to verify connection, mailbox stats, or fetch range from logs alone.

## Fix

Two code changes (committed in v0.11.0 on 2026-04-21):

### 1. `internal/mailclient/mailclient.go`

`SelectInbox()` now returns a `MailboxStats` struct (`Name`, `Messages`, `Recent`, `Unseen`, `UidNext`, `UidValidity`) instead of just `uidNext`.

### 2. `internal/watcher/watcher.go`

`pollOnce` now logs every step of every poll:

- poll start + watch state loaded (with `LastUid`)
- dialing IMAP (host, port, tls, user)
- connected + login timing
- mailbox stats (messages, recent, unseen, uidNext, uidValidity)
- fetch range (`UID > LastUid`, server `UidNext`)
- results (no new messages OR fetched N) with timing
- per-message: saved/duplicate with uid, from, subj, file
- per-rule: matched K URLs, opened/skipped each
- poll complete: processed, newLastUid, total ms

Now the user can immediately see WHERE a poll is stuck or failing.

> **Note:** The verbosity of this fix was itself later identified as a problem (issue 04) and gated behind `--verbose` in v0.14.0. The principle established here — "always emit a heartbeat per cycle" — survived as the **Heartbeat invariant** in `spec/21-app/05-logging-strategy.md`.

## Spec encoding

This issue's lessons are now first-class spec rules:

- `spec/21-app/05-logging-strategy.md` — "Heartbeat invariant": every long-running loop must emit at least one log line per cycle in verbose mode and at least one heartbeat per minute in quiet mode.
- `spec/21-app/02-features/05-watch/01-backend.md` §Heartbeat — formalises the 60-poll heartbeat (`♥ alive — 3m, 60 polls, …`).

## What NOT to repeat

- **Do not** write polling loops that are silent on success. At minimum, emit one line per cycle showing connection status and a relevant counter (e.g. `messages=N uidNext=M`).
- **Do** include enough server-state info (`mailbox.Messages`, `mailbox.UidNext`) in logs to distinguish "nothing new on the server" from "delivery never happened".

## Iteration count

1 — recognised on first inspection of the existing logging.
