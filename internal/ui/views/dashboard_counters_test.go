// dashboard_counters_test.go — headless tests for the dashboard's
// live-counter helpers. Runs under `-tags nofyne`.
package views

import (
	"testing"
	"time"

	"github.com/lovable/email-read/internal/watcher"
)

func TestDashboardAcceptsEvent(t *testing.T) {
	cases := []struct {
		name     string
		eventA   string
		selected string
		want     bool
	}{
		{"all aliases accepts work", "work", "", true},
		{"all aliases accepts personal", "personal", "", true},
		{"selected work accepts work", "work", "work", true},
		{"selected work rejects personal", "personal", "work", false},
		{"selected work rejects empty", "", "work", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DashboardAcceptsEvent(watcher.Event{Alias: c.eventA}, c.selected)
			if got != c.want {
				t.Errorf("DashboardAcceptsEvent(%q, %q)=%v, want %v",
					c.eventA, c.selected, got, c.want)
			}
		})
	}
}

func TestFormatDashboardCounterTile(t *testing.T) {
	if got := FormatDashboardCounterTile("Polls", 0); got != "Polls: 0" {
		t.Errorf("got %q", got)
	}
	if got := FormatDashboardCounterTile("New mail", 42); got != "New mail: 42" {
		t.Errorf("got %q", got)
	}
}

func TestFormatDashboardCounterScope(t *testing.T) {
	if got := FormatDashboardCounterScope(DashboardCounterScope{}); got != "Live counters — all aliases" {
		t.Errorf("empty scope: %q", got)
	}
	if got := FormatDashboardCounterScope(DashboardCounterScope{Alias: "work"}); got != "Live counters — alias=work" {
		t.Errorf("alias scope: %q", got)
	}
}
