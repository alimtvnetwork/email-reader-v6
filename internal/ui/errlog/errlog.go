// Package errlog is the in-memory error-log ring buffer surfaced in the
// desktop UI's "Diagnostics → Error Log" view (Phase 3 of the
// error-trace-logging-upgrade — see .lovable/plan.md).
//
// This package lives below internal/ui *and* internal/ui/views in the
// import graph so both can call ReportError without creating a cycle
// (views/* cannot import ui/* directly — `ui` imports `ui/views`).
//
// Design constraints:
//   - Headless-safe: NO fyne import, so the package compiles under
//     `-tags nofyne` and unit tests run without an OpenGL context.
//   - Bounded memory: capacity is fixed at construction (default 500).
//     When the buffer is full the oldest entry is dropped.
//   - Live updates: Subscribe returns a channel that receives every new
//     entry. Slow/blocked subscribers are dropped (non-blocking send)
//     so a stuck UI never wedges the producer.
//   - Always wired through errtrace: ReportError formats the error with
//     errtrace.Format so the captured frame chain lands in Trace,
//     regardless of whether the calling site itself wrapped properly.
//
// Persistence to disk lands in Phase 4 (data/error-log.jsonl, rotated
// at 5 MB) — this package deliberately stays in-memory only.
package errlog

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// Entry is one row in the error log. Exported so the Fyne view (slice
// 3.3) and any future export pipeline (slice 4.x) can render it.
type Entry struct {
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
	// not wrapped through errtrace this falls back to the same value
	// as Summary so the view always has something to show.
	Trace string
}

// Store is a fixed-capacity ring of Entry values plus a fan-out to live
// subscribers. Concurrency-safe.
type Store struct {
	mu          sync.RWMutex
	entries     []Entry // oldest at index 0, newest at len-1
	cap         int
	nextSeq     uint64
	subscribers []chan Entry
	// persister is called once per Append, after the entry's Seq is
	// assigned and before fan-out, with the lock held. nil disables
	// persistence (the default). Wired by EnablePersistence — see
	// persist.go (Phase 4.1: data/error-log.jsonl).
	persister func(Entry)
	// unread counts entries appended since the last MarkAllRead call.
	// Held as atomic so the sidebar badge can read it without taking
	// the store lock on every paint.
	unread atomic.Int64
}

// DefaultCap is the in-memory ring size. 500 entries × ~1 KiB each ≈
// 0.5 MiB worst case — fine for a desktop app. Phase 4 adds disk
// persistence with a separate (larger) cap.
const DefaultCap = 500

// singleton is the process-wide store used by the package-level helpers
// (ReportError / Snapshot / Subscribe / ...). Lazy init so test
// packages that never log errors don't allocate it.
var (
	singletonOnce sync.Once
	singleton     *Store
)

// instance returns the lazily-built singleton.
func instance() *Store {
	singletonOnce.Do(func() {
		singleton = NewStore(DefaultCap)
	})
	return singleton
}

// NewStore builds an empty store with the given capacity. capacity<=0
// falls back to DefaultCap so misconfiguration cannot produce a
// degenerate (always-overwriting) buffer.
func NewStore(capacity int) *Store {
	if capacity <= 0 {
		capacity = DefaultCap
	}
	return &Store{
		entries: make([]Entry, 0, capacity),
		cap:     capacity,
	}
}

// Append records one entry, evicts the oldest when at capacity, bumps
// the unread counter, and fans out to subscribers (non-blocking).
func (s *Store) Append(e Entry) {
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
	subs := append([]chan Entry(nil), s.subscribers...)
	persister := s.persister
	s.mu.Unlock()

	s.unread.Add(1)

	// Best-effort persist. Done outside the lock so a slow disk does
	// not block in-process subscribers. Persister itself is guarded
	// by its own mutex (see persist.go).
	if persister != nil {
		persister(e)
	}

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
func (s *Store) Snapshot() []Entry {
	s.mu.RLock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	s.mu.RUnlock()
	return out
}

// Subscribe returns a buffered channel that receives every future
// Entry. Capacity 16 absorbs short bursts without dropping; beyond
// that, the producer drops (see Append). Caller must hold a reference
// to the channel for the lifetime they want updates.
func (s *Store) Subscribe() <-chan Entry {
	ch := make(chan Entry, 16)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	return ch
}

// Unread returns the count of entries appended since the last
// MarkAllRead call. Used by the sidebar badge — see slice 3.5.
func (s *Store) Unread() int64 { return s.unread.Load() }

// MarkAllRead resets the unread counter. Called by the Error Log view
// when it becomes visible.
func (s *Store) MarkAllRead() { s.unread.Store(0) }

// Clear drops every entry and resets the unread counter. Surfaces in
// slice 3.3 as the Clear button.
func (s *Store) Clear() {
	s.mu.Lock()
	s.entries = s.entries[:0]
	s.mu.Unlock()
	s.unread.Store(0)
}

// ----- Public package-level API used by every UI view ------------

// ReportError records err in the process-wide error log. component is
// a short subsystem tag ("emails", "settings", ...). No-op when err is
// nil, so call sites can write `errlog.ReportError("emails", err)`
// right next to a `if err != nil { status.SetText(...) }` without an
// extra guard. Safe to call from any goroutine.
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
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Component: component,
		Summary:   err.Error(),
		Trace:     errtrace.Format(err),
	}
	instance().Append(entry)
}

// Snapshot returns the current entries (oldest first). Used by the
// Error Log view for its initial paint.
func Snapshot() []Entry { return instance().Snapshot() }

// Subscribe returns a channel of new entries. Used by the Error Log
// view (live append) and by the sidebar badge updater.
func Subscribe() <-chan Entry { return instance().Subscribe() }

// Unread is the count of entries appended since the last MarkRead
// call.
func Unread() int64 { return instance().Unread() }

// MarkRead resets the unread counter. Called by the Error Log view
// when it becomes visible (slices 3.4 / 3.5).
func MarkRead() { instance().MarkAllRead() }

// Clear drops every recorded entry. Called by the Error Log view's
// Clear button.
func Clear() { instance().Clear() }
