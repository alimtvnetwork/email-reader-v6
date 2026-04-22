// tools.go renders the Tools tab: an AppTabs container holding one tab per
// inline mutating form. Phase 4 lands one form at a time — this file is
// the host that future steps extend (add-rule, remove, read, export,
// diagnose). Each tab is a self-contained widget so failures in one form
// can't break the others.
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
//go:build !nofyne

package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ToolsOptions wires shared callbacks. Currently only OnAccountsChanged
// (fired after add/remove) so the shell can refresh the sidebar account
// picker without each form needing direct access to AppState.
type ToolsOptions struct {
	OnAccountsChanged func()
}

// BuildTools returns the Tools view: tabs across the top, the active form
// in the body. New forms in Steps 16–20 simply append a new tab.
func BuildTools(opts ToolsOptions) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Tools", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Mutating actions. Each form runs inline — no modal popups.")

	tabs := container.NewAppTabs(
		container.NewTabItem("Add account", BuildAddAccountForm(AddAccountFormOptions{
			OnSaved: opts.OnAccountsChanged,
		})),
		container.NewTabItem("Add rule", placeholderTab("Add rule form lands in Step 16.")),
		container.NewTabItem("Remove", placeholderTab("Remove account / rule lands in Step 17.")),
		container.NewTabItem("Read", placeholderTab("One-shot fetch form lands in Step 18.")),
		container.NewTabItem("Export CSV", placeholderTab("CSV export form lands in Step 19.")),
		container.NewTabItem("Diagnose", placeholderTab("Connection diagnose form lands in Step 20.")),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		nil, nil, nil,
		tabs,
	)
}

// placeholderTab is the temporary "coming in Step N" body used by Tools
// tabs that don't have a real form yet. Cleared as Phase 4 lands.
func placeholderTab(msg string) fyne.CanvasObject {
	l := widget.NewLabel(msg)
	l.Wrapping = fyne.TextWrapWord
	return container.NewPadded(l)
}
