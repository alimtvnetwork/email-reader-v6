// poll_chans_test.go covers the CF-W1 fan-out registry. Concurrency
// invariants are the load-bearing piece, so we lean on `t.Parallel`,
// real goroutines, and timeouts rather than mocks.
package core

import (
	"sync"
	"testing"
	"time"
)

// TestPollChanRegistry_AcquireReturnsReadOnlyChan: the public type is
// `<-chan int` so callers cannot close it. We can only assert the
// type at compile time — this test exists to lock the API shape so a
// future signature change is a deliberate, reviewed call.
func TestPollChanRegistry_AcquireReturnsReadOnlyChan(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	var ch <-chan int = r.Acquire("a") // compile-time assertion
	if ch == nil {
		t.Fatalf("Acquire returned nil channel")
	}
}

// TestPollChanRegistry_AcquireSameAliasReusesChannel: a second
// Acquire for the same alias must return the same channel so a
// pending Broadcast still reaches the runner that just restarted.
func TestPollChanRegistry_AcquireSameAliasReusesChannel(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	a1 := r.Acquire("primary")
	a2 := r.Acquire("primary")
	if a1 != a2 {
		t.Fatalf("Acquire returned different channels for same alias")
	}
}

// TestPollChanRegistry_DifferentAliasesAreIsolated: two aliases get
// independent channels — a Broadcast that fills one's buffer must
// not affect the other's delivery.
func TestPollChanRegistry_DifferentAliasesAreIsolated(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	a := r.Acquire("a")
	b := r.Acquire("b")
	r.Broadcast(7)
	mustReceive(t, a, 7, "alias a")
	mustReceive(t, b, 7, "alias b")
}

// TestPollChanRegistry_BroadcastDeliveryCount: the return value is
// the number of channels that received the fresh value. Useful for
// the Settings bridge to log "fanned to N runners".
func TestPollChanRegistry_BroadcastDeliveryCount(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	_ = r.Acquire("a")
	_ = r.Acquire("b")
	_ = r.Acquire("c")
	if got := r.Broadcast(5); got != 3 {
		t.Fatalf("Broadcast count: got %d, want 3", got)
	}
}

// TestPollChanRegistry_BroadcastNoSubscribersIsNoop: zero runners
// means zero deliveries — the publisher must not panic or block.
func TestPollChanRegistry_BroadcastNoSubscribersIsNoop(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	if got := r.Broadcast(99); got != 0 {
		t.Fatalf("got %d deliveries on empty registry, want 0", got)
	}
}

// TestPollChanRegistry_BroadcastDropsStaleWhenFull: cap=1 buffer +
// drop-on-full means the LATEST value always wins. We push two
// values without draining; the second must overwrite the first.
func TestPollChanRegistry_BroadcastDropsStaleWhenFull(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	ch := r.Acquire("a")
	r.Broadcast(10)
	r.Broadcast(20)
	mustReceive(t, ch, 20, "after two Broadcasts, buffer must hold the latest")
}

// TestPollChanRegistry_BroadcastNeverBlocks: even with N alive
// channels and none drained, Broadcast must complete inside a tight
// timeout. Asserts the non-blocking contract that protects the
// Settings publisher from a stuck watcher.
func TestPollChanRegistry_BroadcastNeverBlocks(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	for i := 0; i < 100; i++ {
		_ = r.Acquire(string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}
	done := make(chan struct{})
	go func() { r.Broadcast(5); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Broadcast blocked over 1s with 100 undrained channels")
	}
}

// TestPollChanRegistry_ReleaseRemovesSlot: after Release, Broadcast
// must no longer count that alias. Locks the cleanup contract that
// prevents goroutine + memory leaks across long sessions.
func TestPollChanRegistry_ReleaseRemovesSlot(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	_ = r.Acquire("a")
	_ = r.Acquire("b")
	r.Release("a")
	if r.Len() != 1 {
		t.Fatalf("Len after release: got %d, want 1", r.Len())
	}
	if got := r.Broadcast(1); got != 1 {
		t.Fatalf("Broadcast count after release: got %d, want 1", got)
	}
}

// TestPollChanRegistry_ReleaseUnknownIsNoop: defensive — defer-based
// cleanup may call Release even when Acquire was skipped (e.g. an
// early-return branch). Must not panic.
func TestPollChanRegistry_ReleaseUnknownIsNoop(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	r.Release("ghost") // would panic on a naive map[k]chan
	if r.Len() != 0 {
		t.Fatalf("Release on unknown alias mutated registry")
	}
}

// TestPollChanRegistry_ConcurrentAcquireReleaseBroadcast: the
// load-bearing test. Spin Settings-side Broadcasts and runner-side
// Acquire/Release in parallel; the registry must not race-detect or
// panic.
func TestPollChanRegistry_ConcurrentAcquireReleaseBroadcast(t *testing.T) {
	t.Parallel()
	r := NewPollChanRegistry()
	const aliases = 8
	const iters = 200
	var wg sync.WaitGroup

	// Runner-side: rapid Acquire / drain / Release.
	for i := 0; i < aliases; i++ {
		wg.Add(1)
		alias := string(rune('a' + i))
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				ch := r.Acquire(alias)
				select {
				case <-ch:
				default:
				}
				r.Release(alias)
			}
		}()
	}
	// Settings-side: continuous Broadcast.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iters*aliases; j++ {
			r.Broadcast(j)
		}
	}()
	wg.Wait()
}

func mustReceive(t *testing.T, ch <-chan int, want int, msg string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("%s: got %d, want %d", msg, got, want)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("%s: timed out waiting for %d", msg, want)
	}
}
