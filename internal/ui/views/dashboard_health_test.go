// dashboard_health_test.go — Slice #103: per-account health rollup
// formatter. Verifies the one-line summary string rendered into the
// health label, including counter math and the empty-input fallback.
//
//go:build !nofyne

package views

import (
	"testing"

	"github.com/lovable/email-read/internal/core"
)

func TestFormatHealthRollup_EmptyInput_ReturnsNoAccounts(t *testing.T) {
	if got := formatHealthRollup(nil); got != "Health: (no accounts configured)" {
		t.Fatalf("empty input: got %q", got)
	}
}

func TestFormatHealthRollup_CountsByLevel(t *testing.T) {
	rows := []core.AccountHealthRow{
		{Alias: "a", Health: core.HealthHealthy},
		{Alias: "b", Health: core.HealthHealthy},
		{Alias: "c", Health: core.HealthHealthy},
		{Alias: "d", Health: core.HealthWarning},
		{Alias: "e", Health: core.HealthError},
		{Alias: "f", Health: core.HealthError},
	}
	got := formatHealthRollup(rows)
	want := "Health: 3 ● healthy · 1 ◐ warning · 2 ✗ error"
	if got != want {
		t.Fatalf("rollup mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatHealthRollup_UnknownLevel_IgnoredInCounts(t *testing.T) {
	// Defensive: a future HealthLevel value (e.g. "Unknown") should
	// not crash and should simply not be counted in any bucket.
	rows := []core.AccountHealthRow{
		{Alias: "x", Health: "Unknown"},
		{Alias: "y", Health: core.HealthHealthy},
	}
	got := formatHealthRollup(rows)
	want := "Health: 1 ● healthy · 0 ◐ warning · 0 ✗ error"
	if got != want {
		t.Fatalf("unknown-level rollup: got %q want %q", got, want)
	}
}
