// retention_test.go covers the pure scheduling helpers in retention.go.
// The end-to-end CF-S-RET test (settings → store DELETE) lives in
// cf_acceptance_retention_test.go.
package core

import (
	"testing"
	"time"
)

func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		days uint16
		want time.Time
	}{
		{"disabled", 0, time.Time{}},
		{"one day", 1, now.Add(-24 * time.Hour)},
		{"ninety days", 90, now.Add(-90 * 24 * time.Hour)},
		{"one year", 365, now.Add(-365 * 24 * time.Hour)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RetentionCutoff(now, c.days)
			if !got.Equal(c.want) {
				t.Fatalf("RetentionCutoff(%v) = %v, want %v", c.days, got, c.want)
			}
		})
	}
}

func TestShouldRunRetentionTick(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		last     time.Time
		now      time.Time
		interval int
		days     uint16
		want     bool
	}{
		{"disabled retention skips", now.Add(-48 * time.Hour), now, 24, 0, false},
		{"first ever fire", time.Time{}, now, 24, 90, true},
		{"first ever fire even if disabled is gated", time.Time{}, now, 24, 0, false},
		{"too soon after last", now.Add(-1 * time.Hour), now, 24, 90, false},
		{"exactly at interval", now.Add(-24 * time.Hour), now, 24, 90, true},
		{"well past interval", now.Add(-72 * time.Hour), now, 24, 90, true},
		{"non-positive interval defaults to 24h, too soon", now.Add(-12 * time.Hour), now, 0, 90, false},
		{"non-positive interval defaults to 24h, due", now.Add(-25 * time.Hour), now, -5, 90, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ShouldRunRetentionTick(c.last, c.now, c.interval, c.days)
			if got != c.want {
				t.Fatalf("ShouldRunRetentionTick = %v, want %v", got, c.want)
			}
		})
	}
}
