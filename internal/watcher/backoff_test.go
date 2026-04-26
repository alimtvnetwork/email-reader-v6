package watcher

import (
	"testing"
	"time"
)

// TestNextPollDelay_NoErrors_ReturnsBaseCadence: the happy path —
// streak == 0 means the last poll succeeded, so we use the configured
// cadence with no backoff and no jitter.
func TestNextPollDelay_NoErrors_ReturnsBaseCadence(t *testing.T) {
	got := NextPollDelay(3*time.Second, 0, 0.5)
	if got != 3*time.Second {
		t.Fatalf("NextPollDelay(3s, 0, _) = %s; want 3s", got)
	}
}

// TestNextPollDelay_DoublesPerError: each consecutive error doubles the
// wait until the cap is hit. Jitter=0 keeps the assertion exact.
func TestNextPollDelay_DoublesPerError(t *testing.T) {
	cases := []struct {
		streak int
		want   time.Duration
	}{
		{1, 6 * time.Second},
		{2, 12 * time.Second},
		{3, 24 * time.Second},
		{4, 48 * time.Second},
		{5, 96 * time.Second},
	}
	for _, c := range cases {
		got := NextPollDelay(3*time.Second, c.streak, 0)
		if got != c.want {
			t.Fatalf("streak=%d: got %s, want %s", c.streak, got, c.want)
		}
	}
}

// TestNextPollDelay_CapsAtMaxBackoff: a long outage must not send the
// watcher to sleep for hours.
func TestNextPollDelay_CapsAtMaxBackoff(t *testing.T) {
	got := NextPollDelay(3*time.Second, 50, 0)
	if got != MaxBackoff {
		t.Fatalf("streak=50 (jitter=0) = %s; want %s", got, MaxBackoff)
	}
	// even with full jitter, hard ceiling holds
	got = NextPollDelay(3*time.Second, 50, 0.999)
	if got != MaxBackoff {
		t.Fatalf("streak=50 (jitter=0.999) = %s; want %s (hard cap)", got, MaxBackoff)
	}
}

// TestNextPollDelay_AddsJitter: jitter is additive and proportional to base.
func TestNextPollDelay_AddsJitter(t *testing.T) {
	// streak=1 → 2*base = 6s; jitter=0.5 → +1.5s = 7.5s
	got := NextPollDelay(3*time.Second, 1, 0.5)
	want := 6*time.Second + 1500*time.Millisecond
	if got != want {
		t.Fatalf("got %s; want %s", got, want)
	}
}

// TestNextPollDelay_ClampsBadInputs: negative jitter and zero base must
// not panic or return zero/negative durations.
func TestNextPollDelay_ClampsBadInputs(t *testing.T) {
	if got := NextPollDelay(0, 1, -1); got <= 0 {
		t.Fatalf("base=0, jitter=-1: got non-positive %s", got)
	}
	if got := NextPollDelay(time.Second, 1, 1.5); got <= 0 {
		t.Fatalf("jitter>=1 should clamp, got %s", got)
	}
}
