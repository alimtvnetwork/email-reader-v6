// app.go ties the sidebar (sidebar.go) and detail pane together. Like
// sidebar.go, this file requires the Fyne cgo backend; gate it off with
// `-tags nofyne` to compile/test the rest of the package on headless CI.
//go:build !nofyne

// Package ui hosts the Fyne desktop frontend. It is intentionally split
// from cmd/email-read-ui so internal/ui can be unit-tested with `go test`
// without needing the cgo display libs that linking the binary requires.
package ui

import (
	"context"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/ui/theme"
	"github.com/lovable/email-read/internal/ui/views"
)

// AppVersion is shown in the window title. Bumped per release in lockstep
// with cmd/email-read/main.go.
const AppVersion = "0.28.0"

// Run creates the Fyne app, builds the main window, and blocks until close.
//
// Bootstrap order matches spec/24-…/02-theme-implementation.md §5:
//  1. Construct app  →  2. Apply theme + density  →  3. Build content  →  4. Show.
//
// Theme.Apply + density restore are called BEFORE BuildShell so the very
// first paint already uses our palette + spacing (no white-flash on dark
// mode and no comfortable→compact pop on the second frame).
func Run() {
	a := app.NewWithID("dev.lovable.email-read")
	if r := theme.ApplyToFyne(loadInitialThemeMode()); r.HasError() {
		log.Printf("ui: theme apply: %v (continuing with ThemeDark)", r.Error())
	}
	theme.SetDensity(loadInitialDensity())
	ctx, cancelLive := context.WithCancel(context.Background())
	defer cancelLive()
	startThemeLiveConsumer(ctx)
	// Phase 3.5 — first-error toast notifier. Wires the
	// Fyne app's SendNotification to the errlog 0→1 transition
	// rule (badge-only after the first error in a storm). Held
	// in a package var so BuildShell can reach it for the
	// "user opened the Error Log" reset hook.
	errLogNotifier = NewErrLogNotifier(func(title, body string) {
		a.SendNotification(&fyne.Notification{Title: title, Content: body})
	})
	w := a.NewWindow("email-read · v" + AppVersion)
	w.SetContent(BuildShell(LoadAliases()))
	w.Resize(fyne.NewSize(1000, 680))
	w.CenterOnScreen()
	defer func() {
		if rt := watchRuntimeSingleton; rt != nil {
			rt.Close()
		}
	}()
	w.ShowAndRun()
}

// errLogNotifier is the process-wide first-error toast dispatcher,
// wired in Run(). nil under unit tests / headless harnesses where
// Run is never called — sidebarErrorLogReset() is nil-safe.
var errLogNotifier *ErrLogNotifier

// sidebarErrorLogReset is the OnErrorLogOpened hook handed to every
// NewSidebar call. Pulled out as a function so both BuildShell sites
// (initial build + rebuildShell) share one definition and stay in
// sync. nil-safe so the package compiles + tests run when Run() was
// never called (errLogNotifier == nil).
func sidebarErrorLogReset() {
	if errLogNotifier != nil {
		errLogNotifier.ResetQuietPeriod()
	}
}

// startThemeLiveConsumer subscribes to core.Settings and re-applies the
// theme on every Save / Reset event. Closes the round-trip on Delta #3 +
// Delta #4: a Save in the Settings UI repaints the running app without
// requiring restart. Safe no-op when Settings cannot construct (e.g. no
// config yet) — the bootstrap theme stays in effect.
//
// Spec: spec/21-app/02-features/07-settings/01-backend.md §9 (Subscribe)
//   - spec/24-app-design-system-and-ui/02-theme-implementation.md §5.
func startThemeLiveConsumer(ctx context.Context) {
	s := core.NewSettings(time.Now)
	if s.HasError() {
		log.Printf("ui: theme live consumer: settings init: %v (skipping)", s.Error())
		return
	}
	events, _ := s.Value().Subscribe(ctx)
	go forwardThemeEvents(events)
}

