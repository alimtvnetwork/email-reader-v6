// maintenance_log_emit_test.go verifies the structured-log sink path
// (logRetentionSweep / logAnalyzeRun / logVacuumRun / logWalCheckpoint)
// actually emits a slog record carrying the spec-mandated
// `component=maintenance event=<name>` keys at the right level.
//
// Spec: spec/23-app-database/04 §6 — INFO on success, WARN on error.
package ui

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// withTestLogger swaps maintenanceLogger for one that writes to a
// bytes.Buffer, then restores it. Synchronises the once so a real
// process logger built earlier doesn't leak into the test.
func withTestLogger(t *testing.T, fn func(buf *bytes.Buffer)) {
	t.Helper()
	var buf bytes.Buffer
	prev := maintenanceLogger
	prevOnce := maintenanceLoggerOnce
	maintenanceLogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})).
		With(slog.String("component", componentMaintenance))
	maintenanceLoggerOnce = sync.Once{}
	maintenanceLoggerOnce.Do(func() {}) // mark consumed so production lazy-init no-ops
	t.Cleanup(func() {
		maintenanceLogger = prev
		maintenanceLoggerOnce = prevOnce
	})
	fn(&buf)
}

func TestLogRetentionSweep_EmitsStructured(t *testing.T) {
	withTestLogger(t, func(buf *bytes.Buffer) {
		logRetentionSweep(7, nil)
		out := buf.String()
		for _, want := range []string{"component=maintenance", "event=prune", "deleted=7", "level=INFO"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in %q", want, out)
			}
		}
	})
}

func TestLogRetentionSweep_ErrorAtWarn(t *testing.T) {
	withTestLogger(t, func(buf *bytes.Buffer) {
		logRetentionSweep(0, errors.New("disk full"))
		out := buf.String()
		for _, want := range []string{"component=maintenance", "event=prune", "deleted=0", "level=WARN"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in %q", want, out)
			}
		}
		if !strings.Contains(out, "disk full") {
			t.Errorf("error text lost: %q", out)
		}
	})
}

func TestLogAnalyzeVacuumWalCheckpoint_AllEmitEvent(t *testing.T) {
	cases := []struct {
		name  string
		fire  func()
		event string
		key   string
	}{
		{"analyze", func() { logAnalyzeRun(1500, nil) }, "analyze", "triggered_at=1500"},
		{"vacuum", func() { logVacuumRun(2048, nil) }, "vacuum", "reclaimed_bytes=2048"},
		{"wal", func() { logWalCheckpoint(42, nil) }, "wal_checkpoint", "pages=42"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withTestLogger(t, func(buf *bytes.Buffer) {
				c.fire()
				out := buf.String()
				if !strings.Contains(out, "event="+c.event) {
					t.Errorf("missing event=%s in %q", c.event, out)
				}
				if !strings.Contains(out, c.key) {
					t.Errorf("missing %q in %q", c.key, out)
				}
				if !strings.Contains(out, "component=maintenance") {
					t.Errorf("missing component tag in %q", out)
				}
			})
		})
	}
}

// TestMaintenanceSlog_LazyAndStable verifies the helper returns the
// same logger across calls (so callers can rely on a single sink) and
// that it carries the component tag even on the very first access.
func TestMaintenanceSlog_LazyAndStable(t *testing.T) {
	// Use a fresh test logger so we don't accidentally hit os.Stdout.
	withTestLogger(t, func(buf *bytes.Buffer) {
		a := maintenanceSlog()
		b := maintenanceSlog()
		if a != b {
			t.Fatalf("maintenanceSlog should return the same logger across calls")
		}
		a.LogAttrs(context.Background(), slog.LevelInfo, "probe")
		if !strings.Contains(buf.String(), "component=maintenance") {
			t.Errorf("logger missing component tag: %q", buf.String())
		}
	})
}
