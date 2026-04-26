// watch_bridge.go relays events from the low-level watcher.Bus
// (per-poll signals: poll_ok, poll_error, baseline, heartbeat, …) onto
// the high-level core.WatchEvent stream that core.Watch.Subscribe()
// returns. With the bridge wired, a UI/CLI consumer that subscribes to
// `*core.Watch` ALONE sees the full picture — no need to also know
// about `internal/watcher` or its bus shape.
//
// Why a separate file (and why now): the bridge is the missing rung in
// the "Watch service shell" + "real LoopFactory" pair. Lifecycle events
// (Start/Stop/Error from core.Watch) and runtime signals (Heartbeat /
// Error from watcher.Bus) used to live on two streams; views had to
// merge them by hand. The bridge converges them so the contract
// documented in spec/21-app/02-features/05-watch/01-backend.md §4 ("a
// single Subscribe() returns the unified stream") holds at runtime.
//
// Concurrency: BridgeWatcherBus spawns ONE goroutine that drains the
// watcher.Bus subscription. It exits on ctx.Done() — no `Close` needed
// by the caller. The publisher writes are non-blocking (Bus.Publish
// drops on full subscriber buffers) so a slow UI cannot wedge the
// poll loop.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/watcher"
)

// BridgeWatcherBus subscribes to `src` and republishes onto `dst`,
// translating watcher.Event into WatchEvent. Returns a stop func that
// is safe to call from `defer` (idempotent).
//
// Both bus pointers may be nil — if either is nil the bridge is a
// silent no-op and stop is a no-op too. This mirrors the Bus.Publish
// nil-tolerance contract so callers don't need conditional plumbing.
func BridgeWatcherBus(ctx context.Context, src *watcher.Bus, dst *eventbus.Bus[WatchEvent]) func() {
	if src == nil || dst == nil {
		return func() {}
	}
	events, cancel := src.Subscribe()
	bridgeCtx, bridgeCancel := context.WithCancel(ctx)
	go runBridgeLoop(bridgeCtx, events, dst)
	stopped := false
	return func() {
		if stopped {
			return
		}
		stopped = true
		bridgeCancel()
		cancel()
	}
}

// runBridgeLoop drains `events` until ctx is cancelled or the channel
// closes (the latter happens when the unsubscribe func is called). Each
// translated event is forwarded via TranslateWatcherEvent — a pure
// function so the table is unit-testable without any goroutine plumbing.
func runBridgeLoop(ctx context.Context, events <-chan watcher.Event, dst *eventbus.Bus[WatchEvent]) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			out, forward := TranslateWatcherEvent(ev)
			if forward {
				dst.Publish(out)
			}
		}
	}
}

// TranslateWatcherEvent maps one low-level watcher.Event into a
// high-level WatchEvent. Returns (zero, false) for kinds the WatchEvent
// stream does not cover (e.g. EventStarted/EventStopped — those are
// owned by core.Watch.Start/Stop and would double-publish if mirrored).
//
// Mapping table (locked by tests):
//
//	watcher.EventPollOK      → WatchHeartbeat (silent OK pulse)
//	watcher.EventBaseline    → WatchHeartbeat (initial cursor set)
//	watcher.EventHeartbeat   → WatchHeartbeat (60-poll alive ping)
//	watcher.EventNewMail     → WatchHeartbeat (visible activity)
//	watcher.EventRuleMatch   → WatchHeartbeat (visible activity)
//	watcher.EventUrlOpened   → WatchHeartbeat (visible activity, OK or fail)
//	watcher.EventUidValReset → WatchHeartbeat (server-side mailbox reset note)
//	watcher.EventPollError   → WatchError    (carries Err)
//	watcher.EventStarted     → drop (Watch.Start already publishes WatchStart)
//	watcher.EventStopped     → drop (Watch.Stop already publishes WatchStop)
//
// Future revision (non-blocking): once the Watch view exposes a typed
// activity feed, EventNewMail / EventRuleMatch / EventUrlOpened can be
// promoted to dedicated WatchEventKinds — the table here is the only
// thing that needs to change.
func TranslateWatcherEvent(ev watcher.Event) (WatchEvent, bool) {
	switch ev.Kind {
	case watcher.EventStarted, watcher.EventStopped:
		return WatchEvent{}, false
	case watcher.EventPollError:
		return WatchEvent{
			Kind:    WatchError,
			Alias:   ev.Alias,
			At:      ev.At,
			Err:     ev.Err,
			Message: watcherEventMessage(ev),
		}, true
	default:
		return WatchEvent{
			Kind:    WatchHeartbeat,
			Alias:   ev.Alias,
			At:      ev.At,
			Message: watcherEventMessage(ev),
		}, true
	}
}

// watcherEventMessage renders a short, human-readable summary of an
// event for the Message field of WatchEvent. Kept tiny on purpose —
// callers that want full structured detail should subscribe to the
// underlying watcher.Bus directly (the Watch view does this for its
// raw-log tab).
func watcherEventMessage(ev watcher.Event) string {
	switch ev.Kind {
	case watcher.EventPollOK:
		return "poll ok"
	case watcher.EventBaseline:
		return "baseline set"
	case watcher.EventHeartbeat:
		return "alive"
	case watcher.EventNewMail:
		return "new mail"
	case watcher.EventRuleMatch:
		return "rule match · " + ev.RuleName
	case watcher.EventUrlOpened:
		if ev.OpenOK {
			return "url opened"
		}
		return "url open failed"
	case watcher.EventUidValReset:
		return "uidvalidity reset"
	case watcher.EventPollError:
		return "poll error"
	}
	return string(ev.Kind)
}
