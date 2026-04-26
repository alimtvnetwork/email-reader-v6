// tools_recent_opens_format.go holds the pure formatting + filter helpers
// for the "Recent opens" sub-tool. Lives without the !nofyne tag so the
// table-driven tests run on headless CI.
//
// Spec: spec/21-app/02-features/06-tools/02-frontend.md (Recent opens),
// activates Delta #1 (PascalCase OpenedUrls) Alias/Origin filters in a
// real caller of `core.Tools.RecentOpenedUrls`.
package views

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lovable/email-read/internal/core"
)

// RecentOpensFilter is the user-facing filter form translated to a
// `core.OpenedUrlListSpec`. Empty Alias / Origin = no constraint.
type RecentOpensFilter struct {
	Alias    string // free-text alias filter
	Origin   string // "" | "manual" | "rule" | "cli"
	LimitStr string // free-text; parsed via ParseRecentOpensLimit
}

// OriginChoices is the ordered list of dropdown labels. "All" maps to ""
// in the resulting spec, the others map 1:1 to OpenUrlOrigin values.
var OriginChoices = []string{"All", "manual", "rule", "cli"}

// ParseRecentOpensLimit accepts free-text and clamps to [1,1000]. Empty
// or non-numeric falls back to 100 — matches core's default.
func ParseRecentOpensLimit(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 100
	}
	if n > 1000 {
		return 1000
	}
	return n
}

// BuildRecentOpensSpec converts the form values into a validated spec
// the core layer accepts. The "All" sentinel maps to empty Origin.
func BuildRecentOpensSpec(f RecentOpensFilter) core.OpenedUrlListSpec {
	origin := strings.TrimSpace(f.Origin)
	if origin == "All" || origin == "" {
		origin = ""
	}
	return core.OpenedUrlListSpec{
		Alias:  strings.TrimSpace(f.Alias),
		Origin: core.OpenUrlOrigin(origin),
		Limit:  ParseRecentOpensLimit(f.LimitStr),
	}
}

// FormatRecentOpensRow renders one row as a single human-readable line.
// Used by both the live UI and the table-test below to lock the format.
func FormatRecentOpensRow(r core.OpenedUrlRow) string {
	when := "unknown"
	if !r.OpenedAt.IsZero() {
		when = r.OpenedAt.Format("2006-01-02 15:04:05")
	}
	flags := formatRecentOpensFlags(r)
	rule := r.RuleName
	if rule == "" {
		rule = "—"
	}
	return fmt.Sprintf("%s  [%s/%s] rule=%s%s  %s",
		when, nonEmpty(r.Alias, "—"), nonEmpty(string(r.Origin), "—"),
		rule, flags, truncURL(r.Url))
}

// formatRecentOpensFlags renders the boolean Delta-#1 columns as a
// compact suffix; empty when no flags are set.
func formatRecentOpensFlags(r core.OpenedUrlRow) string {
	var parts []string
	if r.IsDeduped {
		parts = append(parts, "deduped")
	}
	if r.IsIncognito {
		parts = append(parts, "incognito")
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ",") + ")"
}

// FormatRecentOpensSummary renders the header line shown above the
// scrollable list. Empty Alias/Origin become "all".
func FormatRecentOpensSummary(spec core.OpenedUrlListSpec, count int, elapsed time.Duration) string {
	a := spec.Alias
	if a == "" {
		a = "all aliases"
	}
	o := string(spec.Origin)
	if o == "" {
		o = "all origins"
	}
	return fmt.Sprintf("%d row(s) — alias=%s, origin=%s, limit=%d (took %s)",
		count, a, o, spec.Limit, elapsed.Round(time.Millisecond))
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// truncURL is shared with watch_events.go (same package).

