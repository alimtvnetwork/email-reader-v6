// watch_runtime.go owns the process-wide *core.Watch singleton and
// the long-lived collaborators it depends on (config snapshot, store,
// rules engine, browser launcher, watcher event bus). The whole graph
// is built lazily on first access so headless tooling and the CLI —
// which never touch the UI — pay nothing.
//
// Why a singleton: the Watch service owns goroutines and a runners
// map, both of which MUST be shared across views. A second Watch
// would happily start a second IMAP poll loop for the same alias and
// double the network traffic / DB writes. One Watch per process is
// the contract; this file enforces it.
//
// Build-tag-free on purpose: every dep here is framework-agnostic so
// headless CI (-tags nofyne) can still link and unit-test the
// runtime. The Fyne wiring lives in app.go behind the !nofyne tag.
package ui

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/eventbus"
	"github.com/lovable/email-read/internal/rules"
	"github.com/lovable/email-read/internal/store"
	"github.com/lovable/email-read/internal/watcher"
)

// WatchRuntime bundles the singleton *core.Watch plus the deps that
// must outlive any single view. Held by a package-level pointer
// (watchRuntimeSingleton) and built once via sync.Once.
type WatchRuntime struct {
	Watch        *core.Watch
	Bus          *watcher.Bus
	Store        *store.Store
	Settings     *core.Settings
	PollChans    *core.PollChanRegistry
	cfgMu        sync.RWMutex
	cfg          *config.Config
	pollSecondsF func() int
	closers      []func() error
}

// PollSeconds returns the current cadence honoured by new Start
// calls. Reads the latest Settings snapshot (live) so the user does
// not have to restart the app after editing it. Falls back to the
// boot-time config value if Settings is unavailable.
func (rt *WatchRuntime) PollSeconds() int {
	if rt.pollSecondsF != nil {
		return rt.pollSecondsF()
	}
	rt.cfgMu.RLock()
	defer rt.cfgMu.RUnlock()
	if rt.cfg == nil {
		return 3
	}
	return rt.cfg.Watch.PollSeconds
}

// Close drains the closers stack in reverse insertion order. Safe to
// call multiple times — closers self-clear after the first call. Used
// by Run() on shutdown so the SQLite store flushes cleanly.
func (rt *WatchRuntime) Close() {
	for i := len(rt.closers) - 1; i >= 0; i-- {
		if err := rt.closers[i](); err != nil {
			log.Printf("ui: watch runtime close[%d]: %v", i, err)
		}
	}
	rt.closers = nil
}

var (
	watchRuntimeOnce      sync.Once
	watchRuntimeSingleton *WatchRuntime
	watchRuntimeErr       error
)

// WatchRuntimeOrNil returns the lazily-built singleton, or nil on
// setup failure (logged once, never re-attempted — matches the
// project's "degrade to read-only UI" stance for backend init
// failures). Callers MUST handle nil; the Watch view falls back to its
// pre-existing placeholder Start button when nil is returned.
func WatchRuntimeOrNil() *WatchRuntime {
	watchRuntimeOnce.Do(func() {
		rt, err := buildWatchRuntime(context.Background())
		if err != nil {
			log.Printf("ui: watch runtime: %v (Start/Stop disabled)", err)
			watchRuntimeErr = err
			return
		}
		watchRuntimeSingleton = rt
	})
	return watchRuntimeSingleton
}

// buildWatchRuntime assembles every dep, in the order CLI's runWatch
// uses, and wires them into a *core.Watch. Errors here are
// non-fatal for the UI process: the caller logs and keeps running.
func buildWatchRuntime(ctx context.Context) (*WatchRuntime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errtrace.Wrap(err, "load config")
	}
	st, err := store.Open()
	if err != nil {
		return nil, errtrace.Wrap(err, "open store")
	}
	rt := &WatchRuntime{
		Store:   st,
		cfg:     cfg,
		closers: []func() error{st.Close},
	}
	if err := attachRuntimeServices(ctx, rt); err != nil {
		rt.Close()
		return nil, err
	}
	return rt, nil
}

