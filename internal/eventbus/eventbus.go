// Package eventbus is a generic, non-blocking pub/sub fan-out used by
// `core.Watch` to stream `WatchEvent`s to UI / CLI consumers without
// the publisher ever stalling on a slow subscriber.
//
// Design mirrors the proven `watcher.Bus` and `core.accountEventBus`
// shapes (sync.RWMutex over a slice of buffered chans, drop-on-full
// publish), but is generic so other features can reuse it later.
//
// Spec: spec/21-app/02-features/05-watch/01-backend.md §4 (event bus).
package eventbus

import "sync"

// Publisher is the slim contract `core.Watch` depends on. Tests inject
// a stub; production wires `*Bus[WatchEvent]`.
type Publisher[T any] interface {
	Publish(ev T)
}

// Bus is a generic, non-blocking fan-out. Zero value is unusable; use
// `New[T](buf)`.
type Bus[T any] struct {
	mu          sync.RWMutex
	subscribers []chan T
	bufSize     int
}

// New returns a Bus whose subscribers each get a buffered channel of
// size `buf` (default 64 if buf <= 0).
func New[T any](buf int) *Bus[T] {
	if buf <= 0 {
		buf = 64
	}
	return &Bus[T]{bufSize: buf}
}

// Subscribe returns a receive channel and an unsubscribe func. Always
// call the returned func to release the slot — leaked subscribers leak
// a buffered channel.
func (b *Bus[T]) Subscribe() (<-chan T, func()) {
	ch := make(chan T, b.bufSize)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch, func() { b.unsubscribe(ch) }
}

// Publish sends `ev` to every subscriber. Slow subscribers drop the
// event (non-blocking send) — the publisher never stalls. Safe to call
// on a nil Bus (silent no-op) so optional buses don't need nil-checks
// at call sites.
func (b *Bus[T]) Publish(ev T) {
	if b == nil {
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

// SubscriberCount is exported for tests / diagnostics.
func (b *Bus[T]) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

func (b *Bus[T]) unsubscribe(ch chan T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subscribers {
		if s == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}
