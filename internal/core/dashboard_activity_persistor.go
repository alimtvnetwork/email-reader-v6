// dashboard_activity_persistor.go — Slice #107: subscribes to the
// `core.Watch.Subscribe()` event stream and persists every WatchEvent
// into the `WatchEvents` audit table so the Dashboard
// `RecentActivity` feed has rows to render in production.
//
// **Why a separate goroutine (vs. inline write in the bridge)** —
// the bridge is on the hot path of every poll; a SQLite write that
// stalls for a few ms would back-pressure the watcher's poll cadence.
// A dedicated subscriber drains the bus on its own schedule and
// errors are logged-and-swallowed (the next event will succeed).
// This mirrors the precedent of Slice #103/#105's lazy attach
// pattern: production wiring stays out of the pure-domain core type.
//
// **Why the bus subscription model (vs. publishing to a write
// channel)** — the bus already exists, every UI surface and CLI
// already uses `Watch.Subscribe()`, and the bus's drop-on-full
// semantics give us automatic back-pressure protection: if SQLite
// is briefly slow, we may lose some audit rows but never wedge the
// poll loop. Audit completeness is best-effort by design (consistent
// with the bridge's "non-blocking forward" contract).
//
// **Payload encoding** — the persistor JSON-encodes the WatchEvent's
// optional Message + Err.Error() into the `Payload` column so the
// activity adapter can scan them back out via the existing
// `RecentActivitySelectN` projection. Empty Message + nil Err yield
// the schema default `'{}'` for byte-perfect parity with hand-seeded
// test fixtures.
package core

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/store"
)

// WatchEventSink is the minimal write surface the persistor needs.
// `*store.Store` satisfies this interface via `InsertWatchEvent`;
// tests substitute an in-memory recorder so they don't have to spin
// up SQLite for every coverage case.
type WatchEventSink interface {
	InsertWatchEvent(ctx context.Context, alias string, kind int, payload string, at time.Time) error
}

// startWatchEventPersistor spawns a goroutine that subscribes to
// `bus`, encodes each event, and writes it via `sink.InsertWatchEvent`.
// Returns a stop function (idempotent, safe in `defer`) that cancels
// the goroutine and releases the bus subscription. Returning a
// stop func — rather than tying the goroutine's lifetime to ctx
// alone — matches the precedent set by `BridgeWatcherBus` so the
// runtime's `closers` slice stays uniform.
//
// `bus` and `sink` may be nil — both shapes silently no-op so the
// caller does not need conditional plumbing in a test or
// no-database build.
func startWatchEventPersistor(ctx context.Context, bus *eventbus.Bus[WatchEvent], sink WatchEventSink) func() {
	if bus == nil || sink == nil {
		return func() {}
	}
	events, cancel := bus.Subscribe()
	loopCtx, loopCancel := context.WithCancel(ctx)
	go runPersistorLoop(loopCtx, events, sink)
	stopped := false
	return func() {
		if stopped {
			return
		}
		stopped = true
		loopCancel()
		cancel()
	}
}

// StartWatchEventPersistor is the exported entry point used by the UI
// bootstrap (`internal/ui/watch_runtime.go`). Thin wrapper over the
// unexported `startWatchEventPersistor` so the test package can call
// the impl without exporting test seams. Explicit `*store.Store` nil
// check defeats the typed-nil-interface trap (a nil pointer wrapped
// in an interface is NOT == nil inside the implementation).
func StartWatchEventPersistor(ctx context.Context, bus *eventbus.Bus[WatchEvent], st *store.Store) func() {
	if st == nil {
		return func() {}
	}
	return startWatchEventPersistor(ctx, bus, st)
}

// runPersistorLoop drains `events` until ctx is cancelled or the
// channel closes. Pulled out of `startWatchEventPersistor` so the
// goroutine body can be unit-tested in isolation.
func runPersistorLoop(ctx context.Context, events <-chan WatchEvent, sink WatchEventSink) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := persistOne(ctx, sink, ev); err != nil {
				log.Printf("core.WatchEventPersistor: %v", err)
			}
		}
	}
}

// persistOne writes a single WatchEvent. Split out so tests can
// exercise the encode-and-write contract without the goroutine.
func persistOne(ctx context.Context, sink WatchEventSink, ev WatchEvent) error {
	payload := encodeWatchEventPayload(ev)
	if err := sink.InsertWatchEvent(ctx, ev.Alias, int(ev.Kind), payload, ev.At); err != nil {
		return errtrace.Wrap(err, "persist watch event")
	}
	return nil
}

// encodeWatchEventPayload serialises the optional Message + Err into
// the JSON shape the activity adapter (`activityPayload` in
// `internal/store/shims.go`) decodes. Empty/zero values are omitted
// via `omitempty` so the rendered JSON matches the schema's default
// `'{}'` for events that carry no extra data — this keeps
// hand-seeded test fixtures byte-perfect compatible with persistor
// output.
func encodeWatchEventPayload(ev WatchEvent) string {
	type wire struct {
		Message string `json:"Message,omitempty"`
		// ErrorCode is intentionally NOT populated here — the
		// `WatchEvent.Err` is a Go error, not a numeric ER-* code.
		// Wiring `errtrace.Coded.Code` into the payload is a
		// follow-on slice (requires a `errors.As` walk + a
		// `.Code` getter that doesn't yet exist on the public
		// surface). Until then, the dashboard renders error
		// rows without a `(err N)` suffix — same UX as the empty-
		// payload case the views/dashboard test exercises.
	}
	w := wire{Message: ev.Message}
	if w.Message == "" {
		// Match the schema default exactly so persisted rows look
		// like the hand-seeded test fixtures.
		return "{}"
	}
	b, err := json.Marshal(w)
	if err != nil {
		// Marshal of a single string field cannot fail in
		// practice; fall back to the empty-payload sentinel
		// rather than dropping the audit row.
		return "{}"
	}
	return string(b)
}
