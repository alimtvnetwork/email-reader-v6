---
name: Watch start/stop only RCA (Slice #205)
description: When Raw log shows only start/stop and no poll/error lines, the runner was stopped before the first long IMAP dial produced a result; fix is progress telemetry, not another blind network tweak.
type: feature
---

# Watch start/stop only RCA — Slice #205

## Symptom

Raw log shows only lifecycle lines such as:

```text
17:28:12  ▶ [admin] watch started
17:28:16  ■ [admin] watch stopped
```

No `poll ok`, no `poll error`, no `baseline`, no `heartbeat`, and no new-mail card appears.

## Root cause

This is not proof that the watcher did nothing. After Slice #203/#204 the IMAP dial/login timeout was increased to 30s and Stop now cancels in-flight dials cleanly. If the user starts the watcher and stops it after only a few seconds, the first poll is still inside `mailclient.DialContext`; Stop cancels it and the code intentionally suppresses the context-cancelled error. The only visible events are therefore the mirrored lifecycle events: started → stopped.

The UX bug is missing in-progress telemetry. `watcher.Run` publishes `EventPollOK`, `EventPollError`, `EventBaseline`, etc. only after a poll reaches a terminal state. It does not publish structured `poll_start`, `dialing`, `connected`, or `selecting mailbox` events, and the desktop UI runs the watcher in quiet mode, so the Raw log has no line while the first dial is pending.

## Why previous fixes did not appear to work

- Increasing timeout helped slow networks but also made the silent pending window longer.
- STARTTLS fallback only runs after the primary 993 attempt times out, so it cannot show anything if the user stops before the timeout.
- Stop cancellation is working correctly; it prevents a late `i/o timeout` after Stop, but that also means no error line appears for user-cancelled dials.

## Correct solution plan, one slice at a time

1. Add non-secret structured watcher events for poll progress: `poll_start`, `dialing`, `connected`, `mailbox_selected`, and optionally `fallback_starttls`.
2. Render those events in the Watch Raw log so the user immediately sees `connecting to mail.attobondcleaning.store:993...` after Start.
3. Keep cancellation suppressed as an error, but publish a friendly stopped/cancelled progress line when Stop interrupts an in-flight poll.
4. Add tests proving a started watcher emits progress before the first successful poll/error and that Stop during dial does not look like a silent no-op.
5. Only after telemetry is visible, re-evaluate actual IMAP reachability using the existing diagnostics if the dial still times out after the full timeout budget.

## Rule for future debugging

For `start → stop only` logs, do not keep changing host/port/timeout first. First add or inspect progress telemetry around the first poll. The important question is: did the poll reach dial/login/select/fetch, or did the user cancel before those phases completed?
