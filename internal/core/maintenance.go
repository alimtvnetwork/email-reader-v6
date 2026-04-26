// maintenance.go owns the long-running maintenance goroutine that
// invokes the OpenedUrls retention sweeper on a schedule. The actual
// "should we sweep?" decision and the cutoff calculation live in
// retention.go (pure helpers); the prune SQL lives in
// internal/store/vacuum.go. This file only owns *when* the helpers
// fire and *how* they observe the current Settings snapshot.
//
// Spec: spec/23-app-database/04-retention-and-vacuum.md §2.
//
// Concurrency model:
//   - One goroutine per Maintenance instance.
//   - The goroutine wakes on Ticker ticks (default 1 minute) and on
//     ctx.Done. On every tick it asks ShouldRunRetentionTick whether
//     a sweep is due; if so, it calls Pruner with the cutoff derived
//     from the current Settings snapshot.
//   - lastRun is owned by the goroutine — never read or written from
//     outside it, so no mutex is needed.
//   - Stop cancels ctx and waits (bounded) for the goroutine to exit.
package core

import (
	"context"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// Pruner is the seam over store.PruneOpenedUrlsBefore. Returning the
// row count keeps the seam test-friendly without coupling to *sql.DB.
// Errors are logged at the call site (a logger is wired in a follow-up
// — until then, errors silently re-arm the next tick so transient DB
// busyness does not wedge the loop).
type Pruner func(ctx context.Context, cutoff time.Time) (int64, error)

// SnapshotSource returns the current OpenUrlsRetentionDays. Reading
// it on every tick (rather than caching at construction) means the
// user can change the knob in Settings and the sweep starts honouring
// it within one tick — no restart required.
type SnapshotSource func() uint16

// MaintenanceOptions configures a Maintenance loop. All fields default
// to the production values when zero so callers in cmd/* can pass an
// almost-empty struct.
type MaintenanceOptions struct {
	// Pruner is required: nothing to do without a sweeper.
	Pruner Pruner
	// Retention is required: nothing to schedule without the knob.
	Retention SnapshotSource
	// Now defaults to time.Now.
	Now func() time.Time
	// TickInterval defaults to 1 minute. The retention tick fires at
	// most once per RetentionIntervalHours regardless of TickInterval —
	// TickInterval just sets the *check* cadence.
	TickInterval time.Duration
	// RetentionIntervalHours defaults to 24h.
	RetentionIntervalHours int
	// OnSweep is an optional observer (logging / metrics / tests). Fired
	// after every sweep attempt, success or error.
	OnSweep func(deleted int64, err error)
}

// Maintenance is the goroutine handle. Construct via NewMaintenance,
// call Start once, call Stop once on shutdown.
type Maintenance struct {
	opts    MaintenanceOptions
	startMu sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewMaintenance validates required fields and returns a ready-to-Start
// instance. The goroutine is NOT spawned until Start is called.
func NewMaintenance(opts MaintenanceOptions) errtrace.Result[*Maintenance] {
	if opts.Pruner == nil || opts.Retention == nil {
		return errtrace.Err[*Maintenance](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"NewMaintenance: Pruner and Retention required",
		))
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TickInterval <= 0 {
		opts.TickInterval = time.Minute
	}
	if opts.RetentionIntervalHours <= 0 {
		opts.RetentionIntervalHours = 24
	}
	return errtrace.Ok(&Maintenance{opts: opts})
}

// Start spawns the maintenance goroutine. Idempotent: a second call
// while already running is a no-op.
func (m *Maintenance) Start(parent context.Context) {
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if m.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done
	go m.run(ctx, done)
}

// Stop cancels the goroutine context and waits up to `timeout` for it
// to exit. A zero/negative timeout means "wait forever". Idempotent.
func (m *Maintenance) Stop(timeout time.Duration) {
	m.startMu.Lock()
	cancel, done := m.cancel, m.done
	m.cancel, m.done = nil, nil
	m.startMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if timeout <= 0 {
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// run is the goroutine body. Wakes every TickInterval and asks
// ShouldRunRetentionTick whether to sweep. lastRun is goroutine-local.
func (m *Maintenance) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(m.opts.TickInterval)
	defer ticker.Stop()
	var lastRun time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastRun = m.maybeSweep(ctx, lastRun)
		}
	}
}

// maybeSweep performs one tick's worth of work: evaluates the helper,
// invokes the Pruner if due, fires OnSweep, and returns the new
// lastRun. Pulled out of run() to keep that function under the
// 15-statement linter cap and to make the per-tick logic unit-testable
// in isolation.
func (m *Maintenance) maybeSweep(ctx context.Context, lastRun time.Time) time.Time {
	now := m.opts.Now()
	days := m.opts.Retention()
	if !ShouldRunRetentionTick(lastRun, now, m.opts.RetentionIntervalHours, days) {
		return lastRun
	}
	cutoff := RetentionCutoff(now, days)
	deleted, err := m.opts.Pruner(ctx, cutoff)
	if m.opts.OnSweep != nil {
		m.opts.OnSweep(deleted, err)
	}
	if err != nil {
		// Re-arm: do not bump lastRun so the next tick retries.
		return lastRun
	}
	return now
}
