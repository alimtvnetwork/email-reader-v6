// watch.go is the `core.Watch` service: a single source of truth for
// which aliases are currently being watched. It owns a `runners` map
// keyed by Alias, manages goroutine lifecycle (Start spawns, Stop
// cancels), and fans out `WatchEvent`s via an `eventbus.Publisher`.
//
// This first slice ships the service shell — runners map, start/stop
// pipeline, event bus, and the `LoopFactory` seam. Wiring the real
// `internal/watcher.Run` behind `LoopFactory` lands in a follow-up
// slice (it requires threading `*config.Account`, `*rules.Engine`,
// `*browser.Launcher`, `*store.Store` through the call site, which is
// out of scope here). Tests use a stub LoopFactory.
//
// Spec: spec/21-app/02-features/05-watch/01-backend.md §1–§4.
package core

import (
	"context"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/eventbus"
)

// WatchEventKind discriminates lifecycle + runtime events emitted on
// the Watch bus. Mirrors the spec §4 enum (Start / Stop / Error /
// Heartbeat); low-level watcher.Bus signals are folded onto this
// stream by core.BridgeWatcherBus (see watch_bridge.go).
//
// **Slice #107 extension** — added two business-event kinds
// (`WatchEmailStored`, `WatchRuleMatched`) so the Dashboard
// `RecentActivity` feed can surface user-visible audit entries
// instead of only watcher-lifecycle pings. The bridge promotes the
// corresponding `watcher.Event` kinds onto these new values; the
// activity adapter (`dashboard_activity_source.go`) maps them to the
// spec `ActivityEmailStored` / `ActivityRuleMatched` strings.
//
// **Why an extension is safe** — `WatchEventKind` is `uint8`, the
// integer is persisted to `WatchEvents.Kind`, and the
// `mapWatchKindToActivityKind` adapter already drops unknown kinds
// (forward-compat). Older binaries reading a DB written by a newer
// binary therefore see *fewer* activity rows, never a crash.
type WatchEventKind uint8

const (
	WatchStart       WatchEventKind = 1
	WatchStop        WatchEventKind = 2
	WatchError       WatchEventKind = 3
	WatchHeartbeat   WatchEventKind = 4
	WatchEmailStored WatchEventKind = 5 // spec ActivityEmailStored — one per persisted message
	WatchRuleMatched WatchEventKind = 6 // spec ActivityRuleMatched — one per rule hit
)

// String returns the canonical log form.
func (k WatchEventKind) String() string {
	switch k {
	case WatchStart:
		return "Start"
	case WatchStop:
		return "Stop"
	case WatchError:
		return "Error"
	case WatchHeartbeat:
		return "Heartbeat"
	case WatchEmailStored:
		return "EmailStored"
	case WatchRuleMatched:
		return "RuleMatched"
	}
	return "Unknown"
}

// WatchEvent is the published payload. Consumers switch on Kind.
type WatchEvent struct {
	Kind    WatchEventKind
	Alias   string
	At      time.Time
	Message string
	Err     error // populated when Kind == WatchError
}

// WatchOptions is the request shape for Start. PollSeconds is the
// initial cadence; live updates flow through Settings.Subscribe (the
// CF-W1 contract — out of scope for this slice).
type WatchOptions struct {
	Alias       string
	PollSeconds int
}

// LoopFactory is the seam over `internal/watcher.Run`. The real
// factory wraps `watcher.Options` + `watcher.Run`; tests inject a stub
// that records calls. Each Loop gets its own goroutine spawned by
// Watch; Loop.Run blocks until ctx is cancelled.
type LoopFactory interface {
	New(opts WatchOptions) Loop
}

// Loop is the runnable produced by LoopFactory.New. Implementations
// MUST honour ctx.Done() and return promptly on cancellation.
type Loop interface {
	Run(ctx context.Context) error
}

// runner pairs a Loop with its cancel func + the goroutine done-chan
// so Stop can both cancel ctx AND wait for clean shutdown.
type runner struct {
	loop   Loop
	cancel context.CancelFunc
	done   chan struct{}
}

// Watch is the service. Concurrency: `mu` guards `runners`; the bus
// has its own internal locking. Never iterate `runners` while holding
// `mu` for write — Stop's goroutine-wait is done outside the lock.
type Watch struct {
	loop LoopFactory
	bus  *eventbus.Bus[WatchEvent]
	now  func() time.Time

	mu      sync.RWMutex
	runners map[string]*runner
}

// WatcherEventPublisher is the narrow bridge back into the low-level
// watcher.Bus. core.Watch itself still owns lifecycle state, but the
// desktop Raw log subscribes to watcher.Event, not WatchEvent; publishing
// the immediate Start/Stop/Error mirrors there guarantees the user sees a
// line even when the real IMAP loop has not reached its first tick yet.
type WatcherEventPublisher interface {
	PublishWatcherLifecycle(kind string, alias string, at time.Time, err error)
}

// NewWatch constructs a Watch. `now` is injectable for deterministic
// timestamps in tests (production passes `time.Now`). `loop` and
// `bus` are required; nil triggers ER-COR-21701.
func NewWatch(lf LoopFactory, bus *eventbus.Bus[WatchEvent], now func() time.Time) errtrace.Result[*Watch] {
	if lf == nil || bus == nil || now == nil {
		return errtrace.Err[*Watch](errtrace.NewCoded(errtrace.ErrCoreInvalidArgument, "NewWatch: nil dep"))
	}
	return errtrace.Ok(&Watch{
		loop: lf, bus: bus, now: now, runners: map[string]*runner{},
	})
}

