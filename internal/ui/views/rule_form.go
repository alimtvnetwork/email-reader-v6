// rule_form.go — pure-Go validator for the Add Rule form. Lives in the
// views package (no fyne imports here) so the validation rules are
// unit-testable on headless CI without cgo.
package views

import (
	"fmt"
	"regexp"
	"strings"
)

// RuleFormInput is the raw user input from the Add Rule form. All fields
// are strings because they come straight from Fyne Entry widgets.
type RuleFormInput struct {
	Name         string
	UrlRegex     string
	FromRegex    string
	SubjectRegex string
	BodyRegex    string
	Enabled      bool
}

// RuleFormResult holds the cleaned values plus a list of validation errors.
type RuleFormResult struct {
	Valid        bool
	Errors       []string
	Name         string
	UrlRegex     string
	FromRegex    string
	SubjectRegex string
	BodyRegex    string
	Enabled      bool
}

// ValidateRuleForm trims input, compiles every non-empty regex up-front so
// we never persist a broken rule, and reports every problem in one pass.
// Mirrors the validation in core.AddRule but runs client-side first so the
// form can highlight issues before hitting disk.
func ValidateRuleForm(in RuleFormInput) RuleFormResult {
	out := RuleFormResult{
		Name:         strings.TrimSpace(in.Name),
		UrlRegex:     strings.TrimSpace(in.UrlRegex),
		FromRegex:    strings.TrimSpace(in.FromRegex),
		SubjectRegex: strings.TrimSpace(in.SubjectRegex),
		BodyRegex:    strings.TrimSpace(in.BodyRegex),
		Enabled:      in.Enabled,
	}
	var errs []string

	if out.Name == "" {
		errs = append(errs, "name is required")
	}
	if out.UrlRegex == "" {
		errs = append(errs, "urlRegex is required")
	}

	for label, pat := range map[string]string{
		"urlRegex":     out.UrlRegex,
		"fromRegex":    out.FromRegex,
		"subjectRegex": out.SubjectRegex,
		"bodyRegex":    out.BodyRegex,
	} {
		if pat == "" {
			continue
		}
		if _, err := regexp.Compile(pat); err != nil {
			errs = append(errs, fmt.Sprintf("invalid %s: %s", label, err.Error()))
		}
	}

	out.Errors = errs
	out.Valid = len(errs) == 0
	return out
}
