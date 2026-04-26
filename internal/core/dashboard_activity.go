// dashboard_activity.go — P3.4 RecentActivity core (pure-Go).
//
// Spec: spec/21-app/02-features/01-dashboard/01-backend.md §2.2, §3.2
//       spec/21-app/02-features/01-dashboard/00-overview.md (struct)
//
// This slice lands the **pure-Go core** of RecentActivity:
//
//   - Types: ActivityKind enum + ActivityRow struct (spec-verbatim).
//   - Service method: (*DashboardService).RecentActivity(ctx, limit, src)
//     with limit guarded (≥1 required, >200 silently clamped) and a
//     function-typed source dependency, matching the precedent set by
//     AccountHealth in dashboard_health.go (P3.3).
//
// The store-side `Store.QueryRecentActivity` shim that backs the
// production source lands later when the M0008 `WatchEvent` table
// migration is committed. Until then UI sites inject either a stub or
// an event-bus-backed adapter — keeping this slice pure-Go means we
// are unblocked for tests immediately.
//
// Why a function-typed dep (vs. an interface): same precedent as
// `configLoader` / `emailsCounter` / `accountHealthSource` — single
// method deps are easier to fake as a closure.
package core

import (
	"context"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// ActivityKind is the PascalCase enum used by the WatchEvent.Kind
// column. Values from spec/21-app/02-features/01-dashboard/00-overview.md.
type ActivityKind string

const (
	ActivityPollStarted   ActivityKind = "PollStarted"
	ActivityPollSucceeded ActivityKind = "PollSucceeded"
	ActivityPollFailed    ActivityKind = "PollFailed"
	ActivityEmailStored   ActivityKind = "EmailStored"
	ActivityRuleMatched   ActivityKind = "RuleMatched"
)

// ActivityRow is the projection rendered by the RecentActivityList on
// the Dashboard. Field names mirror the spec verbatim.
type ActivityRow struct {
	OccurredAt time.Time
	Alias      string
	Kind       ActivityKind
	Message    string
	ErrorCode  int // 0 if Kind != PollFailed
}

// RecentActivityLimitMax is the hard upper bound from the spec
// (§2.2). Requests above it are silently clamped — not an error,
// because a generous "give me everything recent" call is reasonable
// UI behaviour and shouldn't punish the caller.
const RecentActivityLimitMax = 200

// activitySource fetches up to `limit` most-recent ActivityRow
// entries from the WatchEvent log, sorted by OccurredAt DESC.
// Production impl: a thin shim over `Store.QueryRecentActivity`
// (lands once M0008 ships). Tests: a one-line closure.
//
// The source is responsible for:
//   - Honouring the limit it receives (the service has already
//     clamped the user-provided value).
//   - Returning rows already sorted DESC.
type activitySource func(ctx context.Context, limit int) errtrace.Result[[]ActivityRow]

// RecentActivity returns the most recent activity rows from the
// WatchEvent log. Behaviour from spec §2.2:
//
//   - limit < 1                → ErrCoreInvalidArgument (caller bug).
//     The spec assigns a dedicated `21102 DashboardInvalidLimit`
//     code that arrives with the codes.yaml extension slice; until
//     then ErrCoreInvalidArgument is the closest existing envelope
//     and carries the same caller-bug semantics.
//   - 1 ≤ limit ≤ 200          → forwarded verbatim to source.
//   - limit > 200              → silently clamped to 200; no error.
//
// Source errors are wrapped with ErrDbQueryEmail (broad store-query
// envelope, same pattern as LoadStats / AccountHealth). A dedicated
// ErrDbQueryWatchEvent code arrives with the store-shim slice.
func (s *DashboardService) RecentActivity(ctx context.Context, limit int, src activitySource) errtrace.Result[[]ActivityRow] {
	if src == nil {
		return errtrace.Err[[]ActivityRow](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"DashboardService.RecentActivity: src is nil"))
	}
	if limit < 1 {
		return errtrace.Err[[]ActivityRow](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"DashboardService.RecentActivity: limit must be >= 1").
			WithContext("limit", limit))
	}
	if limit > RecentActivityLimitMax {
		limit = RecentActivityLimitMax
	}
	res := src(ctx, limit)
	if res.HasError() {
		return errtrace.Err[[]ActivityRow](
			errtrace.WrapCode(res.Error(), errtrace.ErrDbQueryEmail,
				"core.DashboardService.RecentActivity").
				WithContext("limit", limit),
		)
	}
	return errtrace.Ok(res.Value())
}
