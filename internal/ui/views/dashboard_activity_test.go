// dashboard_activity_test.go — Slice #105: recent-activity formatter
// + glyph table. Verifies the multi-line readout rendered into the
// activity label, including header, glyphs, message + ErrorCode
// suffixes, and the empty-input fallback.
//
//go:build !nofyne

package views

import (
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/core"
)

func TestFormatRecentActivity_Empty_NoActivityFallback(t *testing.T) {
	got := formatRecentActivity(nil)
	if !strings.Contains(got, "(no recent activity)") {
		t.Fatalf("empty: missing fallback line: %q", got)
	}
}

func TestFormatRecentActivity_HeaderAndRows(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 5, 30, 0, time.UTC)
	rows := []core.ActivityRow{
		{OccurredAt: now, Alias: "work", Kind: core.ActivityPollSucceeded},
		{OccurredAt: now.Add(-1 * time.Minute), Alias: "home",
			Kind: core.ActivityPollFailed, Message: "boom", ErrorCode: 21104},
	}
	got := formatRecentActivity(rows)
	if !strings.HasPrefix(got, "Recent activity:") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "10:05:30  work  ✓ PollSucceeded") {
		t.Fatalf("succeeded row mismatch: %q", got)
	}
	if !strings.Contains(got, "10:04:30  home  ✗ PollFailed · boom (err 21104)") {
		t.Fatalf("failed row mismatch: %q", got)
	}
}

func TestFormatRecentActivity_OmitsZeroErrorCodeAndEmptyMessage(t *testing.T) {
	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	rows := []core.ActivityRow{{OccurredAt: now, Alias: "a", Kind: core.ActivityPollStarted}}
	got := formatRecentActivity(rows)
	if strings.Contains(got, "(err") {
		t.Fatalf("zero ErrorCode leaked into output: %q", got)
	}
	if strings.Contains(got, " · ") {
		t.Fatalf("empty Message produced separator: %q", got)
	}
}

func TestActivityGlyph_Table(t *testing.T) {
	cases := []struct {
		in   core.ActivityKind
		want string
	}{
		{core.ActivityPollStarted, "▶"},
		{core.ActivityPollSucceeded, "✓"},
		{core.ActivityPollFailed, "✗"},
		{core.ActivityEmailStored, "✉"},
		{core.ActivityRuleMatched, "◆"},
		{core.ActivityKind("FuturePeacock"), "•"}, // unknown future kind
		{core.ActivityKind(""), "•"},              // zero value
	}
	for _, c := range cases {
		if got := activityGlyph(c.in); got != c.want {
			t.Fatalf("kind=%q: got %q, want %q", c.in, got, c.want)
		}
	}
}
