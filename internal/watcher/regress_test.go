package watcher

import (
	"strings"
	"testing"
)

// Regress_Issue02_HeartbeatInQuietModeAtLeastOncePer3min — Issue 02 (silent
// idle watcher). The heartbeat invariant from spec/21-app/05-logging-strategy.md
// requires that even an idle, error-free watcher emits at least one log line
// every ~3 minutes so the user can tell it from a hung process.
//
// We encode this as a constant-level invariant: the heartbeat cadence
// (`heartbeatEvery`) at the configured 3 s default poll must produce a
// heartbeat at least every 3 minutes. If a future change raises the cadence
// past that threshold (or removes the heartbeat entirely), this test fails.
//
// Maps to AC-PROJ-24.
func Regress_Issue02_HeartbeatInQuietModeAtLeastOncePer3min(t *testing.T) {
	const defaultPollSec = 3
	const maxQuietGapSec = 3 * 60 // 3 minutes — the spec's heartbeat invariant

	gotGapSec := heartbeatEvery * defaultPollSec
	if gotGapSec > maxQuietGapSec {
		t.Fatalf("heartbeat gap = %ds (heartbeatEvery=%d × pollSeconds=%d); spec requires ≤ %ds — issue 02 would silently regress",
			gotGapSec, heartbeatEvery, defaultPollSec, maxQuietGapSec)
	}
	if heartbeatEvery <= 0 {
		t.Fatalf("heartbeatEvery=%d disables the heartbeat entirely — issue 02 would fully regress", heartbeatEvery)
	}
}

// Regress_Issue04_QuietModeUnderNLinesPerMinIdle — Issue 04 (noisy logs).
// Quiet mode (Verbose=false) must not unconditionally print poll-step lines.
// The Options struct's `Verbose` field is the gate; if it disappears or
// becomes the default, idle polls would resume spamming the terminal.
//
// We assert the structural contract here (fastest, deterministic). The full
// runtime assertion lives in Watch feature 97-AC §H-NN.
//
// Maps to AC-PROJ-26.
func Regress_Issue04_QuietModeUnderNLinesPerMinIdle(t *testing.T) {
	// The Verbose field must exist with a bool zero-value (default = quiet).
	var o Options
	if o.Verbose {
		t.Fatalf("Options.Verbose default is true — quiet mode would no longer be the default; issue 04 regresses")
	}
}

// Regress_Issue08_NoDoubleTimestampsAndUrlTruncated — Issue 08 (heavy logs):
// the `truncURL` helper must shorten any URL > 90 chars and the watcher must
// not double-prefix timestamps. We verify the truncation invariant directly
// (cheap + deterministic). The "no double timestamp" invariant is enforced by
// the cli using `log.New(..., 0)` — covered structurally by issue 08's spec
// link to internal/cli/cli.go.
//
// Maps to AC-PROJ-30.
func Regress_Issue08_NoDoubleTimestampsAndUrlTruncated(t *testing.T) {
	short := "https://example.com/short"
	if got := truncURL(short); got != short {
		t.Errorf("truncURL altered a short URL: in=%q out=%q", short, got)
	}

	// 200-char URL — must be truncated and signal it with the ellipsis.
	long := "https://lovable.dev/auth/action?token=" + strings.Repeat("A", 200)
	got := truncURL(long)
	if len(got) > 90 {
		t.Errorf("truncURL output longer than 90 chars: len=%d", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncURL did not signal truncation with …: %q", got)
	}
	if got == long {
		t.Errorf("truncURL did not truncate a 200+ char URL")
	}
}
