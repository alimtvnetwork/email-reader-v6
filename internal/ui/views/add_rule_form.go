// add_rule_form.go renders the Add Rule inline form: name, urlRegex,
// fromRegex (optional), subjectRegex (optional), bodyRegex (optional),
// enabled. Submit validates via ValidateRuleForm (pure Go, tested) then
// calls core.AddRule. Errors render inline; success shows a status banner
// noting whether an existing rule was replaced.
//
// Edit mode: when Initial is non-nil the form starts pre-filled with that
// rule and the Name entry is locked (name is the immutable key — AddRule
// uses upsert-by-name semantics, so renaming would create a new rule).
//
// Behind the !nofyne build tag because it imports the Fyne widget set.
//go:build !nofyne

package views

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
)

// AddRuleFormOptions wires the form to its side effects.
//
// **Phase 2.7 migration.** The old shape defaulted `Save` to the
// deprecated package-level `core.AddRule`. The new shape accepts a
// typed `*core.RulesService` (constructed once at app boot via
// `core.NewDefaultRulesService`). `Save` survives as an optional
// override used exclusively by tests to inject deterministic
// outcomes; when nil we delegate to `Service.Add`. When both Service
// and Save are nil we render an inline error banner instead of
// panicking on submit — keeps headless / partial-bootstrap previews
// safe. OnSaved fires after a successful save so the shell can
// refresh dependent views.
type AddRuleFormOptions struct {
	Service *core.RulesService // production seam — constructed in app bootstrap
	Save    func(in core.RuleInput) errtrace.Result[*core.AddRuleResult]
	OnSaved func()
	Initial *config.Rule // nil ⇒ Add mode; non-nil ⇒ Edit mode
}

// ruleFormEntries holds the six Fyne widgets that make up the Add Rule
// form. Grouping them keeps BuildAddRuleForm under the 15-statement limit
// (AC-PROJ-20).
type ruleFormEntries struct {
	name         *widget.Entry
	urlRegex     *widget.Entry
	fromRegex    *widget.Entry
	subjectRegex *widget.Entry
	bodyRegex    *widget.Entry
	enabled      *widget.Check
}

// BuildAddRuleForm returns the inline Add Rule form widget.
// In Edit mode (opts.Initial != nil) the same widget is used but
// pre-filled with the existing rule's values and the Name entry locked.
func BuildAddRuleForm(opts AddRuleFormOptions) fyne.CanvasObject {
	editing := opts.Initial != nil
	if opts.Save == nil && opts.Service != nil {
		// Bind the service's typed Add to the Save shape so downstream
		// submit/clear closures see one uniform seam.
		opts.Save = opts.Service.Add
	}
	e := newRuleFormEntries()
	status := newStatusLabel()
	form := buildRuleForm(e)
	if editing {
		applyInitialRuleToEntries(*opts.Initial, e)
	}
	clear := func() {
		resetRuleEntries(e)
		if editing {
			applyInitialRuleToEntries(*opts.Initial, e)
		}
	}
	submit := newRuleSubmitButton(opts, e, status, clear, editing)
	clearBtn := widget.NewButton(ruleClearLabel(editing), func() { clear(); status.SetText("") })
	return container.NewPadded(container.NewVBox(
		form,
		widget.NewSeparator(),
		container.NewHBox(submit, clearBtn),
		status,
	))
}

// newRuleFormEntries constructs the six entry widgets with their
// placeholders.
func newRuleFormEntries() *ruleFormEntries {
	name := widget.NewEntry()
	name.SetPlaceHolder("e.g. lovable-magic-link")
	urlRegex := widget.NewEntry()
	urlRegex.SetPlaceHolder(`required — e.g. https://app\.example\.com/auth\?token=\S+`)
	fromRegex := widget.NewEntry()
	fromRegex.SetPlaceHolder("optional — e.g. noreply@example\\.com")
	subjectRegex := widget.NewEntry()
	subjectRegex.SetPlaceHolder("optional — e.g. (?i)sign.?in")
	bodyRegex := widget.NewEntry()
	bodyRegex.SetPlaceHolder("optional — body must match this pattern")
	enabled := widget.NewCheck("Enabled", nil)
	enabled.SetChecked(true)
	return &ruleFormEntries{name, urlRegex, fromRegex, subjectRegex, bodyRegex, enabled}
}

