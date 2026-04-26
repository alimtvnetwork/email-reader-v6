// watch_bridge_test.go locks the watcher.Bus → core.WatchEvent
// translation table and the goroutine lifecycle of BridgeWatcherBus.
// All tests stay framework-free: no IMAP server, no real watcher.Run.
package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/watcher"
)

// Test_TranslateWatcherEvent_Mapping pins the kind-by-kind translation
// contract documented in watch_bridge.go. If you add a new
// watcher.EventKind, add a row here.
func Test_TranslateWatcherEvent_Mapping(t *testing.T) {
	t.Parallel()
	type want struct {
		forward bool
		kind    WatchEventKind
	}
	cases := []struct {
		in   watcher.EventKind
		want want
	}{
		{watcher.EventStarted, want{false, 0}},
		{watcher.EventStopped, want{false, 0}},
		{watcher.EventPollError, want{true, WatchError}},
		{watcher.EventPollOK, want{true, WatchHeartbeat}},
		{watcher.EventBaseline, want{true, WatchHeartbeat}},
		{watcher.EventHeartbeat, want{true, WatchHeartbeat}},
		{watcher.EventNewMail, want{true, WatchHeartbeat}},
		{watcher.EventRuleMatch, want{true, WatchHeartbeat}},
		{watcher.EventUrlOpened, want{true, WatchHeartbeat}},
		{watcher.EventUidValReset, want{true, WatchHeartbeat}},
	}
	for _, c := range cases {
		out, fwd := TranslateWatcherEvent(watcher.Event{Kind: c.in, Alias: "a"})
		if fwd != c.want.forward {
			t.Errorf("kind=%s forward=%v want %v", c.in, fwd, c.want.forward)
		}
		if fwd && out.Kind != c.want.kind {
			t.Errorf("kind=%s mapped=%v want %v", c.in, out.Kind, c.want.kind)
		}
		if fwd && out.Alias != "a" {
			t.Errorf("alias dropped during translation for kind=%s", c.in)
		}
	}
}

// Test_TranslateWatcherEvent_PreservesError checks that the Err field
// is carried verbatim onto WatchError so log consumers can format the
// underlying cause.
func Test_TranslateWatcherEvent_PreservesError(t *testing.T) {
	t.Parallel()
	root := errors.New("dial: refused")
	out, fwd := TranslateWatcherEvent(watcher.Event{Kind: watcher.EventPollError, Err: root, Alias: "a"})
	if !fwd || out.Kind != WatchError {
		t.Fatalf("expected WatchError, got %+v fwd=%v", out, fwd)
	}
	if !errors.Is(out.Err, root) {
		t.Errorf("Err lost: got %v want %v", out.Err, root)
	}
}

// Test_TranslateWatcherEvent_PreservesAt confirms that the source
// timestamp survives translation — this matters for event ordering when
// the bridge falls behind.
func Test_TranslateWatcherEvent_PreservesAt(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	out, _ := TranslateWatcherEvent(watcher.Event{Kind: watcher.EventHeartbeat, At: at, Alias: "a"})
	if !out.At.Equal(at) {
		t.Errorf("At drift: got %v want %v", out.At, at)
	}
}

// Test_TranslateWatcherEvent_RuleMatchMessageHasName proves the
// human-readable Message field includes the rule name so a UI surfacing
// only WatchEvent.Message still has enough context.
func Test_TranslateWatcherEvent_RuleMatchMessageHasName(t *testing.T) {
	t.Parallel()
	out, _ := TranslateWatcherEvent(watcher.Event{Kind: watcher.EventRuleMatch, RuleName: "lovable-verify"})
	if out.Message != "rule match · lovable-verify" {
		t.Errorf("Message=%q want %q", out.Message, "rule match · lovable-verify")
	}
}

// Test_BridgeWatcherBus_NilTolerant proves nil src OR nil dst is a
// no-op and never panics — matches the Bus.Publish nil-tolerance
// contract elsewhere in the package.
func Test_BridgeWatcherBus_NilTolerant(t *testing.T) {
	t.Parallel()
	stop := BridgeWatcherBus(context.Background(), nil, nil)
	stop() // must not panic
	stop() // idempotent
	stop2 := BridgeWatcherBus(context.Background(), watcher.NewBus(4), nil)
	stop2()
	stop3 := BridgeWatcherBus(context.Background(), nil, eventbus.New[WatchEvent](4))
	stop3()
}

