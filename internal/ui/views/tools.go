// tools.go renders the Tools tab: an AppTabs container holding one tab
// per inline mutating form. Phase 4 lands one form at a time; this file
// is the host that future steps extend.
//
// **Slice #116c (Phase 6.3)** added `ToolsFactory` to `ToolsOptions`.
// The four sub-tabs that need a `*core.Tools` instance (OpenUrl, Read,
// Export, Recent opens) now consume the factory from the `*Services`
// bundle instead of calling `config.Load()` inline. This closes the
// remaining Tools-route entries in the AST guard's
// `viewLayerGlobalsAllowlist` (`tools_openurl.go`, `tools_read.go`).
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
//go:build !nofyne

package views

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
)

// ToolsFactory is the per-call `*core.Tools` builder injected from the
// shell's `*Services` bundle. Returning a fresh instance each call
// preserves the live-config-edit semantics from Phase 4. Nil-safe at
// the sub-tab layer — when nil, each sub-tab renders the documented
// degraded-path status line instead of panicking.
type ToolsFactory = func() (*core.Tools, error)

// ToolsOptions wires shared callbacks. OnAccountsChanged fires after the
// Add Account form saves so the shell can refresh the sidebar picker.
// OnRulesChanged fires after the Add Rule form saves.
//
// **Phase 2.7 migration.** `RulesService` is the typed seam used by
// the embedded Add Rule form (constructed via `core.NewDefaultRulesService`
// in app bootstrap). Nil-safe: when absent the form falls back to its
// own degraded-path message.
//
// **Slice #116c (Phase 6.3) addition.** `ToolsFactory` is the typed
// seam consumed by OpenUrl / Read / Export / Recent opens. Threaded
// through from `services.Tools` in `app.go::viewFor`.
type ToolsOptions struct {
	OnAccountsChanged func()
	OnRulesChanged    func()
	RulesService      *core.RulesService
	ToolsFactory      ToolsFactory
}

// BuildTools returns the Tools view with one tab per form / sub-tool.
// As of Slice #116c the four `*core.Tools`-consuming tabs receive
// their factory through the parent options bundle.
func BuildTools(opts ToolsOptions) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle("Tools", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Mutating actions and read-only diagnostics. Each tab runs inline — no modal popups.")

	tabs := container.NewAppTabs(
		container.NewTabItem("Add account", BuildAddAccountForm(AddAccountFormOptions{
			OnSaved: opts.OnAccountsChanged,
		})),
		container.NewTabItem("Add rule", BuildAddRuleForm(AddRuleFormOptions{
			Service: opts.RulesService,
			OnSaved: opts.OnRulesChanged,
		})),
		container.NewTabItem("Doctor", BuildDoctorTab()),
		container.NewTabItem("Diagnose", BuildDiagnoseTab()),
		container.NewTabItem("Read", BuildReadTab(opts.ToolsFactory)),
		container.NewTabItem("Export CSV", BuildExportTab(opts.ToolsFactory)),
		container.NewTabItem("OpenUrl", BuildOpenUrlTab(opts.ToolsFactory)),
		container.NewTabItem("Recent opens", BuildRecentOpensTab(opts.ToolsFactory)),
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
