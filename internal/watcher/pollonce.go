// pollonce.go — exported one-shot poll API for the watcher.
//
// Why this exists
//   `core.EmailsService.Refresh` (P4.4) takes a `Refresher` interface
//   with a single `PollOnce(ctx, alias) error` method. That seam was
//   landed without a production implementation because watcher only
//   exposed the long-running `Run` loop. This file is the production
//   adapter — the missing one-line bootstrap wire-up.
//
// What it does NOT do
//   - Does NOT replace `Run`. The long-running loop stays the source
//     of truth for cadence, back-off, heartbeats, live-reload, and
//     event publishing. `PollOnce` is a manual, on-demand "drain
//     once" — same per-cycle work, no scheduling.
//   - Does NOT manage a connection pool or persistent client. Each
//     PollOnce call dials, selects, fetches, persists, and closes,
//     identical to one tick of `Run`. This matches `Run`'s own
//     per-tick lifecycle so behavior is consistent whether the user
//     waits for the next scheduled tick or hits "🔄 Refresh".
//
// Why a `Watcher` registry type (vs. exported func)
//   Refresher.PollOnce takes an alias string only — no Options. The
//   adapter has to translate alias → the right Options instance
//   (Account, Engine, Launcher, Store, Bus, …) so the existing
//   per-alias `pollOnce` machinery works unchanged. A registry struct
//   indexed by alias is the smallest abstraction that does that.
//
// Concurrency
//   Read-only after `Register`. The expected bootstrap pattern is to
//   build the Watcher once, register every account before any UI
//   click can fire, and then never mutate. RegisterMu protects the
//   map regardless so unit tests that register/lookup concurrently
//   stay race-clean.

package watcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
)

// Watcher is the alias-indexed Refresher implementation that satisfies
// `core.Refresher`. Build one at bootstrap, Register each account's
// Options on it, then pass it to `(*core.EmailsService).WithRefresher`.
type Watcher struct {
	mu       sync.RWMutex
	accounts map[string]Options
}

// NewWatcher constructs an empty registry.
func NewWatcher() *Watcher {
	return &Watcher{accounts: make(map[string]Options)}
}

// Register stores the per-alias Options. Subsequent calls for the
// same alias overwrite (intentional — supports live config reloads).
//
// Returns an error only if the Options is missing the Account.Alias
// field (defensive; the watcher's own runtime would also fail).
func (w *Watcher) Register(opts Options) error {
	alias := opts.Account.Alias
	if alias == "" {
		return fmt.Errorf("watcher.Register: Options.Account.Alias is empty")
	}
	w.mu.Lock()
	w.accounts[alias] = opts
	w.mu.Unlock()
	return nil
}

// PollOnce satisfies `core.Refresher`. Looks up the alias's Options
// and runs exactly one poll cycle (the same `pollOnce` the runLoop
// uses on every tick). Returns the underlying error unwrapped — the
// caller (`core.EmailsService.Refresh`) wraps it with the
// `ErrWatcherPollCycle` registry code.
//
// Logger handling: we pass the registered Options.Logger straight
// through (or a discard logger if none was set) so the
// "🔄 Refresh" cycle's progress shows up in the same operator log
// as the scheduled ticks.
func (w *Watcher) PollOnce(ctx context.Context, alias string) error {
	w.mu.RLock()
	opts, ok := w.accounts[alias]
	w.mu.RUnlock()
	if !ok {
		return fmt.Errorf("watcher.PollOnce: no account registered for alias %q", alias)
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	_, err := pollOnce(ctx, opts, logger)
	return err
}
