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
	Maintenance  *core.Maintenance
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
	rt.Settings = initSettings(rt)
	rt.Bus = watcher.NewBus(64)
	rt.PollChans = core.NewPollChanRegistry()
	engine, launcher := buildEngineAndLauncher(rt.cfg)
	if rt.Settings != nil {
		startSettingsBridge(ctx, rt.Settings, launcher, rt)
	}
	lf, err := buildLoopFactory(rt, engine, launcher)
	if err != nil {
		return err
	}
	if err := attachWatchAndBridge(ctx, rt, lf); err != nil {
		return err
	}
	startMaintenance(ctx, rt)
	return nil
}

// startMaintenance spawns the OpenedUrls retention sweeper goroutine
// when both Settings and Store are wired. Failures are logged and
// non-fatal — the rest of the runtime stays functional, the audit
// table just grows unbounded until restart.
func startMaintenance(ctx context.Context, rt *WatchRuntime) {
	if rt.Settings == nil || rt.Store == nil {
		return
	}
	res := core.NewMaintenance(maintenanceOptionsFor(ctx, rt))
	if res.HasError() {
		log.Printf("ui: watch runtime: maintenance init: %v", res.Error())
		return
	}
	rt.Maintenance = res.Value()
	rt.Maintenance.Start(ctx)
	rt.closers = append(rt.closers, func() error {
		rt.Maintenance.Stop(2 * time.Second)
		return nil
	})
}

// maintenanceOptionsFor wires every store seam — prune, analyze,
// vacuum, vacuum-gate, wal-checkpoint — and the matching log
// observers. Split out of startMaintenance to stay under the
// 15-statement linter cap and to keep all maintenance plumbing in one
// place. Spec/23-app-database/04 §2.
func maintenanceOptionsFor(ctx context.Context, rt *WatchRuntime) core.MaintenanceOptions {
	vacuumGate := func(ctx context.Context) (bool, error) {
		fl, pages, err := rt.Store.FreelistRatio(ctx)
		if err != nil {
			return false, err
		}
		return store.ShouldVacuum(fl, pages), nil
	}
	pruner := func(ctx context.Context, cutoff time.Time) (int64, error) {
		batch := pruneBatchFromSettings(ctx, rt.Settings)
		n, _, err := rt.Store.PruneOpenedUrlsBeforeBatched(ctx, cutoff, batch)
		return n, err
	}
	snap := snapshotForMaintenance(ctx, rt.Settings)
	return core.MaintenanceOptions{
		Pruner:             pruner,
		Analyzer:           rt.Store.Analyze,
		Vacuumer:           rt.Store.Vacuum,
		VacuumGate:         vacuumGate,
		WalCheckpointer:    rt.Store.WalCheckpointTruncate,
		Retention:          func() uint16 { return retentionFromSettings(ctx, rt.Settings) },
		VacuumWeekday:      snap.WeeklyVacuumOn,
		VacuumHourLocal:    int(snap.WeeklyVacuumHourLocal),
		WalCheckpointHours: int(snap.WalCheckpointHours),
		OnSweep:            logRetentionSweep,
		OnAnalyze:          logAnalyzeRun,
		OnVacuum:           logVacuumRun,
		OnWalCheckpoint:    logWalCheckpoint,
	}
}

// pruneBatchFromSettings reads the live PruneBatchSize knob. Errors fall
// back to store.DefaultPruneBatchSize so a transient Settings hiccup
// cannot wedge the sweeper.
func pruneBatchFromSettings(ctx context.Context, s *core.Settings) int {
	r := s.Get(ctx)
	if r.HasError() {
		return store.DefaultPruneBatchSize
	}
	v := int(r.Value().PruneBatchSize)
	if v <= 0 {
		return store.DefaultPruneBatchSize
	}
	return v
}

