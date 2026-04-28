// dashboard_activity_source.go — Slice #104: production
// `activitySource` adapter that bridges `Store.QueryRecentActivity`
// into the function-typed dependency consumed by
// `(*DashboardService).RecentActivity`.
//
// **Why a separate file** — `dashboard_activity.go` is intentionally
// pure-Go (no `database/sql`, no `*store.Store` import) so its tests
// run without spinning up SQLite. Co-locating the store adapter
// would either drag the store import into the pure file (breaking
// the boundary) or require build tags. A separate file keeps the
// pure-vs-impure split self-evident at file granularity, mirroring
// the precedent set by `dashboard_health_source.go`.
//
// **Why a separate type for `StoreActivityRow`** — the store shim
// returns its own struct (intentional; see the doc-comment on
// `store.StoreActivityRow`) so the store package doesn't import
// `core`. This adapter does the trivial field-by-field copy plus
// the integer→string `ActivityKind` mapping.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// ActivitySource is the **exported** alias for the `activitySource`
// function-type dep, introduced in Slice #104 so the UI bootstrap
// (`internal/ui`) can name the production adapter returned by
// `NewStoreActivitySource` and pass it into
// `(*DashboardService).RecentActivity` via a typed `Services` bundle
// field. Mirrors `AccountHealthSource` from Slice #103. Zero
// runtime cost — Go aliases share the underlying type.
type ActivitySource = activitySource

// NewStoreActivitySource wraps `(*store.Store).QueryRecentActivity`
// in the `activitySource` shape consumed by
// `(*DashboardService).RecentActivity`. Returns nil when `s` is nil
// so the caller can decide whether to bail or fall back to a stub
// source — same convention as `NewStoreAccountHealthSource`.
//
// **Kind mapping**: the store returns the raw `core.WatchEventKind`
// integer enum (1..6); this adapter maps it to the spec
// `core.ActivityKind` string enum:
//
//   - 1 (WatchStart)        → ActivityPollStarted
//   - 2 (WatchStop)         → DROPPED (no spec ActivityKind for clean
//     shutdown; Stop is a watcher-internal
//     lifecycle event, not user-visible activity)
//   - 3 (WatchError)        → ActivityPollFailed
//   - 4 (WatchHeartbeat)    → ActivityPollSucceeded (each heartbeat means
//     one successful poll completed)
//   - 5 (WatchEmailStored)  → ActivityEmailStored (Slice #107)
//   - 6 (WatchRuleMatched)  → ActivityRuleMatched (Slice #107)
//
// Unknown integer values are also dropped — the row is silently
// elided from the returned slice so a future watcher emitting
// Kind=7+ can roll out without crashing the dashboard on older
// binaries. The `out` slice's length may therefore be ≤ the store
// row count; the caller (`RecentActivity`) treats this as
// "honoured limit, fewer rows available" which is correct semantics.
//
// Error envelope: store-layer failures are wrapped with `ErrDbOpen`
// (driver-side) — the store has already attached a per-call op
// suffix via `errtrace.Wrap`, so we only add the code, never the
// op string.
func NewStoreActivitySource(s *store.Store) activitySource {
	if s == nil {
		return nil
	}
	return func(ctx context.Context, limit int) errtrace.Result[[]ActivityRow] {
		rows, err := s.QueryRecentActivity(ctx, limit)
		if err != nil {
			return errtrace.Err[[]ActivityRow](
				errtrace.WrapCode(err, errtrace.ErrDbOpen,
					"core.NewStoreActivitySource"),
			)
		}
		out := make([]ActivityRow, 0, len(rows))
		for _, r := range rows {
			ak, ok := mapWatchKindToActivityKind(r.Kind)
			if !ok {
				continue // drop Stop + unknown kinds; see doc-comment.
			}
			out = append(out, ActivityRow{
				OccurredAt: r.OccurredAt,
				Alias:      r.Alias,
				Kind:       ak,
				Message:    r.Message,
				ErrorCode:  r.ErrorCode,
			})
		}
		return errtrace.Ok(out)
	}
}

// mapWatchKindToActivityKind translates the integer
// `core.WatchEventKind` enum stored in `WatchEvents.Kind` to the
// string `core.ActivityKind` enum the dashboard renders. Returns
// `(zero, false)` for kinds that have no spec ActivityKind (Stop)
// or that this binary doesn't recognise (forward-compat).
//
// Co-located here (not in dashboard_activity.go) because the
// mapping is store-shim-specific — the pure-Go core has no need to
// know that `Heartbeat == PollSucceeded`; that's an
// adapter-layer interpretation.
func mapWatchKindToActivityKind(k int) (ActivityKind, bool) {
	switch WatchEventKind(k) {
	case WatchStart:
		return ActivityPollStarted, true
	case WatchHeartbeat:
		return ActivityPollSucceeded, true
	case WatchError:
		return ActivityPollFailed, true
	case WatchEmailStored:
		return ActivityEmailStored, true
	case WatchRuleMatched:
		return ActivityRuleMatched, true
	default:
		// WatchStop + any unknown integer → dropped.
		return "", false
	}
}
