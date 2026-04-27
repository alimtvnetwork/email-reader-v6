//go:build !nofyne

package views

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// RulesOptions wires the rules list to data + actions.
//
// **Phase 2.7 migration.** The old shape defaulted `List`/`Toggle`/
// `Remove` to deprecated package-level wrappers (`core.ListRules`,
// `core.SetRuleEnabled`, `core.RemoveRule`). The new shape accepts a
// typed `*core.RulesService` (constructed once at app boot via
// `core.NewDefaultRulesService`). The three callbacks survive as
// optional test overrides; when nil we delegate to the service.
// When both Service and the override are nil we render a degraded
// view rather than panicking — keeps headless / partial-bootstrap
// previews safe.
type RulesOptions struct {
	Service *core.RulesService // production seam — constructed in app bootstrap
	List    func() errtrace.Result[[]config.Rule]
	Toggle  func(name string, enabled bool) errtrace.Result[struct{}]
	Remove  func(name string) errtrace.Result[struct{}]
	// Rename / Reorder are the Phase 5.6 additions. When nil and a
	// Service is wired, they default to the typed methods on
	// `*core.RulesService` (which return `Result[core.Unit]`, hence
	// the distinct generic instantiation from Toggle/Remove above).
	// When BOTH the override and Service are nil, the corresponding
	// row buttons are hidden — the rest of the view still works,
	// matching the "degraded but functional" contract documented on
	// the Toggle/Remove fallbacks.
	Rename  func(oldName, newName string) errtrace.Result[core.Unit]
	Reorder func(names []string) errtrace.Result[core.Unit]
	// OnRulesChanged fires after a successful Edit/Delete/Rename/Reorder
	// so the shell can refresh dependent views (Tools tab counts,
	// dashboard tile, etc.).
	OnRulesChanged func()
}

func BuildRules(opts RulesOptions) fyne.CanvasObject {
	if opts.Service != nil {
		if opts.List == nil {
			opts.List = opts.Service.List
		}
		if opts.Toggle == nil {
			opts.Toggle = opts.Service.SetEnabled
		}
		if opts.Remove == nil {
			opts.Remove = opts.Service.Remove
		}
		if opts.Rename == nil {
			opts.Rename = opts.Service.Rename
		}
		if opts.Reorder == nil {
			opts.Reorder = opts.Service.Reorder
		}
	}
	if opts.List == nil || opts.Toggle == nil || opts.Remove == nil {
		// Degraded path: bootstrap didn't wire a *RulesService and no
		// test overrides were supplied. Render an inline error panel
		// instead of nil-panicking on the first list call. Rename and
		// Reorder are *not* required for this gate — older callers
		// that wire only List/Toggle/Remove keep working without the
		// new row affordances.
		heading := widget.NewLabelWithStyle("Rules", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		return container.NewVBox(heading,
			widget.NewLabel("⚠ Rules service not wired (no Service or List/Toggle/Remove overrides injected)"))
	}

	heading := widget.NewLabelWithStyle("Rules", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Toggle to enable/disable, or Edit/Delete inline. Add via Tools → Add rule.")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	body := container.NewVBox()

	var reload func()
	reload = func() {
		res := opts.List()
		if res.HasError() {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load rules: " + res.Error().Error()),
			}
			body.Refresh()
			return
		}
		rules := res.Value()
		if len(rules) == 0 {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("No rules configured yet. Add one from Tools → Add rule."),
			}
			body.Refresh()
			status.SetText("0 rules.")
			return
		}

		header := container.NewGridWithColumns(5,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("URL pattern", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Filters", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Enabled", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Actions", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		)
		rows := []fyne.CanvasObject{header, widget.NewSeparator()}
		// Snapshot the current ordered name list once per reload so each
		// row's move-up/move-down handlers can build a permutation
		// without re-listing. Captured by closure into ruleRow below.
		orderedNames := make([]string, len(rules))
		for i, r := range rules {
			orderedNames[i] = r.Name
		}
		enabledCount := 0
		for i, r := range rules {
			if r.Enabled {
				enabledCount++
			}
			rows = append(rows, ruleRow(r, i, orderedNames, opts, status, reload))
		}
		body.Objects = rows
		body.Refresh()
		status.SetText(fmt.Sprintf("%d rule(s), %d enabled.", len(rules), enabledCount))
	}
	reload()

	refreshBtn := widget.NewButton("Refresh", reload)
	scroll := container.NewVScroll(body)
	// Border layout: refresh button pinned left, status label fills the
	// remaining horizontal width. NewHBox would give `status` zero
	// width and TextWrapWord would break each character onto its own
	// line — see accounts.go for the same fix.
	footer := container.NewBorder(nil, nil, refreshBtn, nil, status)
	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), footer),
		nil, nil,
		scroll,
	)
}

