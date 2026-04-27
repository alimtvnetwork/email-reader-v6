// error_log_store.go — in-memory ring buffer of error reports surfaced in
// the desktop UI's "Diagnostics → Error Log" view (Phase 3 of the
// error-trace-logging-upgrade — see .lovable/plan.md).
//
// Design constraints:
//   - Headless-safe: NO fyne import, so the package still compiles under
//     `-tags nofyne` for CI and unit tests pin the wiring without an
//     OpenGL context.
//   - Bounded memory: capacity is fixed at construction (default 500).
//     When the buffer is full the oldest entry is dropped — the user
//     cares about recent failures, not a months-old crash.
//   - Live updates: Subscribe returns a channel that receives every new
//     entry. Slow/blocked subscribers are dropped (non-blocking send) so
//     a stuck UI never wedges the producer. Producers may be any
//     goroutine (watcher, view callbacks, background jobs).
//   - Always wired through errtrace: ReportError formats the error with
//     errtrace.Format so the captured frame chain lands in Trace,
//     regardless of whether the calling site itself wrapped properly.
//
// Persistence to disk lands in Phase 4 (data/error-log.jsonl, rotated
// at 5 MB) — this file deliberately stays in-memory only.
package ui

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// ErrorLogEntry is one row in the error log. Exported so the Fyne view
// (slice 3.3) and any future export pipeline (slice 4.x) can render it.
type ErrorLogEntry struct {
	// Seq is a monotonically-increasing per-store sequence number. Lets
	// the UI compute "unread since last open" without timestamp math.
	Seq uint64
	// Timestamp is when ReportError was called (UTC).
	Timestamp time.Time
	// Component is a short tag identifying the subsystem that reported
	// the error ("emails", "settings", "watcher", ...). Free-form but
	// kept short — the view shows it as a chip next to the summary.
	Component string
	// Summary is the one-line err.Error() string — what the user sees
	// in the status bar today.
	Summary string
	// Trace is the full multi-line errtrace.Format output (summary +
	// "  at file:line (func)" lines). When the originating error was
	// not wrapped through errtrace this falls back to the same value as
	// Summary so the view always has something to show.
	Trace string
}

// errorLogStore is a fixed-capacity ring of ErrorLogEntry values plus a
// fan-out to live subscribers. Concurrency-safe.
type errorLogStore struct {
	mu          sync.RWMutex
	entries     []ErrorLogEntry // oldest at index 0, newest at len-1
	cap         int
	nextSeq     uint64
	subscribers []chan ErrorLogEntry
	// unread counts entries appended since the last MarkAllRead call.
	// Held as atomic so the sidebar badge can read it without taking
	// the store lock on every paint.
	unread atomic.Int64
}

// defaultErrorLogCap is the in-memory ring size. 500 entries × ~1 KiB
// each ≈ 0.5 MiB worst case — fine for a desktop app. Phase 4 adds
// disk persistence with a separate (larger) cap.
const defaultErrorLogCap = 500

// errorLogSingleton is the process-wide store used by ReportError. Lazy
// init so test packages that never log errors don't allocate it.
var (
	errorLogOnce      sync.Once
	errorLogSingleton *errorLogStore
)

// errorLog returns the lazily-built singleton.
func errorLog() *errorLogStore {
	errorLogOnce.Do(func() {
		errorLogSingleton = newErrorLogStore(defaultErrorLogCap)
	})
	return errorLogSingleton
}

// newErrorLogStore builds an empty store with the given capacity. cap<=0
// falls back to defaultErrorLogCap so misconfiguration cannot produce a
// degenerate (always-overwriting) buffer.
func newErrorLogStore(capacity int) *errorLogStore {
	if capacity <= 0 {
		capacity = defaultErrorLogCap
	}
	return &errorLogStore{
		entries: make([]ErrorLogEntry, 0, capacity),
		cap:     capacity,
	}
}

