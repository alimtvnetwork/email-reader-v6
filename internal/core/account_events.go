// account_events.go is the small pub/sub bus for account lifecycle
// events. It exists so consumers like `core.Tools` can invalidate their
// per-alias caches when an account is updated or removed without each
// consumer having to poll the config layer.
//
// Design choice: the existing `AddAccount` / `RemoveAccount` API is
// function-style (not method-on-service), so we keep a package-level
// bus and publish from those functions. Subscribers are optional —
// when no one has subscribed, Publish is a single nil-check + return.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.3 (note on
// AccountEvent → diagnose-cache invalidation hook).
package core

import (
	"sync"
	"time"
)

// AccountEventKind discriminates the lifecycle transitions consumers
// care about. Added kinds (e.g. AccountTested) extend this enum.
type AccountEventKind uint8

const (
	AccountAdded   AccountEventKind = 1 // new alias appeared
	AccountUpdated AccountEventKind = 2 // existing alias re-saved (creds / host changed)
	AccountRemoved AccountEventKind = 3 // alias deleted from config
)

// String returns the canonical log form.
func (k AccountEventKind) String() string {
	switch k {
	case AccountAdded:
		return "Added"
	case AccountUpdated:
		return "Updated"
	case AccountRemoved:
		return "Removed"
	}
	return "Unknown"
}

// AccountEvent is the published payload. Consumers switch on Kind +
// Alias; the rest of the account fields are intentionally absent so
// password material never lands on a fan-out channel.
type AccountEvent struct {
	Kind  AccountEventKind
	Alias string
	At    time.Time
}

// accountBus is the singleton bus. Lazily initialised on first
// Subscribe / Publish so packages that never use it pay nothing.
var accountBus = &accountEventBus{bufSize: 16}

// accountEventBus is the non-blocking fan-out channel set. Slow
// subscribers drop events for that subscriber rather than stalling
// the publisher (mirrors the watcher.Bus contract).
type accountEventBus struct {
	mu          sync.RWMutex
	subscribers []chan AccountEvent
	bufSize     int
}

// SubscribeAccountEvents returns a buffered receive channel and an
// unsubscribe func. Always call the returned func to release the slot
// — leaked subscribers leak a goroutine-attached buffered channel.
//
// Buffer size is 16 events; consumers MUST drain promptly. Drops are
// logged neither here nor by the publisher: the contract is "best
// effort, never block AddAccount".
func SubscribeAccountEvents() (<-chan AccountEvent, func()) {
	return accountBus.subscribe()
}

func (b *accountEventBus) subscribe() (<-chan AccountEvent, func()) {
	ch := make(chan AccountEvent, b.bufSize)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	cancel := func() { b.unsubscribe(ch) }
	return ch, cancel
}

func (b *accountEventBus) unsubscribe(ch chan AccountEvent) {
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

// publishAccountEvent is the package-private publisher invoked from
// AddAccount / RemoveAccount. Intentionally not exported — only the
// account lifecycle functions in this package may emit events, so
// downstream consumers can trust Kind values.
func publishAccountEvent(kind AccountEventKind, alias string) {
	ev := AccountEvent{Kind: kind, Alias: alias, At: time.Now()}
	accountBus.publish(ev)
}

func (b *accountEventBus) publish(ev AccountEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
			// drop — consumer is too slow; never block the publisher.
		}
	}
}

// resetAccountBusForTest clears every subscriber. Test-only.
func resetAccountBusForTest() {
	accountBus.mu.Lock()
	defer accountBus.mu.Unlock()
	for _, ch := range accountBus.subscribers {
		close(ch)
	}
	accountBus.subscribers = nil
}