// Test_BridgeWatcherBus_EndToEnd: publish a poll_ok and a poll_error on
// the source bus, drain WatchEvent from the destination bus, assert the
// translation came through.
func Test_BridgeWatcherBus_EndToEnd(t *testing.T) {
	t.Parallel()
	src := watcher.NewBus(8)
	dst := eventbus.New[WatchEvent](8)
	out, cancel := dst.Subscribe()
	defer cancel()
	stop := BridgeWatcherBus(context.Background(), src, dst)
	defer stop()

	src.Publish(watcher.Event{Kind: watcher.EventPollOK, Alias: "a"})
	src.Publish(watcher.Event{Kind: watcher.EventPollError, Alias: "a", Err: errors.New("x")})

	got := drainWithin(t, out, 2, 500*time.Millisecond)
	if got[0].Kind != WatchHeartbeat || got[1].Kind != WatchError {
		t.Fatalf("unexpected sequence: %+v", got)
	}
	if got[1].Err == nil || got[1].Err.Error() != "x" {
		t.Errorf("error not preserved: %+v", got[1])
	}
}

// Test_BridgeWatcherBus_StopReleasesGoroutine: after stop() the goroutine
// must exit even when more events arrive on src. We verify by counting
// SubscriberCount before/after.
func Test_BridgeWatcherBus_StopReleasesGoroutine(t *testing.T) {
	t.Parallel()
	src := watcher.NewBus(4)
	dst := eventbus.New[WatchEvent](4)
	stop := BridgeWatcherBus(context.Background(), src, dst)
	if got := src.SubscriberCount(); got != 1 {
		t.Fatalf("SubscriberCount after start = %d, want 1", got)
	}
	stop()
	// Allow goroutine + unsubscribe func to complete.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if src.SubscriberCount() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("SubscriberCount never returned to 0; got %d", src.SubscriberCount())
}

// Test_BridgeWatcherBus_DropsLifecycleDuplicates verifies that
// EventStarted and EventStopped do NOT round-trip onto WatchEvent —
// otherwise core.Watch.Start would publish WatchStart and then the
// bridge would publish a SECOND WatchHeartbeat for the same boundary.
func Test_BridgeWatcherBus_DropsLifecycleDuplicates(t *testing.T) {
	t.Parallel()
	src := watcher.NewBus(4)
	dst := eventbus.New[WatchEvent](4)
	out, cancel := dst.Subscribe()
	defer cancel()
	stop := BridgeWatcherBus(context.Background(), src, dst)
	defer stop()

	src.Publish(watcher.Event{Kind: watcher.EventStarted, Alias: "a"})
	src.Publish(watcher.Event{Kind: watcher.EventStopped, Alias: "a"})
	src.Publish(watcher.Event{Kind: watcher.EventPollOK, Alias: "a"})

	got := drainWithin(t, out, 1, 250*time.Millisecond)
	if got[0].Kind != WatchHeartbeat {
		t.Fatalf("first forwarded event kind=%v want WatchHeartbeat", got[0].Kind)
	}
	// Confirm no extra events arrive within a small grace window.
	select {
	case extra := <-out:
		t.Fatalf("unexpected extra event: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

// drainWithin pulls up to `n` events off `ch` or fails the test on
// timeout. Ordered: returned slice preserves arrival order.
func drainWithin(t *testing.T, ch <-chan WatchEvent, n int, timeout time.Duration) []WatchEvent {
	t.Helper()
	out := make([]WatchEvent, 0, n)
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			t.Fatalf("drain timeout: got %d/%d events", len(out), n)
		}
	}
	return out
}

// Test_BridgeWatcherBus_ConcurrentPublishersSafe stresses the bridge
// with two goroutines publishing in parallel — proves the translation +
// republish path is race-free under -race.
func Test_BridgeWatcherBus_ConcurrentPublishersSafe(t *testing.T) {
	t.Parallel()
	src := watcher.NewBus(64)
	dst := eventbus.New[WatchEvent](64)
	out, cancel := dst.Subscribe()
	defer cancel()
	stop := BridgeWatcherBus(context.Background(), src, dst)
	defer stop()

	const perGo = 50
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGo; j++ {
				src.Publish(watcher.Event{Kind: watcher.EventHeartbeat, Alias: "a"})
			}
		}()
	}
	wg.Wait()
	// Drain whatever made it through (drops are fine — non-blocking bus).
	deadline := time.After(250 * time.Millisecond)
	count := 0
loop:
	for {
		select {
		case <-out:
			count++
		case <-deadline:
			break loop
		}
	}
	if count == 0 {
		t.Fatalf("expected at least one forwarded event under concurrent load, got 0")
	}
}