// ruleRow renders a single rule line. `index` is the rule's
// zero-based position in `orderedNames` (the full snapshot from the
// most recent reload) — both are needed to compose the permutation
// passed to `opts.Reorder` when the user clicks ↑/↓.
//
// The Rename + Reorder buttons are conditionally rendered: when the
// corresponding callback is nil (e.g. headless test wiring or a
// pre-Phase-5.6 caller), they're omitted from the row entirely
// rather than rendered-disabled. Disabled buttons would invite
// "why doesn't this do anything?" reports; absence is unambiguous.
func ruleRow(r config.Rule, index int, orderedNames []string, opts RulesOptions, status *widget.Label, reload func()) fyne.CanvasObject {
	name := widget.NewLabel(r.Name)
	urlLbl := widget.NewLabel(r.UrlRegex)
	urlLbl.Wrapping = fyne.TextWrapBreak
	filters := widget.NewLabel(RuleFilters(r))

	check := widget.NewCheck("", nil)
	check.SetChecked(r.Enabled)
	check.OnChanged = func(on bool) {
		if res := opts.Toggle(r.Name, on); res.HasError() {
			status.SetText("⚠ Toggle failed: " + res.Error().Error())
			check.OnChanged = nil
			check.SetChecked(r.Enabled)
			check.OnChanged = func(b bool) { _ = opts.Toggle(r.Name, b); reload() }
			return
		}
		state := "enabled"
		if !on {
			state = "disabled"
		}
		status.SetText(fmt.Sprintf("Rule %q %s.", r.Name, state))
		reload()
	}
	checkCell := container.NewHBox(widget.NewLabel(""), check)

	actionWidgets := []fyne.CanvasObject{}
	// Slice #114: drag-handle reorder. Replaces the previous ↑/↓
	// button pair with a single "⋮⋮" handle that implements
	// `fyne.Draggable`. The handle owns no table state — it just
	// reports the target index back into `moveRule`, which then
	// builds the permutation and calls `opts.Reorder`. Single-row
	// tables omit the handle entirely (no-op gesture wastes screen
	// real estate).
	if opts.Reorder != nil && len(orderedNames) > 1 {
		handle := newDragHandle(index, len(orderedNames), 0, func(target int) {
			moveRule(orderedNames, index, target, opts, status, reload)
		})
		actionWidgets = append(actionWidgets, handle)
	}
	// Rename button — only rendered when Rename is wired.
	if opts.Rename != nil {
		renameBtn := widget.NewButton("Rename", func() {
			openRenameRuleDialog(r, opts, status, reload)
		})
		actionWidgets = append(actionWidgets, renameBtn)
	}
	editBtn := widget.NewButton("Edit", func() { openEditRuleDialog(r, opts, status, reload) })
	delBtn := widget.NewButton("Delete", func() { confirmDeleteRule(r, opts, status, reload) })
	delBtn.Importance = widget.DangerImportance
	actionWidgets = append(actionWidgets, editBtn, delBtn)
	actions := container.NewHBox(actionWidgets...)

	return container.NewGridWithColumns(5, name, urlLbl, filters, checkCell, actions)
}

