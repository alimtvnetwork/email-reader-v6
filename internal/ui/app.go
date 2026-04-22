// app.go ties the sidebar (sidebar.go) and detail pane together. Like
// sidebar.go, this file requires the Fyne cgo backend; gate it off with
// `-tags nofyne` to compile/test the rest of the package on headless CI.
//go:build !nofyne

// Package ui hosts the Fyne desktop frontend. It is intentionally split
// from cmd/email-read-ui so internal/ui can be unit-tested with `go test`
// without needing the cgo display libs that linking the binary requires.
package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/ui/views"
)

// AppVersion is shown in the window title. Bumped per release in lockstep
// with cmd/email-read/main.go.
const AppVersion = "0.26.0"

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
// logged (non-fatal) so the UI still opens with an empty picker.
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
	detail := container.NewStack()
	root := container.NewStack() // we swap the whole shell when accounts change

	// rebuildSidebar rebuilds the entire shell with a fresh aliases list —
	// used after the Add Account form saves so the picker reflects truth.
	var rebuildShell func()
	// rebuildDetail swaps the detail pane to match the current state.Nav().
	var rebuildDetail func()
	gotoNav := func(k NavKind) {
		state.SetNav(k)
		rebuildDetail()
	}
	rebuildDetail = func() {
		for _, it := range NavItems {
			if it.Kind == state.Nav() {
				detail.Objects = []fyne.CanvasObject{viewFor(it, state, gotoNav, rebuildShell)}
				detail.Refresh()
				return
			}
		}
	}

	rebuildShell = func() {
		freshAliases := LoadAliases()
		sidebar := NewSidebar(SidebarOptions{
			State:       state,
			Aliases:     freshAliases,
			OnSelectNav: func(item NavItem) { rebuildDetail() },
		})
		rebuildDetail()
		split := container.NewHSplit(sidebar, container.NewPadded(detail))
		split.SetOffset(0.20)
		root.Objects = []fyne.CanvasObject{split}
		root.Refresh()
	}

	// Re-render the active view if the alias changes so views always reflect
	// the currently selected account.
	state.Subscribe(func(ev AppStateEvent) {
		if ev.PrevAlias != ev.Alias {
			rebuildDetail()
		}
	})

	// Initial build using the aliases passed in (avoids double-loading).
	sidebar := NewSidebar(SidebarOptions{
		State:       state,
		Aliases:     aliases,
		OnSelectNav: func(item NavItem) { rebuildDetail() },
	})
	rebuildDetail()
	split := container.NewHSplit(sidebar, container.NewPadded(detail))
	split.SetOffset(0.20)
	root.Objects = []fyne.CanvasObject{split}
	return root
}

// viewFor returns the widget for a nav destination. Each case picks a real
// view from internal/ui/views or falls back to a placeholder for nav items
// not yet implemented.
func viewFor(item NavItem, state *AppState, gotoNav func(NavKind), onAccountsChanged func()) fyne.CanvasObject {
	switch item.Kind {
	case NavDashboard:
		return views.BuildDashboard(views.DashboardOptions{
			Alias:        state.Alias(),
			OnStartWatch: func() { gotoNav(NavWatch) },
		})
	case NavEmails:
		return views.BuildEmails(views.EmailsOptions{Alias: state.Alias()})
	case NavRules:
		return views.BuildRules(views.RulesOptions{})
	case NavAccounts:
		return views.BuildAccounts(views.AccountsOptions{})
	case NavTools:
		return views.BuildTools(views.ToolsOptions{
			OnAccountsChanged: onAccountsChanged,
		})
	default:
		return placeholderView(item, state)
	}
}

// placeholderView renders the temporary "coming in Step N" content for nav
// items that don't have a real widget yet (only NavWatch as of v0.26.0).
func placeholderView(item NavItem, state *AppState) fyne.CanvasObject {
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