// forwardThemeEvents drains Settings events and re-applies the theme +
// density on every change. Channel close (via ctx cancel) terminates the
// goroutine. A no-op when the mode is unchanged — ApplyToFyne is cheap
// but SetTheme triggers a full repaint, so we skip when not needed.
// Density is updated unconditionally because theme.SetDensity is a single
// guarded int write and Size() consumers re-read on every call.
func forwardThemeEvents(events <-chan core.SettingsEvent) {
	const unset core.ThemeMode = 0 // sentinel: 0 is not a valid ThemeMode
	last := unset
	lastDensity := core.Density(0) // sentinel: 0 is not a valid core.Density
	for ev := range events {
		mode := ev.Snapshot.Theme
		if mode != last {
			last = mode
			if r := theme.ApplyToFyne(mode); r.HasError() {
				log.Printf("ui: theme live apply: %v", r.Error())
			}
		}
		density := ev.Snapshot.Density
		if density != lastDensity {
			lastDensity = density
			theme.SetDensity(coreDensityToTheme(density))
		}
	}
}

// loadInitialThemeMode reads the persisted Settings.Theme. On any error
// (no config yet, parse failure, etc.) we fall back to ThemeDark — the
// default declared by core.DefaultSettingsInput().
func loadInitialThemeMode() core.ThemeMode {
	s := core.NewSettings(time.Now)
	if s.HasError() {
		return core.ThemeDark
	}
	snap := s.Value().Get(context.Background())
	if snap.HasError() {
		return core.ThemeDark
	}
	return snap.Value().Theme
}

// loadInitialDensity reads the persisted Settings.Density. Mirrors
// loadInitialThemeMode; falls back to DensityComfortable on any error.
func loadInitialDensity() theme.Density {
	s := core.NewSettings(time.Now)
	if s.HasError() {
		return theme.DensityComfortable
	}
	snap := s.Value().Get(context.Background())
	if snap.HasError() {
		return theme.DensityComfortable
	}
	return coreDensityToTheme(snap.Value().Density)
}

// coreDensityToTheme translates the core.Density enum (Comfortable=1,
// Compact=2 — non-zero so the zero value can mean "use default" in
// normalize) into the theme.Density enum (Comfortable=0, Compact=1).
// Unknown values fall back to Comfortable so a corrupt config never
// produces an invisible UI.
func coreDensityToTheme(d core.Density) theme.Density {
	if d == core.DensityCompact {
		return theme.DensityCompact
	}
	return theme.DensityComfortable
}

// LoadAliases pulls the configured account aliases from core. Failures are
// logged (non-fatal) so the UI still opens with an empty picker.
func LoadAliases() []string {
	r := core.ListAccounts()
	if r.HasError() {
		log.Printf("ui: load accounts: %v", r.Error())
		return nil
	}
	accts := r.Value()
	out := make([]string, 0, len(accts))
	for _, a := range accts {
		out = append(out, a.Alias)
	}
	return out
}

