---
name: ER-WCH-21412 stale rt.cfg after AddAccount (Slice #211)
description: Watch fails with `account <alias> not found` right after Add Account because `WatchRuntime.cfg` is loaded once at boot and never refreshed; resolver in buildLoopFactory reads the stale snapshot. Fix: on AccountAdded/Updated events, reload config from disk and re-register Refresher.
type: feature
---

## Symptom
- User adds account `admin` via UI → Test Connection succeeds.
- Click Watch → Start → immediately:
  `[ER-WCH-21412] watch: account admin not found` then watch stopped.

## RCA
- `internal/ui/watch_runtime.go` builds `rt.cfg` once via `config.Load()` inside `buildWatchRuntime`.
- `buildLoopFactory.resolver` closure does `rt.cfg.FindAccount(alias)` under RLock — but nothing ever writes to `rt.cfg`.
- `core.AddAccount` persists to disk + publishes `AccountAdded`, but the UI subscriber (`forwardAccountRemovalEvents`) only handled `AccountRemoved` and explicitly commented "AccountAdded / AccountUpdated are handled elsewhere (resolver re-reads cfg before each Start)" — which was false. The resolver re-reads `rt.cfg`, not disk.
- Test Connection works because `core.TestConnection` loads its own fresh config snapshot.

## Fix
- Extended `forwardAccountRemovalEvents` to also handle Added/Updated by calling new `reloadRuntimeConfig(rt, alias)`:
  1. `config.Load()` → write-lock `rt.cfgMu` → swap `rt.cfg`.
  2. Re-register the alias in `rt.Refresher` so the 🔄 Refresh button also picks up the new account.
- Removed/Added/Updated branches all share one event loop; Removed still triggers `Watch.Stop`.

## Verification
- `nix run nixpkgs#go -- vet -tags nofyne ./internal/ui/...` clean.
- `nix run nixpkgs#go -- test -tags nofyne -count=1 ./internal/ui/... ./internal/core/...` green.
