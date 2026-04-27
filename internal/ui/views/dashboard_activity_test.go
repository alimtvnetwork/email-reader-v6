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

// --- Slice #113 additions: per-row helpers used by widget.List ---

func TestFormatActivityRow_Succeeded(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 30, 0, 0, time.UTC)
	r := core.ActivityRow{OccurredAt: now, Alias: "work", Kind: core.ActivityPollSucceeded}
	got := formatActivityRow(r)
	want := "09:30:00  work  ✓ PollSucceeded"
	if got != want {
		t.Fatalf("succeeded row:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatActivityRow_FailedWithErrorCode(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 31, 15, 0, time.UTC)
	r := core.ActivityRow{
		OccurredAt: now, Alias: "home", Kind: core.ActivityPollFailed,
		Message: "boom", ErrorCode: 21104,
	}
	got := formatActivityRow(r)
	want := "09:31:15  home  ✗ PollFailed · boom (err 21104)"
	if got != want {
		t.Fatalf("failed row:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatActivityRow_OmitsEmptyAliasAndZeroErrorCode(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 32, 0, 0, time.UTC)
	r := core.ActivityRow{OccurredAt: now, Kind: core.ActivityPollStarted}
	got := formatActivityRow(r)
	want := "09:32:00  ▶ PollStarted"
	if got != want {
		t.Fatalf("minimal row:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatActivityRowDetail_IncludesUtcDateAndAllFields(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 33, 0, 0, time.UTC)
	r := core.ActivityRow{
		OccurredAt: now, Alias: "work", Kind: core.ActivityPollFailed,
		Message: "imap closed", ErrorCode: 21105,
	}
	got := formatActivityRowDetail(r)
	want := "2026-04-27 09:33:00 UTC · alias=work · kind=PollFailed · msg=imap closed · err=21105"
	if got != want {
		t.Fatalf("detail:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestFormatActivityRowDetail_OmitsEmptyFields(t *testing.T) {
	now := time.Date(2026, 4, 27, 9, 34, 0, 0, time.UTC)
	r := core.ActivityRow{OccurredAt: now, Kind: core.ActivityPollStarted}
	got := formatActivityRowDetail(r)
	want := "2026-04-27 09:34:00 UTC · kind=PollStarted"
	if got != want {
		t.Fatalf("minimal detail:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestActivityImportance_SeverityMapping(t *testing.T) {
	cases := []struct {
		in   core.ActivityKind
		want widget.Importance
	}{
		{core.ActivityPollFailed, widget.DangerImportance},
		{core.ActivityPollSucceeded, widget.SuccessImportance},
		{core.ActivityEmailStored, widget.SuccessImportance},
		{core.ActivityRuleMatched, widget.HighImportance},
		{core.ActivityPollStarted, widget.MediumImportance},
		{core.ActivityKind("FuturePeacock"), widget.MediumImportance},
	}
	for _, c := range cases {
		if got := activityImportance(c.in); got != c.want {
			t.Fatalf("kind=%q: got %v, want %v", c.in, got, c.want)
		}
	}
}