// BuildShell returns the root container: sidebar (with account picker) on
// the left, swapping detail pane on the right. AppState lives for the life
// of the shell so views built later can subscribe to alias/nav transitions.
func BuildShell(aliases []string) fyne.CanvasObject {
	state := NewAppState()
	detail := container.NewStack()
	root := container.NewStack() // we swap the whole shell when accounts change

	// services bundles the typed Phase 2 services (Dashboard / Emails /
	// Rules). Constructed once at boot and threaded into every viewFor
	// arm — replaces the per-call buildDashboardService /
	// buildEmailsService / buildRulesService helpers from P2.3/P2.5/P2.7.
	services := BuildServices()

	// rebuildSidebar rebuilds the entire shell with a fresh aliases list —
	// used after the Add Account form saves so the picker reflects truth.
	var rebuildShell func()
	// rebuildDetail swaps the detail pane to match the current state.Nav().
	var rebuildDetail func()
	gotoNav := func(k NavKind) {
		state.SetNav(k)
		rebuildDetail()
	}
	rebuildDetail = func() {
		for _, it := range NavItems {
			if it.Kind == state.Nav() {
				detail.Objects = []fyne.CanvasObject{viewFor(it, state, services, gotoNav, rebuildShell)}
				detail.Refresh()
				return
			}
		}
	}

	rebuildShell = func() {
		freshAliases := LoadAliases()
		sidebar := NewSidebar(SidebarOptions{
			State:       state,
			Aliases:     freshAliases,
			OnSelectNav: func(item NavItem) { rebuildDetail() },
		})
		rebuildDetail()
		split := container.NewHSplit(sidebar, container.NewPadded(detail))
		split.SetOffset(0.20)
		root.Objects = []fyne.CanvasObject{split}
		root.Refresh()
	}

	// Re-render the active view if the alias changes so views always reflect
	// the currently selected account.
	state.Subscribe(func(ev AppStateEvent) {
		if ev.PrevAlias != ev.Alias {
			rebuildDetail()
		}
	})

	// Initial build using the aliases passed in (avoids double-loading).
	sidebar := NewSidebar(SidebarOptions{
		State:       state,
		Aliases:     aliases,
		OnSelectNav: func(item NavItem) { rebuildDetail() },
	})
	rebuildDetail()
	split := container.NewHSplit(sidebar, container.NewPadded(detail))
	split.SetOffset(0.20)
	root.Objects = []fyne.CanvasObject{split}
	return root
}

// viewFor returns the widget for a nav destination. Each case picks a real
// view from internal/ui/views or falls back to a placeholder for nav items
// not yet implemented. `services` carries the typed Phase 2 service bundle
// constructed once at app boot (see BuildServices in services.go).
func viewFor(item NavItem, state *AppState, services *Services, gotoNav func(NavKind), onAccountsChanged func()) fyne.CanvasObject {
	switch item.Kind {
	case NavDashboard:
		// Slices #103/#105: lazily attach production AccountHealth +
		// RecentActivity sources the first time the dashboard renders
		// with a healthy WatchRuntime. Mirrors the AttachRefresher
		// pattern below (NavEmails) — keeps boot fast and avoids
		// opening SQLite twice when the runtime owns the *store.Store.
		if rt := WatchRuntimeOrNil(); rt != nil && rt.Store != nil {
			services.AttachHealthSource(rt.Store)
			services.AttachActivitySource(rt.Store)
		}
		dashOpts := views.DashboardOptions{
			Alias:          state.Alias(),
			OnStartWatch:   func() { gotoNav(NavWatch) },
			Service:        services.Dashboard,
			HealthSource:   services.HealthSource,
			ActivitySource: services.ActivitySource,
		}
		if rt := WatchRuntimeOrNil(); rt != nil {
			dashOpts.Bus = rt.Bus
		}
		return views.BuildDashboard(dashOpts)
	case NavEmails:
		// Late-bind the production `core.Refresher` to the shared
		// EmailsService. Done here (rather than in BuildServices)
		// because the watcher runtime is built lazily on first
		// access — by the time NavEmails is rendered the registry
		// either exists or has logged its build error. Idempotent.
		if rt := WatchRuntimeOrNil(); rt != nil && rt.Refresher != nil {
			services.AttachRefresher(rt.Refresher)
		}
		return views.BuildEmails(views.EmailsOptions{
			Alias:   state.Alias(),
			Service: services.Emails,
			OpenURL: services.OpenURL, // Slice #117 (Phase 6.5): shell-injected browser launcher
		})
	case NavRules:
		return views.BuildRules(views.RulesOptions{
			Service:        services.Rules,
			OnRulesChanged: onAccountsChanged, // shared shell-rebuild trigger
		})
	case NavAccounts:
		// Slice #112: feed per-row health badges. Reuses the same
		// HealthSource the dashboard rollup uses (lazily attached on
		// first NavDashboard render) so the two surfaces are
		// guaranteed visually consistent. Lazy-attach here too so a
		// user who opens NavAccounts before NavDashboard still gets
		// badges instead of "— Unknown".
		if rt := WatchRuntimeOrNil(); rt != nil && rt.Store != nil {
			services.AttachHealthSource(rt.Store)
		}
		return views.BuildAccounts(views.AccountsOptions{
			OnAccountsChanged: onAccountsChanged,
			LoadHealth:        accountsHealthLoader(services),
		})
	case NavWatch:
		rt := WatchRuntimeOrNil()
		opts := views.WatchOptions{Alias: state.Alias()}
		if rt != nil {
			// Slice #116b (Phase 6.2): expose the singleton on the
			// Services bundle so future view code can pick it up
			// uniformly. The view itself still reads from `opts.Watch`
			// for backward-compat — the AttachWatch call only
			// populates the spec-aligned access path.
			services.AttachWatch(rt.Watch)
			opts.Watch = rt.Watch
			opts.PollSeconds = rt.PollSeconds
			opts.Bus = rt.Bus
		}
		return views.BuildWatch(opts)
	case NavTools:
		return views.BuildTools(views.ToolsOptions{
			OnAccountsChanged: onAccountsChanged,
			OnRulesChanged:    onAccountsChanged, // same shell-rebuild trigger
			RulesService:      services.Rules,
			ToolsFactory:      services.Tools, // Slice #116c (Phase 6.3)
		})
	case NavSettings:
		return views.BuildSettings(views.SettingsOptions{})
	case NavErrorLog:
		// Phase 3.3 — Diagnostics → Error Log. The view pulls from
		// the process-wide errlog ring buffer (defaults filled in
		// inside BuildErrorLog when fields are nil) and uses the
		// Fyne app's clipboard for the Copy button.
		opts := views.ErrorLogOptions{}
		if a := fyne.CurrentApp(); a != nil {
			if w := a.Driver().AllWindows(); len(w) > 0 {
				opts.Clipboard = w[0].Clipboard()
			}
		}
		return views.BuildErrorLog(opts)
	default:
		return placeholderView(item, state)
	}
}

