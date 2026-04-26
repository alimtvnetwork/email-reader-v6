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
// trigger a reload of the static stat cards. Two kinds qualify:
//
//   - EventNewMail   → bumps the persisted "Emails stored" count.
//   - EventUrlOpened → side-effects of a rule firing (and gives the
//     "Loaded at HH:MM:SS" footer immediate human feedback that the
//     watcher just acted). LoadDashboardStats is a fresh COUNT(*) so
//     re-running it is safe and cheap.
//
// Every other kind short-circuits. We also debounce: bursts (a 50-msg
// backfill, or a rule that opens a URL per match) must not trigger 50
// SQL aggregates — one refresh per `minInterval` is enough. The
// debounce window is shared across both event kinds so a NewMail
// followed 100ms later by its UrlOpened doesn't double-refresh.
//
// Returns (shouldRefresh, newLastRefresh). The caller persists
// newLastRefresh and gates the actual refresh() invocation on the bool.
//
// Spec: spec/21-app/02-features/01-dashboard/02-frontend.md — "Emails
// stored auto-bumps within ~1s of NewMail without manual refresh"; the
// UrlOpened trigger extends the same contract to rule-launch feedback.
func ShouldRefreshDashboardOnEvent(ev watcher.Event, last time.Time, now time.Time, minInterval time.Duration) (bool, time.Time) {
	if !dashboardRefreshKind(ev.Kind) {
		return false, last
	}
	if minInterval > 0 && !last.IsZero() && now.Sub(last) < minInterval {
		return false, last
	}
	return true, now
}

// dashboardRefreshKind reports whether ev.Kind is one of the watcher
// signals that should trigger a static-stats reload. Kept as a small
// helper so adding future trigger kinds (e.g. EventUidValReset) is a
// one-line change with an obvious test target.
func dashboardRefreshKind(k watcher.EventKind) bool {
	switch k {
	case watcher.EventNewMail, watcher.EventUrlOpened:
		return true
	default:
		return false
	}
}
