package views

import (
	"strings"
	"testing"
)

func TestValidateRuleForm(t *testing.T) {
	cases := []struct {
		name   string
		in     RuleFormInput
		valid  bool
		errSub []string
	}{
		{
			name:  "happy minimal",
			in:    RuleFormInput{Name: "r", UrlRegex: `https?://.+`, Enabled: true},
			valid: true,
		},
		{
			name:  "happy with all filters",
			in:    RuleFormInput{Name: "r", UrlRegex: `https?://x`, FromRegex: "a@b", SubjectRegex: "(?i)hi", BodyRegex: "click"},
			valid: true,
		},
		{
			name:   "missing name",
			in:     RuleFormInput{UrlRegex: `https?://.+`},
			errSub: []string{"name is required"},
		},
		{
			name:   "missing url regex",
			in:     RuleFormInput{Name: "r"},
			errSub: []string{"urlRegex is required"},
		},
		{
			name:   "bad url regex",
			in:     RuleFormInput{Name: "r", UrlRegex: "[unterminated"},
			errSub: []string{"invalid urlRegex"},
		},
		{
			name:   "bad from regex",
			in:     RuleFormInput{Name: "r", UrlRegex: ".+", FromRegex: "(?P<>x)"},
			errSub: []string{"invalid fromRegex"},
		},
		{
			name:   "bad subject regex",
			in:     RuleFormInput{Name: "r", UrlRegex: ".+", SubjectRegex: "*"},
			errSub: []string{"invalid subjectRegex"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateRuleForm(tc.in)
			if got.Valid != tc.valid {
				t.Errorf("Valid = %v, want %v (errors=%v)", got.Valid, tc.valid, got.Errors)
			}
			for _, want := range tc.errSub {
				found := false
				for _, e := range got.Errors {
					if strings.Contains(e, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got %v", want, got.Errors)
				}
			}
		})
	}
}

func TestValidateRuleForm_Trim(t *testing.T) {
	got := ValidateRuleForm(RuleFormInput{
		Name:     "  r  ",
		UrlRegex: "  .+  ",
	})
	if !got.Valid {
		t.Fatalf("expected valid: %v", got.Errors)
	}
	if got.Name != "r" || got.UrlRegex != ".+" {
		t.Errorf("trim failed: %+v", got)
	}
}