// placeholderView renders the temporary "coming in Step N" content for nav
// items that don't have a real widget yet (only NavWatch as of v0.26.0).
func placeholderView(item NavItem, state *AppState) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(item.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	alias := "(none)"
	if state != nil && state.Alias() != "" {
		alias = state.Alias()
	}
	ctx := widget.NewLabel("Selected account: " + alias)
	body := widget.NewLabel(item.Placeholder)
	body.Wrapping = fyne.TextWrapWord
	return container.NewVBox(heading, widget.NewSeparator(), ctx, body)
}

// Phase 2.8 cleanup: the per-call buildDashboardService /
// buildEmailsService / buildRulesService helpers from P2.3/P2.5/P2.7
// have been hoisted into BuildServices (services.go), removing ~57
// lines of duplicated bootstrap glue. The `config` import in this
// file is now used only by `LoadAliases`/theme code.

// accountsHealthLoader returns the per-row health-badge feed used by
// the Accounts view (Slice #112). Routes through the existing
// `(*core.DashboardService).AccountHealth(ctx, src)` so the rollup +
// per-row data come from the exact same query — no schema drift
// between the two Dashboard surfaces.
//
// Returns nil (the view's degraded path → "— Unknown" placeholders)
// when either the Dashboard service or the HealthSource adapter is
// missing — e.g. store-open failure at boot. Never panics; never
// surfaces an error to the user (the badge is a hint, not the
// authoritative state).
func accountsHealthLoader(services *Services) func(ctx context.Context) map[string]core.HealthLevel {
	if services == nil || services.Dashboard == nil || services.HealthSource == nil {
		return nil
	}
	return func(ctx context.Context) map[string]core.HealthLevel {
		res := services.Dashboard.AccountHealth(ctx, services.HealthSource)
		if res.HasError() {
			log.Printf("ui: accountsHealthLoader: AccountHealth failed: %v", res.Error())
			return nil
		}
		out := make(map[string]core.HealthLevel, len(res.Value()))
		for _, r := range res.Value() {
			out[r.Alias] = r.Health
		}
		return out
	}
}

