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

	"github.com/lovable/email-read/internal/watcher"
)

// DashboardCounterEvent describes one bus event the dashboard accepts.
// Returns false if the event should be ignored (alias mismatch when a
// specific alias is selected). Empty selectedAlias = "all aliases" — the
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
