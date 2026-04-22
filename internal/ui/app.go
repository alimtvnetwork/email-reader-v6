// app.go ties the sidebar (sidebar.go) and detail pane together. Like
// sidebar.go, this file requires the Fyne cgo backend; gate it off with
// `-tags nofyne` to compile/test the rest of the package on headless CI.
//go:build !nofyne

// Package ui hosts the Fyne desktop frontend. It is intentionally split from
// cmd/email-read-ui so internal/ui can be unit-tested with `go test` without
// needing the cgo display libs that linking the binary requires.
package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// AppVersion is shown in the window title. Bumped per release in lockstep
// with cmd/email-read/main.go.
const AppVersion = "0.21.0"

// Run creates the Fyne app, builds the main window, and blocks until close.
func Run() {
	a := app.NewWithID("dev.lovable.email-read")
	w := a.NewWindow("email-read · v" + AppVersion)
	w.SetContent(BuildShell(LoadAliases()))
	w.Resize(fyne.NewSize(1000, 680))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// LoadAliases pulls the configured account aliases from core. Failures are
// logged (non-fatal) so the UI still opens with an empty picker — the user
// can add an account from the Tools view (Phase 4).
func LoadAliases() []string {
	accts, err := core.ListAccounts()
	if err != nil {
		log.Printf("ui: load accounts: %v", err)
		return nil
	}
	out := make([]string, 0, len(accts))
	for _, a := range accts {
		out = append(out, a.Alias)
	}
	return out
}

// BuildShell returns the root container: sidebar (with account picker) on
// the left, swapping detail pane on the right. AppState lives for the life
// of the shell so views built later can subscribe to alias/nav transitions.
func BuildShell(aliases []string) fyne.CanvasObject {
	state := NewAppState()
	detail := container.NewStack(viewFor(NavItems[0], state))

	sidebar := NewSidebar(SidebarOptions{
		State:   state,
		Aliases: aliases,
		OnSelectNav: func(item NavItem) {
			detail.Objects = []fyne.CanvasObject{viewFor(item, state)}
			detail.Refresh()
		},
	})

	// Re-render the active view if the alias changes — Phase 3 views key off
	// state.Alias(); this keeps them in sync without each view subscribing.
	state.Subscribe(func(ev AppStateEvent) {
		if ev.PrevAlias == ev.Alias {
			return
		}
		// Find the current nav item and rebuild its view.
		for _, it := range NavItems {
			if it.Kind == state.Nav() {
				detail.Objects = []fyne.CanvasObject{viewFor(it, state)}
				detail.Refresh()
				return
			}
		}
	})

	split := container.NewHSplit(sidebar, container.NewPadded(detail))
	split.SetOffset(0.20)
	return split
}

// viewFor returns the placeholder content for a nav item. Phase 3 replaces
// these with real Dashboard/Emails/Rules/Accounts/Watch/Tools widgets.
// The selected alias is shown in the placeholder so Step 10 wiring is
// visibly correct from the screen.
func viewFor(item NavItem, state *AppState) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(item.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	alias := "(none)"
	if state != nil && state.Alias() != "" {
		alias = state.Alias()
	}
	ctx := widget.NewLabel("Selected account: " + alias)
	body := widget.NewLabel(item.Placeholder)
	body.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), ctx, body)
}
