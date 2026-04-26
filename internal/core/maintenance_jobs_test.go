// maintenance_jobs_test.go covers the new VACUUM + wal_checkpoint
// scheduling logic in core.Maintenance — specifically maybeWalCheckpoint
// and maybeVacuum. Spec: spec/23-app-database/04 §2 rows 4-5.
package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type vacRecorder struct {
	calls     int
	gateCalls int
	gateOK    bool
	gateErr   error
	vacuumErr error
	reclaimed int64
	last      int64
}

func (r *vacRecorder) gate(_ context.Context) (bool, error) {
	r.gateCalls++
	return r.gateOK, r.gateErr
}
func (r *vacRecorder) vac(_ context.Context) (int64, error) {
	r.calls++
	return r.reclaimed, r.vacuumErr
}

type walRecorder struct {
	calls int
	pages int64
	err   error
}

func (w *walRecorder) fn(_ context.Context) (int64, error) {
	w.calls++
	return w.pages, w.err
}

// newJobsTestMaintenance builds a Maintenance with deterministic clock at
// `now`, the supplied seams, and required (no-op) Pruner/Retention.
func newJobsTestMaintenance(t *testing.T, now time.Time, opts MaintenanceOptions) *Maintenance {
	t.Helper()
	if opts.Pruner == nil {
		opts.Pruner = func(context.Context, time.Time) (int64, error) { return 0, nil }
	}
	if opts.Retention == nil {
		opts.Retention = func() uint16 { return 0 }
	}
	opts.Now = func() time.Time { return now }
	res := NewMaintenance(opts)
	if res.HasError() {
		t.Fatalf("NewMaintenance: %v", res.Error())
	}
	return res.Value()
}

// TestMaybeWalCheckpoint_FiresOnFirstTickThenWaits validates the
// cadence behaviour: first tick always fires (lastRun zero), subsequent
// ticks within WalCheckpointHours are no-ops.
func TestMaybeWalCheckpoint_FiresOnFirstTickThenWaits(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	wal := &walRecorder{pages: 7}
	logged := 0
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		WalCheckpointer:    wal.fn,
		WalCheckpointHours: 6,
		OnWalCheckpoint: func(p int64, err error) {
			if err != nil {
				t.Fatalf("OnWalCheckpoint err: %v", err)
			}
			if p != 7 {
				t.Fatalf("OnWalCheckpoint pages=%d, want 7", p)
			}
			logged++
		},
	})

	// First tick: fires.
	last := m.maybeWalCheckpoint(context.Background(), time.Time{})
	if wal.calls != 1 || logged != 1 || last != now {
		t.Fatalf("first tick: calls=%d logged=%d last=%v", wal.calls, logged, last)
	}
	// Second tick same instant: no-op.
	last = m.maybeWalCheckpoint(context.Background(), last)
	if wal.calls != 1 {
		t.Fatalf("repeat tick should not fire; calls=%d", wal.calls)
	}
}

// TestMaybeWalCheckpoint_NoSeam_IsNoop: Optional. Without
// WalCheckpointer we never log and never advance.
func TestMaybeWalCheckpoint_NoSeam_IsNoop(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	called := false
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		OnWalCheckpoint: func(int64, error) { called = true },
	})
	last := m.maybeWalCheckpoint(context.Background(), time.Time{})
	if called || !last.IsZero() {
		t.Fatalf("no seam: must not fire; called=%v last=%v", called, last)
	}
}

// TestMaybeWalCheckpoint_ErrorRetainsLastRun: an Error must NOT advance
// lastRun so the next tick retries.
func TestMaybeWalCheckpoint_ErrorRetainsLastRun(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	wal := &walRecorder{err: errors.New("disk busy")}
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		WalCheckpointer:    wal.fn,
		WalCheckpointHours: 6,
	})
	last := m.maybeWalCheckpoint(context.Background(), time.Time{})
	if !last.IsZero() {
		t.Fatalf("error must keep lastRun zero, got %v", last)
	}
}

// TestMaybeVacuum_RunsInSlot validates the happy path: in-slot tick,
// gate says go, Vacuumer fires, lastRun advances.
func TestMaybeVacuum_RunsInSlot(t *testing.T) {
	// Sunday 03:00 UTC matches default VacuumWeekday=Sunday + Hour=3
	now := time.Date(2026, 4, 26, 3, 0, 0, 0, time.UTC)
	rec := &vacRecorder{gateOK: true, reclaimed: 4096}
	logged := false
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		Vacuumer:   rec.vac,
		VacuumGate: rec.gate,
		OnVacuum: func(rb int64, err error) {
			logged = true
			if err != nil || rb != 4096 {
				t.Fatalf("OnVacuum: rb=%d err=%v", rb, err)
			}
		},
	})
	last := m.maybeVacuum(context.Background(), time.Time{})
	if rec.calls != 1 || rec.gateCalls != 1 || !logged || last != now {
		t.Fatalf("happy path: calls=%d gate=%d logged=%v last=%v",
			rec.calls, rec.gateCalls, logged, last)
	}
}

// TestMaybeVacuum_GateFalse_BumpsLastRunButSkipsCall ensures we don't
// keep re-asking the gate inside the same slot once it says no.
func TestMaybeVacuum_GateFalse_BumpsLastRunButSkipsCall(t *testing.T) {
	now := time.Date(2026, 4, 26, 3, 0, 0, 0, time.UTC)
	rec := &vacRecorder{gateOK: false}
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		Vacuumer:   rec.vac,
		VacuumGate: rec.gate,
	})
	last := m.maybeVacuum(context.Background(), time.Time{})
	if rec.calls != 0 {
		t.Fatalf("Vacuumer must not fire when gate=false; calls=%d", rec.calls)
	}
	if last != now {
		t.Fatalf("lastRun must advance to skip rest of slot; got %v", last)
	}
}

// TestMaybeVacuum_OutOfSlot_IsNoop: Tuesday at 14:00 must not fire.
func TestMaybeVacuum_OutOfSlot_IsNoop(t *testing.T) {
	now := time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC) // Tuesday
	rec := &vacRecorder{gateOK: true}
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		Vacuumer:   rec.vac,
		VacuumGate: rec.gate,
	})
	last := m.maybeVacuum(context.Background(), time.Time{})
	if rec.calls != 0 || rec.gateCalls != 0 || !last.IsZero() {
		t.Fatalf("out-of-slot: calls=%d gate=%d last=%v",
			rec.calls, rec.gateCalls, last)
	}
}

// TestMaybeVacuum_VacuumError_RetainsLastRun: errors retry within slot.
func TestMaybeVacuum_VacuumError_RetainsLastRun(t *testing.T) {
	now := time.Date(2026, 4, 26, 3, 0, 0, 0, time.UTC)
	rec := &vacRecorder{gateOK: true, vacuumErr: errors.New("locked")}
	m := newJobsTestMaintenance(t, now, MaintenanceOptions{
		Vacuumer:   rec.vac,
		VacuumGate: rec.gate,
	})
	last := m.maybeVacuum(context.Background(), time.Time{})
	if !last.IsZero() {
		t.Fatalf("vacuum error must keep lastRun zero, got %v", last)
	}
}
