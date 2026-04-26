// maintenance_log.go owns the human-readable log line emitted after
// every OpenedUrls retention sweep. Splitting the formatter out of
// watch_runtime.go keeps the message under test without dragging the
// goroutine wiring into the assertions, and lets a future structured-
// logging migration swap the sink without touching message format.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §2.
//
// Format rules (locked by maintenance_log_test.go):
//   - Always one line, prefix "ui: maintenance: retention sweep".
//   - Success: include the deleted-row count even when zero (so an
//     operator grepping for "retention sweep" sees every tick that
//     actually ran, not just non-trivial ones).
//   - Error: include the error string AFTER the deleted count so the
//     count from a partial Exec (sql/driver may return >0 on error)
//     is preserved.
package ui

import (
	"fmt"
	"log"
)

// FormatRetentionSweep returns the exact log line for a single sweep
// outcome. Pure: callable from tests without a logger / sink.
func FormatRetentionSweep(deleted int64, err error) string {
	if err != nil {
		return fmt.Sprintf(
			"ui: maintenance: retention sweep: deleted=%d error=%v",
			deleted, err)
	}
	return fmt.Sprintf(
		"ui: maintenance: retention sweep: deleted=%d ok",
		deleted)
}

// logRetentionSweep is the production OnSweep callback wired by
// startMaintenance. Splitting this out (instead of an inline closure)
// keeps startMaintenance under the 15-statement linter cap and makes
// the sink point obvious to a future structured-logging migration.
func logRetentionSweep(deleted int64, err error) {
	log.Print(FormatRetentionSweep(deleted, err))
}
