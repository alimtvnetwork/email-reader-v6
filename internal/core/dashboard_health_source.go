// dashboard_health_source.go — Slice #102: production
// `accountHealthSource` adapter that bridges `Store.QueryAccountHealth`
// into the function-typed dependency consumed by
// `(*DashboardService).AccountHealth`.
//
// **Why a separate file** — `dashboard_health.go` is intentionally
// pure-Go (no `database/sql`, no `*store.Store` import) so its tests
// run without spinning up SQLite. Co-locating the store adapter
// would either drag the store import into the pure file (breaking
// the boundary) or require build tags. A separate file keeps the
// pure-vs-impure split self-evident at file granularity.
//
// **Why a separate type for `StoreAccountHealthRow`** — the store
// shim returns its own struct (intentional; see the doc-comment on
// `store.StoreAccountHealthRow`) so the store package doesn't import
// `core`. This adapter does the trivial field-by-field copy and
// leaves `Health` zero-valued (the service overwrites it via
// `ComputeHealth`) and `ConsecutiveFailures` zero-valued (the store
// shim doesn't compute it yet — see the deferred-work note in
// `queries.AccountHealthSelectAll`).
package core

import (
	"context"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// NewStoreAccountHealthSource wraps `(*store.Store).QueryAccountHealth`
// in the `accountHealthSource` shape consumed by
// `(*DashboardService).AccountHealth`. Returns nil when `s` is nil so
// the caller can decide whether to bail or fall back to a stub
// source — same convention as `NewDefaultEmailsService`.
//
// Error envelope: store-layer failures are wrapped with `ErrDbOpen`
// (driver-side) — the store has already attached a per-call op
// suffix via `errtrace.Wrap`, so we only add the code, never the
// op string.
func NewStoreAccountHealthSource(s *store.Store) accountHealthSource {
	if s == nil {
		return nil
	}
	return func(ctx context.Context) errtrace.Result[[]AccountHealthRow] {
		rows, err := s.QueryAccountHealth(ctx)
		if err != nil {
			return errtrace.Err[[]AccountHealthRow](
				errtrace.WrapCode(err, errtrace.ErrDbOpen,
					"core.NewStoreAccountHealthSource"),
			)
		}
		out := make([]AccountHealthRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, AccountHealthRow{
				Alias:        r.Alias,
				LastPollAt:   r.LastPollAt,
				LastErrorAt:  r.LastErrorAt,
				EmailsStored: r.EmailsStored,
				UnreadCount:  r.UnreadCount,
				// ConsecutiveFailures left at zero — see
				// queries.AccountHealthSelectAll for the deferred-
				// work rationale. Health left empty — the service
				// overwrites via ComputeHealth.
			})
		}
		return errtrace.Ok(out)
	}
}
