// sidebar_badge_test.go locks the unread-count badge format used in
// the sidebar (Phase 3.4 of the error-trace logging upgrade). Lives
// in package ui (no build tag) so headless CI runs it.
package ui

import "testing"

func TestFormatNavRowLabel_NoBadgeWhenZeroOrNegative(t *testing.T) {
	cases := []int64{0, -1, -100}
	for _, n := range cases {
		if got := formatNavRowLabel("Error Log", n); got != "Error Log" {
			t.Errorf("badge=%d: got %q, want %q", n, got, "Error Log")
		}
	}
}

func TestFormatNavRowLabel_ShowsCount(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{1, "Error Log  (1)"},
		{7, "Error Log  (7)"},
		{99, "Error Log  (99)"},
	}
	for _, c := range cases {
		if got := formatNavRowLabel("Error Log", c.n); got != c.want {
			t.Errorf("badge=%d: got %q, want %q", c.n, got, c.want)
		}
	}
}

func TestFormatNavRowLabel_CollapsesAbove99(t *testing.T) {
	cases := []int64{100, 500, 9999}
	for _, n := range cases {
		if got := formatNavRowLabel("Error Log", n); got != "Error Log  (99+)" {
			t.Errorf("badge=%d: got %q, want %q", n, got, "Error Log  (99+)")
		}
	}
}
