// watch_events_test.go covers the framework-agnostic projections
// from watcher.Event into UI-facing strings + counters. We test
// every kind branch explicitly so the day someone adds a new
// EventKind (e.g. "uidval_reset_v2") the missing-case shows up as a
// failing test rather than a silent default-line in the Raw log tab.
package views

import (
	"errors"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/mailclient"
	"github.com/lovable/email-read/internal/watcher"
)

func atTime(t *testing.T) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, "2026-04-26T12:34:56Z")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tt
}

// TestAccumulateCounters_FoldsEachKind: the counter mapping is the
// only place that knows which kinds bump which fields. A tablular
// test guards each branch.
func TestAccumulateCounters_FoldsEachKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ev   watcher.Event
		want WatchCounters
	}{
		{"poll_ok", watcher.Event{Kind: watcher.EventPollOK}, WatchCounters{Polls: 1}},
		{"baseline", watcher.Event{Kind: watcher.EventBaseline}, WatchCounters{Polls: 1}},
		{"heartbeat", watcher.Event{Kind: watcher.EventHeartbeat}, WatchCounters{Polls: 1}},
		{"new_mail", watcher.Event{Kind: watcher.EventNewMail}, WatchCounters{NewMail: 1}},
		{"rule_match", watcher.Event{Kind: watcher.EventRuleMatch}, WatchCounters{Matches: 1}},
		{"poll_error", watcher.Event{Kind: watcher.EventPollError}, WatchCounters{Errors: 1}},
		{"url_opened ok", watcher.Event{Kind: watcher.EventUrlOpened, OpenOK: true}, WatchCounters{Opens: 1}},
		{"url_opened fail", watcher.Event{Kind: watcher.EventUrlOpened, OpenOK: false}, WatchCounters{OpenFail: 1}},
		{"started/stopped do not count", watcher.Event{Kind: watcher.EventStarted}, WatchCounters{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AccumulateCounters(WatchCounters{}, tc.ev)
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestFormatCounters_StableOrder: the footer string must keep a
// fixed key order so the UI width does not jiggle as numbers grow.
func TestFormatCounters_StableOrder(t *testing.T) {
	t.Parallel()
	c := WatchCounters{Polls: 12, NewMail: 3, Matches: 1, Opens: 1, Errors: 0, OpenFail: 0}
	want := "polls=12 · newMail=3 · matches=1 · opens=1 · errors=0"
	if got := c.FormatCounters(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestFormatRawLogLine_KindCoverage: every EventKind must produce a
// non-empty, non-"unknown" line. Locks the exhaustive switch.
func TestFormatRawLogLine_KindCoverage(t *testing.T) {
	t.Parallel()
	at := atTime(t)
	stats := &mailclient.MailboxStats{Messages: 100, UidNext: 200, Unseen: 5}
	msg := &mailclient.Message{From: "alice@x", Subject: "hi"}
	cases := []watcher.Event{
		{Kind: watcher.EventStarted, At: at, Alias: "a"},
		{Kind: watcher.EventStopped, At: at, Alias: "a"},
		{Kind: watcher.EventBaseline, At: at, Alias: "a", Stats: stats},
		{Kind: watcher.EventPollOK, At: at, Alias: "a", Stats: stats},
		{Kind: watcher.EventPollError, At: at, Alias: "a", Err: errors.New("dial: refused")},
		{Kind: watcher.EventNewMail, At: at, Alias: "a", Message: msg},
		{Kind: watcher.EventRuleMatch, At: at, Alias: "a", RuleName: "verify", Url: "https://x/y"},
		{Kind: watcher.EventUrlOpened, At: at, Alias: "a", Url: "https://x/y", OpenOK: true},
		{Kind: watcher.EventUrlOpened, At: at, Alias: "a", Url: "https://x/y", OpenOK: false, Err: errors.New("no chrome")},
		{Kind: watcher.EventHeartbeat, At: at, Alias: "a", Stats: stats},
		{Kind: watcher.EventUidValReset, At: at, Alias: "a", Stats: stats},
	}
	for _, ev := range cases {
		t.Run(string(ev.Kind), func(t *testing.T) {
			line := FormatRawLogLine(ev)
			if line == "" {
				t.Fatalf("empty line")
			}
			if want := "12:34:56"; line[:len(want)] != want {
				t.Fatalf("missing timestamp prefix: %q", line)
			}
			if line[len(line)-len(" unknown event "):] == " unknown event " {
				t.Fatalf("unexpected unknown-event fallback for kind %q", ev.Kind)
			}
		})
	}
}

// TestFormatRawLogLine_UnknownKindFallback: the default branch must
// still produce a parseable line so a future producer's typo is
// surfaced rather than silently swallowed.
func TestFormatRawLogLine_UnknownKindFallback(t *testing.T) {
	t.Parallel()
	line := FormatRawLogLine(watcher.Event{Kind: "weird", At: atTime(t), Alias: "a"})
	if line == "" || !contains(line, "unknown event") {
		t.Fatalf("default branch did not flag unknown kind: %q", line)
	}
}

func TestFormatRawLogLine_MailTimeoutAddsReachabilityRCA(t *testing.T) {
	t.Parallel()
	err := errtrace.NewCoded(errtrace.ErrMailTimeout, "imap dial timed out")
	line := FormatRawLogLine(watcher.Event{Kind: watcher.EventPollError, At: atTime(t), Alias: "admin", Err: err})
	for _, want := range []string{"TCP never reached IMAP login", "993/143", "open IMAP/Dovecot/firewall"} {
		if !contains(line, want) {
			t.Fatalf("timeout line missing %q: %q", want, line)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestEventToCard_FiltersNoise: only user-meaningful kinds become
// cards. Heartbeats, baseline, ok-polls, started/stopped MUST NOT
// emit cards (the header tracks lifecycle; the Raw log keeps polls).
func TestEventToCard_FiltersNoise(t *testing.T) {
	t.Parallel()
	noise := []watcher.EventKind{
		watcher.EventStarted, watcher.EventStopped, watcher.EventBaseline,
		watcher.EventPollOK, watcher.EventHeartbeat,
	}
	for _, k := range noise {
		t.Run(string(k), func(t *testing.T) {
			if _, ok := EventToCard(watcher.Event{Kind: k, At: atTime(t), Alias: "a"}); ok {
				t.Fatalf("kind %q must not produce a card", k)
			}
		})
	}
}

// TestEventToCard_CardKinds: meaningful kinds DO produce cards with
// the right tone. The tone mapping is part of the visual contract.
func TestEventToCard_CardKinds(t *testing.T) {
	t.Parallel()
	at := atTime(t)
	msg := &mailclient.Message{From: "a@x", Subject: "hi"}
	cases := []struct {
		ev   watcher.Event
		tone CardTone
	}{
		{watcher.Event{Kind: watcher.EventNewMail, At: at, Alias: "a", Message: msg}, CardToneSuccess},
		{watcher.Event{Kind: watcher.EventRuleMatch, At: at, Alias: "a", RuleName: "r", Url: "https://x"}, CardToneInfo},
		{watcher.Event{Kind: watcher.EventUrlOpened, At: at, Alias: "a", Url: "https://x", OpenOK: true}, CardToneSuccess},
		{watcher.Event{Kind: watcher.EventUrlOpened, At: at, Alias: "a", Url: "https://x", OpenOK: false, Err: errors.New("x")}, CardToneError},
		{watcher.Event{Kind: watcher.EventPollError, At: at, Alias: "a", Err: errors.New("x")}, CardToneError},
		{watcher.Event{Kind: watcher.EventUidValReset, At: at, Alias: "a"}, CardToneWarn},
	}
	for _, tc := range cases {
		t.Run(string(tc.ev.Kind), func(t *testing.T) {
			card, ok := EventToCard(tc.ev)
			if !ok {
				t.Fatalf("expected a card for kind %q", tc.ev.Kind)
			}
			if card.Tone != tc.tone {
				t.Fatalf("tone: got %v, want %v", card.Tone, tc.tone)
			}
			if card.Title == "" || card.Body == "" {
				t.Fatalf("empty title/body: %+v", card)
			}
		})
	}
}

// TestEventToCard_NewMail_NilMessageDefensive: a contract violation
// upstream (nil Message on EventNewMail) must NOT panic the UI.
func TestEventToCard_NewMail_NilMessageDefensive(t *testing.T) {
	t.Parallel()
	card, ok := EventToCard(watcher.Event{Kind: watcher.EventNewMail, At: atTime(t), Alias: "a"})
	if !ok {
		t.Fatalf("expected a card even with nil Message")
	}
	if card.Body == "" {
		t.Fatalf("nil message should still produce a body placeholder")
	}
}

// TestAppendBounded_NewestFirst_TrimsToCap: the rolling buffer must
// prepend (newest first) and clamp at cap. Cap <= 0 is a no-op so
// callers can disable the buffer in tests without branching.
func TestAppendBounded_NewestFirst_TrimsToCap(t *testing.T) {
	t.Parallel()
	var buf []int
	for i := 1; i <= 5; i++ {
		buf = AppendBounded(buf, i, 3)
	}
	want := []int{5, 4, 3}
	if len(buf) != len(want) {
		t.Fatalf("len: got %d, want %d (%v)", len(buf), len(want), buf)
	}
	for i, v := range want {
		if buf[i] != v {
			t.Fatalf("buf[%d]: got %d, want %d (full %v)", i, buf[i], v, buf)
		}
	}
	if got := AppendBounded(buf, 99, 0); len(got) != len(buf) {
		t.Fatalf("cap=0 must be a no-op; got len %d, want %d", len(got), len(buf))
	}
}

// TestTruncURL_LongInputClipped: keeps the Raw log tidy when verify
// links push past 200 chars. The ellipsis is a 3-byte UTF-8 rune so
// the byte length is `max-1 + 3` = 92 when max=90.
func TestTruncURL_LongInputClipped(t *testing.T) {
	t.Parallel()
	long := "https://example.com/" + repeat("a", 200)
	got := truncURL(long)
	const wantBytes = 92 // 89 ASCII + 3-byte "…"
	if len(got) != wantBytes {
		t.Fatalf("len: got %d, want %d", len(got), wantBytes)
	}
	if got[len(got)-len("…"):] != "…" {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