// Start launches the watch loop for `opts.Alias`. Returns
// ErrWatchAlreadyStarted if a runner exists; ErrWatchAliasRequired if
// the alias is empty. Publishes a WatchStart event on success.
func (w *Watch) Start(ctx context.Context, opts WatchOptions) errtrace.Result[struct{}] {
	if opts.Alias == "" {
		return errtrace.Err[struct{}](errtrace.NewCoded(errtrace.ErrWatchAliasRequired, "alias empty"))
	}
	if err := w.reserveRunner(opts.Alias); err != nil {
		return errtrace.Err[struct{}](err)
	}
	w.publishLifecycle(WatchStart, opts.Alias, nil, "watch started")
	w.spawnRunner(ctx, opts)
	return errtrace.Ok(struct{}{})
}

// reserveRunner inserts a placeholder under mu.Lock so a concurrent
// Start for the same alias loses cleanly. The placeholder's loop /
// cancel / done are filled in by spawnRunner — callers MUST follow
// reserve+spawn in lock-step.
func (w *Watch) reserveRunner(alias string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.runners[alias]; ok {
		return errtrace.NewCoded(errtrace.ErrWatchAlreadyStarted, "alias "+alias+" already watched")
	}
	w.runners[alias] = &runner{}
	return nil
}

// spawnRunner builds the Loop, fills the placeholder, and launches the
// goroutine. The goroutine signals `done` on exit so Stop can wait.
func (w *Watch) spawnRunner(parent context.Context, opts WatchOptions) {
	loopCtx, cancel := context.WithCancel(parent)
	loop := w.loop.New(opts)
	done := make(chan struct{})
	w.mu.Lock()
	r := w.runners[opts.Alias]
	r.loop, r.cancel, r.done = loop, cancel, done
	w.mu.Unlock()
	go w.runLoop(loopCtx, opts.Alias, loop, done)
}

// runLoop runs the loop and surfaces any non-nil terminal error as a
// WatchError event before signalling `done`.
func (w *Watch) runLoop(ctx context.Context, alias string, loop Loop, done chan struct{}) {
	defer close(done)
	if err := loop.Run(ctx); err != nil && ctx.Err() == nil {
		w.publishLifecycle(WatchError, alias, err, "loop exited with error")
	}
}

// Stop cancels the runner for `alias` and waits up to `timeout` for
// graceful shutdown. Returns ErrWatchNotRunning if no runner exists.
// Always publishes WatchStop after cancel — even if waitOrTimeout
// exceeds the budget — because the cancel signal is already sent and
// the runner will exit imminently. Without this guarantee the UI's
// Subscribe loop never sees the lifecycle event when a poll happens
// to be mid-dial at Stop time, leaving the header stuck on "Watching".
func (w *Watch) Stop(alias string, timeout time.Duration) errtrace.Result[struct{}] {
	r, err := w.takeRunner(alias)
	if err != nil {
		return errtrace.Err[struct{}](err)
	}
	r.cancel()
	waitErr := waitOrTimeout(r.done, timeout)
	w.publishLifecycle(WatchStop, alias, nil, "watch stopped")
	if waitErr != nil {
		return errtrace.Err[struct{}](errtrace.WrapCode(waitErr, errtrace.ErrWatcherShutdown, "Stop "+alias))
	}
	return errtrace.Ok(struct{}{})
}

// takeRunner removes and returns the runner under mu.Lock. Releases
// the lock before any blocking wait so Start/Stop on other aliases
// stay responsive.
func (w *Watch) takeRunner(alias string) (*runner, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, ok := w.runners[alias]
	if !ok {
		return nil, errtrace.NewCoded(errtrace.ErrWatchNotRunning, "alias "+alias+" not watched")
	}
	delete(w.runners, alias)
	return r, nil
}

// IsRunning reports whether `alias` currently has an active runner.
func (w *Watch) IsRunning(alias string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.runners[alias]
	return ok
}

// List returns the set of currently-watched aliases (snapshot, sorted
// in insertion order is NOT guaranteed). Useful for the UI's status
// chip and for the CLI's `email-read watch ls`.
func (w *Watch) List() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, 0, len(w.runners))
	for alias := range w.runners {
		out = append(out, alias)
	}
	return out
}

// Subscribe returns a buffered receive channel for WatchEvents and an
// unsubscribe func. Always call cancel to release the slot.
func (w *Watch) Subscribe() (<-chan WatchEvent, func()) {
	return w.bus.Subscribe()
}

// publish stamps `At` if missing then forwards to the bus.
func (w *Watch) publish(ev WatchEvent) {
	if ev.At.IsZero() {
		ev.At = w.now()
	}
	w.bus.Publish(ev)
}

func (w *Watch) publishLifecycle(kind WatchEventKind, alias string, err error, msg string) {
	ev := WatchEvent{Kind: kind, Alias: alias, Err: err, Message: msg}
	w.publish(ev)
	if p, ok := w.loop.(WatcherEventPublisher); ok {
		p.PublishWatcherLifecycle(kind.String(), alias, ev.At, err)
	}
}

// waitOrTimeout blocks until `done` closes or `timeout` elapses. A
// zero/negative timeout means "wait forever".
func waitOrTimeout(done <-chan struct{}, timeout time.Duration) error {
	if timeout <= 0 {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errtrace.NewCoded(errtrace.ErrWatcherShutdown, "shutdown timed out")
	}
}
