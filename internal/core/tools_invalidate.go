// tools_invalidate.go wires `core.Tools` into the package-level
// AccountEvent bus so the diagnose cache stays consistent with the
// underlying account list. On every AccountUpdated / AccountRemoved
// event we evict the per-alias entry; AccountAdded is a no-op (no
// stale entry can exist for a brand-new alias).
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.3 — "the
// diagnose cache MUST be invalidated when the underlying account
// changes so a credentials-fixed Diagnose run is never short-circuited
// by a 60 s-old failure trail".
//
// Lifecycle: callers invoke `Tools.WatchAccountEvents(ctx)` once at
// startup (typically from `internal/ui/app.go::Run` after `NewTools`).
// The goroutine exits when ctx is cancelled OR the bus closes the
// channel via the unsubscribe func.
package core

import "context"

// WatchAccountEvents subscribes the receiver's diagnose cache to the
// package-level account-event bus and starts a goroutine that evicts
// per-alias entries on Updated / Removed events.
//
// Returns a stop func the caller MAY invoke to unsubscribe early; the
// goroutine also exits automatically when ctx is cancelled. Calling
// stop more than once is a no-op (sync.Once-guarded inside).
//
// Safe to call multiple times — each call adds an independent
// subscription. Production callers typically call it exactly once at
// app bootstrap.
func (t *Tools) WatchAccountEvents(ctx context.Context) (stop func()) {
	ch, cancel := SubscribeAccountEvents()
	go t.runAccountInvalidator(ctx, ch, cancel)
	return cancel
}

// runAccountInvalidator is the loop body — extracted so it stays
// under the 15-statement fn-length linter and is unit-testable via
// the test helper below.
func (t *Tools) runAccountInvalidator(ctx context.Context, ch <-chan AccountEvent, cancel func()) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			t.handleAccountEvent(ev)
		}
	}
}

// handleAccountEvent applies the per-kind eviction policy. AccountAdded
// is a no-op (no cached entry can predate creation); Updated + Removed
// both evict the per-alias entry so the next Diagnose call re-runs the
// IMAP probe live. Empty-alias evictions are explicitly tolerated (the
// cache uses "" as a valid key for "first configured account" runs).
func (t *Tools) handleAccountEvent(ev AccountEvent) {
	switch ev.Kind {
	case AccountUpdated, AccountRemoved:
		t.diagCache().invalidate(ev.Alias)
	default:
		// AccountAdded → nothing to evict.
	}
}
