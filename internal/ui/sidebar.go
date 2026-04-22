// sidebar.go renders the navigation List and the account picker.
// Framework-agnostic data lives in nav.go and state.go (no fyne imports).
//
// Layout inside the sidebar:
//
//	┌──────────────────┐
//	│  email-read      │   header (bold, centered)
//	├──────────────────┤
//	│  Account ▾       │   account picker (Step 10)
//	├──────────────────┤
//	│  Dashboard       │   nav list (Step 9)
//	│  Emails          │
//	│  …               │
//	└──────────────────┘
//
//go:build !nofyne

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SidebarOptions wires the sidebar to app state and account data. Aliases is
// the list of configured account aliases (typically from core.ListAccounts).
// onSelectNav fires whenever the user picks a different nav row.
type SidebarOptions struct {
	State       *AppState
	Aliases     []string
	OnSelectNav func(NavItem)
}

// NewSidebar builds the navigation list + account picker. The first nav row
// is pre-selected so the detail pane is never blank. The account picker is
// pre-selected to State.Alias() if non-empty, else the first alias if any.
func NewSidebar(opts SidebarOptions) fyne.CanvasObject {
	picker := newAccountPicker(opts.State, opts.Aliases)

	list := widget.NewList(
		func() int { return len(NavItems) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(NavItems[i].Title)
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i < 0 || i >= len(NavItems) {
			return
		}
		item := NavItems[i]
		if opts.State != nil {
			opts.State.SetNav(item.Kind)
		}
		if opts.OnSelectNav != nil {
			opts.OnSelectNav(item)
		}
	}
	list.Select(0)

	header := widget.NewLabelWithStyle("email-read", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	top := container.NewVBox(header, picker, widget.NewSeparator())
	return container.NewBorder(top, nil, nil, nil, list)
}

// newAccountPicker builds the dropdown. Empty aliases ⇒ a disabled picker
// labelled "No accounts — add one in Tools" so the user knows what to do.
func newAccountPicker(state *AppState, aliases []string) fyne.CanvasObject {
	if len(aliases) == 0 {
		empty := widget.NewSelect([]string{"No accounts"}, func(string) {})
		empty.PlaceHolder = "No accounts"
		empty.Disable()
		return container.NewPadded(empty)
	}
	sel := widget.NewSelect(aliases, func(picked string) {
		if state != nil {
			state.SetAlias(picked)
		}
	})
	// Pre-select: keep current state if it matches one of the aliases,
	// otherwise default to the first one (and update state to match).
	current := ""
	if state != nil {
		current = state.Alias()
	}
	chosen := aliases[0]
	for _, a := range aliases {
		if a == current {
			chosen = current
			break
		}
	}
	sel.SetSelected(chosen)
	if state != nil && state.Alias() != chosen {
		state.SetAlias(chosen)
	}
	return container.NewPadded(sel)
}
