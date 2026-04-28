---
name: Watch Stop cancels in-flight IMAP poll
description: Slice #202 made watcher Stop cancel/suppress in-flight IMAP dial errors so Raw log stops cleanly after stop.
type: feature
---
# Slice #202 — Watch Stop cancels in-flight IMAP poll

Fixed the Raw log sequence where clicking Stop during an unreachable IMAP poll could still print `[ER-MAIL-21201] poll error` before/around the stop lifecycle line.

- Added `mailclient.DialContext` with a cancellable dialer that closes the socket when the watch context is cancelled.
- `watcher.connectAndSelect` now uses `DialContext` and treats context cancellation as a clean stop, not a poll error.
- `runLoop` now logs `stopped` and exits immediately if a poll returns after cancellation.
- Verified with `nix run nixpkgs#go -- fmt`, `vet -tags nofyne`, and focused tests for `internal/mailclient`, `internal/watcher`, and `internal/core`.

Important RCA: continuing `[ER-MAIL-21201]` while watching means the configured IMAP endpoint is still unreachable (`mail.attobondcleaning.store:993` timeout/DNS issue). This slice fixes Stop behavior and cancellation noise; it cannot make that external mail server reachable.