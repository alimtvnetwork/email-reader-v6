// errors_tail_test.go — unit tests for `email-read errors tail`.
//
// We test the pure helpers (`writeEntry`, `runErrorsTail` against a
// custom data dir) headlessly. The follow loop is exercised via a
// short-lived context that cancels after a tick or two — enough to
// confirm new entries arrive on stdout but quick enough to keep the
// suite fast.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/ui/errlog"
)

// withDataDir installs a test path resolver pointing at a fresh
// temp directory and returns the canonical error-log path inside it.
// Cleans up the override on test exit so other tests stay isolated.
func withDataDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "error-log.jsonl")
	prev := errLogPathResolver
	errLogPathResolver = func() (string, error) { return path, nil }
	t.Cleanup(func() { errLogPathResolver = prev })
	return path
}

// writeJSONL appends each entry as one JSON line. Mirrors what
// errlog.Persistence.Write does on disk so we can seed fixtures
// without spinning up the full Store.
func writeJSONL(t *testing.T, path string, entries []errlog.Entry) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
}

func TestWriteEntry_HeaderAndIndentedTrace(t *testing.T) {
	var buf bytes.Buffer
	writeEntry(&buf, errlog.Entry{
		Seq:       7,
		Timestamp: time.Date(2026, 4, 27, 10, 15, 3, 0, time.UTC),
		Component: "emails",
		Summary:   "boom",
		Trace:     "boom\n  at a.go:1 (f)\n  at b.go:2 (g)",
	})
	got := buf.String()
	if !strings.Contains(got, "[7] 2026-04-27T10:15:03Z  emails  boom") {
		t.Errorf("header missing/wrong:\n%s", got)
	}
	if !strings.Contains(got, "    boom\n") || !strings.Contains(got, "    at a.go:1 (f)\n") {
		t.Errorf("trace not indented:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("missing trailing blank separator:\n%q", got)
	}
}

func TestWriteEntry_SkipsTraceWhenSameAsSummary(t *testing.T) {
	var buf bytes.Buffer
	writeEntry(&buf, errlog.Entry{
		Seq: 1, Timestamp: time.Unix(0, 0).UTC(),
		Component: "x", Summary: "boom", Trace: "boom",
	})
	// Header + single blank line, no indented duplicate.
	if strings.Contains(buf.String(), "    boom") {
		t.Errorf("duplicate trace line not skipped:\n%s", buf.String())
	}
}

func TestWriteEntry_FallbackComponent(t *testing.T) {
	var buf bytes.Buffer
	writeEntry(&buf, errlog.Entry{Seq: 1, Summary: "s"})
	if !strings.Contains(buf.String(), "  -  s") {
		t.Errorf("expected '-' fallback for empty component, got:\n%s", buf.String())
	}
}

func TestRunErrorsTail_EmptyFile(t *testing.T) {
	path := withDataDir(t)
	// Don't create the file — exercises LoadFromFile's missing-file
	// branch.
	var buf bytes.Buffer
	if err := runErrorsTail(context.Background(), &buf, false, 0); err != nil {
		t.Fatalf("runErrorsTail: %v", err)
	}
	if !strings.Contains(buf.String(), "(no entries in") || !strings.Contains(buf.String(), path) {
		t.Errorf("expected friendly empty-state message naming %s, got:\n%s", path, buf.String())
	}
}

func TestRunErrorsTail_PrintsOldestFirst(t *testing.T) {
	path := withDataDir(t)
	writeJSONL(t, path, []errlog.Entry{
		{Seq: 1, Timestamp: time.Unix(100, 0).UTC(), Component: "a", Summary: "first", Trace: "first"},
		{Seq: 2, Timestamp: time.Unix(200, 0).UTC(), Component: "b", Summary: "second", Trace: "second"},
	})
	var buf bytes.Buffer
	if err := runErrorsTail(context.Background(), &buf, false, 0); err != nil {
		t.Fatalf("runErrorsTail: %v", err)
	}
	got := buf.String()
	idx1 := strings.Index(got, "first")
	idx2 := strings.Index(got, "second")
	if idx1 < 0 || idx2 < 0 || idx1 > idx2 {
		t.Errorf("expected first then second, got:\n%s", got)
	}
}

func TestRunErrorsTail_LinesFlagTrims(t *testing.T) {
	path := withDataDir(t)
	writeJSONL(t, path, []errlog.Entry{
		{Seq: 1, Summary: "old"},
		{Seq: 2, Summary: "mid"},
		{Seq: 3, Summary: "new"},
	})
	var buf bytes.Buffer
	if err := runErrorsTail(context.Background(), &buf, false, 1); err != nil {
		t.Fatalf("runErrorsTail: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "old") || strings.Contains(got, "mid") {
		t.Errorf("expected only newest entry, got:\n%s", got)
	}
	if !strings.Contains(got, "new") {
		t.Errorf("expected newest entry 'new' in output:\n%s", got)
	}
}

func TestRunErrorsTail_FollowPicksUpNewEntries(t *testing.T) {
	path := withDataDir(t)
	writeJSONL(t, path, []errlog.Entry{
		{Seq: 1, Summary: "seed"},
	})
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	// Append a new entry shortly after follow starts, then cancel.
	done := make(chan error, 1)
	go func() { done <- runErrorsTail(ctx, &buf, true, 0) }()

	// Give the goroutine time to print the seeded entry, then append.
	time.Sleep(200 * time.Millisecond)
	writeJSONL(t, path, []errlog.Entry{{Seq: 2, Summary: "live"}})

	// Wait at least one poll tick (1s) then cancel.
	time.Sleep(1300 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runErrorsTail: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runErrorsTail did not return after cancel")
	}
	got := buf.String()
	if !strings.Contains(got, "seed") {
		t.Errorf("expected seeded entry, got:\n%s", got)
	}
	if !strings.Contains(got, "live") {
		t.Errorf("expected live-appended entry, got:\n%s", got)
	}
}
