// poll_chans.go implements the PollChanProvider that bridges
// `Settings.Subscribe` cadence updates into every running watcher's
// `PollSecondsCh` (CF-W1). One provider serves the whole process; each
// runner Acquires a slot at Start and Releases it on exit.
//
// Concurrency contract:
//   - Acquire / Release / Broadcast are all goroutine-safe.
//   - Per-alias channels are buffered (cap 1) so Broadcast NEVER
//     blocks: if the watcher hasn't drained the previous value yet,
//     we drop the older value and replace it with the latest. This
//     matches the watcher's "applies on the NEXT tick" contract — an
//     old pending value is stale and must not race ahead of a newer
//     one.
//   - Release is idempotent: a second Release for the same alias is a
//     no-op (handy for defer chains where Release runs even if
//     Acquire was skipped).
//
// Why buffered cap=1 (not unbuffered):
//   An unbuffered chan would force Broadcast to block on each
//   subscriber, which would couple Settings save latency to whatever
//   the slowest watcher's tick interval is. cap=1 + drop-on-full keeps
//   Settings publishes O(N runners) and bounded.
package core

import (
	"sync"
)

// PollChanRegistry is the production PollChanProvider. Zero value is
// usable but the recommended constructor is NewPollChanRegistry which
// pre-allocates the map.
type PollChanRegistry struct {
	mu    sync.RWMutex
	chans map[string]chan int
}

// NewPollChanRegistry returns an empty registry ready for Acquire.
func NewPollChanRegistry() *PollChanRegistry {
	return &PollChanRegistry{chans: make(map[string]chan int)}
}

// Acquire returns the per-alias channel for `alias`, allocating it on
// first call. Subsequent calls for the same alias return the SAME
// channel — important when Stop+Start happens fast enough that the
// new runner overlaps the old one's Release defer (we accept this
// rare re-use because both are "the same alias" and Settings updates
// should reach whichever loop is currently consuming).
//
// The returned chan has type `<-chan int` so callers cannot
// accidentally close it; only the registry owns the close.
func (r *PollChanRegistry) Acquire(alias string) <-chan int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.chans == nil {
		r.chans = make(map[string]chan int)
	}
	ch, ok := r.chans[alias]
	if !ok {
		ch = make(chan int, 1)
		r.chans[alias] = ch
	}
	return ch
}

// Release frees the slot for `alias`. The channel is NOT closed —
// closing would race with an in-flight Broadcast send. Instead we
// just remove the registry entry so future Broadcasts skip it; the
// watcher loop has already exited (Release is deferred AFTER
// watcher.Run returns) so any pending value in the buffer is
// orphaned and GC'd with the channel.
func (r *PollChanRegistry) Release(alias string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.chans, alias)
}

// Broadcast pushes `secs` to every live runner's channel,
// non-blocking (drop on full). Settings publishers MUST call this on
// every cadence change. Returns the number of channels that received
// the value (vs dropped) — useful for tests + diagnostics.
func (r *PollChanRegistry) Broadcast(secs int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	delivered := 0
	for _, ch := range r.chans {
		if trySend(ch, secs) {
			delivered++
		}
	}
	return delivered
}

// trySend attempts a non-blocking send. When the buffer is already
// full we drain one stale value first then send the fresh one — that
// way the LATEST cadence wins even under burst publish (e.g. a user
// dragging a slider). Returns true if the fresh value sits in the
// buffer when we return; false only when the channel is somehow nil
// (defensive — shouldn't happen via Acquire).
func trySend(ch chan int, secs int) bool {
	if ch == nil {
		return false
	}
	select {
	case ch <- secs:
		return true
	default:
	}
	// Buffer full: drain the stale value and retry.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- secs:
		return true
	default:
		return false
	}
}

// Len returns the number of live runners. Test-only convenience —
// production code should not need to introspect the registry size.
func (r *PollChanRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.chans)
}
