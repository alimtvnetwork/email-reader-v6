// rules_format.go — pure-Go helpers for the Rules view.
package views

import "github.com/lovable/email-read/internal/config"

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

func EnabledLabel(enabled bool) string {
	if enabled {
		return "✓ on"
	}
	return "○ off"
}
