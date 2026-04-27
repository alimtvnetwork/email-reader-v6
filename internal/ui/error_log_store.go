// error_log_store.go — thin re-export shim so internal/ui callers can
// keep using `ui.ReportError(...)` while the real ring buffer lives at
// internal/ui/errlog (which both internal/ui *and* internal/ui/views
// can import without creating a cycle). See .lovable/plan.md → Phase 3.
package ui

import "github.com/lovable/email-read/internal/ui/errlog"

// ErrorLogEntry mirrors errlog.Entry so views/code that reads logs out
// of the ui package keeps working without an extra import.
type ErrorLogEntry = errlog.Entry

// ReportError records err in the process-wide error log. See
// errlog.ReportError for the full contract. Safe to call from any
// goroutine; nil err is a no-op.
func ReportError(component string, err error) { errlog.ReportError(component, err) }

// ErrorLogSnapshot returns a copy of the current entries (oldest first).
func ErrorLogSnapshot() []ErrorLogEntry { return errlog.Snapshot() }

// SubscribeErrorLog returns a channel of new entries. Drop-on-block.
func SubscribeErrorLog() <-chan ErrorLogEntry { return errlog.Subscribe() }

// UnreadErrorCount is the count of entries appended since the last
// MarkErrorLogRead call. Used by the sidebar badge.
func UnreadErrorCount() int64 { return errlog.Unread() }

// MarkErrorLogRead resets the unread counter. Called when the Error Log
// view becomes visible (slices 3.4 / 3.5).
func MarkErrorLogRead() { errlog.MarkRead() }

// ClearErrorLog drops every recorded entry.
func ClearErrorLog() { errlog.Clear() }
