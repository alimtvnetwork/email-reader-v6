package views

import (
	"testing"

	"github.com/lovable/email-read/internal/core"
)

func TestFormatEmailsValue(t *testing.T) {
	cases := []struct {
		name string
		in   core.DashboardStats
		want string
	}{
		{"no alias", core.DashboardStats{EmailsTotal: 7}, "7"},
		{"alias subset", core.DashboardStats{Alias: "a", EmailsForAlias: 2, EmailsTotal: 9}, "2 (9 total)"},
		{"alias zero", core.DashboardStats{Alias: "a", EmailsForAlias: 0, EmailsTotal: 0}, "0 (0 total)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatEmailsValue(tc.in); got != tc.want {
				t.Errorf("FormatEmailsValue(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
