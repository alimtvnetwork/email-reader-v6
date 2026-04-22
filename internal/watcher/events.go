// events.go — pub/sub for structured watcher events.
//
// The existing logger output stays untouched (CLI keeps its readable lines).
// In parallel, the watcher publishes structured Events so the upcoming Fyne
// UI can render cards and a live event feed without parsing log strings.
package watcher

import (
	"sync"
	"time"

	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/rules"
)

// EventKind classifies what just happened in the watcher loop.
type EventKind string

const (
	EventStarted     EventKind = "started"      // banner printed, loop entering tick
	EventBaseline    EventKind = "baseline"     // first-run baseline UID set; Stats populated
	EventPollOK      EventKind = "poll_ok"      // a poll completed without error; Stats populated
	EventPollError   EventKind = "poll_error"   // a poll failed; Err populated
	EventNewMail     EventKind = "new_mail"     // a new message was persisted; Message populated
	EventRuleMatch   EventKind = "rule_match"   // rule matched & URL queued; RuleName + Url populated
	EventUrlOpened   EventKind = "url_opened"   // browser launch attempted/succeeded; Url + OpenOK
	EventHeartbeat   EventKind = "heartbeat"    // periodic alive ping; Stats populated
	EventStopped     EventKind = "stopped"      // ctx cancelled, loop exited cleanly
	EventUidValReset EventKind = "uidval_reset" // server's UIDVALIDITY changed
)

// Event is one observable signal from the watcher. Only the fields relevant
// to Kind are populated; the rest are zero values. UI consumers switch on
// Kind. The CLI does not currently read this stream — its existing logger
// output is unchanged.
type Event struct {
	Kind     EventKind
	At       time.Time
	Alias    string
	Stats    *mailclient.MailboxStats // poll_ok, baseline, heartbeat, uidval_reset
	Message  *mailclient.Message      // new_mail
	RuleName string                   // rule_match, url_opened
	Url      string                   // rule_match, url_opened
	OpenOK   bool                     // url_opened (false ⇒ launch error)
	Err      error                    // poll_error, url_opened (when !OpenOK)
}

// Bus is a non-blocking fan-out channel set. Subscribers each get their own
// buffered channel; if a subscriber falls behind we drop events for that
// subscriber rather than stalling the watcher loop. Zero value is unusable;
// use NewBus.
type Bus struct {
	mu          sync.RWMutex
	subscribers []chan Event
	bufSize     int
}

// NewBus returns a Bus where each subscriber gets a buffered channel of size
// buf (default 64 if buf <= 0).
func NewBus(buf int) *Bus {
	if buf <= 0 {
		buf = 64
	}
	return &Bus{bufSize: buf}
}

// Subscribe returns a new receive channel and an unsubscribe func. Always
// call the returned func to release resources.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, b.bufSize)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	cancel := func() {
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
	return ch, cancel
}

// Publish sends ev to every subscriber. Slow subscribers drop the event
// (non-blocking send) — the watcher loop must never stall on a UI consumer.
func (b *Bus) Publish(ev Event) {
	if b == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
			// drop — consumer is too slow
		}
	}
}

// SubscriberCount is exported for tests / diagnostics.
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// Compile-time guarantee that we don't accidentally drop a key field.
var _ = rules.MatchResult{}
