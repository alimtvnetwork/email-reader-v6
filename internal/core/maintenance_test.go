// maintenance_test.go covers the Maintenance scheduler in isolation.
// It uses an injected fake clock + ticker (via TickInterval = 5ms) and
// a recording Pruner, so each test runs in well under a second.
package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// recordingPruner returns a Pruner closure that captures every cutoff
// it is called with and the count of invocations.
type recordingPruner struct {
	mu       sync.Mutex
	cutoffs  []time.Time
	invokes  int32
	deleted  int64
	errFirst error
}

func (r *recordingPruner) fn(_ context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	r.cutoffs = append(r.cutoffs, cutoff)
	r.mu.Unlock()
	n := atomic.AddInt32(&r.invokes, 1)
	if n == 1 && r.errFirst != nil {
		return 0, r.errFirst
	}
	return r.deleted, nil
}

func (r *recordingPruner) calls() int { return int(atomic.LoadInt32(&r.invokes)) }

// fakeClock is a thread-safe monotonic clock the test advances by hand.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func TestNewMaintenance_RequiresPrunerAndRetention(t *testing.T) {
	r := NewMaintenance(MaintenanceOptions{})
	if !r.HasError() {
		t.Fatal("expected error for empty opts")
	}
	pruner := func(context.Context, time.Time) (int64, error) { return 0, nil }
	if r := NewMaintenance(MaintenanceOptions{Pruner: pruner}); !r.HasError() {
		t.Fatal("expected error without Retention")
	}
	if r := NewMaintenance(MaintenanceOptions{Retention: func() uint16 { return 0 }}); !r.HasError() {
		t.Fatal("expected error without Pruner")
	}
}

func TestMaintenance_FiresOnFirstTickWhenEnabled(t *testing.T) {
	rec := &recordingPruner{deleted: 7}
	clk := &fakeClock{now: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)}
	swept := make(chan int64, 4)
	m := mustMaintenance(t, MaintenanceOptions{
		Pruner:                 rec.fn,
		Retention:              func() uint16 { return 90 },
		Now:                    clk.Now,
		TickInterval:           5 * time.Millisecond,
		RetentionIntervalHours: 24,
		OnSweep:                func(d int64, _ error) { swept <- d },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	defer m.Stop(time.Second)

	select {
	case got := <-swept:
		if got != 7 {
			t.Fatalf("OnSweep deleted = %d, want 7", got)
		}
	case <-time.After(time.Second):
		t.Fatal("retention sweep did not fire within 1s")
	}
	if rec.calls() < 1 {
		t.Fatalf("Pruner not invoked")
	}
	rec.mu.Lock()
	cutoff := rec.cutoffs[0]
	rec.mu.Unlock()
	want := clk.Now().Add(-90 * 24 * time.Hour)
	if !cutoff.Equal(want) {
		t.Fatalf("cutoff = %v, want %v", cutoff, want)
	}
}

func TestMaintenance_DoesNotFireWhenDisabled(t *testing.T) {
	rec := &recordingPruner{}
	m := mustMaintenance(t, MaintenanceOptions{
		Pruner:       rec.fn,
		Retention:    func() uint16 { return 0 }, // disabled
		TickInterval: 3 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	time.Sleep(40 * time.Millisecond) // many ticks
	m.Stop(time.Second)
	if rec.calls() != 0 {
		t.Fatalf("Pruner called %d times despite Retention=0", rec.calls())
	}
}

func TestMaintenance_DebouncesWithinInterval(t *testing.T) {
	rec := &recordingPruner{}
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	m := mustMaintenance(t, MaintenanceOptions{
		Pruner:                 rec.fn,
		Retention:              func() uint16 { return 30 },
		Now:                    clk.Now,
		TickInterval:           2 * time.Millisecond,
		RetentionIntervalHours: 24,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	// Many ticks at the same wall-clock instant — only the first must sweep.
	time.Sleep(60 * time.Millisecond)
	m.Stop(time.Second)
	if got := rec.calls(); got != 1 {
		t.Fatalf("Pruner called %d times, want 1 (clock did not advance past 24h)", got)
	}
}

func TestMaintenance_RetriesAfterPrunerError(t *testing.T) {
	rec := &recordingPruner{errFirst: errors.New("transient busy")}
	clk := &fakeClock{now: time.Unix(1_700_000_000, 0).UTC()}
	m := mustMaintenance(t, MaintenanceOptions{
		Pruner:                 rec.fn,
		Retention:              func() uint16 { return 30 },
		Now:                    clk.Now,
		TickInterval:           3 * time.Millisecond,
		RetentionIntervalHours: 24,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	// Wait for at least 2 sweep attempts: the first errors and must
	// NOT bump lastRun, so the second tick re-fires immediately.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if rec.calls() >= 2 {
			break
		}
		time.Sleep(3 * time.Millisecond)
	}
	m.Stop(time.Second)
	if rec.calls() < 2 {
		t.Fatalf("Pruner called %d times after first errored, want ≥2", rec.calls())
	}
}

func TestMaintenance_StopIsIdempotent(t *testing.T) {
	rec := &recordingPruner{}
	m := mustMaintenance(t, MaintenanceOptions{
		Pruner:       rec.fn,
		Retention:    func() uint16 { return 90 },
		TickInterval: 5 * time.Millisecond,
	})
	ctx := context.Background()
	m.Start(ctx)
	m.Stop(time.Second)
	m.Stop(time.Second) // must not panic / hang
	// Second Start should also work.
	m.Start(ctx)
	m.Stop(time.Second)
}

func mustMaintenance(t *testing.T, opts MaintenanceOptions) *Maintenance {
	t.Helper()
	r := NewMaintenance(opts)
	if r.HasError() {
		t.Fatalf("NewMaintenance: %v", r.Error())
	}
	return r.Value()
}

// _ keeps errtrace symbol live for future direct error-code asserts.
var _ = errtrace.Ok[struct{}]
