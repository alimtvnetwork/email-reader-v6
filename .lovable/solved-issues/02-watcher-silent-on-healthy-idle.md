# Solved — Watcher appears frozen when idle (no log output)

## Description
After `email-read watch <alias>` printed `starting watcher (poll=3s, host=...)`, the terminal stayed silent for minutes. User assumed the process was hung or broken, even though `WatchState` in SQLite proved the watcher had successfully completed at least one poll (`ab|1|`).

## Root Cause
`internal/watcher/watcher.go` `pollOnce` only logged on:
- error
- first-run baseline
- new messages found
- per-rule URL open

A healthy poll with zero new messages emitted **no log lines**, making a healthy watcher visually identical to a frozen one. There was also no way for the user to verify connection, mailbox stats, or fetch range from logs alone.

## Steps to Reproduce (with old code)
1. `email-read watch <alias>` with valid credentials and an empty inbox.
2. Wait — observe no output.
3. Cannot tell whether poll succeeded, login failed silently, or the process hung.

## Solution
Made two code changes (session 02 on 2026-04-21):

1. `internal/mailclient/mailclient.go` — `SelectInbox()` now returns a `MailboxStats` struct (Name, Messages, Recent, Unseen, UidNext, UidValidity) instead of just `uidNext`.

2. `internal/watcher/watcher.go` — `pollOnce` now logs every step of every poll:
   - poll start + watch state loaded (with lastUid)
   - dialing IMAP (host, port, tls, user)
   - connected + login timing
   - mailbox stats (messages, recent, unseen, uidNext, uidValidity)
   - fetch range (UID > lastUid, server uidNext)
   - results (no new messages OR fetched N) with timing
   - per-message: saved/duplicate with uid, from, subj, file
   - per-rule: matched K URLs, opened/skipped each
   - poll complete: processed, newLastUid, total ms

Now the user can immediately see WHERE a poll is stuck or failing.

## Iteration Count
1 (recognised on first inspection of the existing logging).

## Learning
- For long-running background processes, always log a per-cycle heartbeat — even success. "No news is good news" is a terrible UX for CLI watchers.
- Include enough server-state info (mailbox.Messages, mailbox.UidNext) in logs to distinguish "nothing new" from "delivery never happened".

## What NOT to Repeat
- Do not write polling loops that are silent on success. At minimum, emit one line per cycle showing connection status and a relevant counter (e.g. `messages=N uidNext=M`).
