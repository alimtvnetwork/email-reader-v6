package core

import (
	"testing"
	"time"
)

// helpers
func at(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

// TestShouldRunWalCheckpoint covers the three branches: zero lastRun
// (first tick), interval not yet reached, interval crossed.
func TestShouldRunWalCheckpoint(t *testing.T) {
	now := at(2026, 4, 26, 12)
	cases := []struct {
		name    string
		last    time.Time
		hours   int
		wantRun bool
	}{
		{"first tick", time.Time{}, 6, true},
		{"5h after last → wait", at(2026, 4, 26, 7), 6, false},
		{"6h exactly → fire", at(2026, 4, 26, 6), 6, true},
		{"8h after last → fire", at(2026, 4, 26, 4), 6, true},
		{"defaults to 6h when 0", at(2026, 4, 26, 5), 0, true},
	}
	for _, c := range cases {
		got := ShouldRunWalCheckpoint(c.last, now, c.hours)
		if got != c.wantRun {
			t.Errorf("%s: got %v, want %v", c.name, got, c.wantRun)
		}
	}
}

// TestShouldRunWeeklyVacuum covers weekday/hour matching and the
// 23-hour debounce that prevents firing twice within the same slot.
func TestShouldRunWeeklyVacuum(t *testing.T) {
	// Sunday 2026-04-26 03:00 UTC
	sun3am := at(2026, 4, 26, 3)
	cases := []struct {
		name    string
		last    time.Time
		now     time.Time
		wkday   time.Weekday
		hour    int
		wantRun bool
	}{
		{"never ran, in slot", time.Time{}, sun3am, time.Sunday, 3, true},
		{"never ran, wrong weekday", time.Time{}, at(2026, 4, 25, 3), time.Sunday, 3, false},
		{"never ran, wrong hour", time.Time{}, at(2026, 4, 26, 4), time.Sunday, 3, false},
		{"already ran 5min ago in slot", sun3am.Add(-5 * time.Minute), sun3am, time.Sunday, 3, false},
		{"ran a week ago, back in slot", at(2026, 4, 19, 3), sun3am, time.Sunday, 3, true},
		{"out-of-range hour clamped to 23", time.Time{}, at(2026, 4, 26, 23), time.Sunday, 99, true},
		{"out-of-range hour clamped to 0", time.Time{}, at(2026, 4, 26, 0), time.Sunday, -5, true},
	}
	for _, c := range cases {
		got := ShouldRunWeeklyVacuum(c.last, c.now, c.wkday, c.hour)
		if got != c.wantRun {
			t.Errorf("%s: got %v, want %v", c.name, got, c.wantRun)
		}
	}
}
