package watcher

import (
	"log"
	"strings"
	"testing"
	"time"
)

// TestCF_W_BACKOFF_StreakDrivesNextDelay validates the integration path:
// a fresh pollState with no streak yields the base cadence; bumping the
// consecutive-error counter walks the doubling pattern through nextDelay,
// which is what runLoop actually calls between ticks. This pins the
// observable contract: streak resets on success, grows on errors, caps at
// MaxBackoff. The actual loop wiring is exercised by poll_reload_test +
// regress_test under -race.
func TestCF_W_BACKOFF_StreakDrivesNextDelay(t *testing.T) {
	base := 3 * time.Second
	var sink strings.Builder
	logger := log.New(&sink, "", 0)

	st := &pollState{}

	// streak=0 → base cadence, no log line
	if got := nextDelay(base, st, logger, "alias"); got != base {
		t.Fatalf("streak=0: got %s, want %s", got, base)
	}
	if sink.Len() != 0 {
		t.Fatalf("streak=0 must not log backoff; got %q", sink.String())
	}

	// streak=3 → between 24s (no jitter) and 27s (full jitter)
	st.consecutiveErrors = 3
	got := nextDelay(base, st, logger, "alias")
	if got < 24*time.Second || got > 27*time.Second {
		t.Fatalf("streak=3: got %s, want in [24s,27s]", got)
	}
	if !strings.Contains(sink.String(), "backing off") {
		t.Fatalf("streak=3 must log backoff line; got %q", sink.String())
	}

	// streak=999 → hard cap (5min)
	st.consecutiveErrors = 999
	got = nextDelay(base, st, logger, "alias")
	if got != MaxBackoff {
		t.Fatalf("streak=999: got %s, want %s (cap)", got, MaxBackoff)
	}

	// success path: handlePollOK clears the streak so next call returns base.
	st.consecutiveErrors = 0
	if got := nextDelay(base, st, logger, "alias"); got != base {
		t.Fatalf("after reset: got %s, want %s", got, base)
	}
}

// TestCF_W_BACKOFF_LogPollErrorIncrementsStreak: every EventPollError must
// bump the counter so the next nextDelay call sees the higher streak.
// Splits out from the integration test above because it only needs the
// error-handler — no Bus, no logger format.
func TestCF_W_BACKOFF_LogPollErrorIncrementsStreak(t *testing.T) {
	st := &pollState{}
	logger := log.New(&strings.Builder{}, "", 0)
	opts := Options{Verbose: true}

	for i := 1; i <= 4; i++ {
		logPollError(logger, opts, st, errStub{})
		if st.consecutiveErrors != i {
			t.Fatalf("after %d errors: streak=%d, want %d", i, st.consecutiveErrors, i)
		}
	}
}

type errStub struct{}

func (errStub) Error() string { return "stub error" }
