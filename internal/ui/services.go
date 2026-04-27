// services.go — typed Phase 2 service bundle for the UI shell.
//
// **Phase 2.8 hoist.** Before this slice each `viewFor` switch arm
// constructed its own `*core.DashboardService` / `*core.EmailsService` /
// `*core.RulesService` via per-call `build*Service` helpers. That
// worked but cost three extra function-call hops per nav switch and
// made it impossible to share a single `*EmailsService` between the
// dashboard's email-counter dependency and the Emails view itself.
//
// The new shape constructs all three services exactly once at app
// boot, bundles them in `Services`, and threads a single `*Services`
// pointer through every `viewFor` arm. Wiring is now:
//
//	BuildServices() ──► Services{ Dashboard, Emails, Rules }
//	                    ▲           ▲          ▲      ▲
//	                    │           │          │      │
//	                  Dashboard   Emails    Rules+Tools
//
// The dashboard's `emailsCounter` dep now points at `Emails.Count`
// instead of the deprecated `core.CountEmails` package wrapper —
// closing the last legacy-wrapper call site in the UI bootstrap.

//go:build !nofyne

package ui

import (
	"context"
	"log"

	"github.com/lovable/email-read/internal/browser"
	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

// ToolsFactory builds a fresh `*core.Tools` per invocation so that
// live config edits (browser path, dedup window, allow-localhost)
// take effect without a process restart. Returned `*core.Tools`
// instances are independent — each owns its own dedup map — but
// share the same persistent store via the slim `openedUrlRecorder`
// the caller wires (UI sub-tools pass `noopOpenedUrlStore{}` because
// manual launches use EmailId=0 and never touch the OpenedUrls FK).
//
// Slice #116c (Phase 6.3) wire-up: the Tools sub-tab views consume
// this factory through `views.ToolsOptions.ToolsFactory` instead of
// reaching for `config.Load()` directly. That moves the last two
// `config.Load()` call sites in `internal/ui/views/tools_*.go` off
// the AST-guard allowlist and routes Tools construction through the
// `*Services` bundle like every other typed service.
type ToolsFactory = func() (*core.Tools, error)

// Services is the typed dependency bundle threaded through `viewFor`.
// All fields are nil-safe at the view layer (each view has a
// degraded-path branch) so a partial-bootstrap failure surfaces as an
// inline status banner rather than a panic.
//
// `HealthSource` is the production `core.AccountHealthSource` adapter
// returned by `core.NewStoreAccountHealthSource(rt.Store)`. It stays
// nil until `AttachHealthSource` is called from the lazy
// `WatchRuntimeOrNil()` path (mirrors how `AttachRefresher` wires the
// production refresher into `Emails`).
//
// `ActivitySource` is the production `core.ActivitySource` adapter
// returned by `core.NewStoreActivitySource(rt.Store)`. Same lazy-
// attach pattern as `HealthSource` (Slice #105). When nil the
// dashboard hides its recent-activity list rather than panicking.
//
// `Watch` is the long-lived `*core.Watch` singleton owned by
// `WatchRuntimeOrNil()`. Stays nil until `AttachWatch` is called
// from the NavWatch arm — mirrors `HealthSource` / `ActivitySource`
// so every typed service is reachable through the same `*Services`
// pointer (Slice #116b, Phase 6.2). Views that fall back to
// reaching into `WatchRuntimeOrNil()` directly still work; the
// field is the spec-aligned access path.
type Services struct {
	Dashboard      *core.DashboardService
	Emails         *core.EmailsService
	Rules          *core.RulesService
	Accounts       *core.AccountsService // Slice #115 (Phase 6.1) typed shell over the Accounts free funcs
	Watch          *core.Watch           // Slice #116b (Phase 6.2) singleton lazily attached from WatchRuntime
	HealthSource   core.AccountHealthSource
	ActivitySource core.ActivitySource
	// Tools is the per-call factory for `*core.Tools` consumed by the
	// Tools tab sub-routes (OpenUrl / Read / Export / Recent opens).
	// Slice #116c (Phase 6.3) hoist: replaces inline
	// `config.Load() + core.NewTools(...)` in
	// `internal/ui/views/tools_*.go` so view files no longer reach
	// for `config.Load()`. Returns a fresh `*core.Tools` per call to
	// preserve the existing semantics (live config edits picked up
	// without restart). Stays nil when constructor wiring fails;
	// view code falls back to the documented degraded-path message.
	Tools ToolsFactory
}

// BuildServices constructs all four typed services (Phase 2 +
// Slice #115 Accounts shell). Call once at app boot. On any
// individual constructor failure the corresponding field stays nil
// and a log line is emitted — bootstrap continues so the rest of the
// UI still renders.
//
// Construction order matters only for `Dashboard`: it consumes
// `Emails.Count` as its emails-counter dep, so `Emails` must be
// built first. When `Emails` is nil the dashboard falls back to a
// closure that always returns 0 — keeps the UI rendering even if the
// store opener is broken.
func BuildServices() *Services {
	s := &Services{}

	if r := core.NewDefaultEmailsService(); r.HasError() {
		log.Printf("services: NewDefaultEmailsService failed: %v", r.Error())
	} else {
		s.Emails = r.Value()
	}

	if r := core.NewDefaultRulesService(); r.HasError() {
		log.Printf("services: NewDefaultRulesService failed: %v", r.Error())
	} else {
		s.Rules = r.Value()
	}

	// Slice #115: Accounts shell. Stateless construction can't fail
	// today, but the Result envelope leaves the door open for future
	// deps (e.g. an injected config loader for tests) without a
	// signature break.
	if r := core.NewDefaultAccountsService(); r.HasError() {
		log.Printf("services: NewDefaultAccountsService failed: %v", r.Error())
	} else {
		s.Accounts = r.Value()
	}

	// Pick the emails-counter dep for the dashboard. Prefer the
	// typed service; fall back to a zero-counter so the dashboard
	// still renders when Emails construction failed (degraded but
	// non-fatal — surfaces as "Emails stored: 0" until the store
	// recovers).
	counter := s.dashboardEmailsCounter()
	if r := core.NewDashboardService(config.Load, counter); r.HasError() {
		log.Printf("services: NewDashboardService failed: %v", r.Error())
	} else {
		s.Dashboard = r.Value()
	}

	// Slice #116c: default Tools factory. Same `config.Load() +
	// core.NewTools(...)` shape that used to live inline inside
	// `views/tools_openurl.go::buildOpenUrlTools` and
	// `views/tools_read.go::buildReadTools`. Re-evaluated per call so
	// browser-path / dedup-window edits land without restart.
	s.Tools = defaultToolsFactory()

	return s
}

// defaultToolsFactory returns the production `ToolsFactory` used by
// the Tools tab sub-routes when running under the real shell. Splits
// out of `BuildServices` so tests can swap a fake factory by setting
// `services.Tools = func() (*core.Tools, error) { ... }` directly.
//
// The closure mirrors the pre-Slice-#116c per-call constructors:
// load config, build a fresh `*browser.Launcher` from `cfg.Browser`,
// and wrap both in `core.NewTools` with the spec defaults. Each call
// returns an independent `*core.Tools` instance — the in-memory dedup
// map is not shared across sub-tabs, matching the prior behaviour.
func defaultToolsFactory() ToolsFactory {
	return func() (*core.Tools, error) {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		r := core.NewTools(browser.New(cfg.Browser), defaultOpenedUrlRecorder{}, core.DefaultToolsConfig())
		if r.HasError() {
			return nil, r.Error()
		}
		return r.Value(), nil
	}
}

// defaultOpenedUrlRecorder is the no-op `openedUrlRecorder` used by
// every UI-driven Tools sub-route (manual launches set `EmailId=0`,
// so the FK-bound persistent insert is skipped and the in-memory
// dedup index alone protects against rapid double-clicks).
//
// Mirrors the pre-Slice-#116c `noopOpenedUrlStore` that lived inside
// `views/tools_openurl.go`. Hoisted here so the type is built once
// alongside the factory, the views package no longer needs its own
// copy, and the contract (HasOpenedUrl / RecordOpenedUrl /
// RecordOpenedUrlExt all silently succeed) is documented in one place.
type defaultOpenedUrlRecorder struct{}

func (defaultOpenedUrlRecorder) HasOpenedUrl(_ context.Context, _ int64, _ string) (bool, error) {
	return false, nil
}

func (defaultOpenedUrlRecorder) RecordOpenedUrl(_ context.Context, _ int64, _, _ string) (bool, error) {
	return true, nil
}

func (defaultOpenedUrlRecorder) RecordOpenedUrlExt(_ context.Context, _ store.OpenedUrlInsert) (bool, error) {
	return true, nil
}

// dashboardEmailsCounter returns the emails-counter callback the
// dashboard service depends on. Routes through `Emails.Count` when
// the typed service is available; otherwise returns a zero-counter
// closure so dashboard construction never fails on a soft-broken
// emails service.
func (s *Services) dashboardEmailsCounter() func(ctx context.Context, alias string) errtrace.Result[int] {
	if s.Emails != nil {
		return s.Emails.Count
	}
	return func(_ context.Context, _ string) errtrace.Result[int] {
		return errtrace.Ok(0)
	}
}

// AttachRefresher injects the production `core.Refresher` into the
// shared `EmailsService` once the `WatchRuntime` is built. This is
// the bootstrap completion of the P4.4 seam: `Refresh(ctx, alias)`
// becomes a working call rather than the documented "no refresher
// wired" error.
//
// Safe to call with a nil receiver, nil Emails, or nil refresher —
// each is logged as a no-op so a partial bootstrap (e.g. store-open
// failure prevented WatchRuntime from building) cannot crash the
// shell. Idempotent: a second call simply replaces the wired
// refresher, matching the semantics of `WithRefresher`.
func (s *Services) AttachRefresher(refresher core.Refresher) {
	if s == nil || s.Emails == nil {
		return
	}
	if refresher == nil {
		return
	}
	s.Emails.WithRefresher(refresher)
}

// AttachWatch stores the long-lived `*core.Watch` singleton on the
// bundle so views can pick it up via `services.Watch` instead of
// reaching back into `WatchRuntimeOrNil()` each time. **Slice #116b
// (Phase 6.2)** wire-up: completes the spec-aligned shape where every
// typed service is reachable through a single `*Services` pointer.
//
// Lazy-attach from `NavWatch` (and any other view that needs it)
// mirrors `AttachHealthSource` / `AttachActivitySource` — the watch
// runtime is built on first nav, so calling here guarantees the field
// is populated the first time the view is rendered with a healthy
// runtime, without forcing a second `WatchRuntime` build at boot.
//
// Safe to call with a nil receiver, nil watch, or after a previous
// successful attach (idempotent — a second call simply re-stores the
// same singleton pointer).
func (s *Services) AttachWatch(w *core.Watch) {
	if s == nil || w == nil {
		return
	}
	s.Watch = w
}

// AttachActivitySource builds a production `core.ActivitySource` from
// the supplied `*store.Store` and stores it on the bundle so the
// Dashboard view can pass it into `(*core.DashboardService).RecentActivity`.
//
// Slice #105 wire-up: same lazy-attach precedent as
// `AttachHealthSource` (the store is opened lazily by
// `WatchRuntimeOrNil()`); calling here from the NavDashboard arm
// guarantees the source is attached the first time the user lands on
// the dashboard with a healthy runtime, without forcing a second
// `store.Open()` at boot (which would risk SQLite lock contention).
//
// Safe to call with a nil receiver, nil store, or after a previous
// successful attach (idempotent — a second call simply replaces the
// adapter with one bound to the same underlying *store.Store, which
// is harmless because the underlying store is shared).
func (s *Services) AttachActivitySource(st *store.Store) {
	if s == nil || st == nil {
		return
	}
	src := core.NewStoreActivitySource(st)
	if src == nil {
		log.Printf("services: AttachActivitySource: NewStoreActivitySource returned nil")
		return
	}
	s.ActivitySource = src
}

// AttachHealthSource builds a production `core.AccountHealthSource`
// from the supplied `*store.Store` and stores it on the bundle so the
// Dashboard view can pass it into `(*core.DashboardService).AccountHealth`.
//
// Slice #103 wire-up: the store is opened lazily by
// `WatchRuntimeOrNil()` (see `internal/ui/watch_runtime.go`); calling
// here from the NavDashboard arm guarantees the source is attached
// the first time the user lands on the dashboard with a healthy
// runtime, without forcing a second `store.Open()` at boot (which
// would risk SQLite lock contention).
//
// Safe to call with a nil receiver, nil store, or after a previous
// successful attach (idempotent — a second call simply replaces the
// adapter with one bound to the same underlying *store.Store, which
// is harmless because the underlying store is shared).
func (s *Services) AttachHealthSource(st *store.Store) {
	if s == nil || st == nil {
		return
	}
	src := core.NewStoreAccountHealthSource(st)
	if src == nil {
		log.Printf("services: AttachHealthSource: NewStoreAccountHealthSource returned nil")
		return
	}
	s.HealthSource = src
}
