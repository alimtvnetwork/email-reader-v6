//go:build race

// perfgate_race_test.go — race-build half of the PerfGate skip helper.
//
// Wall-clock p95 budgets in *_bench_test.go are calibrated for an
// optimised binary. The race detector adds 2-20× overhead per memory
// access, so under `go test -race` (and especially `-race -count>1`)
// these gates flap with false positives that say nothing about real
// production performance — they only say "the race detector is slow,
// which we already knew".
//
// Slice #192 introduced this file (paired with perfgate_norace_test.go)
// so every PerfGate can `if perfGateSkipRace(t) { return }` without
// per-test boilerplate. We deliberately log AND skip — the log line
// makes it visible in CI output that the gate ran in race mode and was
// intentionally short-circuited rather than silently passing.
package core

import "testing"

// perfGateSkipRace returns true when the binary was built with -race.
// In that case it also calls t.Skip with a clear reason so the test
// status shows SKIP (not PASS), preserving the "this gate did not
// actually run" signal in CI dashboards.
func perfGateSkipRace(t *testing.T) bool {
	t.Helper()
	t.Skip("PerfGate skipped under -race: race detector overhead invalidates wall-clock p95 budgets (Slice #192). Run without -race for budget enforcement.")
	return true
}
