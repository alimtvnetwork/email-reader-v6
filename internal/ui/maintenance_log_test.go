// maintenance_log_test.go locks the structured-log "tail" emitted by
// the maintenance loop. Spec/23-app-database/04 §6 mandates the
// `component=maintenance event=… key=value` shape; the prefix
// (`component=maintenance`) is added by slog.With at logger
// construction, so the Format* helpers return only the event tail.
//
// The format is part of the operational contract — ops greps for
// `component=maintenance event=prune` to confirm the daily tick is
// alive. Changing the tail breaks dashboards / runbooks, so this
// suite must fail loudly on any drift.
package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatRetentionSweep_Success_IncludesCountAndOk(t *testing.T) {
	got := FormatRetentionSweep(42, nil)
	want := "event=prune deleted=42 ok"
	if got != want {
		t.Fatalf("success format mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFormatRetentionSweep_Success_ZeroCountStillLogged(t *testing.T) {
	// The zero-deleted case is the most common and most valuable for
	// liveness monitoring — operators MUST be able to confirm the
	// sweeper is firing even on an idle DB. Lock the line shape.
	got := FormatRetentionSweep(0, nil)
	want := "event=prune deleted=0 ok"
	if got != want {
		t.Fatalf("zero-count format mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFormatRetentionSweep_Error_PreservesCountAndError(t *testing.T) {
	// Partial Exec may return >0 deleted alongside an error from the
	// driver. Format must surface both.
	err := errors.New("disk full")
	got := FormatRetentionSweep(7, err)
	if !strings.HasPrefix(got, "event=prune ") {
		t.Errorf("missing canonical event prefix: %q", got)
	}
	if !strings.Contains(got, "deleted=7") {
		t.Errorf("partial-delete count lost: %q", got)
	}
	if !strings.Contains(got, "error=disk full") {
		t.Errorf("error message lost: %q", got)
	}
	if strings.Contains(got, " ok") {
		t.Errorf("error case must not include the ok marker: %q", got)
	}
}

func TestFormatRetentionSweep_SingleLine(t *testing.T) {
	// Multi-line log entries break per-line grepping. The format must
	// stay single-line in BOTH branches.
	for _, c := range []struct {
		name string
		out  string
	}{
		{"ok", FormatRetentionSweep(3, nil)},
		{"err", FormatRetentionSweep(3, errors.New("boom"))},
	} {
		if strings.ContainsAny(c.out, "\n\r") {
			t.Errorf("%s: log line contains newline: %q", c.name, c.out)
		}
	}
}

func TestFormatAnalyzeRun_Success_IncludesTriggerAndOk(t *testing.T) {
	got := FormatAnalyzeRun(1500, nil)
	want := "event=analyze triggered_at=1500 ok"
	if got != want {
		t.Fatalf("analyze success format mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFormatAnalyzeRun_Error_PreservesTriggerAndError(t *testing.T) {
	got := FormatAnalyzeRun(1234, errors.New("locked"))
	if !strings.HasPrefix(got, "event=analyze ") {
		t.Errorf("missing canonical event prefix: %q", got)
	}
	if !strings.Contains(got, "triggered_at=1234") {
		t.Errorf("trigger count lost: %q", got)
	}
	if !strings.Contains(got, "error=locked") {
		t.Errorf("error message lost: %q", got)
	}
	if strings.Contains(got, " ok") {
		t.Errorf("error case must not include the ok marker: %q", got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("analyze log line contains newline: %q", got)
	}
}
