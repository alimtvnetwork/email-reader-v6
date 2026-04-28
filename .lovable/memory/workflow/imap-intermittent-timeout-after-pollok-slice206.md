---
name: IMAP intermittent timeout after successful poll-ok (Slice #206)
description: If admin gets poll-ok lines then a 993/143 timeout later, credentials/config are valid; root cause is per-poll reconnect churn hitting network/host throttling, not constant server failure.
type: feature
---

# IMAP intermittent timeout after successful poll-ok — Slice #206

## Symptom

The Watch Raw log shows successful polls first, then a later timeout:

```text
17:35:05  · [admin] poll ok (messages=210 uidnext=279 unseen=0)
17:35:10  · [admin] poll ok (messages=210 uidnext=279 unseen=0)
17:35:16  · [admin] poll ok (messages=210 uidnext=279 unseen=0)
17:36:19  ✗ [admin] poll error: [ER-MAIL-21201] ... 993 timed out ... 143 timed out
```

## Root cause

This log proves the account, password, DNS, mailbox, and server settings are basically correct: the same process logged in, selected the mailbox, and read stable mailbox stats (`messages=210 uidnext=279`) multiple times.

The failure is intermittent TCP reachability after repeated polling. Current watcher behavior opens a fresh IMAP TCP/TLS/login/select session every poll cycle. With a very short cadence (about 5–6 seconds in the log), shared-hosting/cPanel/Dovecot/firewall systems can temporarily throttle, tarpitting, or drop new IMAP connections from the client IP. That produces `dial tcp ... i/o timeout` even though the server worked seconds earlier.

The STARTTLS fallback is not helping here: when 993 times out, trying 143 also times out and delays the visible error by another timeout window. For this host, fallback should be treated as diagnostic only unless 143 is known to be reachable.

## What NOT to conclude

- Do not call this a wrong password: `poll ok` means LOGIN succeeded.
- Do not call this a permanently down IMAP host: earlier polls succeeded.
- Do not keep blindly increasing dial timeout: it only makes each failed cycle wait longer.
- Do not assume STARTTLS fallback fixes it: the evidence shows 143 times out too.

## Correct solution plan

1. Respect the user's requested cadence. If they explicitly request 5s, do **not** force a 60s clamp in Settings or watcher runtime; 5s is the app default again as of Slice #207.
2. Treat 5s as a product requirement but explain the trade-off honestly: a `dial tcp ... i/o timeout` is still a network/server reachability timeout, and frequent reconnects can amplify it on shared mail hosts.
3. Stop using STARTTLS fallback automatically after a 993 timeout for this account/host, or make it opt-in/diagnostic, because it doubles the failed wait here.
4. Add a circuit breaker/backoff for dial timeouts so after one network timeout the watcher waits longer before trying another brand-new connection, without changing the configured 5s cadence on successful polls.
5. Best long-term fix: reuse one IMAP session or implement IMAP IDLE so the app does not login/logout every few seconds.
6. Keep the `poll ok` heartbeat visible so the user can distinguish healthy idle mailbox state from delivery problems.

## Future debugging rule

When logs contain both `poll ok` and later `i/o timeout`, treat it as intermittent network/server throttling caused or amplified by reconnect frequency. Do not blame credentials if there has been a `poll ok`; prefer backoff/session reuse/IDLE over forcing a slower user-visible cadence.