// snapshotForMaintenance returns the live Settings snapshot. Errors fall
// back to DefaultSettingsInput-shaped defaults so the maintenance loop
// always has sane scheduling knobs.
func snapshotForMaintenance(ctx context.Context, s *core.Settings) core.SettingsSnapshot {
	r := s.Get(ctx)
	if r.HasError() {
		d := core.DefaultSettingsInput()
		return core.SettingsSnapshot{
			WeeklyVacuumOn:        d.WeeklyVacuumOn,
			WeeklyVacuumHourLocal: d.WeeklyVacuumHourLocal,
			WalCheckpointHours:    d.WalCheckpointHours,
		}
	}
	return r.Value()
}

// retentionFromSettings reads the live snapshot's retention knob.
// Errors fall back to 0 (disabled) so a transient Settings hiccup
// cannot cause an aggressive over-prune.
func retentionFromSettings(ctx context.Context, s *core.Settings) uint16 {
	r := s.Get(ctx)
	if r.HasError() {
		return 0
	}
	return r.Value().OpenUrlsRetentionDays
}

// buildLoopFactory wires the resolver closure + RealLoopFactoryDeps and
// returns the constructed factory. Resolver reads `rt.cfg` under the
// runtime's RWMutex so a future config-reload path can swap accounts
// safely between Start calls.
func buildLoopFactory(rt *WatchRuntime, engine *rules.Engine, launcher *browser.Launcher) (core.LoopFactory, error) {
	resolver := func(alias string) *config.Account {
		rt.cfgMu.RLock()
		defer rt.cfgMu.RUnlock()
		return rt.cfg.FindAccount(alias)
	}
	res := core.NewRealLoopFactory(core.RealLoopFactoryDeps{
		Resolver:  resolver,
		Engine:    engine,
		Launcher:  launcher,
		Store:     rt.Store,
		Bus:       rt.Bus,
		Logger:    log.New(os.Stdout, "watch ", log.LstdFlags),
		Verbose:   false,
		PollChans: rt.PollChans,
	})
	if res.HasError() {
		return nil, errtrace.Wrap(res.Error(), "build loop factory")
	}
	return res.Value(), nil
}

// attachWatchAndBridge constructs core.Watch with its own destination
// event bus and wires the watcher.Bus → WatchEvent bridge so a single
// `core.Watch.Subscribe()` returns the unified stream (lifecycle +
// runtime signals). The bridge stop is deferred to runtime Close.
func attachWatchAndBridge(ctx context.Context, rt *WatchRuntime, lf core.LoopFactory) error {
	dstBus := eventbus.New[core.WatchEvent](32)
	wRes := core.NewWatch(lf, dstBus, time.Now)
	if wRes.HasError() {
		return errtrace.Wrap(wRes.Error(), "build watch")
	}
	rt.Watch = wRes.Value()
	stopBridge := core.BridgeWatcherBus(ctx, rt.Bus, dstBus)
	rt.closers = append(rt.closers, func() error { stopBridge(); return nil })
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
// three consumers: (1) the cadence accessor used by new Start calls,
// (2) the per-runner PollChans registry that pushes the new cadence
// into every LIVE runner (CF-W1, mid-loop), and (3) the launcher's
// `Reload` for browser overrides (CF-T1). Mirrors the CLI's
// `startReloadBridges` shape so behaviour is consistent across
// surfaces.
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
	go forwardSettingsEvents(events, launcher, rt, &liveMu, &livePoll)
}

// forwardSettingsEvents drains the Settings event stream and applies
// each snapshot. Extracted from startSettingsBridge so the parent
// stays under the 15-statement cap and so the loop body is reusable
// from future tests.
func forwardSettingsEvents(events <-chan core.SettingsEvent, launcher *browser.Launcher, rt *WatchRuntime, liveMu *sync.RWMutex, livePoll *int) {
	for ev := range events {
		liveMu.Lock()
		*livePoll = int(ev.Snapshot.PollSeconds)
		liveMu.Unlock()
		if rt.PollChans != nil {
			rt.PollChans.Broadcast(int(ev.Snapshot.PollSeconds))
		}
		if launcher != nil {
			launcher.Reload(config.Browser{
				ChromePath:   ev.Snapshot.BrowserOverride.ChromePath,
				IncognitoArg: ev.Snapshot.BrowserOverride.IncognitoArg,
			})
		}
	}
}
