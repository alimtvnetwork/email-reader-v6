// sidebar.go renders NavItems as a Fyne List + an account picker on top.
// The data lives in nav.go and AppState lives in state.go (both fyne-free)
// so headless CI can test the canonical nav order and observer pattern.
//go:build !nofyne

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SidebarOptions wires the sidebar to the shared AppState plus the live
// account list. OnSelectNav is invoked synchronously when the user picks
// a row so the shell can swap the detail pane.
type SidebarOptions struct {
	State       *AppState
	Aliases     []string
	OnSelectNav func(NavItem)
}

// NewSidebar builds the sidebar: header, account picker, nav list.
func NewSidebar(opts SidebarOptions) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("email-read", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	var picker *widget.Select
	if len(opts.Aliases) == 0 {
		picker = widget.NewSelect([]string{"No accounts — add one"}, nil)
		picker.Disable()
	} else {
		picker = widget.NewSelect(opts.Aliases, func(a string) {
			if opts.State != nil {
				opts.State.SetAlias(a)
			}
		})
		// Pre-select either the previously chosen alias or the first one so
		// the detail pane always has a useful default.
		if opts.State != nil && opts.State.Alias() != "" {
			picker.SetSelected(opts.State.Alias())
		} else {
			picker.SetSelected(opts.Aliases[0])
		}
	}

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
	// Pre-select the row matching state.Nav() (or the first row).
	preIdx := 0
	if opts.State != nil {
		for i, it := range NavItems {
			if it.Kind == opts.State.Nav() {
				preIdx = i
				break
			}
		}
	}
	list.Select(preIdx)

	top := container.NewVBox(
		header,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Account", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		picker,
		widget.NewSeparator(),
	)
	return container.NewBorder(top, nil, nil, nil, list)
}
