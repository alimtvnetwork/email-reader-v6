---
name: Watch Raw log silent on Start — RCA + Slice #199 fix
description: Watch header can show Watching while Raw log has no immediate line because Start/Stop lifecycle was only on core.Watch, not watcher.Bus. Fixed by mirroring lifecycle to watcher.Bus before loop start.
type: feature
---

# Symptom

User clicks Watch → Start. Header/status can indicate watching, but the
Raw log tab/file shows nothing immediately. This differs from earlier
versions where Start produced a visible line and poll results followed.

# Root cause

The Watch UI consumes two related streams:

- Header status subscribes to `core.Watch.Subscribe()` (`WatchEvent`).
- Raw log/Cards/footer subscribe directly to `watcher.Bus` (`watcher.Event`).

`core.Watch.Start()` published `WatchStart` only on the `core.Watch` bus.
`watcher.Run()` published `watcher.EventStarted`, but only after the real
loop goroutine entered `Run`. If the loop failed fast, alias resolution failed,
or the user was watching the Raw tab before the low-level loop reached its
first publish, the header could update while Raw log had no line proving the
subscriber was alive.

So the remaining bug was not account deletion or IMAP credentials; it was a
split lifecycle stream. Start/Stop/Error were not guaranteed to be mirrored to
the same bus Raw log reads.

# Fix (Slice #199)

- Added a narrow `core.WatcherEventPublisher` seam.
- `core.Watch` now publishes lifecycle via `publishLifecycle`, which still
  writes `WatchEvent` and additionally mirrors Start/Stop/Error to a loop
  factory that supports the seam.
- `realLoopFactory` implements the seam by publishing corresponding
  `watcher.EventStarted`, `watcher.EventStopped`, and `watcher.EventPollError`
  onto the configured `watcher.Bus`.
- `watcher.Options.SuppressLifecycleEvents` prevents duplicate Started/Stopped
  lines when `core.Watch` is the owner of the UI bus; direct CLI watcher runs
  keep the original watcher lifecycle publishing.
- Added regression test `TestWatch_Start_MirrorsLifecycleToWatcherBus` so a
  missing/fast-failing account still produces an immediate Raw-log `started`
  event.

# Verification

- `nix run nixpkgs#go -- test -tags nofyne -count=1 ./internal/core/... ./internal/watcher/...` → PASS.
- `nix run nixpkgs#go -- vet -tags nofyne ./internal/core/... ./internal/watcher/...` → PASS.

# User-facing expectation

After restarting the desktop app with this build, clicking Start should always
append at least:

`▶ [<alias>] watch started`

in Raw log immediately. If credentials are wrong, poll-error lines should then
follow; if no account is selected, Start remains disabled.
