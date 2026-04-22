// rules_format.go — pure-Go helpers for the Rules view. Lives outside
// rules.go (which is fyne-tagged) so the formatting logic is unit-testable
// on headless CI.
package views

import "github.com/lovable/email-read/internal/config"

// RuleFilters collapses the optional regex columns (from / subject / body)
// into a single short label for the table. Empty filters are omitted; an
// all-empty rule is reported as "(none)".
func RuleFilters(r config.Rule) string {
	parts := make([]string, 0, 3)
	if r.FromRegex != "" {
		parts = append(parts, "from")
	}
	if r.SubjectRegex != "" {
		parts = append(parts, "subject")
	}
	if r.BodyRegex != "" {
		parts = append(parts, "body")
	}
	if len(parts) == 0 {
		return "(none)"
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += " · " + p
	}
	return out
}

// EnabledLabel returns a stable badge string used in the table cell. Kept
// here (not inline) so tests can lock the user-visible wording.
func EnabledLabel(enabled bool) string {
	if enabled {
		return "✓ on"
	}
	return "○ off"
}
