// rules.go renders the Rules tab: a scrollable table of configured rules
// with an enable/disable toggle per row. Toggles call core.SetRuleEnabled
// and re-load on success so the table always reflects on-disk truth.
//
// Behind the !nofyne build tag because it imports the Fyne widget set; the
// pure-Go formatting helpers live in rules_format.go for headless tests.
//go:build !nofyne

package views

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
)

// RulesOptions wires the rules view to its data + side effects. Both
// loader and toggler default to the real core implementations but can be
// replaced in tests.
type RulesOptions struct {
	List   func() ([]config.Rule, error)        // override for tests
	Toggle func(name string, enabled bool) error // override for tests
}

// BuildRules returns the Rules view. Initial load is synchronous (cheap —
// just config.json). Failures render inline so the rest of the UI stays
// usable.
func BuildRules(opts RulesOptions) fyne.CanvasObject {
	if opts.List == nil {
		opts.List = core.ListRules
	}
	if opts.Toggle == nil {
		opts.Toggle = core.SetRuleEnabled
	}

	heading := widget.NewLabelWithStyle("Rules", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Toggle to enable/disable. Add or remove via the Tools view (Phase 4).")
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	// Body container is rebuilt on every reload so toggles always reflect
	// the freshly-saved config.json (defends against drift if the user
	// edits the file by hand between toggles).
	body := container.NewVBox()

	var reload func()
	reload = func() {
		rules, err := opts.List()
		if err != nil {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("⚠ Failed to load rules: " + err.Error()),
			}
			body.Refresh()
			return
		}
		if len(rules) == 0 {
			body.Objects = []fyne.CanvasObject{
				widget.NewLabel("No rules configured yet. Add one from Tools → Add rule."),
			}
			body.Refresh()
			status.SetText("0 rules.")
			return
		}

		header := container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("URL pattern", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Filters", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Enabled", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		)

		rows := []fyne.CanvasObject{header, widget.NewSeparator()}
		enabledCount := 0
		for _, r := range rules {
			if r.Enabled {
				enabledCount++
			}
			rows = append(rows, ruleRow(r, opts.Toggle, status, reload))
		}
		body.Objects = rows
		body.Refresh()
		status.SetText(fmt.Sprintf("%d rule(s), %d enabled.", len(rules), enabledCount))
	}
	reload()

	refreshBtn := widget.NewButton("Refresh", reload)

	scroll := container.NewVScroll(body)
	return container.NewBorder(
		container.NewVBox(heading, subtitle, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), container.NewHBox(refreshBtn, status)),
		nil, nil,
		scroll,
	)
}

// ruleRow renders one row of the rules table: name, regex, filters, toggle.
// The Check widget owns the enabled state; on change it persists via
// opts.Toggle and triggers reload to re-sync from disk.
func ruleRow(r config.Rule, toggle func(string, bool) error, status *widget.Label, reload func()) fyne.CanvasObject {
	name := widget.NewLabel(r.Name)
	urlLbl := widget.NewLabel(r.UrlRegex)
	urlLbl.Wrapping = fyne.TextWrapBreak // long regexes shouldn't blow out the column
	filters := widget.NewLabel(RuleFilters(r))

	check := widget.NewCheck("", nil)
	check.SetChecked(r.Enabled)
	check.OnChanged = func(on bool) {
		if err := toggle(r.Name, on); err != nil {
			status.SetText("⚠ Toggle failed: " + err.Error())
			// Revert UI to last-known truth so the checkbox doesn't lie.
			check.OnChanged = nil
			check.SetChecked(r.Enabled)
			check.OnChanged = func(b bool) { _ = toggle(r.Name, b); reload() }
			return
		}
		state := "enabled"
		if !on {
			state = "disabled"
		}
		status.SetText(fmt.Sprintf("Rule %q %s.", r.Name, state))
		reload()
	}
	// Right-align the check in its grid cell.
	checkCell := container.NewHBox(layoutSpacer(), check)

	return container.NewGridWithColumns(4, name, urlLbl, filters, checkCell)
}

// layoutSpacer returns an expanding horizontal spacer so widgets pack to
// the right edge of their grid cell. Cheap helper — avoids pulling in
// container.NewBorder for a single trailing widget.
func layoutSpacer() fyne.CanvasObject {
	s := widget.NewLabel("")
	return s
}
