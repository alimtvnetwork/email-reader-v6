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

func TestShouldRefreshDashboardOnEvent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	debounce := 750 * time.Millisecond
	newMail := watcher.Event{Kind: watcher.EventNewMail, Alias: "work"}

	cases := []struct {
		name        string
		ev          watcher.Event
		last        time.Time
		now         time.Time
		minInterval time.Duration
		wantRefresh bool
		wantLastEq  string // "now" or "last"
	}{
		{"new mail first time fires", newMail, time.Time{}, now, debounce, true, "now"},
		{"new mail within debounce skipped", newMail, now.Add(-200 * time.Millisecond), now, debounce, false, "last"},
		{"new mail just past debounce fires", newMail, now.Add(-800 * time.Millisecond), now, debounce, true, "now"},
		{"poll_ok never fires", watcher.Event{Kind: watcher.EventPollOK, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"rule_match never fires", watcher.Event{Kind: watcher.EventRuleMatch, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"heartbeat never fires", watcher.Event{Kind: watcher.EventHeartbeat, Alias: "work"}, now.Add(-10 * time.Second), now, debounce, false, "last"},
		{"zero debounce always fires for new mail", newMail, now.Add(-1 * time.Millisecond), now, 0, true, "now"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, next := ShouldRefreshDashboardOnEvent(c.ev, c.last, c.now, c.minInterval)
			if ok != c.wantRefresh {
				t.Errorf("refresh=%v, want %v", ok, c.wantRefresh)
			}
			want := c.last
			if c.wantLastEq == "now" {
				want = c.now
			}
			if !next.Equal(want) {
				t.Errorf("next=%v, want %v", next, want)
			}
		})
	}
}
