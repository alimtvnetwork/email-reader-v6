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

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/core"
	"github.com/lovable/email-read/internal/errtrace"
	"github.com/lovable/email-read/internal/store"
)

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
type Services struct {
	Dashboard    *core.DashboardService
	Emails       *core.EmailsService
	Rules        *core.RulesService
	HealthSource core.AccountHealthSource
}

// BuildServices constructs all three Phase 2 services. Call once at
// app boot. On any individual constructor failure the corresponding
// field stays nil and a log line is emitted — bootstrap continues so
// the rest of the UI still renders.
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

	return s
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
