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

// Analyzer is the seam over store.Analyze. Optional: when nil the
// ANALYZE-after-N-deletes logic is skipped and the cumulative counter
// is never reset (which is harmless — Pruner still runs).
type Analyzer func(ctx context.Context) error

// AnalyzeThresholdRows is the cumulative-delete count above which the
// Maintenance loop fires Analyzer and resets the counter. Mirrors
// store.AnalyzeThreshold (kept here as a separate const so the core
// package does not import store at construction time).
const AnalyzeThresholdRows int64 = 1000

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
	// Analyzer is optional. When non-nil, the loop tracks cumulative
	// deletes across ticks and invokes Analyzer once the count crosses
	// AnalyzeThresholdRows; the counter then resets to zero. Spec
	// 23-app-database/04 §2.
	Analyzer Analyzer
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
	// OnAnalyze is an optional observer fired when Analyzer runs.
	// Receives the cumulative-delete count that triggered it and the
	// Analyzer's error (nil on success). Used by tests + structured logs.
	OnAnalyze func(triggeredAt int64, err error)
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
	var cumDeletes int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastRun, cumDeletes = m.maybeSweep(ctx, lastRun, cumDeletes)
		}
	}
}

// maybeSweep performs one tick's worth of work: evaluates the helper,
// invokes the Pruner if due, fires OnSweep, optionally runs Analyzer
// when cumulative deletes cross the threshold, and returns the new
// lastRun + cumulative-delete tally. Pulled out of run() to keep that
// function under the 15-statement linter cap and to make the per-tick
// logic unit-testable in isolation.
func (m *Maintenance) maybeSweep(ctx context.Context, lastRun time.Time, cum int64) (time.Time, int64) {
	now := m.opts.Now()
	days := m.opts.Retention()
	if !ShouldRunRetentionTick(lastRun, now, m.opts.RetentionIntervalHours, days) {
		return lastRun, cum
	}
	cutoff := RetentionCutoff(now, days)
	deleted, err := m.opts.Pruner(ctx, cutoff)
	if m.opts.OnSweep != nil {
		m.opts.OnSweep(deleted, err)
	}
	if err != nil {
		return lastRun, cum // re-arm: do not bump lastRun
	}
	cum = m.maybeAnalyze(ctx, cum+deleted)
	return now, cum
}

// maybeAnalyze runs the Analyzer (when configured) once cumulative
// deletes cross AnalyzeThresholdRows, then resets the counter. When
// no Analyzer is configured the counter is left unchanged so a future
// reconfiguration still benefits from accumulated history.
func (m *Maintenance) maybeAnalyze(ctx context.Context, cum int64) int64 {
	if m.opts.Analyzer == nil || cum < AnalyzeThresholdRows {
		return cum
	}
	err := m.opts.Analyzer(ctx)
	if m.opts.OnAnalyze != nil {
		m.opts.OnAnalyze(cum, err)
	}
	if err != nil {
		return cum // keep the tally so the next sweep retries
	}
	return 0
}
