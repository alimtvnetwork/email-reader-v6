// add_rule_form.go renders the Add Rule inline form: name, urlRegex,
// fromRegex (optional), subjectRegex (optional), bodyRegex (optional),
// enabled. Submit validates via ValidateRuleForm (pure Go, tested) then
// calls core.AddRule. Errors render inline; success shows a status banner
// noting whether an existing rule was replaced.
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

	"github.com/lovable/email-read/internal/core"
)

// AddRuleFormOptions wires the form to its side effects. Save defaults to
// core.AddRule; tests inject a stub. OnSaved fires after a successful save
// so the shell can refresh dependent views (Rules tab counts, etc.).
type AddRuleFormOptions struct {
	Save    func(in core.RuleInput) (*core.AddRuleResult, error)
	OnSaved func()
}

// BuildAddRuleForm returns the inline Add Rule form widget.
func BuildAddRuleForm(opts AddRuleFormOptions) fyne.CanvasObject {
	if opts.Save == nil {
		opts.Save = core.AddRule
	}

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

	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Name", name),
		widget.NewFormItem("URL regex", urlRegex),
		widget.NewFormItem("From regex", fromRegex),
		widget.NewFormItem("Subject regex", subjectRegex),
		widget.NewFormItem("Body regex", bodyRegex),
		widget.NewFormItem("Enabled", enabled),
	)

	clear := func() {
		name.SetText("")
		urlRegex.SetText("")
		fromRegex.SetText("")
		subjectRegex.SetText("")
		bodyRegex.SetText("")
		enabled.SetChecked(true)
	}

	submit := widget.NewButton("Save rule", func() {
		v := ValidateRuleForm(RuleFormInput{
			Name:         name.Text,
			UrlRegex:     urlRegex.Text,
			FromRegex:    fromRegex.Text,
			SubjectRegex: subjectRegex.Text,
			BodyRegex:    bodyRegex.Text,
			Enabled:      enabled.Checked,
		})
		if !v.Valid {
			status.SetText("⚠ " + strings.Join(v.Errors, " · "))
			return
		}
		res, err := opts.Save(core.RuleInput{
			Name:         v.Name,
			UrlRegex:     v.UrlRegex,
			FromRegex:    v.FromRegex,
			SubjectRegex: v.SubjectRegex,
			BodyRegex:    v.BodyRegex,
			Enabled:      v.Enabled,
		})
		if err != nil {
			status.SetText("⚠ Save failed: " + err.Error())
			return
		}
		verb := "Saved"
		if res.Replaced {
			verb = "Replaced"
		}
		state := "enabled"
		if !res.Rule.Enabled {
			state = "disabled"
		}
		status.SetText(fmt.Sprintf("✓ %s %q (%s). Config: %s",
			verb, res.Rule.Name, state, res.ConfigPath))
		clear()
		if opts.OnSaved != nil {
			opts.OnSaved()
		}
	})
	submit.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear", func() {
		clear()
		status.SetText("")
	})

	return container.NewPadded(container.NewVBox(
		form,
		widget.NewSeparator(),
		container.NewHBox(submit, clearBtn),
		status,
	))
}