// buildRuleForm composes the widget.Form rows from the six entries.
func buildRuleForm(e *ruleFormEntries) *widget.Form {
	return widget.NewForm(
		widget.NewFormItem("Name", e.name),
		widget.NewFormItem("URL regex", e.urlRegex),
		widget.NewFormItem("From regex", e.fromRegex),
		widget.NewFormItem("Subject regex", e.subjectRegex),
		widget.NewFormItem("Body regex", e.bodyRegex),
		widget.NewFormItem("Enabled", e.enabled),
	)
}

// resetRuleEntries clears every input back to its initial state.
func resetRuleEntries(e *ruleFormEntries) {
	e.name.SetText("")
	e.urlRegex.SetText("")
	e.fromRegex.SetText("")
	e.subjectRegex.SetText("")
	e.bodyRegex.SetText("")
	e.enabled.SetChecked(true)
}

// applyInitialRuleToEntries pre-fills the form widgets from an existing
// rule. Locks the Name entry (name is the immutable upsert key).
func applyInitialRuleToEntries(r config.Rule, e *ruleFormEntries) {
	e.name.SetText(r.Name)
	e.name.Disable()
	e.urlRegex.SetText(r.UrlRegex)
	e.fromRegex.SetText(r.FromRegex)
	e.subjectRegex.SetText(r.SubjectRegex)
	e.bodyRegex.SetText(r.BodyRegex)
	e.enabled.SetChecked(r.Enabled)
}

// newRuleSubmitButton wires the primary button: validate → call opts.Save
// → render status → run OnSaved hook on success.
func newRuleSubmitButton(opts AddRuleFormOptions, e *ruleFormEntries, status *widget.Label, clear func(), editing bool) *widget.Button {
	submit := widget.NewButton(ruleSubmitLabel(editing), func() {
		v := ValidateRuleForm(ruleFormInputFromEntries(e))
		if !v.Valid {
			status.SetText("⚠ " + strings.Join(v.Errors, " · "))
			return
		}
		if opts.Save == nil {
			// Degraded path: bootstrap didn't wire a *RulesService and
			// no test override was supplied. Surface the wiring gap
			// instead of nil-panicking on submit.
			status.SetText("⚠ Rules service not wired (no Service or Save injected)")
			return
		}
		r := opts.Save(ruleInputFromValid(v))
		if r.HasError() {
			status.SetText("⚠ Save failed: " + r.Error().Error())
			return
		}
		status.SetText(formatRuleSavedMessage(r.Value()))
		if !editing {
			clear()
		}
		if opts.OnSaved != nil {
			opts.OnSaved()
		}
	})
	submit.Importance = widget.HighImportance
	return submit
}

// ruleFormInputFromEntries pulls the current entry values into the
// validation input struct.
func ruleFormInputFromEntries(e *ruleFormEntries) RuleFormInput {
	return RuleFormInput{
		Name:         e.name.Text,
		UrlRegex:     e.urlRegex.Text,
		FromRegex:    e.fromRegex.Text,
		SubjectRegex: e.subjectRegex.Text,
		BodyRegex:    e.bodyRegex.Text,
		Enabled:      e.enabled.Checked,
	}
}

// ruleInputFromValid maps a validated form into core.RuleInput.
func ruleInputFromValid(v RuleFormResult) core.RuleInput {
	return core.RuleInput{
		Name:         v.Name,
		UrlRegex:     v.UrlRegex,
		FromRegex:    v.FromRegex,
		SubjectRegex: v.SubjectRegex,
		BodyRegex:    v.BodyRegex,
		Enabled:      v.Enabled,
	}
}

// formatRuleSavedMessage renders the success banner including whether an
// existing rule was replaced and the resulting Enabled state.
func formatRuleSavedMessage(res *core.AddRuleResult) string {
	verb := "Saved"
	if res.Replaced {
		verb = "Replaced"
	}
	state := "enabled"
	if !res.Rule.Enabled {
		state = "disabled"
	}
	return fmt.Sprintf("✓ %s %q (%s). Config: %s", verb, res.Rule.Name, state, res.ConfigPath)
}
