---
name: Slice #201 — blockers 1/2/3/5 shipped
description: doctor browser subcommand + race-gate CI + mockimap E.1 + advisory benchgate; what landed and what's still deferred.
type: feature
---
# Slice #201 — Blockers 1/2/3/5 shipped (2026-04-28)

User asked for "All of 1/2/3/5". All four landed in one slice; #4 (canvas) and #6 (human review) remain sandbox-blocked.

## What landed

| # | Deliverable | Files |
|---|---|---|
| 1 | `email-read doctor browser [--probe URL]` + richer `Open()` error wrap | `internal/browser/diagnose.go`, `diagnose_test.go`, `internal/browser/browser.go` (Open wrap), `internal/cli/cli.go` (subcommand) |
| 5 | GitHub Actions: fast vet+test gate AND `-race -count=2` sweep | `.github/workflows/go-test.yml`, `.github/workflows/race-gate.yml` |
| 3 | Mock IMAP server (E.1 scope: CAPABILITY, LOGIN, SELECT, NOOP, LOGOUT + Deliver/FailNextLogin injection) | `internal/mockimap/server.go`, `server_test.go` |
| 2 | Advisory bench harness (`Measure`, `Report`, ModeAdvisory/ModeEnforcing) | `internal/benchgate/benchgate.go`, `benchgate_test.go` |

CLI version bumped 0.29.0 → 0.30.0 per project rule.

## Verification
- `nix run nixpkgs#go -- vet -tags nofyne ./...` — clean.
- `nix run nixpkgs#go -- test -tags nofyne ./internal/{browser,mockimap,benchgate}/...` — all green.
- Browser: 4 new tests (Diagnose picks override, no-browser path, FormatReport sections, plus existing 3 still pass).
- mockimap: 4 tests covering happy login/select, bad password, FailNextLogin injection, Deliver mutation.
- benchgate: 3 tests covering happy-path, breach detection, error preservation.

## What's still deferred
- **#4 Canvas harness #118e** — needs cgo + GL workstation; sandbox cannot build Fyne.
- **#6 Tools spec sign-off + AC-SF-21 tab-order** — requires a human reviewer.
- **Bench gates beyond scaffolding** — `internal/benchgate` is the harness only. Wiring per-feature `DefaultGates` (AC-DBP-01..06, AC-SP-01..05) onto real workloads needs stable hardware to declare meaningful budgets; flip `Mode = ModeEnforcing` there.
- **E2E slices E.5–E.12** — multi-process tests using `mockimap` against `watcher.Run`; the mock is ready, the wiring is the next slice.

## Why advisory mode for benchgate
The sandbox runs on shared CPU; p95 timings would flap. Advisory mode lets the harness land + render results without producing false PR failures. Promotion to enforcing is a one-line change once the workflow runs on a stable runner.
