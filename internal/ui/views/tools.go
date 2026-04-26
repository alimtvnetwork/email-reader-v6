// tools.go renders the Tools tab: an AppTabs container holding one tab
// per inline mutating form. Phase 4 lands one form at a time; this file
// is the host that future steps extend.
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
//go:build !nofyne

package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ToolsOptions wires shared callbacks. OnAccountsChanged fires after the
// Add Account form saves so the shell can refresh the sidebar picker.
// OnRulesChanged fires after the Add Rule form saves.
type ToolsOptions struct {
	OnAccountsChanged func()
	OnRulesChanged    func()
}

// BuildTools returns the Tools view with one tab per form / sub-tool.
// As of 2026-04-26 it ships: Add account, Add rule, Doctor, Diagnose;
// Read / Export CSV / OpenUrl land with `core.Tools`.
func BuildTools(opts ToolsOptions) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Tools", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Mutating actions and read-only diagnostics. Each tab runs inline — no modal popups.")

	tabs := container.NewAppTabs(
		container.NewTabItem("Add account", BuildAddAccountForm(AddAccountFormOptions{
			OnSaved: opts.OnAccountsChanged,
		})),
		container.NewTabItem("Add rule", BuildAddRuleForm(AddRuleFormOptions{
			OnSaved: opts.OnRulesChanged,
		})),
		container.NewTabItem("Doctor", BuildDoctorTab()),
		container.NewTabItem("Diagnose", BuildDiagnoseTab()),
		container.NewTabItem("Read", BuildReadTab()),
		container.NewTabItem("Export CSV", BuildExportTab()),
		container.NewTabItem("OpenUrl", BuildOpenUrlTab()),
		container.NewTabItem("Recent opens", BuildRecentOpensTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		nil, nil, nil,
		tabs,
	)
}

// placeholderTab is the temporary "coming in Step N" body used by Tools
// tabs that don't have a real form yet.
func placeholderTab(msg string) fyne.CanvasObject {
	l := widget.NewLabel(msg)
	l.Wrapping = fyne.TextWrapWord
	return container.NewPadded(l)
}
