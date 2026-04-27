// accounts_format.go — pure-Go helpers for the Accounts view.
package views

import (
	"fmt"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
)

// formatAccountHealthBadge renders a one-glyph + label badge for the
// per-row health column on the Accounts table. Glyphs match the
// dashboard rollup (`dashboard_health_test.go`) so the two surfaces
// stay visually consistent:
//
//   - Healthy → "● Healthy"
//   - Warning → "◐ Warning"
//   - Error   → "✗ Error"
//   - empty   → "— Unknown"  (alias configured but health not loaded yet)
//   - other   → "? <raw>"    (defensive: future HealthLevel values)
//
// Pure function — no I/O, no fyne import — so it stays unit-testable
// without spinning up the Fyne driver.
func formatAccountHealthBadge(level core.HealthLevel) string {
	switch level {
	case core.HealthHealthy:
		return "● Healthy"
	case core.HealthWarning:
		return "◐ Warning"
	case core.HealthError:
		return "✗ Error"
	case "":
		return "— Unknown"
	default:
		return "? " + string(level)
	}
}

func AccountServer(a config.Account) string {
	tag := "TLS"
	if !a.UseTLS {
		tag = "PLAIN"
	}
	port := a.ImapPort
	if port == 0 {
		port = 993
	}
	host := a.ImapHost
	if host == "" {
		host = "(unset)"
	}
	return fmt.Sprintf("%s:%d (%s)", host, port, tag)
}

func LastSeenLabel(uid uint32) string {
	if uid == 0 {
		return "(never watched)"
	}
	return fmt.Sprintf("%d", uid)
}
