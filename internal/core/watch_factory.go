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

// RealLoopFactoryDeps bundles the long-lived collaborators that every
// runner shares: the rules engine, browser launcher, persistent store,
// shared watcher event bus, and a logger. Engine + Launcher are passed
// by pointer so live config reloads upstream propagate without
// re-wiring runners. Bus may be nil (CLI mode); Logger nil falls back
// to a discard logger inside Run.
type RealLoopFactoryDeps struct {
	Resolver AccountResolver
	Engine   *rules.Engine
	Launcher *browser.Launcher
	Store    *store.Store
	Bus      *watcher.Bus
	Logger   *log.Logger
	Verbose  bool
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
func (l *realLoop) Run(ctx context.Context) error {
	if l.acct == nil {
		return errtrace.NewCoded(
			errtrace.ErrWatchAccountNotFound,
			"watch: account "+l.opts.Alias+" not found",
		)
	}
	return watcher.Run(ctx, watcher.Options{
		Account:     *l.acct,
		PollSeconds: l.opts.PollSeconds,
		Engine:      l.deps.Engine,
		Launcher:    l.deps.Launcher,
		Store:       l.deps.Store,
		Logger:      l.deps.Logger,
		Verbose:     l.deps.Verbose,
		Bus:         l.deps.Bus,
		// PollSecondsCh is intentionally nil here: live cadence updates
		// are out of scope for the Watch wiring slice. CF-W1 (Settings
		// → live PollSeconds) lands when Settings.Subscribe is wired
		// through Watch in a follow-up.
		PollSecondsCh: nil,
	})
}