// moveRule swaps two positions in the ordered name list and pushes
// the resulting permutation through `opts.Reorder`. Builds a fresh
// slice (rather than mutating `orderedNames` in place) so the
// snapshot captured by sibling rows in this reload-cycle stays
// stable if the Reorder fails — a half-mutated snapshot would make
// the second click's permutation incoherent.
func moveRule(orderedNames []string, from, to int, opts RulesOptions, status *widget.Label, reload func()) {
	if from < 0 || to < 0 || from >= len(orderedNames) || to >= len(orderedNames) || from == to {
		return // defensive: button-render gates already prevent this
	}
	perm := make([]string, len(orderedNames))
	copy(perm, orderedNames)
	perm[from], perm[to] = perm[to], perm[from]
	if res := opts.Reorder(perm); res.HasError() {
		status.SetText("⚠ Reorder failed: " + res.Error().Error())
		return
	}
	status.SetText(fmt.Sprintf("Moved rule %q.", orderedNames[from]))
	if opts.OnRulesChanged != nil {
		opts.OnRulesChanged()
	}
	reload()
}

// openRenameRuleDialog prompts for a new name and pipes the result
// through `opts.Rename`. Reuses Fyne's `dialog.NewForm` for the
// input affordance — matches the existing confirm-style dialog
// idiom in this view (see confirmDeleteRule).
//
// Same-name (post-trim) is gated client-side BEFORE calling Rename:
// the core service returns `ErrRuleRenameNoop` for that case
// (intentional UI-bug surface), but we don't want to flash an error
// banner just because the user closed the dialog without changing
// anything. Empty input is similarly gated.
func openRenameRuleDialog(r config.Rule, opts RulesOptions, status *widget.Label, reload func()) {
	parent := currentParentWindow()
	if parent == nil {
		status.SetText("⚠ Cannot open Rename dialog: no parent window.")
		return
	}
	entry := widget.NewEntry()
	entry.SetText(r.Name)
	entry.Validator = func(s string) error {
		if len(s) == 0 {
			return errtrace.New("name required")
		}
		return nil
	}
	d := dialog.NewForm("Rename rule", "Rename", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("New name", entry)},
		func(ok bool) {
			if !ok {
				return
			}
			newName := entry.Text
			// Client-side no-op short-circuit (see func doc).
			if newName == "" || newName == r.Name {
				return
			}
			if res := opts.Rename(r.Name, newName); res.HasError() {
				status.SetText("⚠ Rename failed: " + res.Error().Error())
				return
			}
			status.SetText(fmt.Sprintf("✓ Renamed %q → %q", r.Name, newName))
			if opts.OnRulesChanged != nil {
				opts.OnRulesChanged()
			}
			reload()
		}, parent)
	d.Resize(fyne.NewSize(420, 180))
	d.Show()
}

// openEditRuleDialog shows the Add Rule form in edit mode inside a modal
// dialog. On successful Update the dialog closes and the table reloads
// via OnRulesChanged + the local reload.
func openEditRuleDialog(r config.Rule, opts RulesOptions, status *widget.Label, reload func()) {
	parent := currentParentWindow()
	if parent == nil {
		status.SetText("⚠ Cannot open Edit dialog: no parent window.")
		return
	}
	var d dialog.Dialog
	form := BuildAddRuleForm(AddRuleFormOptions{
		Service: opts.Service, // share the same *RulesService instance
		Initial: &r,
		OnSaved: func() {
			if d != nil {
				d.Hide()
			}
			if opts.OnRulesChanged != nil {
				opts.OnRulesChanged()
			}
			reload()
		},
	})
	d = dialog.NewCustom("Edit rule: "+r.Name, "Close", form, parent)
	d.Resize(fyne.NewSize(560, 440))
	d.Show()
}

// confirmDeleteRule shows a yes/no confirm before calling Remove.
func confirmDeleteRule(r config.Rule, opts RulesOptions, status *widget.Label, reload func()) {
	parent := currentParentWindow()
	if parent == nil {
		status.SetText("⚠ Cannot open Delete confirm: no parent window.")
		return
	}
	msg := fmt.Sprintf("Permanently remove rule %q? This cannot be undone.", r.Name)
	dialog.ShowConfirm("Delete rule", msg, func(yes bool) {
		if !yes {
			return
		}
		res := opts.Remove(r.Name)
		if res.HasError() {
			status.SetText("⚠ Delete failed: " + res.Error().Error())
			return
		}
		status.SetText("✓ Removed rule " + r.Name)
		if opts.OnRulesChanged != nil {
			opts.OnRulesChanged()
		}
		reload()
	}, parent)
}
