---
id: 20260427-170000-migrate-race-count2-not-idempotent
title: store/migrate m0011..m0014 fail under `-race -count=2`
status: open
severity: medium
discovered_by: Slice #190 (full race sweep after errlog fix unmasked it)
discovered_at: 2026-04-27
---

## Symptom

`nix run nixpkgs#go -- test -tags nofyne -race -count=2 ./internal/store/migrate/`
passes on iter 1 but fails on iter 2 with:

- `Version 12 not registered (m0012 init missing?)`
- `Version 13 not registered (m0013 init missing?)`
- `Version 14 not registered (m0014 init missing?)`
- `ledger row count for v11 = 0, want 1`
- cascading `no such table: Emails` failures.

`-count=1` is green. Smells like a package-level migration registry
populated via `init()` or `sync.Once` that doesn't re-arm between test
iterations, OR test bodies that mutate the shared registry without
restoring it via `t.Cleanup`.

## Suggested triage

1. Audit `internal/store/migrate/registry.go` (or equivalent) for
   `sync.Once` / package-level `var registry = ...` patterns.
2. Add a `resetRegistryForTest()` helper and call it from `t.Cleanup`
   in every `TestM00xx_RegistryEntry` test.
3. Re-run `-race -count=2 ./internal/store/migrate/` to confirm green.

## Why this matters

Pre-existing — masked previously because the canonical race sweep
panicked earlier in `internal/ui/errlog` (Slice #189) and never
reached migrate. Slice #190 fixed errlog and exposed this.
