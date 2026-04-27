// error_log_test.go covers the headless-safe helpers used by the
// Diagnostics → Error Log view (formatRow, loadEntriesNewestFirst,
// truncate). NB: these helpers live in error_log.go which is
// !nofyne-gated, so the test inherits that constraint via the
// package's build matrix — but the helpers themselves only touch
// fmt/sort/time, no OpenGL needed at runtime.
//go:build !nofyne

package views

import (
	"testing"
	"time"

	"github.com/lovable/email-read/internal/ui/errlog"
)

func TestLoadEntriesNewestFirst_SortsBySeqDesc(t *testing.T) {
	src := func() []errlog.Entry {
		return []errlog.Entry{
			{Seq: 1, Component: "a"},
			{Seq: 3, Component: "c"},
			{Seq: 2, Component: "b"},
		}
	}
	got := loadEntriesNewestFirst(src)
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}
	if got[0].Seq != 3 || got[1].Seq != 2 || got[2].Seq != 1 {
		t.Fatalf("sort order = [%d %d %d], want [3 2 1]", got[0].Seq, got[1].Seq, got[2].Seq)
	}
}

func TestFormatRow_ShortSummary(t *testing.T) {
	e := errlog.Entry{
		Timestamp: time.Date(2026, 4, 27, 12, 34, 56, 0, time.UTC),
		Component: "emails",
		Summary:   "open failed",
	}
	got := formatRow(e)
	// Format uses local time; just assert the component + summary
	// land in the line — the timestamp prefix is locale-dependent.
	if !contains(got, "emails") || !contains(got, "open failed") {
		t.Fatalf("formatRow=%q missing component/summary", got)
	}
}

func TestFormatRow_TruncatesLongSummary(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	e := errlog.Entry{Component: "watch", Summary: long}
	got := formatRow(e)
	if len(got) > 120 { // 80 + prefix + safety
		t.Fatalf("expected truncated row, got len=%d", len(got))
	}
	if !contains(got, "…") {
		t.Fatalf("expected ellipsis in truncated row, got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string changed: %q", got)
	}
	if got := truncate("abcdefghij", 5); got != "abcd…" {
		t.Errorf("truncate(10,5) = %q, want %q", got, "abcd…")
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

func TestOpenLogFile_EmptyPath(t *testing.T) {
	if got := openLogFile("", func(string) error { return nil }); got != "Disk log unavailable." {
		t.Errorf("empty path = %q, want %q", got, "Disk log unavailable.")
	}
}

func TestOpenLogFile_NilOpener(t *testing.T) {
	if got := openLogFile("/tmp/x.jsonl", nil); got != "Open handler not wired." {
		t.Errorf("nil opener = %q, want %q", got, "Open handler not wired.")
	}
}

func TestOpenLogFile_Success(t *testing.T) {
	var got string
	msg := openLogFile("/tmp/x.jsonl", func(p string) error {
		got = p
		return nil
	})
	if got != "/tmp/x.jsonl" {
		t.Errorf("opener received %q, want %q", got, "/tmp/x.jsonl")
	}
	if msg != "Opened /tmp/x.jsonl" {
		t.Errorf("status = %q, want %q", msg, "Opened /tmp/x.jsonl")
	}
}

func TestOpenLogFile_Failure(t *testing.T) {
	msg := openLogFile("/tmp/x.jsonl", func(string) error { return errBoom })
	if !contains(msg, "Open failed:") || !contains(msg, "boom") {
		t.Errorf("failure status = %q, want prefix 'Open failed:' and message 'boom'", msg)
	}
}

type sentinelErr string

func (e sentinelErr) Error() string { return string(e) }

var errBoom sentinelErr = "boom"
