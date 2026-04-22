// sidebar.go defines the left-rail navigation: a fixed list of NavItems
// rendered as a Fyne List widget. Selecting an item invokes the onSelect
// callback with the chosen NavItem so app.go can swap the detail pane.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

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
)

// NavItem is one row in the sidebar.
type NavItem struct {
	Kind        NavKind
	Title       string // displayed in the sidebar
	Placeholder string // shown in the detail pane until the real view lands
}

// NavItems is the canonical, ordered nav list. Keep it small — Fyne sidebars
// don't paginate, and the spec calls for these six.
var NavItems = []NavItem{
	{NavDashboard, "Dashboard", "Counts, recent events, and a Start Watch button land here in Step 11."},
	{NavEmails, "Emails", "Email list + detail (subject, body, links) lands in Step 12."},
	{NavRules, "Rules", "Rule table with enable/disable toggles lands in Step 13."},
	{NavAccounts, "Accounts", "Account table (alias, host, last UID) lands in Step 14."},
	{NavWatch, "Watch", "Live watcher: structured cards + raw log tabs land in Steps 21–23."},
	{NavTools, "Tools", "Inline forms for read / export-csv / diagnose land in Steps 18–20."},
}

// NewSidebar builds the navigation list. onSelect is invoked synchronously
// on the UI goroutine whenever the user picks a row. The first row is
// pre-selected so the detail pane is never blank.
func NewSidebar(onSelect func(NavItem)) fyne.CanvasObject {
	list := widget.NewList(
		func() int { return len(NavItems) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(NavItems[i].Title)
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if onSelect != nil && i >= 0 && i < len(NavItems) {
			onSelect(NavItems[i])
		}
	}
	list.Select(0)

	header := widget.NewLabelWithStyle("email-read", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	return container.NewBorder(header, nil, nil, nil, list)
}
