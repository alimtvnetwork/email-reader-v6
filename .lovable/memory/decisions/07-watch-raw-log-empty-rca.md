---
name: Watch Raw log empty despite "Watching" status — RCA
description: Header shows "● Watching · <alias>" but Raw log + Cards stay empty. Root cause is almost always alias-filter mismatch or watcher silently exiting before publishing. Diagnose with the four checks below.
type: feature
---

# Symptom

Watch view header shows `● Watching · <alias>` and the indeterminate
progress bar animates, but **Raw log tab is completely empty** — no
`✓ poll-ok` lines, no heartbeats, nothing. Cards tab also stays at
"(awaiting first event…)".

In the previous version (pre-Slice #116), the user remembers seeing
poll-ok / pull lines stream in immediately.

# How the wiring SHOULD work

`internal/ui/watch_runtime.go` (singleton):
1. Builds `rt.Bus = watcher.NewBus(64)` once.
2. Hands the SAME `rt.Bus` to `RealLoopFactoryDeps.Bus` (so every
   `pollOnce` writes events to it — see `internal/watcher/watcher.go`
   lines 134/148/157/162/177/195/336/381/388/423).
3. Bridges `rt.Bus → dstBus` so `core.Watch.Subscribe()` returns the
   unified stream for the header status label.

`internal/ui/app.go::renderDetail` for `NavWatch`:
- `opts.Watch = rt.Watch` (header Start/Stop)
- `opts.Bus = rt.Bus`   (Raw log + Cards + counters)

`internal/ui/views/watch.go::subscribeWatchBus`:
- Subscribes to `opts.Bus`, filters `if ev.Alias != opts.Alias { continue }`,
  formats every event into a Raw log line.

So per poll cycle there should be **at least one** `EventPollOK` line
(plus `EventStarted` once at boot, `EventBaseline` after first poll,
`EventHeartbeat` on idle gaps). An empty Raw log means none of these
are reaching the subscriber.

# Root causes (ranked)

## 1. Alias filter mismatch (most likely)

`subscribeWatchBus` drops every event whose `ev.Alias != opts.Alias`.
The watcher publishes with `opts.Account.Alias` from `config.json`.
The Watch view reads `state.Alias()` which is set from the sidebar
account picker — the picker's items come from `LoadAliases() →
core.ListAccounts() → a.Alias` for each account, so they MUST match.

But: if the user **manually added** an account whose alias differs by
case, whitespace, or a hidden Unicode char (cf. solved-issue #03 for
the password equivalent), the picker label and `Account.Alias` no
longer compare equal, and 100% of events are filtered out.

**Diagnose**: run `email-read doctor <alias>` (extended in v0.13.0).
Compare the bytes of the alias the picker shows with `Account.Alias`
in `~/.config/email-read/config.json`.

## 2. Watcher exited before first publish

`opts.Watch.Start()` returns nil even if the loop fails fast (e.g.
`ErrConfigAccountMissing`, dial failure with no retry budget left,
panic in `pollOnce` recovered by goleak guard). The header IsRunning
flips back to false on the next `renderHeaderState`, but the user may
see "● Watching" for a frame.

**Diagnose**: check `data/error-log.jsonl` — `ReportWatchEventError`
mirrors any `EventPollError` there. If it's empty AND Raw log is
empty, the loop never ran a single iteration.

## 3. `EMAIL_READ_SEED_PASSWORD` missing

Per Slice #195 (mem://workflow/seed-password-env), the default admin
account is **skipped** at seed time when the env var is unset. If the
user's "Admin" entry is the seeded one, it has no password and IMAP
LOGIN fails immediately with `ErrAccountAuthFailed`. The watcher
publishes `EventPollError`, the bridge mirrors it to Error Log, and
the Raw log shows it ONCE — but if the loop's retry-then-exit policy
kicks in, the next poll never happens.

**Diagnose**: open NavErrorLog. If it shows `AUTHENTICATIONFAILED` or
`ErrAccountAuthFailed`, this is it.

## 4. Stale singleton after config edit

`buildWatchRuntime` is `sync.Once` — accounts added/edited AFTER first
NavDashboard render are NOT visible to the loop factory's resolver
unless `rt.cfg` is refreshed. The picker may show the new alias but
`resolver(alias)` returns nil → `pollOnce` errors → see #2.

**Diagnose**: restart the binary. If Raw log fills immediately, this
was it.

# Fix order

1. Confirm by checking Error Log first (rules out #2/#3/#4 in one
   look).
2. If Error Log is empty too: alias mismatch (#1) — re-add the
   account fresh and re-select it.
3. If Error Log shows auth failures: re-seed via
   `EMAIL_READ_SEED_PASSWORD` (mem://workflow/seed-password-env) or
   re-add the account with the correct password.
4. If everything looks right but Raw log is still empty after a
   restart: file a diagnose dump (`email-read doctor <alias>`) and
   inspect bytes for hidden Unicode in the alias.

# What we did NOT change

This RCA is diagnosis-only. No code edits were made for the empty
Raw log — every wiring path audited (`watch_runtime.go`,
`watch.go::subscribeWatchBus`, `watcher.go` publish sites) is correct
by inspection. The bug is in user-side configuration or in the
seed-password-env regression surface, not in the view.
