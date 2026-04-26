// maintenance_analyze_test.go covers the ANALYZE-after-N-deletes
// scheduling logic in core.Maintenance.maybeSweep / maybeAnalyze.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §2 row 3.
//
// We exercise the unexported maybeSweep directly: it is the per-tick
// pure-ish function and that lets us assert on `lastRun`, the
// cumulative-delete tally, and the OnAnalyze callback without
// spinning a goroutine or a fake clock for the ticker.
package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type analyzeRecorder struct {
	calls       int
	triggeredAt []int64
	err         error
}

func (a *analyzeRecorder) fn(_ context.Context) error {
	a.calls++
	return a.err
}

// newTestMaintenance constructs a Maintenance instance with a fake
// clock + the supplied Pruner / Analyzer. Retention always returns 1
// day so ShouldRunRetentionTick fires on every call (lastRun zero or
// stale).
func newTestMaintenance(t *testing.T, pruner Pruner, analyzer Analyzer, onA func(int64, error)) *Maintenance {
	t.Helper()
	res := NewMaintenance(MaintenanceOptions{
		Pruner:                 pruner,
		Analyzer:               analyzer,
		Retention:              func() uint16 { return 1 },
		OnAnalyze:              onA,
		Now:                    func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
		RetentionIntervalHours: 24,
	})
	if res.HasError() {
		t.Fatalf("NewMaintenance: %v", res.Error())
	}
	return res.Value()
}

// TestMaintenance_AnalyzeFires_AfterCumulativeDeletesCrossThreshold:
// three sweeps of 400 rows each → 400, 800, 1200. ANALYZE must fire
// only on the third (1200 ≥ 1000) and the counter must reset.
func TestMaintenance_AnalyzeFires_AfterCumulativeDeletesCrossThreshold(t *testing.T) {
	const perTick int64 = 400
	pruner := func(_ context.Context, _ time.Time) (int64, error) { return perTick, nil }
	rec := &analyzeRecorder{}
	m := newTestMaintenance(t, pruner, rec.fn, func(at int64, err error) {
		if err != nil {
			t.Fatalf("OnAnalyze err: %v", err)
		}
		_ = at
	})

	var cum int64
	var last time.Time
	// Tick 1 — cum=400, no analyze
	last, cum = m.maybeSweep(context.Background(), last, cum)
	if rec.calls != 0 || cum != 400 {
		t.Fatalf("after t1: calls=%d cum=%d, want 0/400", rec.calls, cum)
	}
	// Tick 2 — gate ShouldRunRetentionTick by zeroing last (24h cadence)
	last = time.Time{}
	last, cum = m.maybeSweep(context.Background(), last, cum)
	if rec.calls != 0 || cum != 800 {
		t.Fatalf("after t2: calls=%d cum=%d, want 0/800", rec.calls, cum)
	}
	// Tick 3 — crosses threshold; analyze fires; cum resets
	last = time.Time{}
	_, cum = m.maybeSweep(context.Background(), last, cum)
	if rec.calls != 1 {
		t.Fatalf("after t3: calls=%d, want 1", rec.calls)
	}
	if cum != 0 {
		t.Fatalf("after t3 cum=%d, want 0 (reset)", cum)
	}
}

// TestMaintenance_AnalyzeKeepsTally_OnError: when Analyzer fails the
// cumulative counter must NOT reset — the next tick retries instead
// of waiting for another 1 000 deletes.
func TestMaintenance_AnalyzeKeepsTally_OnError(t *testing.T) {
	pruner := func(_ context.Context, _ time.Time) (int64, error) { return 1500, nil }
	rec := &analyzeRecorder{err: errors.New("disk busy")}
	m := newTestMaintenance(t, pruner, rec.fn, nil)

	_, cum := m.maybeSweep(context.Background(), time.Time{}, 0)
	if rec.calls != 1 {
		t.Fatalf("calls=%d, want 1", rec.calls)
	}
	if cum != 1500 {
		t.Fatalf("cum=%d, want 1500 (retained on error)", cum)
	}
}

// TestMaintenance_NoAnalyzer_LeavesTallyAlone: when Analyzer is nil
// the cumulative counter still grows but never resets, and no log
// callback fires. Proves the optional seam is opt-in.
func TestMaintenance_NoAnalyzer_LeavesTallyAlone(t *testing.T) {
	pruner := func(_ context.Context, _ time.Time) (int64, error) { return 1500, nil }
	called := false
	m := newTestMaintenance(t, pruner, nil, func(int64, error) { called = true })

	_, cum := m.maybeSweep(context.Background(), time.Time{}, 0)
	if cum != 1500 {
		t.Fatalf("cum=%d, want 1500", cum)
	}
	if called {
		t.Fatalf("OnAnalyze must not fire when Analyzer is nil")
	}
}
