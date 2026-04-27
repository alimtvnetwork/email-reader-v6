// sidebar_badge.go computes the row label for a NavItem, optionally
// suffixed with an unread-count badge (e.g. "Error Log  (3)"). Lives
// in a fyne-free file so headless tests can lock the formatting
// rules — Phase 3.4 of the error-trace logging upgrade.
package ui

import "fmt"

// formatNavRowLabel renders the sidebar list label for one nav row.
// When badge > 0 it appends "  (N)" so the unread count is visible
// without a separate widget. Counts > 99 collapse to "(99+)" so a
// runaway producer can't shove the label off-screen.
//
// Pure function — no fyne types — so the rendering rules are
// unit-testable under -tags nofyne and the sidebar binder just calls
// this with the live count from BadgeSource.
func formatNavRowLabel(title string, badge int64) string {
	if badge <= 0 {
		return title
	}
	if badge > 99 {
		return fmt.Sprintf("%s  (99+)", title)
	}
	return fmt.Sprintf("%s  (%d)", title, badge)
}
