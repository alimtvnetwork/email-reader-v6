// Package ui hosts the Fyne desktop frontend. It is intentionally split from
// cmd/email-read-ui so that internal/ui can be unit-tested with `go test`
// without needing the cgo display libraries that linking the binary requires.
//
// Layout (Step 9):
//
//	┌──────────────────────────────────────────┐
//	│  email-read · v0.20.0                    │  ← window title
//	├──────────────┬───────────────────────────┤
//	│ Dashboard    │                           │
//	│ Emails       │                           │
//	│ Rules        │     <active view>         │
//	│ Accounts     │                           │
//	│ Watch        │                           │
//	│ Tools        │                           │
//	└──────────────┴───────────────────────────┘
//
// The sidebar is a fixed-width List; selecting an item swaps the right pane
// content. Account picker (Step 10) and real views (Phase 3+) plug in here.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// AppVersion is shown in the window title. Bumped per release in lockstep
// with cmd/email-read/main.go and cmd/email-read-ui/main.go.
const AppVersion = "0.20.0"

// Run creates the Fyne app, builds the main window, and blocks until the
// user closes it. Called from cmd/email-read-ui/main.go.
func Run() {
	a := app.NewWithID("dev.lovable.email-read")
	w := a.NewWindow("email-read · v" + AppVersion)
	w.SetContent(BuildShell())
	w.Resize(fyne.NewSize(1000, 680))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// BuildShell returns the root container: sidebar on the left, swapping
// detail pane on the right. Exported so tests (and Step 10 wiring) can
// build the same tree without spinning up an actual window.
func BuildShell() fyne.CanvasObject {
	detail := container.NewStack(viewFor(NavItems[0]))

	sidebar := NewSidebar(func(item NavItem) {
		detail.Objects = []fyne.CanvasObject{viewFor(item)}
		detail.Refresh()
	})

	// Split with a fixed sidebar width. HSplit ratio of ~0.18 puts the
	// sidebar at ~180px on a 1000px window — comfortable for the nav labels.
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
