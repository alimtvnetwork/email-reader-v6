// errlog_notifier_test.go locks the 0→1 transition rule and the
// reset-after-MarkRead semantics that Phase 3.5 of the error-trace
// logging upgrade depends on. Headless — no fyne.
package ui

import (
	"sync"
	"testing"

	"github.com/lovable/email-read/internal/ui/errlog"
)

// recorder is a minimal ToastFn that captures every fired toast.
type recorder struct {
	mu     sync.Mutex
	titles []string
	bodies []string
}

func (r *recorder) toast(title, body string) {
	r.mu.Lock()
	r.titles = append(r.titles, title)
	r.bodies = append(r.bodies, body)
	r.mu.Unlock()
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.titles)
}

// runHandle drains entries through n.handle directly (bypasses the
// real Subscribe channel) so the test stays deterministic — no
// goroutine, no flake.
func runHandle(n *ErrLogNotifier, entries ...errlog.Entry) {
	for _, e := range entries {
		n.handle(e)
	}
}

func TestNotifier_FirstErrorFiresToast(t *testing.T) {
	r := &recorder{}
	n := &ErrLogNotifier{toast: r.toast}
	runHandle(n, errlog.Entry{Component: "emails", Summary: "open failed"})
	if r.count() != 1 {
		t.Fatalf("toasts fired = %d, want 1", r.count())
	}
}

func TestNotifier_StormIsBadgeOnly(t *testing.T) {
	r := &recorder{}
	n := &ErrLogNotifier{toast: r.toast}
	runHandle(n,
		errlog.Entry{Component: "emails", Summary: "first"},
		errlog.Entry{Component: "emails", Summary: "second"},
		errlog.Entry{Component: "rules", Summary: "third"},
	)
	if r.count() != 1 {
		t.Fatalf("toasts fired = %d, want exactly 1 (storm collapses to one)", r.count())
	}
}

func TestNotifier_ResetReopensToastWindow(t *testing.T) {
	r := &recorder{}
	n := &ErrLogNotifier{toast: r.toast}
	runHandle(n, errlog.Entry{Component: "emails", Summary: "first"})
	n.ResetQuietPeriod()
	runHandle(n, errlog.Entry{Component: "watcher", Summary: "second"})
	if r.count() != 2 {
		t.Fatalf("toasts fired = %d, want 2 (reset reopens window)", r.count())
	}
}

func TestNotifier_NilToastIsNoopButTracksState(t *testing.T) {
	// nil toast must not panic; storm flag must still flip so a
	// later test injection of toast wouldn't double-fire.
	n := &ErrLogNotifier{toast: nil}
	runHandle(n, errlog.Entry{Component: "x", Summary: "y"})
	n.mu.Lock()
	stormed := n.inStorm
	n.mu.Unlock()
	if !stormed {
		t.Fatalf("inStorm = false after handle, want true")
	}
}

func TestToastTitle_FallbackWhenComponentEmpty(t *testing.T) {
	if got := toastTitle(errlog.Entry{Component: ""}); got != "email-read · Error" {
		t.Errorf("toastTitle(empty)=%q", got)
	}
	if got := toastTitle(errlog.Entry{Component: "emails"}); got != "email-read · Error in emails" {
		t.Errorf("toastTitle(emails)=%q", got)
	}
}

func TestToastBody_TruncatesLongSummary(t *testing.T) {
	long := ""
	for i := 0; i < 500; i++ {
		long += "x"
	}
	body := toastBody(errlog.Entry{Summary: long})
	// 140 cap on the summary part + the trailing call-to-action.
	if !contains(body, "…") {
		t.Errorf("expected ellipsis in long-summary toast body")
	}
	if !contains(body, "Open Diagnostics") {
		t.Errorf("missing call-to-action in body: %q", body)
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
