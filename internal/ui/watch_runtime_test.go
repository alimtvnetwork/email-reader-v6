// watch_runtime_test.go covers the framework-agnostic helpers in
// watch_runtime.go: the PollSeconds fallback chain and Close
// idempotency. We deliberately do not exercise WatchRuntimeOrNil end
// to end — it touches the real config / store on disk, which belongs
// in an integration test under a different harness.
package ui

import (
	"errors"
	"testing"

	"github.com/lovable/email-read/internal/config"
)

// TestWatchRuntime_PollSeconds_PrefersLiveAccessor: when the
// Settings-backed accessor is wired (pollSecondsF != nil) it must win
// over the boot-time config snapshot — that is the whole point of
// CF-W1 live updates.
func TestWatchRuntime_PollSeconds_PrefersLiveAccessor(t *testing.T) {
	t.Parallel()
	rt := &WatchRuntime{
		cfg:          &config.Config{Watch: config.Watch{PollSeconds: 5}},
		pollSecondsF: func() int { return 17 },
	}
	if got := rt.PollSeconds(); got != 17 {
		t.Fatalf("PollSeconds: got %d, want 17 (live accessor)", got)
	}
}

// TestWatchRuntime_PollSeconds_FallsBackToConfig: with no live
// accessor, the boot-time config value is used. Guards against a
// regression where pollSecondsF goes nil and the call panics.
func TestWatchRuntime_PollSeconds_FallsBackToConfig(t *testing.T) {
	t.Parallel()
	rt := &WatchRuntime{cfg: &config.Config{Watch: config.Watch{PollSeconds: 7}}}
	if got := rt.PollSeconds(); got != 7 {
		t.Fatalf("PollSeconds: got %d, want 7 (config fallback)", got)
	}
}

// TestWatchRuntime_PollSeconds_DefaultsWhenEverythingNil: empty
// runtime must still return a sensible cadence so the Watch view does
// not pass 0 (which would loop the watcher tightly).
func TestWatchRuntime_PollSeconds_DefaultsWhenEverythingNil(t *testing.T) {
	t.Parallel()
	rt := &WatchRuntime{}
	if got := rt.PollSeconds(); got != 3 {
		t.Fatalf("PollSeconds: got %d, want 3 (default)", got)
	}
}

// TestWatchRuntime_Close_RunsClosersInReverse: the closers stack must
// drain LIFO (matches defer semantics and matches the spec contract
// "store closes last after settings unsubscribe").
func TestWatchRuntime_Close_RunsClosersInReverse(t *testing.T) {
	t.Parallel()
	var order []string
	rt := &WatchRuntime{closers: []func() error{
		func() error { order = append(order, "first"); return nil },
		func() error { order = append(order, "second"); return nil },
		func() error { order = append(order, "third"); return nil },
	}}
	rt.Close()
	want := []string{"third", "second", "first"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("close order: got %v, want %v", order, want)
	}
}

// TestWatchRuntime_Close_IsIdempotent: calling Close twice must not
// re-run closers. Important because Run()'s defer and a panic
// recovery could both invoke it.
func TestWatchRuntime_Close_IsIdempotent(t *testing.T) {
	t.Parallel()
	calls := 0
	rt := &WatchRuntime{closers: []func() error{
		func() error { calls++; return nil },
	}}
	rt.Close()
	rt.Close()
	if calls != 1 {
		t.Fatalf("closer ran %d times across two Close() calls; want 1", calls)
	}
}

// TestWatchRuntime_Close_LogsButContinuesOnError: a closer that
// returns an error must not stop earlier closers from running. We
// don't capture log output here (would couple the test to log
// formatting); we only verify that the second closer still ran.
func TestWatchRuntime_Close_LogsButContinuesOnError(t *testing.T) {
	t.Parallel()
	ran := false
	rt := &WatchRuntime{closers: []func() error{
		func() error { ran = true; return nil },                  // older — runs LAST in LIFO
		func() error { return errors.New("simulated close fail") }, // newer — runs FIRST
	}}
	rt.Close()
	if !ran {
		t.Fatalf("earlier closer did not run after a later one returned an error")
	}
}
