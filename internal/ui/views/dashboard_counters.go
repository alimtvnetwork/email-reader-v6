// dashboard_counters.go — pure helpers for the dashboard's live
// counter row. Framework-agnostic so the alias-filter contract is
// unit-testable under `-tags nofyne`.
//
// The dashboard reuses the WatchCounters projection from
// watch_events.go so the footer numbers shown on the Watch page and
// the tile values shown on the Dashboard cannot drift apart.
//
// Spec: spec/21-app/02-features/01-dashboard/02-frontend.md (live
// counters tile row).
package views

import (
	"fmt"
	"time"

	"github.com/lovable/email-read/internal/watcher"
)

// DashboardAcceptsEvent reports whether a bus event should feed the
// dashboard's counters. Empty selectedAlias = "all aliases" — the
// dashboard is the only surface that opts into cross-alias aggregation.
func DashboardAcceptsEvent(ev watcher.Event, selectedAlias string) bool {
	if selectedAlias == "" {
		return true
	}
	return ev.Alias == selectedAlias
}

// FormatDashboardCounterTile renders one of the four live tiles. The
// dashboard uses single-number tiles (vs. the watch footer's compound
// line) because each value sits in its own card.
func FormatDashboardCounterTile(label string, n int) string {
	return fmt.Sprintf("%s: %d", label, n)
}

// DashboardCounterScope captures the alias-aggregation mode for the
// counter row. "" = all aliases, otherwise the alias being shown.
type DashboardCounterScope struct {
	Alias string
}

// FormatDashboardCounterScope renders the muted caption shown above
// the live tile row, e.g. "Live counters — alias=work" / "all aliases".
func FormatDashboardCounterScope(s DashboardCounterScope) string {
	if s.Alias == "" {
		return "Live counters — all aliases"
	}
	return "Live counters — alias=" + s.Alias
}

// ShouldRefreshDashboardOnEvent decides whether a watcher event should
// trigger a reload of the static "Emails stored" tile. Only EventNewMail
// bumps the persisted count, so every other kind short-circuits. We
// also debounce: bursts of new mail (e.g., a 50-message backfill) must
// not trigger 50 SQL aggregates — one refresh per `minInterval` is
// enough because LoadDashboardStats is a fresh COUNT(*) anyway.
//
// Returns (shouldRefresh, newLastRefresh). The caller persists
// newLastRefresh and gates the actual refresh() invocation on the bool.
//
// Spec: spec/21-app/02-features/01-dashboard/02-frontend.md — "Emails
// stored auto-bumps within ~1s of NewMail without manual refresh".
func ShouldRefreshDashboardOnEvent(ev watcher.Event, last time.Time, now time.Time, minInterval time.Duration) (bool, time.Time) {
	if ev.Kind != watcher.EventNewMail {
		return false, last
	}
	if minInterval > 0 && !last.IsZero() && now.Sub(last) < minInterval {
		return false, last
	}
	return true, now
}
