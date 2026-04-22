package views

import (
	"testing"

	"github.com/lovable/email-read/internal/config"
)

func TestRuleFilters(t *testing.T) {
	cases := []struct {
		name string
		in   config.Rule
		want string
	}{
		{"none", config.Rule{}, "(none)"},
		{"from only", config.Rule{FromRegex: "x"}, "from"},
		{"subject only", config.Rule{SubjectRegex: "y"}, "subject"},
		{"body only", config.Rule{BodyRegex: "z"}, "body"},
		{"from+subject", config.Rule{FromRegex: "x", SubjectRegex: "y"}, "from · subject"},
		{"all three", config.Rule{FromRegex: "x", SubjectRegex: "y", BodyRegex: "z"}, "from · subject · body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RuleFilters(tc.in); got != tc.want {
				t.Errorf("RuleFilters(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEnabledLabel(t *testing.T) {
	if EnabledLabel(true) != "✓ on" {
		t.Errorf("on label drifted: %q", EnabledLabel(true))
	}
	if EnabledLabel(false) != "○ off" {
		t.Errorf("off label drifted: %q", EnabledLabel(false))
	}
}
