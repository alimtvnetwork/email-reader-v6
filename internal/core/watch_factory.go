// Package core — watch_factory.go wires the LoopFactory seam to the
// real `internal/watcher.Run` poll loop. The Watch service (watch.go)
// owns lifecycle (Start/Stop, runners map, eventbus); this file owns
// dependency assembly: resolve alias → config.Account, hand the watcher
// its Engine / Launcher / Store / Logger / Bus, and adapt the
// fire-and-forget `watcher.Run` signature to the `Loop` interface.
//
// Why a separate file: keeps watch.go free of concrete watcher/config
// imports so unit tests can stay light. Real dependencies enter here
// only — and only when the app boots (cmd/app, cmd/cli).
package core

import (
	"context"
	"log"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

// AccountResolver returns a snapshot of the account for `alias`. The
// resolver is called once per Start (NOT per poll) so a Settings reload
// between Start calls picks up edits to host / port / credentials. The
// snapshot is then frozen for the runner's lifetime — restart the
// runner to re-resolve. Returns nil on miss; the factory translates
// that to ER-WCH-21412.
type AccountResolver func(alias string) *config.Account

// PollChanProvider is the optional CF-W1 seam: the factory asks for a
// per-alias receive channel at Start time, and releases it when the
// runner exits. The provider's job is to fan Settings cadence updates
// into every live runner's channel. Two methods so the provider can
// distinguish "runner started" (allocate / register) from "runner
// stopped" (free the slot, never publish to it again).
//
// Why an interface (vs a func): Acquire-only would leak channels on
// Stop because the factory cannot tell the provider when to forget.
// Release closes the loop without forcing the watcher loop to call
// `close()` on a channel it does not own (which would race with the
// provider's fan-out goroutine).
//
// Nil provider is fine — the factory then passes a nil
// `PollSecondsCh` to watcher.Run, preserving the pre-CF-W1 behaviour.
type PollChanProvider interface {
	Acquire(alias string) <-chan int
	Release(alias string)
}

// RealLoopFactoryDeps bundles the long-lived collaborators that every
// runner shares: the rules engine, browser launcher, persistent store,
// shared watcher event bus, and a logger. Engine + Launcher are passed
// by pointer so live config reloads upstream propagate without
// re-wiring runners. Bus may be nil (CLI mode); Logger nil falls back
// to a discard logger inside Run. PollChans is the optional CF-W1
// provider — see PollChanProvider.
type RealLoopFactoryDeps struct {
	Resolver  AccountResolver
	Engine    *rules.Engine
	Launcher  *browser.Launcher
	Store     *store.Store
	Bus       *watcher.Bus
	Logger    *log.Logger
	Verbose   bool
	PollChans PollChanProvider
}

// NewRealLoopFactory validates `deps` (Resolver + Store are mandatory —
// a runner without persistence or alias lookup would silently no-op)
// and returns a LoopFactory. Engine + Launcher are intentionally
// optional so the watcher can run in "save only" or "match only" modes
// per its own banner contract (see watcher.logBannerRules /
// logBannerBrowser).
func NewRealLoopFactory(deps RealLoopFactoryDeps) errtrace.Result[LoopFactory] {
	if deps.Resolver == nil || deps.Store == nil {
		return errtrace.Err[LoopFactory](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument,
			"NewRealLoopFactory: Resolver and Store are required",
		))
	}
	return errtrace.Ok[LoopFactory](&realLoopFactory{deps: deps})
}

// realLoopFactory is the production LoopFactory: New() resolves the
// alias and produces a realLoop. Resolution failures are deferred to
// Loop.Run so that Watch.Start can still register the runner — the
// goroutine then exits with the typed error and Watch publishes a
// WatchError event (matching the spec'd error-surfacing path).
type realLoopFactory struct {
	deps RealLoopFactoryDeps
}

// PublishWatcherLifecycle mirrors core.Watch lifecycle events onto the
// low-level watcher.Bus that powers the desktop Raw log tab. watcher.Run
// also publishes started/stopped once it enters/exits, but this immediate
// mirror covers fast-fail paths and validates that the Raw-log subscriber is
// alive the instant the user clicks Start.
func (f *realLoopFactory) PublishWatcherLifecycle(kind string, alias string, at time.Time, err error) {
	if f.deps.Bus == nil {
		return
	}
	f.deps.Bus.Publish(watcherLifecycleEvent(kind, alias, at, err))
}

func watcherLifecycleEvent(kind string, alias string, at time.Time, err error) watcher.Event {
	switch kind {
	case WatchStart.String():
		return watcher.Event{Kind: watcher.EventStarted, Alias: alias, At: at}
	case WatchStop.String():
		return watcher.Event{Kind: watcher.EventStopped, Alias: alias, At: at}
	case WatchError.String():
		return watcher.Event{Kind: watcher.EventPollError, Alias: alias, At: at, Err: err}
	}
	return watcher.Event{Kind: watcher.EventHeartbeat, Alias: alias, At: at}
}

// New builds the per-runner Loop. We resolve the account here (cheap,
// non-blocking) so a missing alias surfaces as soon as Run starts
// rather than at first poll tick.
func (f *realLoopFactory) New(opts WatchOptions) Loop {
	acct := f.deps.Resolver(opts.Alias)
	return &realLoop{
		opts: opts,
		acct: acct,
		deps: f.deps,
	}
}

// realLoop adapts watcher.Run to the core.Loop interface. It carries
// the resolved account snapshot + shared deps; Run is the single
// blocking entrypoint that the Watch goroutine waits on.
type realLoop struct {
	opts WatchOptions
	acct *config.Account
	deps RealLoopFactoryDeps
}

// Run blocks until ctx is cancelled or watcher.Run returns. A nil
// account (alias not found at New time) is surfaced as
// ErrWatchAccountNotFound so Watch.runLoop publishes a typed
// WatchError event instead of a generic shutdown message.
//
// CF-W1: when a PollChanProvider is wired, we Acquire a per-alias
// channel before the loop runs and defer Release after watcher.Run
// returns — this is the precise lifetime the provider needs to know
// to stop publishing into a dead channel.
func (l *realLoop) Run(ctx context.Context) error {
	if l.acct == nil {
		return errtrace.NewCoded(
			errtrace.ErrWatchAccountNotFound,
			"watch: account "+l.opts.Alias+" not found",
		)
	}
	var pollCh <-chan int
	if l.deps.PollChans != nil {
		pollCh = l.deps.PollChans.Acquire(l.opts.Alias)
		defer l.deps.PollChans.Release(l.opts.Alias)
	}
	return watcher.Run(ctx, watcher.Options{
		Account:       *l.acct,
		PollSeconds:   l.opts.PollSeconds,
		Engine:        l.deps.Engine,
		Launcher:      l.deps.Launcher,
		Store:         l.deps.Store,
		Logger:        l.deps.Logger,
		Verbose:       l.deps.Verbose,
		Bus:           l.deps.Bus,
		PollSecondsCh: pollCh,
	})
}
