# 04 — No `-race` gate in PR pipeline

## Description
The race detector (`go test -race`) catches goroutine data races but is
not currently a required PR gate. Race issues have been caught manually
in the past (e.g. CF_A2 race fixed in Slice #24 via `config.WithWriteLock`).

## Impact
Risk of regressions in concurrent code paths (watcher loops, event bus,
maintenance scheduler) that pass normal CI but would fail under `-race`.

## Workaround
Manual race sweeps with:
```bash
nix run nixpkgs#go -- test -tags nofyne -race -count=2 ./...
```
Documented as the canonical race-stress invocation in `mem://workflow/01-status`.

## Status
⏳ Pending — requires external CI runner provisioning. Tracked in
`.lovable/plan.md` cross-cutting items.

### 2026-04-27 update (Slices #189–#192)
The manual race-stress invocation above is now **fully green** end-to-end:

- Slice #189 surfaced two latent bugs the race sweep had been hiding:
  `errlog.Persistence` nil-writer deref and `store/migrate` registry
  wipe-not-restore.
- Slices #190 + #191 fixed both.
- Slice #192 added a `perfGateSkipRace(t)` helper (build-tag pair
  `internal/core/perfgate_{race,norace}_test.go`) so wall-clock p95
  budgets don't flap under race-detector overhead.

Net effect: any developer can run the canonical sweep locally and trust
the result. CI provisioning is now purely a deployment concern, not a
correctness one — the gate itself is ready to be wired in.
