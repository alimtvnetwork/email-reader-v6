// dashboard.go — pure-Go dashboard stat aggregation.
//
// **Phase 2.2 refactor.** This file used to expose a single
// package-level `LoadDashboardStats` that reached for `config.Load()`
// (a process-global) and called the `CountEmails` package func
// (also reaching for the default store). That made dashboard logic
// untestable without spinning up a real config file and a real
// SQLite DB.
//
// The new shape mirrors the existing `core.Tools` template
// (`tools.go:76-112`):
//
//   - `DashboardService` struct holds the two injected dependencies
//     (cfg loader, emails counter) plus zero hidden state.
//   - `NewDashboardService` is the explicit constructor; both
//     dependencies are required (nil → ErrCoreInvalidArgument).
//   - `LoadStats` is the typed method that replaces the old
//     `LoadDashboardStats` package func.
//   - The package-level `LoadDashboardStats` stays as a deprecated
//     thin wrapper that builds a default-injected service per call.
//     Wrapper goes away in P2.8 once the UI is fully wired.
//
// Lives in core (no Fyne) so the CLI can render a "status" command
// without pulling in the UI tree.
package core

import (
	"context"

	"github.com/lovable/email-read/internal/config"
	"github.com/lovable/email-read/internal/errtrace"
)

// DashboardStats is the projection rendered by the Dashboard view.
type DashboardStats struct {
	Accounts       int
	RulesTotal     int
	RulesEnabled   int
	EmailsTotal    int
	EmailsForAlias int
	Alias          string // echoed back so the view can render the active scope
}

// configLoader returns the live config snapshot. Functional shape
// (instead of a single-method interface) chosen to match the
// `urlLauncher`/`openedUrlRecorder` style already established in
// `tools.go` — keeps fakes a one-line closure in tests.
type configLoader func() (*config.Config, error)

// emailsCounter returns the number of stored emails for an alias
// scope (empty string = global total). Implementation in production
// is the package-level `CountEmails`; tests inject a fake.
//
// We use a function type (not an interface) for the same reason:
// 1-method dependencies are clearer and easier to fake as closures.
type emailsCounter func(ctx context.Context, alias string) errtrace.Result[int]

// DashboardService aggregates per-load dashboard stats from injected
// config + emails-store dependencies. The struct itself is stateless
// — no caches, no mutexes — so concurrent LoadStats calls are safe.
type DashboardService struct {
	loadCfg     configLoader
	countEmails emailsCounter
}

// NewDashboardService constructs a DashboardService. Both dependencies
// are required: passing nil for either returns ErrCoreInvalidArgument
// (no defensive default-injection — the bootstrap site is the right
// place to make that decision).
func NewDashboardService(loadCfg configLoader, countEmails emailsCounter) errtrace.Result[*DashboardService] {
	if loadCfg == nil {
		return errtrace.Err[*DashboardService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewDashboardService: loadCfg is nil"))
	}
	if countEmails == nil {
		return errtrace.Err[*DashboardService](errtrace.NewCoded(
			errtrace.ErrCoreInvalidArgument, "NewDashboardService: countEmails is nil"))
	}
	return errtrace.Ok(&DashboardService{
		loadCfg:     loadCfg,
		countEmails: countEmails,
	})
}

// LoadStats reads config + emails store and returns aggregate counts.
// Alias may be empty; when set, EmailsForAlias is populated as well.
//
// Error envelope mirrors the pre-refactor LoadDashboardStats: config
// failures wrap as ErrConfigOpen, store failures wrap as ErrDbQueryEmail
// with scope/alias context. Behaviour is byte-equivalent to the old
// package-level func; the only change is dependency source.
func (s *DashboardService) LoadStats(ctx context.Context, alias string) errtrace.Result[DashboardStats] {
	cfg, err := s.loadCfg()
	if err != nil {
		return errtrace.Err[DashboardStats](
			errtrace.WrapCode(err, errtrace.ErrConfigOpen, "core.DashboardService.LoadStats"),
		)
	}
	stats := DashboardStats{
		Accounts:     len(cfg.Accounts),
		RulesTotal:   len(cfg.Rules),
		RulesEnabled: CountEnabledRules(cfg.Rules),
		Alias:        alias,
	}
	totalRes := s.countEmails(ctx, "")
	if totalRes.HasError() {
		return errtrace.Err[DashboardStats](
			errtrace.WrapCode(totalRes.Error(), errtrace.ErrDbQueryEmail, "core.DashboardService.LoadStats").
				WithContext("scope", "total"),
		)
	}
	stats.EmailsTotal = totalRes.Value()
	if alias != "" {
		nRes := s.countEmails(ctx, alias)
		if nRes.HasError() {
			return errtrace.Err[DashboardStats](
				errtrace.WrapCode(nRes.Error(), errtrace.ErrDbQueryEmail, "core.DashboardService.LoadStats").
					WithContext("scope", "alias").
					WithContext("alias", alias),
			)
		}
		stats.EmailsForAlias = nRes.Value()
	}
	return errtrace.Ok(stats)
}
