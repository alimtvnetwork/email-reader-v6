// error_log_store_test.go — unit tests for the in-memory error-log ring
// buffer (slice 3.1 of the error-trace upgrade — see .lovable/plan.md).
// All tests run under -tags nofyne (no fyne import in error_log_store.go).
package ui

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

func TestErrorLogStore_AppendAndSnapshot(t *testing.T) {
	s := newErrorLogStore(3)
	s.append(ErrorLogEntry{Component: "a", Summary: "1"})
	s.append(ErrorLogEntry{Component: "b", Summary: "2"})
	got := s.Snapshot()
	if len(got) != 2 {
		t.Fatalf("Snapshot len = %d, want 2", len(got))
	}
	if got[0].Summary != "1" || got[1].Summary != "2" {
		t.Fatalf("Snapshot order wrong: %+v", got)
	}
	if got[0].Seq != 1 || got[1].Seq != 2 {
		t.Fatalf("Seq not monotonic: %d, %d", got[0].Seq, got[1].Seq)
	}
}

func TestErrorLogStore_RingEvictsOldest(t *testing.T) {
	s := newErrorLogStore(2)
	for i := 0; i < 5; i++ {
		s.append(ErrorLogEntry{Summary: time.Now().String()})
	}
	got := s.Snapshot()
	if len(got) != 2 {
		t.Fatalf("len = %d, want cap 2", len(got))
	}
	if got[0].Seq != 4 || got[1].Seq != 5 {
		t.Fatalf("expected last two seqs (4, 5), got %d, %d", got[0].Seq, got[1].Seq)
	}
}

func TestErrorLogStore_SubscribeReceivesAppends(t *testing.T) {
	s := newErrorLogStore(10)
	ch := s.Subscribe()
	s.append(ErrorLogEntry{Summary: "hello"})
	select {
	case e := <-ch:
		if e.Summary != "hello" {
			t.Fatalf("got %q", e.Summary)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive append within 1s")
	}
}

func TestErrorLogStore_SlowSubscriberDoesNotBlockProducer(t *testing.T) {
	s := newErrorLogStore(100)
	ch := s.Subscribe() // buffer 16, never drained
	// Push 200 entries; producer must not block even though the
	// subscriber's buffer overflows after 16.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			s.append(ErrorLogEntry{Summary: "x"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("producer wedged by slow subscriber — fan-out should be non-blocking")
	}
	_ = ch // keep the channel alive (otherwise lint may flag)
}

func TestReportError_FillsTraceFromErrtrace(t *testing.T) {
	// Reset the singleton state for this test.
	errorLog().Clear()

	cause := errtrace.New("dial imap")
	wrapped := errtrace.Wrap(cause, "watcher.pollOnce")
	ReportError("watcher", wrapped)

	got := ErrorLogSnapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	e := got[0]
	if e.Component != "watcher" {
		t.Errorf("component = %q, want watcher", e.Component)
	}
	if !strings.Contains(e.Trace, "dial imap") {
		t.Errorf("Trace missing summary: %q", e.Trace)
	}
	if !strings.Contains(e.Trace, "  at ") {
		t.Errorf("Trace missing frame chain: %q", e.Trace)
	}
}

func TestReportError_NilIsNoOp(t *testing.T) {
	errorLog().Clear()
	ReportError("emails", nil)
	if got := len(ErrorLogSnapshot()); got != 0 {
		t.Fatalf("nil err should be no-op, got %d entries", got)
	}
}

func TestReportError_PlainErrorStillRecorded(t *testing.T) {
	errorLog().Clear()
	ReportError("emails", errors.New("plain"))
	got := ErrorLogSnapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Trace == "" {
		t.Error("Trace must fall back to summary even without errtrace wrapping")
	}
}

func TestUnreadAndMarkRead(t *testing.T) {
	errorLog().Clear()
	if got := UnreadErrorCount(); got != 0 {
		t.Fatalf("after Clear: unread = %d, want 0", got)
	}
	ReportError("a", errors.New("e1"))
	ReportError("a", errors.New("e2"))
	if got := UnreadErrorCount(); got != 2 {
		t.Fatalf("unread after 2 appends = %d, want 2", got)
	}
	MarkErrorLogRead()
	if got := UnreadErrorCount(); got != 0 {
		t.Fatalf("after MarkRead: unread = %d, want 0", got)
	}
}

func TestErrorLogStore_ConcurrentAppends(t *testing.T) {
	s := newErrorLogStore(1000)
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				s.append(ErrorLogEntry{Summary: "concurrent"})
			}
		}()
	}
	wg.Wait()
	if got := len(s.Snapshot()); got != 500 {
		t.Fatalf("expected 500 entries, got %d", got)
	}
	// Sequences must be unique and monotonic.
	seen := make(map[uint64]bool, 500)
	var prev uint64
	for _, e := range s.Snapshot() {
		if seen[e.Seq] {
			t.Fatalf("duplicate seq %d", e.Seq)
		}
		seen[e.Seq] = true
		if e.Seq <= prev {
			t.Fatalf("seq not monotonic: %d after %d", e.Seq, prev)
		}
		prev = e.Seq
	}
}
