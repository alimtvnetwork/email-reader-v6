//go:build !race

// perfgate_norace_test.go — non-race-build half of the PerfGate skip
// helper. See perfgate_race_test.go for the rationale.
//
// In a normal `go test` run (no -race) the gates must run, so the
// helper returns false and never skips. Keeping the function name and
// signature identical to the race-build version means callers don't
// need build tags themselves.
package core

import "testing"

// perfGateSkipRace is the no-op variant for non-race builds: returns
// false so callers proceed with the budget assertion.
func perfGateSkipRace(_ *testing.T) bool { return false }
