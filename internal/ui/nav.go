// nav.go holds the framework-agnostic sidebar data: NavKind constants and
// the NavItems slice. No fyne imports here on purpose so this file (and the
// matching tests) compile without cgo / OpenGL dev libs — handy for CI on
// headless boxes and for unit-testing the canonical nav order.
package ui

// NavKind identifies a sidebar entry. Stable string values so tests and
// future state-persistence (`data/ui-state.json`, Phase 6) can match by key.
type NavKind string

const (
	NavDashboard NavKind = "dashboard"
	NavEmails    NavKind = "emails"
	NavRules     NavKind = "rules"
	NavAccounts  NavKind = "accounts"
	NavWatch     NavKind = "watch"
	NavTools     NavKind = "tools"
	NavSettings  NavKind = "settings"
	// NavErrorLog is the Diagnostics → Error Log view added in
	// Phase 3.4 of the error-trace logging upgrade. Sits at the
	// bottom of the sidebar (under "Diagnostics") so it doesn't
	// crowd the day-to-day Dashboard/Emails/Rules surfaces.
	NavErrorLog NavKind = "error_log"
)

// NavItem is one row in the sidebar.
type NavItem struct {
	Kind        NavKind
	Title       string // displayed in the sidebar
	Placeholder string // shown in the detail pane until the real view lands
	// Group, when non-empty, renders a small italic header above this
	// row in the sidebar (e.g. "Diagnostics"). Items with the same
	// Group on consecutive rows share one header — see sidebar.go.
	Group string
}

// NavItems is the canonical, ordered nav list. Keep it small — Fyne sidebars
// don't paginate, and the spec calls for these six.
var NavItems = []NavItem{
	{NavDashboard, "Dashboard", "Counts, recent events, and a Start Watch button land here in Step 11.", ""},
	{NavEmails, "Emails", "Email list + detail (subject, body, links) lands in Step 12.", ""},
	{NavRules, "Rules", "Rule table with enable/disable toggles lands in Step 13.", ""},
	{NavAccounts, "Accounts", "Account table (alias, host, last UID) lands in Step 14.", ""},
	{NavWatch, "Watch", "Live watcher: structured cards + raw log tabs land in Steps 21–23.", ""},
	{NavTools, "Tools", "Inline forms for read / export-csv / diagnose land in Steps 18–20.", ""},
	{NavSettings, "Settings", "Theme, poll cadence, browser path, and density toggle.", ""},
	{NavErrorLog, "Error Log", "In-memory ring of every UI failure with full file:line traces.", "Diagnostics"},
}