// append records one entry, evicts the oldest when at capacity, bumps
// the unread counter, and fans out to subscribers (non-blocking).
func (s *errorLogStore) append(e ErrorLogEntry) {
	s.mu.Lock()
	s.nextSeq++
	e.Seq = s.nextSeq
	if len(s.entries) >= s.cap {
		// Drop the oldest entry. Copy-shift is O(cap) but cap is 500
		// and ReportError is rare — far simpler than a head-pointer
		// ring and avoids any "Subscribe sees indices in the wrong
		// order" surprises.
		copy(s.entries, s.entries[1:])
		s.entries = s.entries[:len(s.entries)-1]
	}
	s.entries = append(s.entries, e)
	subs := append([]chan ErrorLogEntry(nil), s.subscribers...)
	s.mu.Unlock()

	s.unread.Add(1)

	// Fan out without holding the lock. select+default makes a stuck
	// subscriber drop the event rather than wedge ReportError.
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Snapshot returns a copy of all current entries, oldest first. Safe to
// call from any goroutine; the returned slice is owned by the caller.
func (s *errorLogStore) Snapshot() []ErrorLogEntry {
	s.mu.RLock()
	out := make([]ErrorLogEntry, len(s.entries))
	copy(out, s.entries)
	s.mu.RUnlock()
	return out
}

// Subscribe returns a buffered channel that receives every future
// ErrorLogEntry. Capacity 16 absorbs short bursts without dropping;
// beyond that, the producer drops (see append). Caller must hold a
// reference to the channel for the lifetime they want updates.
func (s *errorLogStore) Subscribe() <-chan ErrorLogEntry {
	ch := make(chan ErrorLogEntry, 16)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	return ch
}

// Unread returns the count of entries appended since the last
// MarkAllRead call. Used by the sidebar badge — see slice 3.5.
func (s *errorLogStore) Unread() int64 { return s.unread.Load() }

// MarkAllRead resets the unread counter. Called by the Error Log view
// when it becomes visible.
func (s *errorLogStore) MarkAllRead() { s.unread.Store(0) }

// Clear drops every entry and resets the unread counter. Surfaces in
// slice 3.3 as the Clear button.
func (s *errorLogStore) Clear() {
	s.mu.Lock()
	s.entries = s.entries[:0]
	s.mu.Unlock()
	s.unread.Store(0)
}

// ----- Public package-level API used by every UI view ------------

// ReportError records err in the process-wide error log. component is a
// short subsystem tag ("emails", "settings", ...). No-op when err is
// nil, so call sites can write `ui.ReportError("emails", err)` right
// next to a `if err != nil { status.SetText(...) }` without an extra
// guard. Safe to call from any goroutine.
//
// The Trace field is filled from errtrace.Format(err) so the captured
// frame chain lands in the log even when the immediate caller used a
// plain error type.
func ReportError(component string, err error) {
	if err == nil {
		return
	}
	if component == "" {
		component = "ui"
	}
	entry := ErrorLogEntry{
		Timestamp: time.Now().UTC(),
		Component: component,
		Summary:   err.Error(),
		Trace:     errtrace.Format(err),
	}
	errorLog().append(entry)
}

// ErrorLogSnapshot returns the current entries (oldest first). Used by
// the Error Log view for its initial paint.
func ErrorLogSnapshot() []ErrorLogEntry { return errorLog().Snapshot() }

// SubscribeErrorLog returns a channel of new entries. Used by the Error
// Log view (live append) and by the sidebar badge updater.
func SubscribeErrorLog() <-chan ErrorLogEntry { return errorLog().Subscribe() }

// UnreadErrorCount is the count of entries appended since the last
// MarkErrorLogRead call.
func UnreadErrorCount() int64 { return errorLog().Unread() }

// MarkErrorLogRead resets the unread counter. Called by the Error Log
// view when it becomes visible (slices 3.4 / 3.5).
func MarkErrorLogRead() { errorLog().MarkAllRead() }

// ClearErrorLog drops every recorded entry. Called by the Error Log
// view's Clear button.
func ClearErrorLog() { errorLog().Clear() }
