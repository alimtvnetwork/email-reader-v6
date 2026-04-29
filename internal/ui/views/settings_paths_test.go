// settings_paths_test.go covers the pure-Go helpers behind the
// Slice #212 Settings path-panel redesign. No Fyne app is needed —
// the helpers take a `fyne.Clipboard` interface (we satisfy with a
// recorder) and plain `func(string) error` seams.
//go:build !nofyne

package views

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

// recordingClipboard is a fyne.Clipboard that just remembers the
// last SetContent payload. SatisfiesClipboard.
type recordingClipboard struct{ last string }

func (r *recordingClipboard) Content() string        { return r.last }
func (r *recordingClipboard) SetContent(s string)    { r.last = s }

var _ fyne.Clipboard = (*recordingClipboard)(nil)

func TestHandleCopyPath(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		if got := handleCopyPath("", &recordingClipboard{}, "Config"); !strings.Contains(got, "empty path") {
			t.Fatalf("want 'empty path' status, got %q", got)
		}
	})
	t.Run("nil clipboard", func(t *testing.T) {
		if got := handleCopyPath("/tmp/x", nil, "Config"); !strings.Contains(got, "not wired") {
			t.Fatalf("want 'not wired' status, got %q", got)
		}
	})
	t.Run("happy path", func(t *testing.T) {
		cb := &recordingClipboard{}
		got := handleCopyPath("/tmp/x", cb, "Data dir")
		if cb.last != "/tmp/x" {
			t.Fatalf("clipboard not written, got %q", cb.last)
		}
		if !strings.Contains(got, "Copied Data dir") {
			t.Fatalf("want 'Copied Data dir...' status, got %q", got)
		}
	})
}

func TestHandleOpenPath(t *testing.T) {
	t.Run("nil opener", func(t *testing.T) {
		if got := handleOpenPath("/tmp/x", nil, "Config"); !strings.Contains(got, "not wired") {
			t.Fatalf("want 'not wired', got %q", got)
		}
	})
	t.Run("opener error", func(t *testing.T) {
		got := handleOpenPath("/tmp/x", func(string) error { return errors.New("boom") }, "Config")
		if !strings.Contains(got, "open failed") || !strings.Contains(got, "boom") {
			t.Fatalf("want wrapped error, got %q", got)
		}
	})
	t.Run("happy path", func(t *testing.T) {
		var seen string
		got := handleOpenPath("/tmp/x", func(p string) error { seen = p; return nil }, "Email archive")
		if seen != "/tmp/x" {
			t.Fatalf("opener called with %q, want %q", seen, "/tmp/x")
		}
		if !strings.Contains(got, "Opened Email archive") {
			t.Fatalf("want 'Opened Email archive', got %q", got)
		}
	})
}

func TestHandleRevealPath(t *testing.T) {
	t.Run("nil reveal", func(t *testing.T) {
		if got := handleRevealPath("/tmp/x", nil, "Config"); !strings.Contains(got, "not wired") {
			t.Fatalf("want 'not wired', got %q", got)
		}
	})
	t.Run("reveal error", func(t *testing.T) {
		got := handleRevealPath("/tmp/x", func(string) error { return errors.New("nope") }, "Config")
		if !strings.Contains(got, "reveal failed") || !strings.Contains(got, "nope") {
			t.Fatalf("want wrapped error, got %q", got)
		}
	})
	t.Run("happy path", func(t *testing.T) {
		var seen string
		got := handleRevealPath("/tmp/cfg.json", func(p string) error { seen = p; return nil }, "Config")
		if seen != "/tmp/cfg.json" {
			t.Fatalf("reveal called with %q", seen)
		}
		if !strings.Contains(got, "Revealed Config") {
			t.Fatalf("want 'Revealed Config', got %q", got)
		}
	})
}