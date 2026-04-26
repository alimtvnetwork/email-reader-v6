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
	urlOpened := watcher.Event{Kind: watcher.EventUrlOpened, Alias: "work", Url: "https://x", OpenOK: true}
	urlOpenedFail := watcher.Event{Kind: watcher.EventUrlOpened, Alias: "work", OpenOK: false}

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
		{"url_opened first time fires", urlOpened, time.Time{}, now, debounce, true, "now"},
		{"url_opened within debounce skipped", urlOpened, now.Add(-100 * time.Millisecond), now, debounce, false, "last"},
		{"url_opened past debounce fires", urlOpened, now.Add(-1 * time.Second), now, debounce, true, "now"},
		{"url_opened failure still triggers refresh", urlOpenedFail, time.Time{}, now, debounce, true, "now"},
		{"new_mail then url_opened share debounce", urlOpened, now.Add(-50 * time.Millisecond), now, debounce, false, "last"},
		{"poll_ok never fires", watcher.Event{Kind: watcher.EventPollOK, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"rule_match never fires", watcher.Event{Kind: watcher.EventRuleMatch, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"heartbeat never fires", watcher.Event{Kind: watcher.EventHeartbeat, Alias: "work"}, now.Add(-10 * time.Second), now, debounce, false, "last"},
		{"poll_error never fires", watcher.Event{Kind: watcher.EventPollError, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"uidval_reset never fires", watcher.Event{Kind: watcher.EventUidValReset, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"started never fires", watcher.Event{Kind: watcher.EventStarted, Alias: "work"}, time.Time{}, now, debounce, false, "last"},
		{"zero debounce always fires for new mail", newMail, now.Add(-1 * time.Millisecond), now, 0, true, "now"},
		{"zero debounce always fires for url_opened", urlOpened, now.Add(-1 * time.Millisecond), now, 0, true, "now"},
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

// TestDashboardRefreshKind locks down the exact set of trigger kinds.
// Adding a kind here should be a deliberate, reviewed change — the
// dashboard runs SQL aggregates so over-triggering wastes cycles.
func TestDashboardRefreshKind(t *testing.T) {
	allKinds := []watcher.EventKind{
		watcher.EventStarted, watcher.EventBaseline, watcher.EventPollOK,
		watcher.EventPollError, watcher.EventNewMail, watcher.EventRuleMatch,
		watcher.EventUrlOpened, watcher.EventHeartbeat, watcher.EventStopped,
		watcher.EventUidValReset,
	}
	want := map[watcher.EventKind]bool{
		watcher.EventNewMail:   true,
		watcher.EventUrlOpened: true,
	}
	for _, k := range allKinds {
		got := dashboardRefreshKind(k)
		if got != want[k] {
			t.Errorf("dashboardRefreshKind(%q)=%v, want %v", k, got, want[k])
		}
	}
}
