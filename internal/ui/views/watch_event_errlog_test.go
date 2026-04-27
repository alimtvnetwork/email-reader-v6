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

// TestReportWatchEventError_PollErrorReachesErrlog locks the
// regression the user reported: a poll error visible in the Watch
// view's Raw log was NOT mirrored into Diagnostics → Error Log
// (data/error-log.jsonl) because subscribeWatchBus never called
// errlog.ReportError. The fix lives in ReportWatchEventError.
func TestReportWatchEventError_PollErrorReachesErrlog(t *testing.T) {
	store := errlog.NewStore(errlog.Options{})
	prev := errlog.SetDefault(store)
	t.Cleanup(func() { errlog.SetDefault(prev) })

	wrapped := errtrace.Wrap(errors.New("dial: refused"), "watcher.poll")
	ReportWatchEventError(watcher.Event{
		Kind:  watcher.EventPollError,
		At:    time.Now(),
		Alias: "atto",
		Err:   wrapped,
	})

	got := store.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Component != "watcher.atto" {
		t.Errorf("component: got %q want %q", got[0].Component, "watcher.atto")
	}
	if !strings.Contains(got[0].Message, "dial: refused") {
		t.Errorf("message lost cause: %q", got[0].Message)
	}
}

// TestReportWatchEventError_OpenFailReachesErrlog locks that
// EventUrlOpened with OpenOK=false also flows into the error log so
// users debugging "the link didn't open" can copy the trace.
func TestReportWatchEventError_OpenFailReachesErrlog(t *testing.T) {
	store := errlog.NewStore(errlog.Options{})
	prev := errlog.SetDefault(store)
	t.Cleanup(func() { errlog.SetDefault(prev) })

	ReportWatchEventError(watcher.Event{
		Kind:   watcher.EventUrlOpened,
		At:     time.Now(),
		Alias:  "atto",
		Url:    "https://example.test",
		OpenOK: false,
		Err:    errors.New("exec: chrome not found"),
	})
	got := store.Snapshot()
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
	store := errlog.NewStore(errlog.Options{})
	prev := errlog.SetDefault(store)
	t.Cleanup(func() { errlog.SetDefault(prev) })

	ReportWatchEventError(watcher.Event{
		Kind:   watcher.EventUrlOpened,
		At:     time.Now(),
		Alias:  "atto",
		OpenOK: true,
	})
	if got := store.Snapshot(); len(got) != 0 {
		t.Fatalf("success event leaked into errlog: %+v", got)
	}
}

// TestReportWatchEventError_NonErrorKindsIgnored locks that benign
// event kinds (heartbeat, baseline, new mail, etc.) do not feed the
// error log.
func TestReportWatchEventError_NonErrorKindsIgnored(t *testing.T) {
	store := errlog.NewStore(errlog.Options{})
	prev := errlog.SetDefault(store)
	t.Cleanup(func() { errlog.SetDefault(prev) })

	for _, k := range []watcher.EventKind{
		watcher.EventStarted, watcher.EventStopped, watcher.EventBaseline,
		watcher.EventPollOK, watcher.EventNewMail, watcher.EventRuleMatch,
		watcher.EventHeartbeat, watcher.EventUidValReset,
	} {
		ReportWatchEventError(watcher.Event{Kind: k, At: time.Now(), Alias: "atto"})
	}
	if got := store.Snapshot(); len(got) != 0 {
		t.Fatalf("non-error events leaked into errlog: %+v", got)
	}
}