// attachRuntimeServices builds Settings + the watcher.Bus + the real
// LoopFactory, then constructs Watch. Split out of buildWatchRuntime
// to keep the parent function under the project-wide 15-line cap.
func attachRuntimeServices(ctx context.Context, rt *WatchRuntime) error {
	settings := initSettings(rt)
	rt.Settings = settings

	bus := watcher.NewBus(64)
	rt.Bus = bus
	rt.PollChans = core.NewPollChanRegistry()

	engine, launcher := buildEngineAndLauncher(rt.cfg)
	if settings != nil {
		startSettingsBridge(ctx, settings, launcher, rt)
	}

	resolver := func(alias string) *config.Account {
		rt.cfgMu.RLock()
		defer rt.cfgMu.RUnlock()
		return rt.cfg.FindAccount(alias)
	}
	lfRes := core.NewRealLoopFactory(core.RealLoopFactoryDeps{
		Resolver:  resolver,
		Engine:    engine,
		Launcher:  launcher,
		Store:     rt.Store,
		Bus:       bus,
		Logger:    log.New(os.Stdout, "watch ", log.LstdFlags),
		Verbose:   false,
		PollChans: rt.PollChans,
	})
	if lfRes.HasError() {
		return errtrace.Wrap(lfRes.Error(), "build loop factory")
	}
	wRes := core.NewWatch(lfRes.Value(), eventbus.New[core.WatchEvent](32), time.Now)
	if wRes.HasError() {
		return errtrace.Wrap(wRes.Error(), "build watch")
	}
	rt.Watch = wRes.Value()
	return nil
}

// initSettings constructs the Settings client. Returns nil on failure
// so the Settings → cadence/launcher bridge is silently skipped (the
// pre-Settings baseline cadence still works).
func initSettings(rt *WatchRuntime) *core.Settings {
	res := core.NewSettings(time.Now)
	if res.HasError() {
		log.Printf("ui: watch runtime: settings init: %v", res.Error())
		return nil
	}
	return res.Value()
}

// buildEngineAndLauncher mirrors the CLI helper of the same intent.
// Engine + launcher errors are logged, never fatal — the watcher's own
// banner reports degraded modes.
func buildEngineAndLauncher(cfg *config.Config) (*rules.Engine, *browser.Launcher) {
	engine, ruleErr := rules.New(cfg.Rules)
	if ruleErr != nil {
		log.Printf("ui: watch runtime: rules: %v", ruleErr)
	}
	launcher := browser.New(cfg.Browser)
	if _, err := launcher.Path(); err != nil {
		log.Printf("ui: watch runtime: browser: %v (URLs will be skipped)", err)
	}
	return engine, launcher
}

// startSettingsBridge subscribes to Settings ONCE and fans events to
// the launcher (CF-T1) and the cadence accessor (CF-W1, applied on
// the NEXT Start since live in-loop reload is wired in a follow-up
// slice). Mirrors the CLI's startReloadBridges shape so behaviour is
// consistent across surfaces.
func startSettingsBridge(ctx context.Context, s *core.Settings, launcher *browser.Launcher, rt *WatchRuntime) {
	events, cancel := s.Subscribe(ctx)
	rt.closers = append(rt.closers, func() error { cancel(); return nil })

	var liveMu sync.RWMutex
	livePoll := rt.cfg.Watch.PollSeconds
	rt.pollSecondsF = func() int {
		liveMu.RLock()
		defer liveMu.RUnlock()
		return livePoll
	}
	go func() {
		for ev := range events {
			liveMu.Lock()
			livePoll = int(ev.Snapshot.PollSeconds)
			liveMu.Unlock()
			if launcher != nil {
				launcher.Reload(config.Browser{
					ChromePath:   ev.Snapshot.BrowserOverride.ChromePath,
					IncognitoArg: ev.Snapshot.BrowserOverride.IncognitoArg,
				})
			}
		}
	}()
}
