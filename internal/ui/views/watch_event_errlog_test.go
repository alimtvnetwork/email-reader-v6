package views

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/ui/errlog"
	"github.com/lovable/email-read/internal/watcher"
)

// snapshotAfter clears the singleton, runs fn, and returns whatever
// landed in the log. Tests in this file mutate the process-wide
// errlog singleton, so they must run sequentially (no t.Parallel).
// The singleton is fine to share across tests: Clear() resets it
// completely, and other test files that rely on the log assert their
// own contents after their own ReportError calls.
func snapshotAfter(t *testing.T, fn func()) []errlog.Entry {
	t.Helper()
	errlog.Clear()
	t.Cleanup(errlog.Clear)
	fn()
	return errlog.Snapshot()
}

// TestReportWatchEventError_PollErrorReachesErrlog locks the
// regression the user reported: a poll error visible in the Watch
// view's Raw log was NOT mirrored into Diagnostics → Error Log
// (data/error-log.jsonl) because subscribeWatchBus never called
// errlog.ReportError. The fix lives in ReportWatchEventError.
func TestReportWatchEventError_PollErrorReachesErrlog(t *testing.T) {
	wrapped := errtrace.Wrap(errors.New("dial: refused"), "watcher.poll")
	got := snapshotAfter(t, func() {
		ReportWatchEventError(watcher.Event{
			Kind:  watcher.EventPollError,
			At:    time.Now(),
			Alias: "atto",
			Err:   wrapped,
		})
	})
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Component != "watcher.atto" {
		t.Errorf("component: got %q want %q", got[0].Component, "watcher.atto")
	}
	if !strings.Contains(got[0].Summary, "dial: refused") {
		t.Errorf("summary lost cause: %q", got[0].Summary)
	}
	if !strings.Contains(got[0].Trace, "watcher.poll") {
		t.Errorf("trace lost wrap context: %q", got[0].Trace)
	}
}

// TestReportWatchEventError_OpenFailReachesErrlog locks that
// EventUrlOpened with OpenOK=false also flows into the error log so
// users debugging "the link didn't open" can copy the trace.
func TestReportWatchEventError_OpenFailReachesErrlog(t *testing.T) {
	got := snapshotAfter(t, func() {
		ReportWatchEventError(watcher.Event{
			Kind:   watcher.EventUrlOpened,
			At:     time.Now(),
			Alias:  "atto",
			Url:    "https://example.test",
			OpenOK: false,
			Err:    errors.New("exec: chrome not found"),
		})
	})
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Component != "watcher.openurl.atto" {
		t.Errorf("component: got %q", got[0].Component)
	}
}

// TestReportWatchEventError_OpenSuccessIgnored locks the negative
// path: a successful URL open carries no error and must not appear
// in the error log (otherwise we'd pollute it with every click).
func TestReportWatchEventError_OpenSuccessIgnored(t *testing.T) {
	got := snapshotAfter(t, func() {
		ReportWatchEventError(watcher.Event{
			Kind:   watcher.EventUrlOpened,
			At:     time.Now(),
			Alias:  "atto",
			OpenOK: true,
		})
	})
	if len(got) != 0 {
		t.Fatalf("success event leaked into errlog: %+v", got)
	}
}

// TestReportWatchEventError_NonErrorKindsIgnored locks that benign
// event kinds (heartbeat, baseline, new mail, etc.) do not feed the
// error log.
func TestReportWatchEventError_NonErrorKindsIgnored(t *testing.T) {
	got := snapshotAfter(t, func() {
		for _, k := range []watcher.EventKind{
			watcher.EventStarted, watcher.EventStopped, watcher.EventBaseline,
			watcher.EventPollOK, watcher.EventNewMail, watcher.EventRuleMatch,
			watcher.EventHeartbeat, watcher.EventUidValReset,
		} {
			ReportWatchEventError(watcher.Event{Kind: k, At: time.Now(), Alias: "atto"})
		}
	})
	if len(got) != 0 {
		t.Fatalf("non-error events leaked into errlog: %+v", got)
	}
}
