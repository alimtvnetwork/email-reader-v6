// backoff.go — exponential backoff with jitter for consecutive poll errors.
//
// Spec: spec/21-app/02-features/05-watch/01-backend.md (CF-W-BACKOFF).
// On each consecutive EventPollError, the watcher waits a longer interval
// before the next poll attempt, capped at 5 minutes, with full jitter to
// avoid thundering-herd on a flaky server. A successful poll (or any
// non-error event) resets the streak to zero.
//
// The function is pure: it takes the base cadence + consecutive-error count
// and a jitter source (math/rand-compatible func) and returns the next
// sleep. This keeps it trivially testable without a fake clock.
package watcher

import (
	"time"
)

// MaxBackoff caps the exponential growth so a long outage still polls at
// least once every 5 minutes (when service returns the watcher recovers
// without operator intervention).
const MaxBackoff = 5 * time.Minute

// NextPollDelay computes the wait before the next poll attempt.
//
//   - base:        the configured PollSeconds cadence (e.g. 3s).
//   - consecutiveErrors: how many EventPollError fired in a row before this
//     call. 0 means "the last poll succeeded" → return base unchanged.
//   - jitterFrac: a number in [0.0, 1.0); multiplied by the chosen backoff
//     to produce additive jitter. Tests pass a deterministic value;
//     production passes rand.Float64().
//
// Doubling pattern: errors=1 → 2*base, errors=2 → 4*base, errors=3 → 8*base, …
// All capped at MaxBackoff. The jitter is added on top, also capped at
// MaxBackoff so the upper bound is a hard ceiling.
func NextPollDelay(base time.Duration, consecutiveErrors int, jitterFrac float64) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if consecutiveErrors <= 0 {
		return base
	}
	// Cap the exponent before shifting to avoid Duration overflow on long outages.
	exp := consecutiveErrors
	if exp > 20 {
		exp = 20
	}
	d := base << uint(exp) // base * 2^exp
	if d <= 0 || d > MaxBackoff {
		d = MaxBackoff
	}
	if jitterFrac < 0 {
		jitterFrac = 0
	}
	if jitterFrac >= 1 {
		jitterFrac = 0.999
	}
	jitter := time.Duration(float64(base) * jitterFrac)
	out := d + jitter
	if out > MaxBackoff {
		out = MaxBackoff
	}
	return out
}
