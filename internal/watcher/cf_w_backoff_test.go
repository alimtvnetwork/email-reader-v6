package watcher

import (
	"log"
	"strings"
	"testing"
	"time"
)

// TestFixedRateNextDelay keeps the user's requested cadence literal: 5s
// means poll starts are spaced about 5s apart. A slow/timeout poll therefore
// does not add another full 5s sleep after it finishes.
func TestFixedRateNextDelay(t *testing.T) {
	base := 3 * time.Second
	started := time.Now().Add(-1 * time.Second)
	if got := nextDelay(base, started); got < 1900*time.Millisecond || got > 2100*time.Millisecond {
		t.Fatalf("elapsed 1s: got %s, want about 2s", got)
	}

	started = time.Now().Add(-4 * time.Second)
	if got := nextDelay(base, started); got != 0 {
		t.Fatalf("elapsed beyond base: got %s, want immediate retry", got)
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
