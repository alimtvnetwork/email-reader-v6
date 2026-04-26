// dashboard_health.go — P3.3 AccountHealth core (pure-Go).
//
// Spec: spec/21-app/02-features/01-dashboard/01-backend.md §2.3, §3.3
//       spec/21-app/02-features/01-dashboard/00-overview.md (struct shapes)
//
// This slice lands the **pure-Go core** of AccountHealth:
//
//   - Types: HealthLevel, AccountHealthRow.
//   - Pure function: ComputeHealth(row, now) — the 4-branch decision
//     matrix from §3.3 of the spec, kept as a free function so it can
//     be table-tested without any service plumbing.
//   - Service method: (*DashboardService).AccountHealth(ctx) joins
//     `cfg.Accounts` (one row per configured alias, even if the
//     watcher has never reported on it) with rows from an injected
//     `accountHealthSource`. Aliases configured but absent from the
//     source surface as "Warning" with zero-value timestamps —
//     spec test #5 (`AccountHealth_NoWatchStateRow_ReturnsWarning`).
//
// The store-side `Store.QueryAccountHealth` shim that backs
// `accountHealthSource` in production lands in a later slice once
// the `WatchState` table + M0009 migration ship. Until then UI sites
// inject either a stub source or a temporary adapter — keeping this
// slice pure-Go means we are unblocked for tests immediately and can
// rip-and-replace the source impl without touching this file.
//
// Why a function-typed dependency (vs. an interface): same precedent
// as `configLoader` / `emailsCounter` in dashboard.go — single-method
// dependencies are easier to fake as one-line closures in tests.
package core

import (
	"context"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// HealthLevel is the per-account status bucket rendered by the
// Dashboard. Values are PascalCase per spec/21-app §04-enum-standards.
type HealthLevel string

const (
	HealthHealthy HealthLevel = "Healthy"
	HealthWarning HealthLevel = "Warning"
	HealthError   HealthLevel = "Error"
)

// AccountHealthRow is the projection rendered by the per-account
// health badge on the Dashboard. Field names mirror
// `spec/21-app/02-features/01-dashboard/00-overview.md` §4 verbatim
// — do not rename without updating the spec first.
type AccountHealthRow struct {
	Alias               string
	LastPollAt          time.Time // zero = never polled
	LastErrorAt         time.Time // zero = never errored
	ConsecutiveFailures int
	Health              HealthLevel
	EmailsStored        int
	UnreadCount         int
}

// ComputeHealth applies the 4-branch decision matrix from
// spec §3.3. Pure function — no I/O, no clock. Caller must pass
// `now` explicitly so tests stay deterministic.
//
// Decision order (matches spec; first match wins):
//  1. ConsecutiveFailures >= 3 → Error (the watcher is stuck).
//  2. now - LastPollAt > 15min → Warning (stale; watcher silent).
//  3. LastErrorAt > LastPollAt → Warning (last activity was a fail).
//  4. otherwise              → Healthy.
//
// Edge cases (encoded so the table test pins them):
//   - LastPollAt == zero is treated as "infinitely stale" by branch
//     (2): `now.Sub(zero)` is a very large positive duration.
//   - LastErrorAt == zero never beats a non-zero LastPollAt in
//     branch (3) — `time.Time{}.After(t)` is false for any t after
//     the zero value.
func ComputeHealth(row AccountHealthRow, now time.Time) HealthLevel {
	switch {
	case row.ConsecutiveFailures >= 3:
		return HealthError
	case now.Sub(row.LastPollAt) > 15*time.Minute:
		return HealthWarning
	case row.LastErrorAt.After(row.LastPollAt):
		return HealthWarning
	default:
		return HealthHealthy
	}
}

// accountHealthSource fetches the live health-relevant columns for
// every alias the watcher has touched. Production impl: a thin shim
// over `Store.QueryAccountHealth` (lands in a later slice once
// WatchState + M0009 ship). Tests: a one-line closure.
//
// The returned slice is keyed by Alias; the service is responsible
// for joining it against the configured account list and synthesising
// "Warning" rows for aliases the source omits (spec §2.3).
type accountHealthSource func(ctx context.Context) errtrace.Result[[]AccountHealthRow]

// AccountHealth returns one AccountHealthRow per **configured**
// alias. Aliases present in the source are returned with their live
// counters; aliases configured but absent from the source surface as
// `Warning` rows with zero-value timestamps.
//
// Health is computed via `ComputeHealth(row, s.now())` on every row
// — the source is allowed to leave `Health` empty; the service is
// authoritative for that field.
//
// Errors:
//   - source error           → wrapped as ErrDbQueryEmail (broad
//     store-query envelope, same as LoadStats; a dedicated
//     ErrDbQueryWatchState code arrives with the store-shim slice).
//   - unset clock dep        → never (constructor guards both deps;
//     AccountHealth uses cfg + source + clock — clock-injection
//     follows in P3.3b once a FakeClock is wired through the
//     constructor).
//
// Until the constructor accepts a clock dep, this method uses
// `time.Now()` directly. The unit test passes `now` to ComputeHealth
// in a separate table to avoid coupling to wall-clock.
func (s *DashboardService) AccountHealth(ctx context.Context, src accountHealthSource) errtrace.Result[[]AccountHealthRow] {
	if src == nil {
		return errtrace.Err[[]AccountHealthRow](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"DashboardService.AccountHealth: src is nil"))
	}
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[[]AccountHealthRow](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen,
				"core.DashboardService.AccountHealth"),
		)
	}
	srcRes := src(ctx)
	if srcRes.HasError() {
		return errtrace.Err[[]AccountHealthRow](
			errtrace.WrapCode(srcRes.Error(), errtrace.ErrDbQueryEmail,
				"core.DashboardService.AccountHealth").
				WithContext("scope", "watch_state"),
		)
	}
	byAlias := make(map[string]AccountHealthRow, len(srcRes.Value()))
	for _, r := range srcRes.Value() {
		byAlias[r.Alias] = r
	}
	now := time.Now()
	out := make([]AccountHealthRow, 0, len(cfg.Accounts))
	for _, acc := range cfg.Accounts {
		row, ok := byAlias[acc.Alias]
		if !ok {
			// Configured but watcher hasn't reported yet — synthesise
			// a Warning row so the UI can render the badge greyed out
			// instead of silently dropping the alias. Spec test #5.
			row = AccountHealthRow{Alias: acc.Alias}
		}
		row.Health = ComputeHealth(row, now)
		out = append(out, row)
	}
	return errtrace.Ok(out)
}
