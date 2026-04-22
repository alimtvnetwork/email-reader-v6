// sidebar.go renders NavItems as a Fyne List widget. The data lives in
// nav.go (no fyne imports) so it can be tested without cgo.
//
// Build tag: this file is only compiled when cgo is available (which is what
// fyne requires anyway). On headless CI boxes without OpenGL/X11 dev headers
// you can `go test ./internal/ui -tags nofyne` to skip it; the nav.go data
// + tests still run.
//go:build !nofyne

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

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
