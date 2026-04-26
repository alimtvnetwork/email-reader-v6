// maintenance_log_jobs_test.go pins the structured-log tail for the
// VACUUM and wal_checkpoint events. Spec/23-app-database/04 §6.
package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatVacuumRun_Success(t *testing.T) {
	got := FormatVacuumRun(2097152, nil)
	want := "event=vacuum reclaimed_bytes=2097152 ok"
	if got != want {
		t.Fatalf("vacuum success format mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFormatVacuumRun_NegativeReclaimedPreserved(t *testing.T) {
	// Rare: SQLite may grow the file. We surface the negative so an
	// operator notices instead of silently zeroing.
	got := FormatVacuumRun(-512, nil)
	if !strings.Contains(got, "reclaimed_bytes=-512") {
		t.Fatalf("negative reclaimed lost: %q", got)
	}
}

func TestFormatVacuumRun_Error(t *testing.T) {
	got := FormatVacuumRun(123, errors.New("locked"))
	if !strings.HasPrefix(got, "event=vacuum ") {
		t.Errorf("missing event prefix: %q", got)
	}
	if !strings.Contains(got, "reclaimed_bytes=123") || !strings.Contains(got, "error=locked") {
		t.Errorf("error format missing fields: %q", got)
	}
	if strings.Contains(got, " ok") || strings.ContainsAny(got, "\n\r") {
		t.Errorf("must not include 'ok' or newlines: %q", got)
	}
}

func TestFormatWalCheckpoint_Success(t *testing.T) {
	got := FormatWalCheckpoint(42, nil)
	want := "event=wal_checkpoint pages=42 ok"
	if got != want {
		t.Fatalf("wal_checkpoint format mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFormatWalCheckpoint_Error(t *testing.T) {
	got := FormatWalCheckpoint(0, errors.New("io error"))
	if !strings.HasPrefix(got, "event=wal_checkpoint ") {
		t.Errorf("missing event prefix: %q", got)
	}
	if !strings.Contains(got, "pages=0") || !strings.Contains(got, "error=io error") {
		t.Errorf("error format missing fields: %q", got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("contains newline: %q", got)
	}
}
