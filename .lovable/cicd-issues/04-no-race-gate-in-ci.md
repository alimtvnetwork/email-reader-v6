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
