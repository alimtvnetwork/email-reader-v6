// app.go ties the sidebar (sidebar.go) and detail pane together. Like
// sidebar.go, this file requires the Fyne cgo backend; gate it off with
// `-tags nofyne` to compile/test the rest of the package on headless CI.
//go:build !nofyne

// Package ui hosts the Fyne desktop frontend. It is intentionally split from
// cmd/email-read-ui so internal/ui can be unit-tested with `go test` without
// needing the cgo display libs that linking the binary requires.
//
// Layout (Step 9):
//
//	┌──────────────────────────────────────────┐
//	│  email-read · v0.20.0                    │
//	├──────────────┬───────────────────────────┤
//	│ Dashboard    │                           │
//	│ Emails       │                           │
//	│ Rules        │     <active view>         │
//	│ Accounts     │                           │
//	│ Watch        │                           │
//	│ Tools        │                           │
//	└──────────────┴───────────────────────────┘
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// AppVersion is shown in the window title. Bumped per release in lockstep
// with cmd/email-read/main.go.
const AppVersion = "0.20.0"

// Run creates the Fyne app, builds the main window, and blocks until close.
func Run() {
	a := app.NewWithID("dev.lovable.email-read")
	w := a.NewWindow("email-read · v" + AppVersion)
	w.SetContent(BuildShell())
	w.Resize(fyne.NewSize(1000, 680))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// BuildShell returns the root container: sidebar on the left, swapping
// detail pane on the right.
func BuildShell() fyne.CanvasObject {
	detail := container.NewStack(viewFor(NavItems[0]))

	sidebar := NewSidebar(func(item NavItem) {
		detail.Objects = []fyne.CanvasObject{viewFor(item)}
		detail.Refresh()
	})

	split := container.NewHSplit(sidebar, container.NewPadded(detail))
	split.SetOffset(0.18)
	return split
}

// viewFor returns the placeholder content for a nav item. Phase 3 replaces
// these with real Dashboard/Emails/Rules/Accounts/Watch/Tools widgets.
func viewFor(item NavItem) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(item.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	body := widget.NewLabel(item.Placeholder)
	body.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), body)
}
